# ICMP stability and network tuning

Install and Update apply a conservative, reversible profile. Menu **12** reapplies
it; **13** restores previous values. Uninstall also attempts restoration. Originals
are in `/etc/namedicmp/network-tuning.json`; boot persistence is limited to
`/etc/sysctl.d/91-namedicmp-network.conf`. Administrator changes are preserved.
Unsupported/restricted kernel keys warn without disabling installation.

| Setting | Minimum applied; larger existing values retained |
| --- | --- |
| `net.core.rmem_max` / `net.core.wmem_max` | 4 MiB each; permits the raw socket's requested buffers |
| `net.core.netdev_max_backlog` | 4096 |
| `net.ipv4.icmp_msgs_per_sec` | 5000; existing unlimited zero retained |
| `net.ipv4.icmp_msgs_burst` | 200 |

**The ICMP message sysctls govern kernel-generated ICMP messages according to
the kernel's rate mask. They do NOT pace or accelerate the raw Echo packets
written by namedICMP, and cannot override provider DDoS detection.**
Kernel Echo suppression remains the separate legacy `90-namedicmp.conf` profile,
with its existing original-value backup/restore. No firewall is flushed, ICMP
error limiting is not globally disabled, and reverse-path filtering/TCP security
settings are not weakened.

## Application pacing

A single bounded ICMP writer reserves capacity for control/ACK/probe messages
and fairly schedules bulk data. A slow outgoing socket no longer runs inline in
the receive loop. Defaults also apply when loading legacy config files:

```json
{
  "icmp_packets_per_second": 2500,
  "icmp_burst_packets": 32
}
```

These are aggregate **outgoing** packet limits, not Mbps guarantees or inbound
firewall protection. Control/ACK traffic shares the budget. Use a rate agreed
with your provider; larger rates may improve throughput on approved networks
with sufficient capacity. Zero packets/sec explicitly disables pacing; do not
do this to bypass a provider restriction. Restart after config changes.

Provider null-routing, account suspension, host shutdown, upstream policing and
carrier ICMP blocking cannot be prevented by guest sysctls. Ask the provider
whether ICMP tunneling is permitted and which rates it supports. This release
does not promise to eliminate every speed-test disconnect.

## Diagnose a speed-test disconnect

Capture `/opt/namedicmp/namedicmp diagnostics` immediately before and after the
test without restarting. Counters distinguish DNS, destination-policy, timeout,
network and other outbound errors, output/session queue drops, socket write
errors and session-limit rejections. `outbound_errors` is cumulative, not itself
proof of a server crash. User secrets/destinations are not logged.
Check `systemctl show namedicmp -p ActiveState -p NRestarts -p Result` and provider
events for restarts/OOM/mitigation. Do not share `state.json` or credentials.

## فارسی

نصب و آپدیت، تنظیمات محدود و قابل‌بازگشت را خودکار اعمال می‌کنند. گزینهٔ ۱۲
برای اعمال مجدد و ۱۳ برای بازگردانی است. مقدارهای بزرگ‌تر موجود کاهش پیدا
نمی‌کنند و تغییر دستی مدیر حفظ می‌شود. این تنظیمات فایروال یا حفاظت TCP را
خاموش نمی‌کنند. محدودیت‌های `icmp_msgs_*` مربوط به پیام‌های کرنل هستند؛
بسته‌های خام تونل جداگانه داخل برنامه صف‌بندی و تنظیم نرخ می‌شوند.
هیچ تنظیم داخل VPS نمی‌تواند خاموش‌شدن یا مسدودسازی از طرف ضد‌دیداس میزبان
را تضمینی متوقف کند. مجازبودن ICMP و نرخ مجاز را با میزبان هماهنگ کنید.
