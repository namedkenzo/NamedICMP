# Verification report — 0.2.1-alpha interface update

Prepared 2026-10-13 (Asia/Tehran). This update changes the panel presentation,
static font/icon assets, navigation accessibility and the release version.
The core, wire codec, store and outbound source files are byte-identical to 0.2.0.

| Check | Result |
| --- | --- |
| Chromium 153 with real Go panel API | 82 layout checks passed: login/home/users/outbounds, Persian/English, dark/light, widths 360/390/768/1280/1440 |
| Browser errors and CSP | No unexpected HTTP, JavaScript or console errors; self-hosted fonts loaded under the existing same-origin CSP |
| Visual inspection | Desktop dark/light overview, Persian login, English light overview and mobile Persian dialog inspected |
| Native dialog | Escape closes the dialog; long mobile forms scroll while action buttons remain visible |
| Motion | Decorative rings present, reduced-motion disables animations, pause styling works for hidden-tab handling |
| Existing DOM/API smoke | Login, metrics, users, encryption default/selection, exact URI/clipboard, outbounds, themes/languages, navigation and logout passed |
| Go checks | All package race tests and go vet passed |
| Installer/update mocks | Passed; original state-preserving Update path remains available |
| Build | Linux amd64 and arm64 static archives rebuilt for 0.2.1-alpha |

The chart inspection used temporary browser fixture history to review shape and
layout. No fixture data was added to the product. Production charts show measured
history only. Browser images were used for internal review and are not release
assets. No new ICMP/WAN interoperability or speed claim is made by a theme update.

Self-hosted font assets (SHA-256):

- Manrope WOFF2: `d579bc0735655ad821c850324876cd843ca44e3e6765ce031fc8411a64daf244`
- Vazirmatn v33.003 WOFF2: `4e3fa217d38fdafc1fea4414ceb58ca5e662cf0ab5fa735a8c8c20e8b42cad92`

The following is the previous core verification record, retained for continuity.

---

# Verification report — 0.2.0-alpha

Prepared 2026-10-12. Go 1.27.2, Linux amd64, AMD EPYC 9V74.
Dependencies verified with go.sum: klauspost/compress v1.20.3,
golang.org/x/crypto v0.57.0 and golang.org/x/sys v0.48.0.

## Reproduced defects

The following regression cases failed against the original 0.1 source and pass
with the revised core. These are real loopback socket tests with an injected
ICMP transport; they identify server defects, not the client's UDP capabilities.

| Regression | Before | Revised behavior |
| --- | --- | --- |
| UDP connection ID reused for two targets | Second target timed out | Independent destination flows reply correctly |
| UDP datagram of 4096 bytes | Silently rejected by 2000-byte cap | Entire datagram echoed; 4096 bytes counted in each direction |
| TCP and UDP share one client ID | TCP handshake collided with UDP session | Both protocols remain usable independently |

The user reported that TCP worked on a real configuration before this change.
That report is not treated as an independently verified UDP/WAN acceptance test.

## Successful checks

| Check | Result |
| --- | --- |
| `go test -race ./...` | All packages passed; no race detected in exercised paths |
| `go vet ./...` | Passed |
| `go mod verify` | All modules verified |
| TCP ordering/loss/compression | 142,080 bytes delivered correctly with duplicate uploads and one deliberately lost download frame; exact directional accounting |
| Repeated core integrations | Loss/compression, user isolation, quota, UDP regressions and all four encryption modes passed ten repetitions |
| Encrypted TCP + UDP integration | None, AES-128-GCM, AES-256-GCM and ChaCha20-Poly1305: real TCP echo plus 4096-byte UDP and subsequent UDP traffic with a shared ID; envelope overhead excluded from quota |
| Independent AEAD vectors | AES-128/256 and ChaCha20 vectors generated with Python cryptography passed; modified ciphertext rejected |
| Encryption routing | Mixed modes on the same source/echo ID, wrong key, plaintext downgrade and deleted cached credentials tested |
| Legacy migration | Old state retains access key, counters, quota and administrator hash; missing encryption remains none; missing UDP session limit becomes 128 |
| Persistent keys | Key sizes, unrelated-edit stability and disabling encryption tested |
| Retransmission scheduler | Initial packet order, ACK removal, selective retry, one congestion reduction per timeout burst and rejection of unsent-frame ACKs tested |
| Persistence/quota | Isolation, concurrent spending, graceful counters, conservative crash recovery, exclusive lock, disk-failure denial and skipping idle file replacement passed |
| Outbounds | SOCKS5 authentication/CONNECT, UDP ASSOCIATE, malformed/fragmented datagram recovery, direct UDP truncation recovery and HTTP prefetched bytes passed |
| Routing policy | Private targets denied by default; HTTP UDP fails without bypassing the selected outbound |
| Panel API | Login/origin/CSRF/user lifecycle/logout; default-off encryption and omitted-field preservation on edits passed |
| DOM smoke using real API | Login, metrics, users, default none, AES selection, exact URI and clipboard, outbounds, languages, themes, navigation and logout passed |
| JavaScript/Bash syntax | Passed |
| Installer menu | Grouping, no ANSI for non-TTY output and strict action failure without closing the menu passed |
| Update mocks | Running/stopped service, byte-for-byte config/state preservation, three unique backups and failed-health executable rollback passed |
| Cross compilation | Static Linux amd64 and arm64 distribution archives built; public-document membership and SHA-256 checks verified; amd64 binary reports 0.2.0-alpha |
| Wire fuzzing | `FuzzDecode`, 15 seconds / two workers: 96,796 executions, no failure |
| Encryption fuzzing | `FuzzEncryptedDecoder`, 15 seconds / two workers: 879,580 executions, no failure |

