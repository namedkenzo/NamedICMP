# Architecture and wire contract

Server release: 0.3.0. The independently versioned namedP Windows client is 0.1.0-alpha. Client and server share the original codec and selective-repeat package in this repository; they are separate executables. No upstream PingTunnel/gohome/3x-ui sources or binaries are distributed.

## Data flow

1. A Linux raw IPv4 ICMP socket reads echo messages. Checksums, optional AEAD authentication, protobuf lengths, magic and reply type are checked before dispatch.
2. Plaintext uses a positive numeric credential to resolve one user. Encrypted envelopes use numeric zero and are resolved by AEAD authentication, then mapped to a private positive routing ID. The incoming encryption mode must match the user setting. Disabled, expired, exhausted or storage-faulted users are rejected. PING probes have a per-user token bucket.
3. A bounded session is selected by user ID, source IP, connection ID and TCP/UDP. UDP additionally includes target host:port. This separates both protocols and multiple UDP destinations even when a client reuses an ID. The ICMP echo identifier can change through NAT without changing ownership.
4. A per-session goroutine dials the chosen outbound. TCP selective repeat orders and deduplicates frames; UDP forwards a bounded datagram.
5. Unique application bytes acquire durable per-user credit and update directional counters before forwarding. Protocol controls and TCP retransmissions do not charge again.
6. Responses use the user's numeric credential (plaintext) or zero (encrypted) and selected encryption, the latest echo ID/sequence and requested reply type, and the correct TCP bit. The response envelope's reverse protocol is -1, so the server does not process its own replies.

The panel runs in the same Go binary with separate HTTP goroutines. It does not call service restarts for CRUD. ICMP and HTTP share the state mutex briefly; durable grant acquisition and periodic flush can block on disk. This is not complete process isolation, and storage performance must be included in throughput benchmarks.

## Envelope mapping

The independently written codec uses protobuf wire types directly; it does not distribute a copied `.proto` or generated upstream bindings.

| Field number | Interpretation | Encoding |
| --- | --- | --- |
| 1 | Connection ID | string |
| 2 | DATA=0, PING=1, KICK=2 | int32 |
| 3 | Target host:port | string |
| 4 | Datagram or serialized TCP frame | bytes |
| 5 | ICMP reply type | sint32 / zigzag |
| 6 | Magic 0xdead | sint32 / zigzag |
| 7 | Numeric credential (plaintext); zero (encrypted) | sint32 / zigzag |
| 8 | Idle timeout seconds | int32 |
| 9 | TCP mode | int32 |
| 10 | Peer buffer size (observed, local allocation remains bounded) | int32 |
| 11 | Peer ID window | int32 |
| 12 | Initial retransmission delay in milliseconds | int32 |
| 13 | Peer compression preference | int32 |
| 14 | Peer statistics option (ignored) | int32 |

TCP frame fields: kind=1, resend=2, timestamp=3, ID=4, nested data=5, packed/unpacked ACK or REQ IDs=6, upstream local ack flag=7 (ignored). Nested data fields: kind=1, payload=2, compressed=3. Frame kinds are DATA/REQ/ACK/PING/PONG, and nested kinds are application data/CONN/CONNRSP/CLOSE/heartbeat. IDs wrap at 1,000,000; outgoing application frames are at most 888 bytes.

## Optional encryption

Each user has `encryption=none` by default, including migration from 0.1. AES-128-GCM uses a random 16-byte key; AES-256-GCM and ChaCha20-Poly1305 use random 32-byte keys. They are generated independently using `crypto/rand` and stored as standard Base64. The selected mode and secret persist across unrelated edits. Disabling encryption removes the encryption secret; changing mode generates a new one and revokes only that user's active sessions.

The complete protobuf envelope is encrypted as a random 12-byte nonce followed by ciphertext and the 16-byte authentication tag. There is no custom key-ID prefix. This agrees with the reference client's raw-Base64-key mode; no upstream encryption implementation or passphrase KDF was copied. The cryptographic implementations are Go's standard AES/GCM and the general `golang.org/x/crypto/chacha20poly1305` dependency.

