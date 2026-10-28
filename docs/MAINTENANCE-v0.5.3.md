# v0.5.3: maintenance findings and validation scope

This update follows the v0.5.2 load/admission investigation. It improves the web panel's managed ICMP core without changing client applications, connection links or default limits.

## Findings and design decisions

### 1. A connected TCP socket is not a completed proxy handshake

Previously, proxy negotiation used a fixed ten-second socket deadline after `DialContext` returned. Canceling the caller, or giving it a shorter deadline, did not interrupt a blocked SOCKS/HTTP exchange promptly. Go documents that after a successful dial, cancellation no longer affects the returned connection.

The new handshake scope combines the caller context with a ten-second ceiling, applies the earlier socket deadline, and closes the socket on cancellation. The cancellation callback is stopped before a successful socket handoff. If it has already started, the caller waits for that callback before returning, avoiding a late close of an apparently successful tunnel. SOCKS UDP relay name resolution and dialing use this context too.

References: [Go Dialer.DialContext](https://pkg.go.dev/net#Dialer.DialContext), [context.AfterFunc semantics](https://pkg.go.dev/context#AfterFunc), [connection synchronization example](https://pkg.go.dev/context#example-AfterFunc-Connection). The implementation is independently authored.

### 2. An ACK can make a queued retransmission unnecessary

The original packet can be acknowledged while its retransmission waits in the bounded writer queue. Previously the ticket would still consume a socket write and paced capacity. Similarly, a stream could exit with tickets still pending in that queue.

An atomic obsolete marker now allows the stream owner to invalidate these tickets. The writer checks before pacing and immediately before writing. Reliable state remains owned by the stream goroutine; the writer never iterates its maps. In-flight writes cannot be recalled. Successful ACK accounting is unchanged and stale queued retries do not become payload loss. The separate `obsolete_queued_packets` counter represents avoided work, not network loss.

Queue bounds, fair scheduling and rate limits remain in place. Raising or removing them is not a substitute for handling overload. [RFC 8085, section 3.1](https://www.rfc-editor.org/rfc/rfc8085.html#section-3.1) discusses congestion control and burst management for datagram applications; it does not specify a universal ICMP tunnel speed cap. The current pacer is not claimed to provide adaptive end-to-end congestion control.

### 3. HTTP CONNECT success is a status class

Only `200` was accepted. [RFC 9110, section 9.3.6](https://www.rfc-editor.org/rfc/rfc9110.html#section-9.3.6) defines a successful CONNECT response as any `2xx`: the connection becomes a tunnel immediately after the response headers. The parser now accepts valid three-digit statuses from 200 through 299 and keeps buffered tunnel bytes. It still enforces bounded headers and rejects non-success or malformed statuses. This does not add UDP support to HTTP CONNECT.

## Reproducible checks

- `go test -race ./...` and `go vet ./...`.
- `TestProxyHandshakeCancellation`: actual local TCP sockets, cancellation during HTTP headers, SOCKS method negotiation, TCP CONNECT and UDP ASSOCIATE.
- `TestProxyDeadlineAndSuccessfulHandoff`: caller deadline and no late cancellation of a successful connection.
- `TestHTTPConnectSuccessStatuses`: 200/201/204/299 accepted with exact buffered payload; 300/407 and malformed statuses rejected.
- `TestAcknowledgedRetryDoesNotReachSocket`: block the writer, queue a retry, acknowledge the original, release the writer, and verify only the probe and subsequent live data were written. Payload accounting must remain exact.
- `TestClosedStreamCancelsQueuedDispatch`: a closed stream marks its pending ticket obsolete.
- CI runs the new public proxy regression tests against immutable v0.5.2 commit `5ba2d83da75dd587df358daade696187a15b52d6`, requiring both old cancellation and CONNECT-status failures. The helper file is copied only so the suite compiles; old public dial paths do not use it.
- Existing paced/lossy download, IPv4/IPv6 loopback, installer/update/uninstall, UI, version consistency and fuzz checks remain enabled.

## What these checks do not establish

Local tests are not a benchmark of the user's mobile carrier or VPS. They do not prove that provider filtering, an MTU mismatch, a capacity ceiling or a client-specific failure is absent. No Mbps, minimum ping, or universal Speedtest success claim is made. Use the before/after comparison in [SPEEDTEST-STABILITY.md](SPEEDTEST-STABILITY.md) for deployment acceptance. Existing aggregate `dropped` and `write_queue_dropped` values are not measured path packet-loss percentages.
