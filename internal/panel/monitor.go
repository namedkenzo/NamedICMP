package panel

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

const historyLimit = 300
const liveClientLimit = 16

type trafficPoint struct {
	At int64 `json:"at"`
	RX float64 `json:"rx"`
	TX float64 `json:"tx"`
	Upload float64 `json:"upload"`
	Download float64 `json:"download"`
	PacketsIn float64 `json:"packets_in"`
	PacketsOut float64 `json:"packets_out"`
}

// One sampler per panel, irrespective of how many dashboards are open.
// Published snapshots are immutable; subscribers have a one-frame mailbox.
type liveMonitor struct {
	mu sync.Mutex
	data map[string]any
	history []trafficPoint
	clients map[chan []byte]struct{}
	done chan struct{}
	previous map[string]uint64
	at time.Time
	probes [2]*pathProbe
}

func newLiveMonitor(enabled bool) *liveMonitor {
	return &liveMonitor{clients: make(map[chan []byte]struct{}), done: make(chan struct{}),
		previous: make(map[string]uint64), probes: [2]*pathProbe{newPathProbe("ipv4", "1.1.1.1", enabled), newPathProbe("ipv6", "2606:4700:4700::1111", enabled)}}
}

func counterRate(now, previous uint64, seconds float64) float64 {
	if seconds <= 0 || now < previous { return 0 }
	return float64(now-previous)/seconds
}

func (p *Panel) sampleLive(now time.Time) {
	m := p.Live
	data := p.Stats.Get()
	data["core_enabled"] = p.Core != nil
	data["storage_healthy"] = p.Store.Fault() == nil
	data["health"] = readKernelHealth()
	data["probes"] = []probeSnapshot{m.probes[0].snapshot(), m.probes[1].snapshot()}
	point := trafficPoint{At: now.UnixMilli(), RX: data["rx_per_second"].(float64), TX: data["tx_per_second"].(float64)}
	m.mu.Lock()
	defer m.mu.Unlock()
	if p.Core != nil {
		k := p.Core.Metrics()
		seconds := float64(0)
		if !m.at.IsZero() { seconds = now.Sub(m.at).Seconds() }
		for _, key := range []string{"payload_upload", "payload_download", "packets_in", "packets_out", "dropped", "rejected", "outbound_errors"} {
			value := k[key].(uint64)
			k[key+"_per_second"] = counterRate(value, m.previous[key], seconds)
			m.previous[key] = value
		}
		point.Upload = k["payload_upload_per_second"].(float64)
		point.Download = k["payload_download_per_second"].(float64)
		point.PacketsIn = k["packets_in_per_second"].(float64)
		point.PacketsOut = k["packets_out_per_second"].(float64)
		data["core"] = k
	}
	m.at = now
	data["sampled_at_ms"] = point.At
	data["traffic"] = point
	m.data = data
	if len(m.history) == historyLimit { copy(m.history, m.history[1:]); m.history = m.history[:historyLimit-1] }
	m.history = append(m.history, point)
	frame, err := json.Marshal(map[string]any{"type": "sample", "data": data})
	if err != nil { return }
	for ch := range m.clients {
		select { case ch <- frame: default:
			select { case <-ch: default: }
			select { case ch <- frame: default: }
		}
	}
}

func (m *liveMonitor) snapshot() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]any, len(m.data))
	for k,v := range m.data { out[k]=v }
	return out
}

func (m *liveMonitor) subscribe() (chan []byte, []byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	select {case <-m.done: return nil,nil,false; default:}
	if len(m.clients) >= liveClientLimit { return nil,nil,false }
	frame, err := json.Marshal(map[string]any{"type": "snapshot", "data": m.data, "history": m.history, "interval_ms": 1000})
	if err != nil { return nil,nil,false }
	ch := make(chan []byte,1)
	m.clients[ch] = struct{}{}
	return ch,frame,true
}
func (m *liveMonitor) unsubscribe(ch chan []byte) { m.mu.Lock(); delete(m.clients,ch); m.mu.Unlock() }

func (p *Panel) runLive(ctx context.Context) {
	var workers sync.WaitGroup
	for _, probe := range p.Live.probes {
		workers.Add(1)
		go func(probe *pathProbe){defer workers.Done(); probe.run(ctx)}(probe)
	}
	defer workers.Wait()
	defer close(p.Live.done)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done(): return
		case now := <-ticker.C:
			p.Stats.sample()
			p.sampleLive(now)
		}
	}
}
