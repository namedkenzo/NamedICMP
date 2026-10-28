package core

import (
	"context"
	"path/filepath"
	"testing"

	"namedicmp/internal/store"
	"namedicmp/internal/wire"
)

// No writer or network goroutines: saturation tests inspect admission and
// queued responses deterministically, rather than racing the socket writer.
func admissionCore(t *testing.T) (*Core, store.User, Packet) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	u, err := db.PutUser(store.User{Name: "capacity", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	c := &Core{
		Store: db, Config: store.DefaultConfig(),
		sessions: make(map[sessionKey]*session), perUser: make(map[string]int),
		tcpPerUser: make(map[string]int), udpPerUser: make(map[string]int),
		controlOut: make(chan responsePacket, 8), dataOut: make(chan responsePacket, 8),
		udpOutQueue: make(chan responsePacket, 8), stopWrite: make(chan struct{}),
	}
	p := Packet{Source: "192.0.2.1", Message: wire.Message{
		Key: u.Key, Connection: "new-flow", Target: "192.0.2.2:443", TCP: true,
		Payload: wire.EncodeFrame(wire.Frame{DataKind: 1}),
	}}
	return c, u, p
}

func TestSaturatedAdmissionIgnoresLateTCP(t *testing.T) {
	for _, global := range []bool{false, true} {
		t.Run(map[bool]string{false: "user", true: "global"}[global], func(t *testing.T) {
			c, u, p := admissionCore(t)
			if global {
				c.Config.MaxSessions = 1
				c.sessions[sessionKey{Connection: "occupied"}] = &session{}
			} else {
				c.tcpPerUser[u.ID] = c.Config.MaxPerUser
			}
			for _, frame := range []wire.Frame{
				{Kind: 2, IDs: []int32{0}}, // late ACK
				{ID: 1, Data: []byte("late payload")},
				{ID: 2, DataKind: 3}, // late CLOSE
				{ID: 3, DataKind: 4}, // late heartbeat
				{ID: 1, DataKind: 1}, // not an initial CONN
			} {
				p.Message.Payload = wire.EncodeFrame(frame)
				c.Handle(context.Background(), p)
			}
			if c.sessionLimitDrops.Load() != 0 || c.dropped.Load() != 0 || len(c.controlOut) != 0 {
				t.Fatal("stale TCP was charged as capacity pressure or received a KICK")
			}
			p.Message.Payload = wire.EncodeFrame(wire.Frame{DataKind: 1})
			c.Handle(context.Background(), p)
			if c.sessionLimitDrops.Load() != 1 || c.dropped.Load() != 1 || len(c.controlOut) != 1 {
				t.Fatal("genuine opening CONN was not rejected at the same limit")
			}
			if answer := <-c.controlOut; answer.message.Kind != 2 {
				t.Fatal("expected a per-flow rejection for the opening request")
			}
		})
	}
}
