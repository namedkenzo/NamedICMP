package core

import (
	"bytes"
	"context"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"

	"namedicmp/internal/store"
	"namedicmp/internal/wire"
)

func udpEcho(t *testing.T) *net.UDPConn {
	t.Helper()
	c, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	go func() {
		b := make([]byte, 65535)
		for {
			n, peer, err := c.ReadFromUDP(b)
			if err != nil {
				return
			}
			c.WriteToUDP(b[:n], peer)
		}
	}()
	return c
}
func regressionCore(t *testing.T) (*Core, *mockTransport, store.User) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	u, err := s.PutUser(store.User{Name: "dual-protocol", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	tr := &mockTransport{out: make(chan response, 4096), done: make(chan struct{})}
	cfg := store.DefaultConfig()
	cfg.AllowPrivate = true
	c, err := New(s, cfg, tr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Stop)
	return c, tr, u
}
func awaitUDP(t *testing.T, tr *mockTransport, target string, expected []byte) {
	t.Helper()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case r := <-tr.out:
			if r.m.Kind == 0 && r.m.Target == target && bytes.Equal(r.m.Payload, expected) {
				return
			}
		case <-timer.C:
			t.Fatal("UDP reply missing for target " + target)
		}
	}
}

func TestUDPSharedIDMultipleDestinations(t *testing.T) {
	c, tr, u := regressionCore(t)
	a, b := udpEcho(t), udpEcho(t)
	for _, target := range []string{a.LocalAddr().String(), b.LocalAddr().String()} {
		p := Packet{Source: "192.0.2.1", Message: wire.Message{Connection: "shared", Target: target, Key: u.Key, ReplyType: 0, Payload: []byte(target)}}
		c.Handle(context.Background(), p)
		awaitUDP(t, tr, target, []byte(target))
	}
}
func TestUDPPreservesLargeDatagrams(t *testing.T) {
	c, tr, u := regressionCore(t)
	echo := udpEcho(t)
	target := echo.LocalAddr().String()
	payload := bytes.Repeat([]byte("DNS/QUIC"), 512)
	c.Handle(context.Background(), Packet{Source: "192.0.2.1", Message: wire.Message{Connection: "large", Target: target, Key: u.Key, ReplyType: 0, Payload: payload}})
	awaitUDP(t, tr, target, payload)
	got, _ := c.Store.GetUser(u.ID)
	if got.Upload != 4096 || got.Download != 4096 {
		t.Fatal("datagram accounting not exact")
	}
}
func TestTCPAndUDPSharedIDRemainIndependent(t *testing.T) {
	c, tr, u := regressionCore(t)
	udp := udpEcho(t)
	ut := udp.LocalAddr().String()
	base := Packet{Source: "192.0.2.1", Message: wire.Message{Connection: "shared", Target: ut, Key: u.Key, ReplyType: 0, Payload: []byte("UDP")}}
	c.Handle(context.Background(), base)
	awaitUDP(t, tr, ut, []byte("UDP"))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, e := ln.Accept()
		if e != nil {
			return
		}
		defer conn.Close()
		io.Copy(conn, conn)
	}()
	tcp := base
	tcp.Message.Target = ln.Addr().String()
	tcp.Message.TCP = true
	tcp.Message.Window = 32
	tcp.Message.Payload = wire.EncodeFrame(wire.Frame{DataKind: 1})
	c.Handle(context.Background(), tcp)
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		select {
		case answer := <-tr.out:
			if answer.m.Target != "" {
				continue
			}
			f, e := wire.DecodeFrame(answer.m.Payload)
			if e == nil && f.Kind == 0 && f.DataKind == 2 {
				tcp.Message.Payload = wire.EncodeFrame(wire.Frame{Kind: 2, IDs: []int32{f.ID}})
				c.Handle(context.Background(), tcp)
				base.Message.Payload = []byte("still UDP")
				c.Handle(context.Background(), base)
				awaitUDP(t, tr, ut, base.Message.Payload)
				return
			}
		case <-timer.C:
			t.Fatal("TCP handshake blocked by UDP using same connection ID")
		}
	}
}

func BenchmarkSessionIdentity(b *testing.B) {
	p := Packet{Source: "192.0.2.1", Message: wire.Message{Connection: "client-session-identifier", Target: "8.8.8.8:53"}}
	known := map[sessionKey]bool{identity(p, "user-identifier"): true}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if !known[identity(p, "user-identifier")] {
			b.Fatal("identity lookup failed")
		}
	}
}

var benchmarkDelay time.Duration

func BenchmarkPendingDeadlines(b *testing.B) {
	r := NewReliable(20000, 400*time.Millisecond)
	r.cwnd = 1024
	now := time.Now()
	for i := 0; i < 1024; i++ {
		_ = r.Queue(0, nil)
	}
	r.Due(now)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.Due(now.Add(time.Millisecond))
		benchmarkDelay = r.Next(now)
	}
}
