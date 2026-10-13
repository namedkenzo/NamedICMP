package core

import (
	"container/heap"
	"errors"
	"namedicmp/internal/wire"
	"time"
)

type pending struct {
	frame  wire.Frame
	sent   time.Time
	tries  int
	acked  bool
	due    time.Time
	index  int
	fast   bool
	serial uint64
}
type deadlineHeap []*pending

func (h deadlineHeap) Len() int { return len(h) }
func (h deadlineHeap) Less(i, j int) bool {
	if h[i].due.Equal(h[j].due) {
		return h[i].serial < h[j].serial
	}
	return h[i].due.Before(h[j].due)
}
func (h deadlineHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i]; h[i].index = i; h[j].index = j }
func (h *deadlineHeap) Push(v any)   { p := v.(*pending); p.index = len(*h); *h = append(*h, p) }
func (h *deadlineHeap) Pop() any {
	old := *h
	p := old[len(old)-1]
	old[len(old)-1] = nil
	*h = old[:len(old)-1]
	p.index = -1
	return p
}

// Reliable is an event-driven selective-repeat stream. All methods belong to
// one session goroutine. No timer scans run when there is no pending work.
type Reliable struct {
	window             int
	nextSend, nextRecv int32
	send               map[int32]*pending
	order              []int32
	recv               map[int32]wire.Frame
	rto                time.Duration
	cwnd               int
	lastGap            time.Time
	deadlines          deadlineHeap
	lastLoss           time.Time
	nextSerial         uint64
}

func NewReliable(window int, rto time.Duration) *Reliable {
	if window < 8 {
		window = 8
	}
	if rto < 100*time.Millisecond {
		rto = 100 * time.Millisecond
	}
	if rto > 2*time.Second {
		rto = 2 * time.Second
	}
	return &Reliable{window: window, send: map[int32]*pending{}, recv: map[int32]wire.Frame{}, rto: rto, cwnd: min(16, window)}
}
func distance(a, b int32) int32 { return (a - b + wire.Ring) % wire.Ring }
func (r *Reliable) Space() int  { return max(0, min(r.window, r.cwnd, 1024)-len(r.order)) }
func (r *Reliable) Queue(kind int32, data []byte) error {
	if r.Space() == 0 {
		return errors.New("send window full")
	}
	id := r.nextSend
	r.nextSend = (id + 1) % wire.Ring
	r.order = append(r.order, id)
	p := &pending{frame: wire.Frame{ID: id, DataKind: kind, Data: append([]byte(nil), data...)}, index: -1, serial: r.nextSerial}
	r.nextSerial++
	r.send[id] = p
	heap.Push(&r.deadlines, p)
	return nil
}

// Input acknowledges stored frames, deduplicates, and delivers only contiguous
// frames. Buffer bounds and ID ring checks prevent unbounded peer allocations.
func (r *Reliable) Input(f wire.Frame, now time.Time) (controls, delivered []wire.Frame) {
	switch f.Kind {
	case 0:
		d := distance(f.ID, r.nextRecv)
		if d < int32(r.window) {
			if _, exists := r.recv[f.ID]; !exists {
				if len(r.recv) >= 1024 && f.ID != r.nextRecv {
					return nil, nil
				}
				r.recv[f.ID] = f
			}
			controls = append(controls, wire.Frame{Kind: 2, IDs: []int32{f.ID}})
		} else if distance(r.nextRecv, f.ID) <= int32(r.window) {
			return []wire.Frame{{Kind: 2, IDs: []int32{f.ID}}}, nil
		} else {
			return nil, nil
		}
		for {
			v, ok := r.recv[r.nextRecv]
			if !ok {
				break
			}
			delete(r.recv, r.nextRecv)
			r.nextRecv = (r.nextRecv + 1) % wire.Ring
			delivered = append(delivered, v)
		}
		if len(r.recv) > 0 && now.Sub(r.lastGap) > r.rto/2 {
			furthest := int32(0)
			for id := range r.recv {
				furthest = max(furthest, distance(id, r.nextRecv))
			}
			var missing []int32
			for i := int32(0); i < furthest && len(missing) < 64; i++ {
				id := (r.nextRecv + i) % wire.Ring
				if _, ok := r.recv[id]; !ok {
					missing = append(missing, id)
				}
			}
			if len(missing) > 0 {
				controls = append(controls, wire.Frame{Kind: 1, IDs: missing})
				r.lastGap = now
			}
		}
	case 1:
		for _, id := range f.IDs {
			if p := r.send[id]; p != nil && !p.acked && now.Sub(p.sent) > 20*time.Millisecond {
				p.due = now
				p.fast = true
				heap.Fix(&r.deadlines, p.index)
			}
		}
	case 2:
		for _, id := range f.IDs {
			if p := r.send[id]; p != nil && !p.acked && p.tries > 0 {
				p.acked = true
				if p.index >= 0 {
					heap.Remove(&r.deadlines, p.index)
				}
				if p.tries == 1 {
					sample := now.Sub(p.sent) * 2
					r.rto = max(100*time.Millisecond, min(2*time.Second, (r.rto*7+sample)/8))
				}
				if r.cwnd < r.window {
					r.cwnd++
				}
			}
		}
		for len(r.order) > 0 {
			p := r.send[r.order[0]]
			if !p.acked {
				break
			}
			delete(r.send, r.order[0])
			r.order = r.order[1:]
		}
	case 3:
		controls = append(controls, wire.Frame{Kind: 4, Time: f.Time})
	case 4: // RTT sampling uses unambiguous ACKs; peer wall clocks are not trusted.
	}
	return controls, delivered
}
func (r *Reliable) Due(now time.Time) []wire.Frame {
	var out []wire.Frame
	for len(r.deadlines) > 0 {
		p := r.deadlines[0]
		if p.due.After(now) {
			break
		}
		heap.Pop(&r.deadlines)
		f := p.frame
		f.Time = now.UnixNano()
		f.Resend = p.tries > 0
		if p.tries > 0 && !p.fast && now.Sub(r.lastLoss) >= r.rto {
			r.cwnd = max(8, r.cwnd/2)
			r.lastLoss = now
		}
		p.tries++
		p.sent = now
		delay := r.rto * time.Duration(1<<min(p.tries-1, 4))
		p.due = now.Add(min(delay, 5*time.Second))
		p.fast = false
		heap.Push(&r.deadlines, p)
		out = append(out, f)
	}
	return out
}
func (r *Reliable) Next(now time.Time) time.Duration {
	if len(r.deadlines) == 0 {
		return time.Hour
	}
	return max(0, r.deadlines[0].due.Sub(now))
}
func (r *Reliable) Empty() bool { return len(r.order) == 0 }
