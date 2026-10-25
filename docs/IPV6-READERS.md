# IPv6 endpoints and packet readers

NamedICMP is a VPN-over-ICMP web management panel. Settings → IPv4 / IPv6 controls
connection export, not whether a VPS actually has routed public IPv6. Use a
literal address or ASCII/punycode domain. IPv6 domains need AAAA records. Scoped,
link-local and mixed-family endpoints are rejected. Both ICMP families can listen
simultaneously; legacy IPv4 profiles remain valid.

Home addresses start masked; reveal state resets on logout and is not persisted.
Detection prefers public interface addresses, then bounded HTTPS queries to fixed
ipify public-IP endpoints. No credentials or user traffic are sent. Proxy-configured
installs skip reflection to avoid advertising a proxy IP. Disable panel reflection
with `NAMEDICMP_PUBLIC_IP_LOOKUP=0`. Use an explicit IP/domain for NAT/routed setups.
Address detection does not establish external reachability.

Version 0.4.1 fixes the service's missing AF_NETLINK permission, which could
prevent interface discovery even when the VPS had IPv6. Update with the current
installer (**2 — Update**) to refresh the systemd profile; restarting the old
profile alone is insufficient. A local `/proc/net/if_inet6` fallback also supports
discovery when interface enumeration is restricted. Unready, local-only and
link-local addresses must not be exported as public automatic endpoints.

## Packet reading

- Raw IP socket is the existing default.
- Raw packet capture uses Linux AF_PACKET/SOCK_DGRAM and an ICMP-only BPF filter.
  It checks local destinations, framing/checksums, credentials and user policy.
  Captured fragments/IPv6 extension headers are rejected. It requires CAP_NET_RAW
  and AF_PACKET in the service allowlist; update with the current installer.
  Restart after changing this mode; active and configured values are shown separately.
  If the capture socket cannot open, raw IP remains active with an explicit
  warning instead of taking the panel offline. Do not mistake a saved choice
  for confirmation that the reader is active.
- NEW is a firewall conntrack state, not a socket type. There is no fake NEW
  reader. AF_PACKET may see a delivered frame before local IP-layer filtering;
  it cannot recover packets blocked by the carrier, provider or upstream router.
  No firewall, NAT or conntrack rules are changed by this option.

Primary references: [packet(7)](https://man7.org/linux/man-pages/man7/packet.7.html),
[raw(7)](https://man7.org/linux/man-pages/man7/raw.7.html), and
[Netfilter CT/NOTRACK](https://ipset.netfilter.org/iptables-extensions.man.html).

Do not block ICMPv6 neighbor discovery or Packet Too Big. Only ordinary kernel
Echo replies are suppressed; IPv6 is not globally disabled. The HTTP bind is
separate from the exported tunnel endpoint and is preserved on update. Fresh
IPv6 installations use a dual IPv6 HTTP bind unless localhost-only was selected.

Back up data before downgrading; old versions may ignore/discard new connection
settings. Windows/Android packages remain in NamedClients, not in the VPS installer.
Loopback and automated tests do not replace real-device/VPS acceptance.
