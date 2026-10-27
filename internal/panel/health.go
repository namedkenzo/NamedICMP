package panel

import (
	"os"
	"strconv"
	"strings"
	"syscall"
)

// Kernel counters are host-wide totals since boot, not tunnel packet loss.
func parseICMPv4(b []byte) map[string]uint64 {
	lines := strings.Split(string(b), "\n")
	for i := 0; i+1 < len(lines); i++ {
		keys, values := strings.Fields(lines[i]), strings.Fields(lines[i+1])
		if len(keys) < 2 || len(keys) != len(values) || keys[0] != "Icmp:" || values[0] != "Icmp:" {
			continue
		}
		out := map[string]uint64{}
		for j := 1; j < len(keys); j++ {
			if n, err := strconv.ParseUint(values[j], 10, 64); err == nil {
				out[keys[j]] = n
			}
		}
		return out
	}
	return nil
}
func parseICMPv6(b []byte) map[string]uint64 {
	out := map[string]uint64{}
	for _, line := range strings.Split(string(b), "\n") {
		v := strings.Fields(line)
		if len(v) != 2 || !strings.HasPrefix(v[0], "Icmp6") {
			continue
		}
		if n, err := strconv.ParseUint(v[1], 10, 64); err == nil {
			out[strings.TrimPrefix(v[0], "Icmp6")] = n
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
func readKernelHealth() map[string]any {
	v4, _ := os.ReadFile("/proc/net/snmp")
	v6, _ := os.ReadFile("/proc/net/snmp6")
	settings := map[string]any{}
	for _, key := range []string{"net/ipv4/icmp_msgs_per_sec", "net/ipv4/icmp_msgs_burst", "net/ipv4/icmp_ratelimit", "net/ipv4/icmp_echo_ignore_all", "net/ipv6/icmp/ratelimit", "net/ipv6/icmp/echo_ignore_all", "net/core/rmem_max", "net/core/wmem_max"} {
		name := strings.ReplaceAll(key, "/", ".")
		b, err := os.ReadFile("/proc/sys/" + key)
		settings[name] = nil
		if err == nil {
			if n, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64); err == nil {
				settings[name] = n
			}
		}
	}
	var fdUsed any
	if entries, err := os.ReadDir("/proc/self/fd"); err == nil {
		fdUsed = len(entries)
	}
	var limit syscall.Rlimit
	var fdLimit any
	if syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit) == nil {
		fdLimit = limit.Cur
	}
	return map[string]any{"icmp4": parseICMPv4(v4), "icmp6": parseICMPv6(v6), "settings": settings, "fd_used": fdUsed, "fd_limit": fdLimit}
}
