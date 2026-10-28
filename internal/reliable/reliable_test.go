package reliable

import (
	"namedicmp/internal/wire"
	"testing"
	"time"
)

func TestQueuedFramesWaitForPhysicalSend(t *testing.T) {
	r := NewReliable(32, 100*time.Millisecond)
	now := time.Now()
	_ = r.Queue(0, []byte("download"))
	d := r.QueuedDue(now, nil)
	if len(d) != 1 {
		t.Fatal("no dispatch")
	}
	// Twenty RTOs in a local queue must not create twenty copies, shrink cwnd,
	// or make NACK processing heap.Fix an item with index -1.
	r.Input(wire.Frame{Kind: 1, IDs: []int32{0}}, now.Add(time.Second))
	if len(r.QueuedDue(now.Add(2*time.Second), nil)) != 0 || r.send[0].tries != 0 || r.cwnd != 16 {
		t.Fatal("queue delay became network loss")
	}
	sent := now.Add(2 * time.Second)
	d[0].Complete(sent, true)
	r.PollSent(sent)
	if len(r.QueuedDue(sent.Add(50*time.Millisecond), nil)) != 0 {
		t.Fatal("RTO started before socket send")
	}
	if len(r.QueuedDue(sent.Add(101*time.Millisecond), nil)) != 1 {
		t.Fatal("real network loss did not retry")
	}
}
func TestQueueRejectionAndEarlyACK(t *testing.T) {
	r := NewReliable(32, 100*time.Millisecond)
	now := time.Now()
	_ = r.Queue(0, []byte("abc"))
	d := r.QueuedDue(now, nil)[0]
	r.Input(wire.Frame{Kind: 2, IDs: []int32{0}}, now)
	if r.Empty() {
		t.Fatal("ACK of untransmitted frame trusted")
	}
	d.Complete(now, false)
	r.PollSent(now)
	if r.send[0].tries != 0 || r.cwnd != 16 || r.Empty() {
		t.Fatal("local admission failure counted as network loss")
	}
	d = r.QueuedDue(now.Add(101*time.Millisecond), nil)[0]
	r.Input(wire.Frame{Kind: 2, IDs: []int32{0}}, now.Add(102*time.Millisecond))
	d.Complete(now.Add(101*time.Millisecond), true)
	r.PollSent(now.Add(103 * time.Millisecond))
	if !r.Empty() || r.AcknowledgedBytes() != 3 {
		t.Fatal("fast ACK lost before writer completion")
	}
}
func TestDispatchCompletionConcurrentWithOwner(t *testing.T) {
	r := NewReliable(32, 100*time.Millisecond)
	now := time.Now()
	_ = r.Queue(0, []byte("x"))
	d := r.QueuedDue(now, nil)[0]
	done := make(chan struct{})
	go func() { d.Complete(now, true); d.Complete(now, false); close(done) }()
	for i := 0; i < 100; i++ {
		r.PollSent(now)
	}
	<-done
	r.PollSent(now)
	r.Input(wire.Frame{Kind: 2, IDs: []int32{0}}, now.Add(time.Millisecond))
	if !r.Empty() {
		t.Fatal("completion lost")
	}
}

func TestOriginalACKWhileRetryWaitsInWriter(t *testing.T) {
	r := NewReliable(32, 100*time.Millisecond)
	now := time.Now()
	_ = r.Queue(0, []byte("abc"))
	first := r.QueuedDue(now, nil)[0]
	first.Complete(now, true)
	r.PollSent(now)
	retry := r.QueuedDue(now.Add(101*time.Millisecond), nil)[0]
	r.Input(wire.Frame{Kind: 2, IDs: []int32{0}}, now.Add(102*time.Millisecond))
	if !retry.Obsolete() {
		t.Fatal("acknowledged retry was not withdrawn from the writer")
	}
	retry.Complete(now.Add(103*time.Millisecond), true)
	r.PollSent(now.Add(104 * time.Millisecond))
	if !r.Empty() || r.AcknowledgedBytes() != 3 || len(r.QueuedDue(now.Add(time.Second), nil)) != 0 {
		t.Fatal("late retry completion resurrected acknowledged data")
	}
}

