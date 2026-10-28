package core

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"namedicmp/internal/wire"
)

func TestAdmissionRejectionReasonsAreExclusive(t *testing.T) {
	for _, reason := range []string{"global", "tcp_user", "udp_user", "stopping"} {
		t.Run(reason, func(t *testing.T) {
			c, u, p := admissionCore(t)
			switch reason {
			case "global":
				c.Config.MaxSessions = 1
				c.sessions[sessionKey{Connection: "occupied"}] = &session{}
				c.tcpPerUser[u.ID] = c.Config.MaxPerUser // global wins overlap
			case "tcp_user":
				c.tcpPerUser[u.ID] = c.Config.MaxPerUser
			case "udp_user":
				p.Message.TCP = false
				p.Message.Payload = []byte("datagram")
				c.udpPerUser[u.ID] = c.Config.MaxUDPPerUser
			case "stopping":
				c.stopping = true
				c.tcpPerUser[u.ID] = c.Config.MaxPerUser
			}
			c.Handle(context.Background(), p)
			metrics := c.Metrics()
			for _, key := range []string{"global", "tcp_user", "udp_user", "stopping"} {
				want := uint64(0)
				if key == reason {
					want = 1
				}
				if metrics["session_limit_"+key] != want {
					t.Fatalf("reason %s: %+v", reason, metrics)
				}
			}
			if c.sessionLimitDrops.Load() != 1 || c.dropped.Load() != 1 {
				t.Fatal("aggregate counters must count each rejection exactly once")
			}
		})
	}
}

func TestInvalidTargetIsNotCapacityPressure(t *testing.T) {
	c, u, p := admissionCore(t)
	c.tcpPerUser[u.ID] = c.Config.MaxPerUser
	p.Message.Target = "invalid target"
	c.Handle(context.Background(), p)
	if c.invalid.Load() != 1 || c.sessionLimitDrops.Load() != 0 || len(c.controlOut) != 0 {
		t.Fatal("invalid target must not be counted as flow admission pressure")
	}
}

func TestExistingFlowStillReceivesAtCapacity(t *testing.T) {
	c, u, p := admissionCore(t)
	c.Config.MaxSessions = 1
	c.tcpPerUser[u.ID] = c.Config.MaxPerUser
	s := &session{in: make(chan Packet, 1)}
	c.sessions[identity(p, u.ID)] = s
	p.Message.Payload = wire.EncodeFrame(wire.Frame{ID: 1, Data: []byte("existing")})
	c.Handle(context.Background(), p)
	if len(s.in) != 1 || c.sessionLimitDrops.Load() != 0 || len(c.controlOut) != 0 {
		t.Fatal("capacity admission disrupted an existing flow")
	}
}

func TestQueueReasonsSeparateRetriesAndDatagramLoss(t *testing.T) {
	c := &Core{
		controlOut: make(chan responsePacket, 1), dataOut: make(chan responsePacket, 1),
		udpOutQueue: make(chan responsePacket, 1), stopWrite: make(chan struct{}),
	}
	tcp := Packet{Message: wire.Message{TCP: true}}
	data := wire.Message{Payload: wire.EncodeFrame(wire.Frame{Data: []byte("TCP")})}
	c.reply(tcp, data)
	completed := 0
	c.replyTracked(tcp, data, func(_ time.Time, ok bool) {
		if ok {
			t.Fatal("full TCP queue reported successful dispatch")
		}
		completed++
	})
	c.reply(Packet{}, wire.Message{Kind: 1})
	c.reply(Packet{}, wire.Message{Kind: 1})
	c.replyTracked(Packet{}, wire.Message{Payload: []byte("old UDP")}, func(_ time.Time, ok bool) {
		if ok {
			t.Fatal("evicted datagram reported successful dispatch")
		}
		completed++
	})
	c.reply(Packet{}, wire.Message{Payload: []byte("new UDP")})
	if c.tcpQueueRetries.Load() != 1 || c.controlQueueDropped.Load() != 1 || c.udpQueueEvicted.Load() != 1 || c.udpQueueRejected.Load() != 0 || c.writeQueueDrops.Load() != 3 || c.dropped.Load() != 3 || completed != 2 {
		t.Fatal("queue categories or aggregate accounting incorrect")
	}
	if latest := <-c.udpOutQueue; string(latest.message.Payload) != "new UDP" {
		t.Fatal("UDP newest-wins policy changed")
	}
}

func TestMetricsDoNotExposeUserIDs(t *testing.T) {
	c, u, _ := admissionCore(t)
	c.tcpPerUser[u.ID], c.tcpPerUser["other-user"] = 25, 3
	c.udpPerUser[u.ID], c.udpPerUser["other-user"] = 127, 5
	m := c.Metrics()
	if m["busiest_user_tcp_sessions"] != 25 || m["busiest_user_udp_sessions"] != 127 {
		t.Fatal("per-user pressure metric must be a maximum, not the total")
	}
	if _, exists := m[u.ID]; exists {
		t.Fatal("metrics exposed a user identity")
	}
}

func TestConcurrentUDPQueueReasonsRemainBounded(t *testing.T) {
	c := &Core{
		controlOut: make(chan responsePacket, 1), dataOut: make(chan responsePacket, 1),
		udpOutQueue: make(chan responsePacket, 64), stopWrite: make(chan struct{}),
	}
	var workers sync.WaitGroup
	for i := 0; i < 16; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 100; j++ {
				c.reply(Packet{}, wire.Message{Payload: []byte("datagram")})
			}
		}()
	}
	workers.Wait()
	lost := c.udpQueueEvicted.Load() + c.udpQueueRejected.Load()
	if lost+uint64(len(c.udpOutQueue)) != 1600 || c.writeQueueDrops.Load() != lost || c.dropped.Load() != lost {
		t.Fatal("concurrent UDP overflow lost accounting or exceeded queue bounds")
	}
}

func TestUDPCapacityReturnsAfterCancellation(t *testing.T) {
	c, tr, u := regressionCore(t)
	echo := udpEcho(t)
	target := echo.LocalAddr().String()
	// Use the real default per-user UDP cap and a live local UDP peer. This is
	// a flow-lifecycle check, not a public Speedtest reproduction.
	limit := c.Config.MaxUDPPerUser
	packet := func(i int) Packet {
		return Packet{Source: "192.0.2.1", Message: wire.Message{
			Key: u.Key, Connection: fmt.Sprintf("udp-capacity-%d", i),
			Target: target, Payload: []byte("UDP"),
		}}
	for i := 0; i < limit; i++ {
		c.Handle(context.Background(), packet(i))
		awaitUDP(t, tr, target, []byte("UDP"))
	}
	c.Handle(context.Background(), packet(limit))
	if c.sessionUDPUserDrops.Load() != 1 || c.sessionGlobalDrops.Load() != 0 {
		t.Fatal("default UDP cap was not separate from the global cap")
	}
	first := packet(0)
	c.mu.Lock()
	c.sessions[identity(first, u.ID)].cancel()
	c.mu.Unlock()
	deadline := time.Now().Add(2 * time.Second)
	for c.Metrics()["udp_sessions"].(int) != limit-1 {
		if time.Now().After(deadline) {
			t.Fatal("cancelled UDP flow retained its capacity")
		}
		time.Sleep(time.Millisecond)
	}
	c.Handle(context.Background(), packet(limit))
	awaitUDP(t, tr, target, []byte("UDP"))
	if c.sessionUDPUserDrops.Load() != 1 || c.Metrics()["udp_sessions"].(int) != limit {
		t.Fatal("freed UDP capacity could not be reused")
	}
}
