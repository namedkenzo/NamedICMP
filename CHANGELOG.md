# 0.5.3 — 2026-10-28

- Honor cancellation and the caller's earlier deadline throughout SOCKS5/HTTP proxy negotiation, including SOCKS UDP relay lookup; detach cancellation after a successful handoff.
- Skip queued retransmissions once their original data is acknowledged, and invalidate queued sends when a stream closes; preserve pacing, payload accounting and protocol compatibility.
- Accept all valid HTTP CONNECT 2xx responses while preserving buffered tunnel data and rejecting unsuccessful/malformed statuses.
- Expose avoided obsolete sends separately in diagnostics and Dashboard; add real-socket cancellation, CONNECT response and blocked-writer regressions.
- Preserve installed users/settings, quotas, rate/capacity defaults and client versions. No deployment-specific Speedtest success claim.

# 0.5.2 — 2026-10-28

- Ignore stale TCP frames before saturated-table admission checks; retain bounded, isolated new-flow admission.
- Split queue and capacity counters, surface busiest-user pressure and UDP expiry/error diagnostics.
- Add an opt-in, backed-up TCP64/UDP192 per-user capacity trial without changing the global256-flow or packet-rate caps.
- Verify the admission regression against v0.5.1 and test classification, UI labels and safe trial rollback.
- Preserve installed settings and client versions; deployment-specific Speedtest acceptance remains pending.

# 0.5.1 — 2026-10-27

- Start retransmission timing at socket-write completion, keep download EOF data draining, and retry known transient ICMP receive errors.
- Add deterministic queue/completion and lossy download regressions; preserve wire compatibility and accounting.
- Document measured laboratory improvement separately from unresolved deployment-specific failures.

# 0.5.0 — 2026-10-27

- Rename Home to Dashboard and stream real one-second measurements over authenticated WebSocket.
- Add five-minute payload/interface/packet charts and IPv4/IPv6 path RTT, loss and jitter with explicit measurement scope.
- Surface active TCP/UDP flows, queue pressure, pacing, storage health, outbound/drop reasons and kernel/interface diagnostics.
- Keep monitoring shared and bounded; pause hidden views, revoke logged-out streams and label missing/stale data.
- Preserve tunnel protocol, client applications and installed user/settings data.

# 0.3.0 — 2026-10-24

- Promote the server and web panel to a stable release, published as Latest.
- Align the binary, installer, panel API/UI and package metadata to 0.3.0.
- Keep namedP independently versioned at 0.1.0-alpha and distributed separately.
- Preserve the tested 0.3.0-alpha server functionality; no new transport changes.

# 0.3.0-alpha — 2026-10-24

- Add namedP native Windows UI with system/dark/light themes and animated dachshund.
- Add independent Go client, TCP reliability and SOCKS5 TCP/UDP proxy.
- Add full-device IPv4 TUN routing with server bypass, temporary DNS/IPv6 guards and cleanup guardian.
- Fix missing user total usage in panel API and follow system theme by default.
- Avoid copying all users and credentials just to count dashboard users.
- Share the reliability module between client and server; add Windows CI packaging and release notes.
- Linux CI and Windows build/UI checks passed. Windows/VPS network acceptance remains pending.

# 0.2.2-alpha

- Prefer verified Linux release binaries for remote install/update; reuse installed dependencies and remove mandatory install-time tests/compilation.
- Default to namedkenzo/NamedICMP without a repository prompt; add bounded/retried downloads, occupied-port detection, username validation and startup health checks.
- Fix encryption-only GUI authentication, hide private routing IDs from encrypted profiles, and export mutually exclusive numeric/encrypted URIs and commands.
- Add English default and Persian READMEs, Windows diagnosis and upgrade instructions.
- Create draft prereleases with both Linux archives and SHA256SUMS on version tags.

# Changelog

## 0.2.1-alpha

- Original named wordmark, monogram and signal artwork; charcoal/champagne dark
  theme and ivory light theme across login, overview, users, outbounds and dialogs.
- Self-hosted variable Manrope and Vazirmatn fonts, original SVG icon set and
  network chart generated solely from measured history.
- Short page/dialog transitions, subtle hover states and decorative login rings;
  respects reduced motion and pauses animation in hidden tabs.
- Mobile/RTL layouts, clearer focus states, translated accessibility labels and
  active-navigation semantics. Core, protocol and encryption defaults unchanged.

## 0.2.0-alpha

- Fix TCP/UDP session collisions and UDP destination retargeting: protocol and
  UDP target are now part of session ownership. Separate per-user limits.
- Preserve UDP datagrams up to 8192 bytes; drop malformed/truncated SOCKS UDP
  individually. UDP exit no longer sends an ambiguous legacy KICK.
- Mark TCP/UDP replies correctly; refresh echo metadata per flow. Prefer IPv4
  on dual-stack targets and retry alternate validated direct TCP addresses.
- Schedule retransmission with a deadline heap; preserve equal-deadline send
  order, cut congestion once per loss interval, and ignore unsent-frame ACKs.
- Batch socket reads up to 32 frames, keep output across window shrink, preserve
  HTTP CONNECT half-close, and skip unchanged idle-state checkpoints.
- Add per-user none/AES-128-GCM/AES-256-GCM/ChaCha20-Poly1305 selection, default
  off for new and existing users. Random persistent Base64 keys, client CLI
  options, bounded encrypted credential discovery, and no plaintext downgrade.
- Copy exact `pingtunnel://HOST?lport=1080&mode=vpn&key=KEY` connection URI;
  show encryption settings separately because this URI does not encode them.
- Group terminal actions and add update, restart, logs and TCP/UDP diagnostics.
  Update preserves state/config, creates unique backups and restores the old
  executable after failed health validation.
- Add regressions that fail on 0.1, encrypted TCP/UDP integration for all modes,
  externally generated AEAD vectors, legacy migration, relay error recovery,
  installer/update mocks and expanded panel DOM checks.

## 0.1.0-alpha

- Independently authored ICMPv4 codec and TCP/UDP session engine.
- Per-user numeric keys, expiry, quota, directional accounting and durable grants.
- Direct, SOCKS5 TCP/UDP and HTTP CONNECT outbounds.
- Embedded Persian/English panel, RTL, dark/light themes and system overview.
- Interactive Debian/Ubuntu installer, systemd, Certbot deployment/renewal and
  local credential management without restarting ICMP sessions.
- Protocol, quota, crash-recovery, concurrency, proxy, HTTP security and
  injected-transport integration tests; acceptance guide for real VPS testing.

No production or universal mobile-client compatibility claim is made.