func TestOrderingDedupAndWrap(t *testing.T) {
	r := NewReliable(32, 100*time.Millisecond)
	now := time.Now()
	ctrl, out := r.Input(wire.Frame{ID: 1, Data: []byte("B")}, now)
	if len(out) != 0 || len(ctrl) < 2 || ctrl[1].Kind != 1 {
		t.Fatal("missing frame not requested")
	}
	_, out = r.Input(wire.Frame{ID: 0, Data: []byte("A")}, now)
	if len(out) != 2 || string(out[0].Data)+string(out[1].Data) != "AB" {
		t.Fatal("out of order delivery")
	}
	_, out = r.Input(wire.Frame{ID: 0, Data: []byte("A")}, now)
	if len(out) != 0 {
		t.Fatal("duplicate delivery")
	}
	r.nextRecv = wire.Ring - 1
	_, out = r.Input(wire.Frame{ID: 0, Data: []byte("D")}, now)
	if len(out) != 0 {
		t.Fatal("wrap prematurely delivered")
	}
	_, out = r.Input(wire.Frame{ID: wire.Ring - 1, Data: []byte("C")}, now)
	if len(out) != 2 || r.nextRecv != 1 {
		t.Fatal("ring wrap")
	}
}
func TestLostACKRetransmission(t *testing.T) {
	r := NewReliable(16, 100*time.Millisecond)
	now := time.Now()
	r.Queue(0, []byte("data"))
	first := r.Due(now)
	if len(first) != 1 || first[0].Resend {
		t.Fatal("initial send")
	}
	if len(r.Due(now.Add(50*time.Millisecond))) != 0 {
		t.Fatal("premature resend")
	}
	retry := r.Due(now.Add(101 * time.Millisecond))
	if len(retry) != 1 || !retry[0].Resend {
		t.Fatal("lost ACK not retried")
	}
	r.Input(wire.Frame{Kind: 2, IDs: []int32{0}}, now.Add(110*time.Millisecond))
	if !r.Empty() || len(r.Due(now.Add(time.Second))) != 0 {
		t.Fatal("acked frame retained")
	}
}
func TestBoundedWindowAndSelectiveACK(t *testing.T) {
	r := NewReliable(8, 100*time.Millisecond)
	for i := 0; i < 8; i++ {
		if err := r.Queue(0, nil); err != nil {
			t.Fatal(err)
		}
	}
	if r.Queue(0, nil) == nil {
		t.Fatal("unbounded send window")
	}
	now := time.Now()
	r.Due(now)
	r.Input(wire.Frame{Kind: 2, IDs: []int32{1, 2, 3, 4, 5, 6, 7}}, now)
	if r.Space() != 0 {
		t.Fatal("window slid across unacked gap")
	}
	r.Input(wire.Frame{Kind: 2, IDs: []int32{0}}, now)
	if r.Space() != 8 {
		t.Fatal("window not released")
	}
	r.Input(wire.Frame{ID: 50000}, now)
	if len(r.recv) != 0 {
		t.Fatal("unbounded receive window")
	}
}
func BenchmarkReliable(b *testing.B) {
	r := NewReliable(256, 400*time.Millisecond)
	now := time.Now()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = r.Input(wire.Frame{ID: int32(i % wire.Ring), Data: []byte("payload")}, now)
	}
}
func TestFullReceiveBufferCanRecoverMissingCursor(t *testing.T) {
	r := NewReliable(20000, 100*time.Millisecond)
	now := time.Now()
	for i := int32(1); i <= 1024; i++ {
		r.Input(wire.Frame{ID: i, Data: []byte{1}}, now)
	}
	if len(r.recv) != 1024 {
		t.Fatal("receive cap")
	}
	r.Input(wire.Frame{ID: 1025, Data: []byte{1}}, now)
	if len(r.recv) != 1024 {
		t.Fatal("receive map grew past cap")
	}
	_, out := r.Input(wire.Frame{ID: 0, Data: []byte{1}}, now)
	if len(out) != 1025 || len(r.recv) != 0 {
		t.Fatal("buffer cannot recover missing cursor")
	}
}

func TestRetransmitBurstCutsWindowOnceAndKeepsSendOrder(t *testing.T) {
	r := NewReliable(128, 100*time.Millisecond)
	r.cwnd = 64
	for i := 0; i < 64; i++ {
		if err := r.Queue(0, []byte{byte(i)}); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	first := r.Due(now)
	for i, f := range first {
		if f.ID != int32(i) {
			t.Fatal("initial packet batch reordered")
		}
	}
	// An entire loss burst is one congestion event, not 64 independent cuts.
	if len(r.Due(now.Add(101*time.Millisecond))) != 64 || r.cwnd != 32 {
		t.Fatalf("loss burst window=%d", r.cwnd)
	}
	r.Input(wire.Frame{Kind: 2, IDs: []int32{0, 1}}, now.Add(120*time.Millisecond))
	r.Input(wire.Frame{Kind: 1, IDs: []int32{2}}, now.Add(130*time.Millisecond))
	before := r.cwnd
	fast := r.Due(now.Add(130 * time.Millisecond))
	if len(fast) != 1 || fast[0].ID != 2 || r.cwnd != before {
		t.Fatal("selective fast retry changed unrelated deadlines or window")
	}
	for _, f := range r.Due(now.Add(time.Second)) {
		if f.ID < 2 {
			t.Fatal("acknowledged frame retransmitted")
		}
	}
}

func TestUnsentFrameCannotBeAcknowledged(t *testing.T) {
	r := NewReliable(8, 100*time.Millisecond)
	r.Queue(0, []byte("keep"))
	now := time.Now()
	r.Input(wire.Frame{Kind: 2, IDs: []int32{0}}, now)
	if r.Empty() || len(r.Due(now)) != 1 {
		t.Fatal("early ACK discarded queued data")
	}
}
