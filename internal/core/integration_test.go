package core

import (
	"bytes"
	"context"
	"github.com/klauspost/compress/zstd"
	"io"
	"namedicmp/internal/store"
	"namedicmp/internal/wire"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type response struct {
	p Packet
	m wire.Message
}
type mockTransport struct {
	out           chan response
	done          chan struct{}
	once          sync.Once
	dropFirstData bool
	dropped       bool
}

func (m *mockTransport) Read() (Packet, error) { <-m.done; return Packet{}, net.ErrClosed }
func (m *mockTransport) Write(p Packet, msg wire.Message) error {
	if m.dropFirstData && !m.dropped && msg.Kind == 0 {
		f, err := wire.DecodeFrame(msg.Payload)
		if err == nil && f.Kind == 0 && f.DataKind == 0 {
			m.dropped = true
			return nil
		}
	}
	select {
	case m.out <- response{p, msg}:
		return nil
	case <-m.done:
		return net.ErrClosed
	}
}
func TestStreamLossCompressionAndExactAccounting(t *testing.T) {
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
	db, _ := store.Open(filepath.Join(t.TempDir(), "state.json"))
	defer db.Close()
	u, _ := db.PutUser(store.User{Name: "stream", Enabled: true, Quota: 2 << 20})
	tr := &mockTransport{out: make(chan response, 4096), done: make(chan struct{}), dropFirstData: true}
	conf := store.DefaultConfig()
	conf.AllowPrivate = true
	c, err := New(db, conf, tr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop()
	p := Packet{Source: "192.0.2.1", Message: wire.Message{Connection: "stream", Target: ln.Addr().String(), Key: u.Key, TCP: true, ReplyType: 0, Window: 20000, ResendMS: 100, Timeout: 60}}
	send := func(f wire.Frame) {
		packet := p
		packet.Message.Payload = wire.EncodeFrame(f)
		c.Handle(context.Background(), packet)
	}
	send(wire.Frame{DataKind: 1})
	receiver := NewReliable(20000, 100*time.Millisecond)
	connected := false
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for !connected {
		select {
		case <-deadline.C:
			t.Fatal("handshake timeout")
		case answer := <-tr.out:
			f, e := wire.DecodeFrame(answer.m.Payload)
			if e != nil {
				t.Fatal(e)
			}
			controls, data := receiver.Input(f, time.Now())
			for _, v := range controls {
				send(v)
			}
			for _, v := range data {
				if v.DataKind == 2 {
					connected = true
				}
			}
		}
	}
	encoder, _ := zstd.NewWriter(nil, zstd.WithEncoderConcurrency(1))
	defer encoder.Close()
	var expected []byte
	for i := int32(1); i <= 160; i++ {
		payload := bytes.Repeat([]byte{byte(i)}, wire.PayloadSize)
		expected = append(expected, payload...)
		compressed := encoder.EncodeAll(payload, nil)
		send(wire.Frame{ID: i, Data: compressed, Compressed: true})
		if i%7 == 0 {
			send(wire.Frame{ID: i, Data: compressed, Compressed: true})
		}
	}
	var actual []byte
	for len(actual) < len(expected) {
		select {
		case <-deadline.C:
			t.Fatalf("transfer stalled %d/%d", len(actual), len(expected))
		case answer := <-tr.out:
			if answer.m.Kind == 2 {
				t.Fatal("premature KICK")
			}
			f, e := wire.DecodeFrame(answer.m.Payload)
			if e != nil {
				t.Fatal(e)
			}
			controls, data := receiver.Input(f, time.Now())
			for _, v := range controls {
				send(v)
			}
			for _, v := range data {
				if v.DataKind == 0 {
					actual = append(actual, v.Data...)
				}
			}
		}
	}
	if !bytes.Equal(actual, expected) {
		t.Fatal("stream corruption after loss")
	}
	got, _ := db.GetUser(u.ID)
	if got.Upload != int64(len(expected)) || got.Download != int64(len(expected)) {
		t.Fatalf("wrong retransmission/compression usage %+v", got)
	}
	metrics := c.Metrics()
	if metrics["payload_upload"] != uint64(len(expected)) || metrics["payload_download"] != uint64(len(expected)) || metrics["tcp_sessions"] != 1 || metrics["active_users"] != 1 {
		t.Fatalf("dashboard payload/flow counters disagree with exact accounting: %+v", metrics)
	}
}
func (m *mockTransport) Close() error { m.once.Do(func() { close(m.done) }); return nil }
func TestTCPUserIsolationAndDuplicateAccounting(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, e := ln.Accept()
			if e != nil {
				return
			}
			go func() { defer c.Close(); io.Copy(c, c) }()
		}
	}()
	db, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a, _ := db.PutUser(store.User{Name: "A", Enabled: true})
	b, _ := db.PutUser(store.User{Name: "B", Enabled: true})
	transport := &mockTransport{out: make(chan response, 4096), done: make(chan struct{})}
	conf := store.DefaultConfig()
	conf.AllowPrivate = true
	c, err := New(db, conf, transport)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop()
	ctx := context.Background()
	packets := map[int32]Packet{}
	send := func(u store.User, f wire.Frame) {
		t.Helper()
		p := Packet{Source: "192.0.2.1", EchoID: 1, EchoSeq: 2, Message: wire.Message{Connection: "same-id", Target: ln.Addr().String(), Key: u.Key, ReplyType: 0, TCP: true, Window: 32, ResendMS: 100, Timeout: 60, Payload: wire.EncodeFrame(f)}}
		packets[u.Key] = p
		c.Handle(ctx, p)
	}
	send(a, wire.Frame{DataKind: 1})
	send(b, wire.Frame{DataKind: 1})
	waitFor := func(check func(response, wire.Frame) bool) {
		t.Helper()
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				t.Fatal("tunnel response timeout")
			case answer := <-transport.out:
				if answer.m.Kind == 2 {
					continue
				}
				f, err := wire.DecodeFrame(answer.m.Payload)
				if err != nil {
					t.Fatal(err)
				}
				if f.Kind == 0 {
					p := packets[answer.m.Key]
					p.Message.Payload = wire.EncodeFrame(wire.Frame{Kind: 2, IDs: []int32{f.ID}})
					c.Handle(ctx, p)
				}
				if check(answer, f) {
					return
				}
			}
		}
	}
	connected := map[int32]bool{}
	waitFor(func(answer response, f wire.Frame) bool {
		if f.Kind == 0 && f.DataKind == 2 {
			connected[answer.m.Key] = true
		}
		return len(connected) == 2
	})
	send(a, wire.Frame{ID: 1, Data: []byte("alpha")})
	send(a, wire.Frame{ID: 1, Data: []byte("alpha")})
	send(b, wire.Frame{ID: 1, Data: []byte("beta")})
	results := map[int32]string{}
	waitFor(func(answer response, f wire.Frame) bool {
		if f.Kind == 0 && f.DataKind == 0 {
			results[answer.m.Key] += string(f.Data)
		}
		return results[a.Key] == "alpha" && results[b.Key] == "beta"
	})
	av, _ := db.GetUser(a.ID)
	bv, _ := db.GetUser(b.ID)
	if av.Upload != 5 || av.Download != 5 || bv.Upload != 4 || bv.Download != 4 {
		t.Fatalf("duplicate/mixed usage: A=%+v B=%+v", av, bv)
	}
	db.DeleteUser(a.ID)
	c.Revoke(a.ID)
	send(b, wire.Frame{ID: 2, Data: []byte("live")})
	waitFor(func(answer response, f wire.Frame) bool {
		return answer.m.Key == b.Key && f.DataKind == 0 && string(f.Data) == "live"
	})
}
func TestUDPQuotaIsolation(t *testing.T) {
	echo, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	defer echo.Close()
	go func() {
		b := make([]byte, 2000)
		for {
			n, peer, e := echo.ReadFromUDP(b)
			if e != nil {
				return
			}
			echo.WriteToUDP(b[:n], peer)
		}
	}()
	s, _ := store.Open(filepath.Join(t.TempDir(), "state.json"))
	defer s.Close()
	u, _ := s.PutUser(store.User{Name: "quota", Enabled: true, Quota: 6})
	v, _ := s.PutUser(store.User{Name: "other", Enabled: true})
	tr := &mockTransport{out: make(chan response, 100), done: make(chan struct{})}
	conf := store.DefaultConfig()
	conf.AllowPrivate = true
	c, _ := New(s, conf, tr)
	defer c.Stop()
	send := func(user store.User) {
		c.Handle(context.Background(), Packet{Source: "192.0.2.1", Message: wire.Message{Connection: "same", Target: echo.LocalAddr().String(), Key: user.Key, ReplyType: 0, Payload: []byte("abc")}})
	}
	send(u)
	select {
	case answer := <-tr.out:
		if string(answer.m.Payload) != "abc" {
			t.Fatal("UDP reply")
		}
	case <-time.After(time.Second):
		t.Fatal("UDP timeout")
	}
	send(u)
	send(v)
	select {
	case answer := <-tr.out:
		if answer.m.Key != v.Key || string(answer.m.Payload) != "abc" {
			t.Fatal("quota affected unrelated user")
		}
	case <-time.After(time.Second):
		t.Fatal("unrelated user stopped")
	}
	got, _ := s.GetUser(u.ID)
	if got.Used() != 6 {
		t.Fatal("quota accounting")
	}
}

