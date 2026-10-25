# NamedICMP 0.4.2 — UDP scheduling under load

UDP responses now use a separate bounded 64-packet queue rather than waiting
behind up to 2048 bulk TCP packets. When full, newer datagrams replace older ones.
Four UDP opportunities alternate with a waiting TCP packet, preserving TCP
progress, control-message priority and the existing global ICMP rate limit.
No firewall, credentials, quota or protocol changes are required.

Tests cover UDP ordering under a saturated TCP queue, bounded newest-packet
retention and drop accounting, alongside the existing bulk-transfer regressions.
This targets application-induced queue delay, not ISP routing or guaranteed game
ping. Prefer Direct or UDP-capable SOCKS outbounds for games; HTTP is TCP-only.
Update with the current installer, option 2. Active tunnels reconnect once.
