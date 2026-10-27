package core

import (
	"path/filepath"
	"testing"
	"time"

	"namedicmp/internal/store"
	"namedicmp/internal/wire"
)

func TestDispatchTimerStartsAfterSlowSuccessfulSocketWrite(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tr := &blockedWriterTransport{entered: make(chan struct{}), release: make(chan struct{}), done: make(chan struct{}), out: make(chan response, 1)}
	c, err := New(db, store.DefaultConfig(), tr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop()
	completed := make(chan time.Time, 1)
	c.replyTracked(Packet{}, wire.Message{Kind: 1}, func(at time.Time, ok bool) {
		if ok {
			completed <- at
		}
	})
	select {
	case <-tr.entered:
	case <-time.After(time.Second):
		t.Fatal("writer did not start")
	}
	time.Sleep(150 * time.Millisecond) // longer than the initial reliability RTO
	released := time.Now()
	close(tr.release)
	select {
	case at := <-completed:
		if at.Before(released) {
			t.Fatal("socket wait consumed network RTO")
		}
	case <-time.After(time.Second):
		t.Fatal("send completion lost")
	}
}