Because encrypted legacy envelopes do not expose the access key, cold discovery tries configured encrypted credentials. Discovery is rate-limited globally to 32 attempts/second with a burst of 64; these are envelope lookups, each potentially testing multiple keys. Successful source IP/echo ID pairs cache up to four candidates in a bounded 4096-entry map. Ordinary packets reuse cached AEAD objects. Changed or deleted credentials invalidate candidates through an atomic immutable registry. Installations with no encrypted users use a fast plaintext path without the peer-cache mutex. Very large encrypted installations, shared echo IDs and cold-connection storms need separate load testing.

Plaintext packets for encrypted users, encrypted packets with a positive numeric password, and packets authenticated with the wrong key are rejected. The stored positive ID for encrypted users remains private to server routing/accounting. AEAD provides packet integrity/confidentiality, not forward secrecy or a separate replay-defense protocol. Numeric profiles export `pingtunnel://HOST?lport=1080&mode=vpn&key=KEY`. Encrypted profiles instead export `encrypt=MODE&encrypt_key=BASE64` (URL-encoded), with no numeric `key`; CLI commands likewise use exactly one credential method. Existing combined numeric/encrypted profiles must be re-exported.

## Reliability and limits

ACKs acknowledge a stored, actually sent frame. Delivery and quota charging happen only when all prior IDs have arrived. Old duplicates are ACKed without redelivery. Gaps request retransmission. Unacknowledged frames use bounded exponential retransmission with an ACK-sampled RTO. A min-heap selects due frames; ACK removal and explicit REQ update only relevant entries. Equal deadlines preserve queue order. The sending window grows from 16 frames and reduces once per timeout loss interval, rather than once for every lost frame; it is capped at 1024 frames. A bounded receive map permits up to 1024 out-of-order frames plus the missing cursor frame needed to release that buffer.

The TCP socket reader uses one reusable buffer with credit for up to 32 frames. The session copies slices into reliable frame storage before returning read credit. Outstanding reads reserve control space. If congestion reduces the window while a read is in flight, unscheduled output remains buffered until ACKs free space. Half-close is propagated through direct and HTTP CONNECT streams. No busy loop runs for idle socket reads.

These are standard independently implemented selective-repeat and congestion heuristics, not the upstream implementation. No throughput equivalence claim is made. Lost/reordered controls, NAT behavior, mobile apps and large multi-client deployments need real-network acceptance tests. UDP preserves complete datagrams up to 8192 bytes; larger input is explicitly counted and dropped. IPv4 fragmentation can still be affected by the network. No UDP retransmission or custom fragmentation extension is added. Truncated, malformed, fragmented SOCKS UDP and unexpected-source datagrams are dropped individually instead of closing the association.

The default ID window is 20000. Peer window values above the configured `max_window` are clamped. The pending packet channel is capped at 512 entries. The default total session cap is 256, with 32 TCP sessions (`max_sessions_per_user`) and 128 UDP destination flows (`max_udp_sessions_per_user`) per user. Existing configurations missing the new field acquire the UDP default. Session limits, parser limits, bounded zstd output and fixed data-frame sizes constrain peer-controlled memory. Reaching a queue bound drops a packet; TCP may retry, UDP cannot recover it automatically. UDP expiry/failure never emits the legacy ambiguous KICK that could close a TCP flow sharing the same ID. An incoming target-less KICK closes the authenticated user/source/ID group to preserve legacy client semantics.

## Persistence

