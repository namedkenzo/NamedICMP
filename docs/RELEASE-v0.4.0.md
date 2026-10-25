# namedICMP 0.4.0 — IPv4/IPv6 connection settings

- Settings replaces the Outbounds sidebar entry, with Connection and Outbounds tabs.
- IPv4/IPv6 selection, IP/domain override and automatic public-address detection. IPv6 literals use bracketed URI authorities; selected domains carry `family=ipv6`.
- Home addresses stay masked until explicitly revealed, reset on logout.
- ICMPv4/ICMPv6 listeners share credentials, accounting, quotas and paced output. Existing IPv4 profiles remain valid.
- Optional Linux AF_PACKET reader with ICMP filtering, local-destination checks and identical authentication. Default remains raw IP. Reader changes require restart.
- Fresh-install prompts match the settings. IPv6 Echo suppression is reversible and does not disable neighbor discovery or ICMPv6 errors.

NEW is a conntrack state, not a reader mode. AF_PACKET sees only traffic that
reaches the VPS and cannot recover upstream-dropped traffic. See [details](IPV6-READERS.md).
Use namedP with ICMPv6 support; third-party compatibility varies. Tests are not
a real carrier/VPS reachability, throughput or leak acceptance.