The encrypted integration uses the production envelope codec and cryptographic
path around the injected transport, not a privileged raw socket. The DOM test
uses jsdom, with dialog/media-query/clipboard behavior stubbed; it does not
render pixels. The installer uses an isolated mock service, not actual systemd.

## Focused performance comparison

Go 1.27.2, GOMAXPROCS=8, same machine, one-second benchmark periods, three
repetitions. Values below are medians. The baseline is extracted 0.1 source
with equivalent benchmark operations added; its production code is unchanged.

| Operation | 0.1 median | 0.2 median | Allocation change |
| --- | --- | --- | --- |
| No-due retransmit check plus next deadline, 1024 pending frames | 22,950 ns/op | 11.30 ns/op | 0 in both |
| Construct session identity and look it up in a populated map | 59.20 ns/op | 63.09 ns/op | 64 B / 1 allocation becomes 0 B / 0 allocations |
| Contiguous Reliable.Input benchmark | 143.5 ns/op | 130.9 ns/op | 176 B / 4 allocations in both |

The large deadline-check difference measures replacing repeated map scans with
heap-head lookup when no frame is due. It does **not** include queue insertion,
ACK heap removal, encryption, filesystem work or end-to-end transfer, and must
not be advertised as an equivalent Internet speedup. Identity lookup time is
similar/slightly higher while eliminating a per-packet allocation. Small Input
timing differences are not enough to establish a general throughput improvement.

Current benchmarks can be reproduced with:

```bash
GOMAXPROCS=8 go test ./internal/core -run '^$' \
  -bench 'BenchmarkPendingDeadlines|BenchmarkSessionIdentity|BenchmarkReliable' \
  -benchmem -benchtime=1s -count=3
```

The baseline deadline benchmark queues/sends 1024 frames, repeatedly calls
Due at +1 ms before their deadline and stores Next in a package-level duration
sink. The baseline identity benchmark constructs the former string identity
and tests membership in a populated map on every iteration. These sinks prevent
an unused result from being optimized away.

Additional changes batch TCP reads, preserve output across window shrink,
precompute SOCKS UDP headers/special IP ranges and skip unchanged disk flushes.
Their practical benefit and optional encryption cost need a real workload.

## Not verified here

- Raw ICMPv4 and Unix management sockets: this environment denies those sockets.
- Actual apt/systemd installation, capability sandbox, update rollback, uninstall
  and echo-sysctl restoration on a VPS.
- Public HTTP/HTTPS reachability, real ACME issuance and certificate renewal.
- Specific Android/iOS client versions, official CLI binary interoperability,
  import of the requested URI, and application-side UDP ASSOCIATE behavior.
- WAN throughput, latency, NAT, fragmentation, packet loss and congestion versus
  upstream PingTunnel; no faster-WAN or zero-bug claim is made.
- Sustained production load, large encrypted-user cold lookup storms and an
  independent security audit.
- Visual/responsive behavior in a real rendering browser.

Follow `docs/ACCEPTANCE.fa.md` on a test VPS using a UDP-capable client. HTTP
CONNECT and TCP-only clients cannot provide native UDP forwarding. The exact
requested URI omits encryption settings, which must be configured separately.