One writer owns an exclusive flock. State snapshots are written to a 0600 temporary file, fsynced, renamed, then the directory is fsynced. Credits of 256 KiB are persisted before use. Periodic checkpoints save current consumed and outstanding credit only when state is dirty; idle flushes no longer rewrite unchanged state. On graceful shutdown outstanding unused credits are removed; on abrupt recovery they are consumed conservatively. No packet can spend more than the user's remaining quota. Errors in grants/checkpoints cause the core to fail closed until the service is restarted after the underlying storage problem is fixed. Encryption overhead is excluded from application-byte counters.

The JSON snapshot approach is designed for a small or medium installation. Every grant snapshots the state and holds the mutex through fsync; large user counts or slow storage will increase latency. This must be measured rather than described as zero-overhead. A future database or append-only ledger can preserve the same credit contract.

## Outbound behavior

- Direct TCP/UDP dial validated numeric destinations, preferring IPv4 on dual-stack targets. Direct TCP tries remaining validated addresses after failure within the dial timeout; UDP uses the preferred address and does not retry multiple addresses automatically.
- SOCKS5 supports CONNECT, optional username/password and UDP ASSOCIATE. The control stream is held open for UDP. Fragmented SOCKS UDP is rejected.
- HTTP supports CONNECT with optional Basic proxy authentication. Buffered bytes received after the CONNECT headers and TCP half-close are preserved.
- HTTP UDP fails explicitly; there is no fallback that bypasses the selected proxy.
- Targets resolving to private, loopback, link-local or several special ranges are denied by default. `allow_private_targets=true` is for trusted intentional use and local tests.
- Proxy hosts are administrator-controlled and may be local. Target DNS is resolved on the namedICMP server and sent numerically to proxies, preventing a proxy DNS re-resolution from changing the target. There is no remote-DNS option in this release. Special-range prefixes and SOCKS UDP headers are prepared outside the packet loop.
- Existing sessions retain their selected outbound snapshot; edits affect new sessions. Removing an assigned outbound is rejected.
- End-to-end UDP requires the consuming program, ICMP client and chosen outbound to support UDP. Wrapping a TCP-only Shadowsocks transport inside TCP cannot create a UDP path. The server does not infer UDP from TCP application bytes.

## Management and TLS

The administrator password is PBKDF2-HMAC-SHA256 with 210,000 iterations and a random salt. Web sessions live in memory for eight hours and require CSRF tokens on mutations. Login requests are size-limited, origin-checked, rate-limited and concurrency-limited. No forwarded header is trusted to mark a request as HTTPS.

A 0600 Unix socket accepts local administrative credential changes and aggregate `/diagnostics` reads. The service account and root can access it. Diagnostics expose separate TCP/UDP counters, dial errors, oversized UDP drops, version and storage health, without user secrets. Credential changes invalidate web sessions and leave tunnels running. The socket is required in normal service mode and can be disabled only in panel preview mode. Unix socket/raw socket behavior could not be exercised in the build environment, which denies them.

TLS certificate objects are cached in an atomic pointer. SIGHUP validates and swaps a complete new certificate. The Certbot deploy hook copies files before signaling; invalid reloads keep the prior certificate. Enabling TLS for the first time and changing panel bind/port require a restart. Plain HTTP is supported explicitly, including a public bind when selected during installation. Panel TLS and optional ICMP encryption are independent.

The manager has grouped setup/service/panel/maintenance actions and an explicit Update path. It verifies a prebuilt release or builds source before stopping the old service (tests are opt-in for source installs and required in CI), stages the candidate binary, backs up config/binary plus stopped-state data, and probes diagnostics after restart. Failed health validation restores the previous executable while keeping user state. An already stopped service remains stopped. Updating the binary reconnects tunnels once. Installer state transitions are tested with a mock service; actual VPS deployment remains an acceptance step.

## Reference boundary

Upstream repositories were read for protocol field definitions and externally visible behavior. They are not modules or executables in this project. Generic protobuf, RFC 792 echo, RFC 1928/1929 SOCKS and HTTP CONNECT formats necessarily agree on network bytes for interoperability. Similarity of that contract is not a copied implementation or a guarantee of full upstream feature support.

