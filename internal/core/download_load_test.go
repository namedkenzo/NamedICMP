package core

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"namedicmp/internal/store"
	"namedicmp/internal/wire"
)

// Emulate a real paced socket instead of the instantaneous transport used by
// the echo tests. Returning nil for a dropped packet represents loss after a
// successful send, not a local send failure.
type downloadPacedTransport struct {
	*mockTransport
	dataPackets atomic.Uint64
	lostPackets atomic.Uint64
	retransmits atomic.Uint64
}

func (t *downloadPacedTransport) Write(p Packet, m wire.Message) error {
	timer := time.NewTimer(time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-t.done:
		return net.ErrClosed
	}
	if m.Kind == 0 && m.TCP {
		if f, err := wire.DecodeFrame(m.Payload); err == nil && f.Kind == 0 && f.DataKind == 0 {
			count := t.dataPackets.Add(1)
			if f.Resend {
				t.retransmits.Add(1)
			}
			if count%29 == 0 {
				t.lostPackets.Add(1)
				return nil
			}
		}
	}
	return t.mockTransport.Write(p, m)
}

// A speed-test download is not an echo: the remote endpoint can deliver a
// large response and EOF while the client sends no application payload. This
// exercises backlogged sends, retransmission, EOF drain and probe liveness.
func TestDownloadOnlyParallelFlowsSurvivePacingAndLoss(t *testing.T) {
	for _, test := range []struct {
		name string
		rate int
	}{{"default-pacer", 2500}, {"constrained-pacer", 800}} {
		t.Run(test.name, func(t *testing.T) {
			const flowCount = 8
			const payloadSize = 96 * 1024
			started := time.Now()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			db, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			user, err := db.PutUser(store.User{Name: "download-only-load", Enabled: true})
			if err != nil {
				t.Fatal(err)
			}
			transport := &downloadPacedTransport{mockTransport: &mockTransport{out: make(chan response, 4096), done: make(chan struct{})}}
			conf := store.DefaultConfig()
			conf.AllowPrivate = true
			conf.ICMPPacketsPerSecond = test.rate
			server, err := New(db, conf, transport)
			if err != nil {
				t.Fatal(err)
			}
			var targetWorkers sync.WaitGroup
			var listeners []net.Listener
			defer func() {
				cancel()
				for _, listener := range listeners {
					_ = listener.Close()
				}
				stopped := make(chan struct{})
				go func() {
					server.Stop()
					targetWorkers.Wait()
					close(stopped)
				}()
				select {
				case <-stopped:
				case <-time.After(3 * time.Second):
					t.Error("download workers/writer failed to stop after cancellation")
				}
			}()

			type downloadFlow struct {
				id, target string
				reliable   *Reliable
				expected   []byte
				received   []byte
				connected  bool
				remoteEOF  bool
				closeSent  bool
				kicked     bool
			}
			flows := make([]*downloadFlow, 0, flowCount)
			byID := make(map[string]*downloadFlow)
			for i := 0; i < flowCount; i++ {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				listeners = append(listeners, listener)
				payload := make([]byte, payloadSize)
				for j := range payload {
					payload[j] = byte((j*31 + i*17) % 251)
				}
				flow := &downloadFlow{id: fmt.Sprintf("download-%d", i), target: listener.Addr().String(), reliable: NewReliable(20000, 100*time.Millisecond), expected: payload}
				if err := flow.reliable.Queue(1, nil); err != nil {
					t.Fatal(err)
				}
				flows = append(flows, flow)
				byID[flow.id] = flow
				targetWorkers.Add(1)
				go func(listener net.Listener, payload []byte) {
					defer targetWorkers.Done()
					conn, err := listener.Accept()
					if err != nil {
						return
					}
					defer conn.Close()
					stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
					defer stop()
					_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
					_ = writeTCP(conn, payload)
					// Closing here produces EOF immediately after the finite body;
					// the core must still deliver and acknowledge all queued bytes.
				}(listener, payload)
			}
			send := func(flow *downloadFlow, frame wire.Frame) {
				server.Handle(ctx, Packet{Source: "192.0.2.1", Message: wire.Message{Connection: flow.id, Target: flow.target, Key: user.Key, TCP: true, ReplyType: 0, Window: 20000, ResendMS: 100, Timeout: 60, Payload: wire.EncodeFrame(frame)}})
			}
			type delayedControl struct {
				at    time.Time
				flow  *downloadFlow
				frame wire.Frame
			}
			var delayed []delayedControl
			probes := make(map[string]time.Time)
			probeReplies := 0
			lastProbe := time.Time{}
			tick := time.NewTicker(5 * time.Millisecond)
			defer tick.Stop()
			deadline := time.NewTimer(12 * time.Second)
			defer deadline.Stop()
			for {
				now := time.Now()
				for len(delayed) > 0 && !delayed[0].at.After(now) {
					control := delayed[0]
					delayed = delayed[1:]
					send(control.flow, control.frame)
				}
				completed := 0
				for _, flow := range flows {
					if flow.kicked {
						completed++
						continue
					}
					if flow.remoteEOF && !flow.closeSent && flow.reliable.Space() > 0 {
						if err := flow.reliable.Queue(3, nil); err != nil {
							t.Fatal(err)
						}
						flow.closeSent = true
					}
					for _, frame := range flow.reliable.Due(now) {
						send(flow, frame)
					}
				}
				if completed == flowCount {
					break
				}
				for _, at := range probes {
					if now.Sub(at) > 2*time.Second {
						t.Fatal("authenticated probe starved behind download traffic")
					}
				}
				if now.Sub(lastProbe) >= 250*time.Millisecond {
					token := fmt.Sprintf("download-probe-%d", now.UnixNano())
					probes[token] = now
					lastProbe = now
					server.Handle(ctx, Packet{Source: "192.0.2.1", Message: wire.Message{Kind: 1, Key: user.Key, ReplyType: 0, Payload: []byte(token)}})
				}
				select {
				case <-deadline.C:
					t.Fatalf("download stalled/disconnected: sent=%d retries=%d loss=%d", transport.dataPackets.Load(), transport.retransmits.Load(), transport.lostPackets.Load())
				case <-tick.C:
				case answer := <-transport.out:
					if answer.m.Kind == 1 {
						if sent, ok := probes[string(answer.m.Payload)]; ok {
							if time.Since(sent) > 2*time.Second {
								t.Fatal("probe response exceeded download-load deadline")
							}
							delete(probes, string(answer.m.Payload))
							probeReplies++
						}
						continue
					}
					flow := byID[answer.m.Connection]
					if flow == nil {
						t.Fatal("cross-flow download packet")
					}
					if answer.m.Kind == 2 {
						if !flow.remoteEOF || !bytes.Equal(flow.received, flow.expected) {
							t.Fatalf("premature KICK during download: %s received=%d/%d", flow.id, len(flow.received), len(flow.expected))
						}
						flow.kicked = true
						continue
					}
					frame, err := wire.DecodeFrame(answer.m.Payload)
					if err != nil {
						t.Fatal(err)
					}
					controls, delivery := flow.reliable.Input(frame, time.Now())
					for _, control := range controls {
						delayed = append(delayed, delayedControl{at: time.Now().Add(30 * time.Millisecond), flow: flow, frame: control})
					}
					for _, data := range delivery {
						switch data.DataKind {
						case 2:
							flow.connected = true
						case 0:
							if !flow.connected || len(flow.received)+len(data.Data) > len(flow.expected) {
								t.Fatal("unexpected or duplicated download bytes")
							}
							flow.received = append(flow.received, data.Data...)
						case 3:
							flow.remoteEOF = true
							if !bytes.Equal(flow.received, flow.expected) {
								t.Fatalf("EOF truncated download for %s", flow.id)
							}
						case 4:
						default:
							t.Fatal("unexpected download stream control")
						}
					}
				}
			}
			if probeReplies == 0 || transport.lostPackets.Load() == 0 || transport.retransmits.Load() == 0 {
				t.Fatal("test did not exercise probes, loss and retransmission")
			}
			actual, _ := db.GetUser(user.ID)
			t.Logf("download-only: duration=%s packets=%d retransmissions=%d simulated_loss=%d", time.Since(started), transport.dataPackets.Load(), transport.retransmits.Load(), transport.lostPackets.Load())
			if actual.Upload != 0 || actual.Download != flowCount*payloadSize {
				t.Fatalf("download-only accounting is incorrect: upload=%d download=%d", actual.Upload, actual.Download)
			}
		})
	}
}
