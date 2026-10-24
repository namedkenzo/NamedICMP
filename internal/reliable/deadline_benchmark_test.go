package reliable

import (
	"testing"
	"time"
)

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
