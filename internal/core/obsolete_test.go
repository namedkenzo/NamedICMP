package core

import (
	"path/filepath"
	"testing"
	"time"

	"namedicmp/internal/wire"
	"namedicmp/internal/store"
)

func TestAcknowledgedRetryDoesNotReachSocket(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil { t.Fatal(err) }
	defer db.Close()
	tr := &blockedWriterTransport{entered:make(chan struct{}), release:make(chan struct{}), done:make(chan struct{}), out:make(chan response,4)}
	c, err := New(db, store.DefaultConfig(), tr)
	if err != nil { t.Fatal(err) }
	defer c.Stop()
	c.reply(Packet{}, wire.Message{Kind:1})
	select { case <-tr.entered: case <-time.After(time.Second): t.Fatal("writer not blocked") }
	r := NewReliable(32, 100*time.Millisecond)
	now := time.Now()
	r.Queue(0, []byte("already received"))
	first := r.QueuedDue(now, nil)[0]
	first.Complete(now, true)
	r.PollSent(now)
	retry := r.QueuedDue(now.Add(101*time.Millisecond), nil)[0]
	peer := Packet{Message: wire.Message{TCP: true}}
	c.replyQueued(peer, wire.Message{Payload: wire.EncodeFrame(retry.Frame)}, retry.Complete, retry)
	r.Input(wire.Frame{Kind: 2, IDs: []int32{0}}, now.Add(102*time.Millisecond))
	// A subsequent live DATA ticket provides a deterministic writer barrier.
	done := make(chan struct{})
	c.replyTracked(peer, wire.Message{Payload: wire.EncodeFrame(wire.Frame{Data: []byte("live")})}, func(_ time.Time, _ bool) { close(done) })
	close(tr.release)
	select { case <-done: case <-time.After(time.Second): t.Fatal("writer stalled") }
	if c.obsoleteQueued.Load() != 1 || c.sent.Load() != 2 || len(tr.out) != 2 { t.Fatal("acknowledged retry consumed a socket write") }
	if !r.Empty() || r.AcknowledgedBytes() != uint64(len("already received")) { t.Fatal("payload accounting changed") }
}

func TestClosedStreamCancelsQueuedDispatch(t *testing.T) {
	r := NewReliable(32, 100*time.Millisecond)
	r.Queue(0, []byte("closed"))
	d := r.QueuedDue(time.Now(), nil)[0]
	r.CancelQueued()
	if !d.Obsolete() { t.Fatal("closed stream left live queued traffic") }
}
