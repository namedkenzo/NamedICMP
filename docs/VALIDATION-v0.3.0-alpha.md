# namedICMP 0.3.0-alpha / namedP 0.1.0-alpha validation

Base: main commit 802dc1e5f5da30a752dbab46ec77bfaaefd3d58f.

## Executed

- Linux: go test -race ./..., go vet, installer/update regression checks.
- Panel DOM: login, metrics, user CRUD, encryption-only exports, clipboard,
  outbounds, languages, system-light default and dark/light overrides.
- Go codec fuzz checks and server compilation.
- Client/server in-memory ICMP integration: numeric, AES-128, AES-256 and
  ChaCha20; TCP integrity/retransmission/half-close; multi-destination UDP.
- Windows: Go race tests, vet, native self-contained WPF build, engine build,
  independent version validation, PowerShell parse and both-theme render smoke.
- Pinned tun2socks build and Wintun supplier signature verification.
- Local JS/Bash/XML syntax and version-contract checks.

Evidence: [Linux and panel CI](https://github.com/namedkenzo/NamedICMP/actions/runs/40122970355),
[Windows and published client](https://github.com/namedkenzo/NamedICMP/actions/runs/40123144447).
Final publication workflows repeat the build and checks for their release commit.

## Independent distributions

- Server: 0.3.0-alpha, tag v0.3.0-alpha; Linux amd64/arm64 archives + SHA256SUMS.
- Client: 0.1.0-alpha, tag namedp-v0.1.0-alpha; namedP-0.1.0-alpha-windows-x64.zip
  + its own SHA256SUMS. The Windows client is never installed by the Linux manager.

## Outstanding acceptance

Public-VPS Windows tests remain unperformed: real ICMP TCP/UDP, DNS/IPv6 leak
checks, throughput comparison, forced GUI/engine crash cleanup, Defender,
network changes and sleep/resume. CI success does not establish bug-free VPN
operation. The app is unsigned; its Wintun supplier DLL is signature-checked.
VPN is IPv4-only, temporarily blocks IPv6, and preserves more-specific LAN
routes. UDP support requires the application, client and server outbound.
