package panel

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Stats struct {
	mu                  sync.RWMutex
	data                map[string]any
	total, idle, rx, tx uint64
	at                  time.Time
	statePath           string
}

func cpuTicks() (total, idle uint64) {
	b, _ := os.ReadFile("/proc/stat")
	lines := strings.SplitN(string(b), "\n", 2)
	v := strings.Fields(lines[0])
	for i, s := range v {
		if i == 0 || i > 8 {
			continue
		}
		n, _ := strconv.ParseUint(s, 10, 64)
		total += n
		if i == 4 || i == 5 {
			idle += n
		}
	}
	return
}
func (s *Stats) sample() {
	total, idle := cpuTicks()
	cpu := float64(0)
	if total > s.total && idle >= s.idle {
		cpu = 100 * (1 - float64(idle-s.idle)/float64(total-s.total))
	}
	s.total = total
	s.idle = idle
	mem := map[string]uint64{}
	if f, err := os.Open("/proc/meminfo"); err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			v := strings.Fields(scanner.Text())
			if len(v) > 1 {
				n, _ := strconv.ParseUint(v[1], 10, 64)
				mem[strings.TrimSuffix(v[0], ":")] = n * 1024
			}
		}
		f.Close()
	}
	var rx, tx, rxErrors, txErrors, rxDrops, txDrops uint64
	networkAvailable := false
	if b, err := os.ReadFile("/proc/net/dev"); err == nil {
		networkAvailable = true
		for _, line := range strings.Split(string(b), "\n") {
			v := strings.Fields(strings.ReplaceAll(line, ":", " "))
			if len(v) < 17 || v[0] == "lo" {
				continue
			}
			n, _ := strconv.ParseUint(v[1], 10, 64)
			rx += n
			n, _ = strconv.ParseUint(v[9], 10, 64)
			tx += n
			for _, entry := range []struct {
				index  int
				target *uint64
			}{{3, &rxErrors}, {4, &rxDrops}, {11, &txErrors}, {12, &txDrops}} {
				n, _ := strconv.ParseUint(v[entry.index], 10, 64)
				*entry.target += n
			}
		}
	}
	now := time.Now()
	rxRate, txRate := float64(0), float64(0)
	if !s.at.IsZero() {
		seconds := now.Sub(s.at).Seconds()
		if rx >= s.rx {
			rxRate = float64(rx-s.rx) / seconds
		}
		if tx >= s.tx {
			txRate = float64(tx-s.tx) / seconds
		}
	}
	s.rx = rx
	s.tx = tx
	s.at = now
	var fs syscall.Statfs_t
	_ = syscall.Statfs(filepath.Dir(s.statePath), &fs)
	diskTotal := fs.Blocks * uint64(fs.Bsize)
	diskFree := fs.Bavail * uint64(fs.Bsize)
	uptime := float64(0)
	if b, err := os.ReadFile("/proc/uptime"); err == nil {
		v := strings.Fields(string(b))
		if len(v) > 0 {
			uptime, _ = strconv.ParseFloat(v[0], 64)
		}
	}
	host, _ := os.Hostname()
	kernel, _ := os.ReadFile("/proc/sys/kernel/osrelease")
	osName := runtime.GOOS
	if b, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				osName = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
			}
		}
	}
	used := mem["MemTotal"] - min(mem["MemTotal"], mem["MemAvailable"])
	pct := float64(0)
	if mem["MemTotal"] > 0 {
		pct = 100 * float64(used) / float64(mem["MemTotal"])
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	data := map[string]any{"hostname": host, "os": osName, "kernel": strings.TrimSpace(string(kernel)), "architecture": runtime.GOARCH, "cpu_cores": runtime.NumCPU(), "cpu_percent": max(0, min(100, cpu)), "ram_total": mem["MemTotal"], "ram_used": used, "ram_percent": pct, "swap_total": mem["SwapTotal"], "swap_used": mem["SwapTotal"] - min(mem["SwapTotal"], mem["SwapFree"]), "disk_total": diskTotal, "disk_free": diskFree, "uptime_seconds": uptime, "network_rx": rx, "network_tx": tx, "rx_per_second": rxRate, "tx_per_second": txRate, "process_memory": ms.Alloc, "goroutines": runtime.NumGoroutine(), "sampled_at": now.Unix()}
	data["cpu_available"] = total > 0
	data["network_available"] = networkAvailable
	data["interface_rx_errors"], data["interface_tx_errors"] = rxErrors, txErrors
	data["interface_rx_drops"], data["interface_tx_drops"] = rxDrops, txDrops
	data["process_rss"] = nil
	if b, err := os.ReadFile("/proc/self/statm"); err == nil {
		fields := strings.Fields(string(b))
		if len(fields) > 1 {
			if pages, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
				data["process_rss"] = pages * uint64(os.Getpagesize())
			}
		}
	}
	s.mu.Lock()
	s.data = data
	s.mu.Unlock()
}
func NewStats(path string) *Stats {
	s := &Stats{statePath: path}
	s.total, s.idle = cpuTicks()
	s.sample()
	return s
}
func (s *Stats) Run(ctx context.Context) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.sample()
		}
	}
}
func (s *Stats) Get() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := map[string]any{}
	for k, v := range s.data {
		out[k] = v
	}
	return out
}
