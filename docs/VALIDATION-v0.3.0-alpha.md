# 0.3.0 alpha validation handoff

This working copy is based on main commit
`802dc1e5f5da30a752dbab46ec77bfaaefd3d58f`.

## Actually executed in this session

- `node --check internal/panel/web/app.js`: passed.
- `bash scripts/test_installer.sh`: passed (includes expected negative cases).
- `bash scripts/test_update.sh`: passed (includes rollback negative case).
- Original font blobs restored and checked against their Git object hashes.
- XML syntax of both project files, XAML files and the Windows manifest: passed.
- `node --check scripts/ui_smoke.cjs` and installer/build Bash syntax: passed.
- `node scripts/test_versions.cjs`: passed; server, client project, engine,
  manifest, release tags and release-note paths have independent, consistent versions.

## Added, but NOT executed here

- Go client/server in-memory ICMP integration tests for all four credential modes,
  TCP data integrity, dropped-frame retransmission, half-close, and UDP ASSOCIATE
  with multiple destinations.
- User total-usage regression and profile-validation tests.
- Windows build, Go tests, WPF theme/mascot render smoke test, PowerShell syntax
  test, dependency-signature check and release packaging workflow.

The local `go` executable is not the Go compiler; .NET SDK, PowerShell and a
Windows runtime are unavailable. The shell's HTTP proxy connection fails.
Therefore no Go/.NET compilation or Windows network run is claimed.

## GitHub publication blocker

Repository metadata reports the account has push permissions, but the installed
integration rejected creation of branch `namedp-v030` with HTTP 403,
`Resource not accessible by integration`. No branch, commit, tag or release was
created remotely. Write-capable GitHub integration authorization is required.
No credentials should be pasted into chat or committed to the repository.

After fixing access, push this work to `namedp-v030` to run checks without
publishing. Inspect both platform results and generated theme previews, fix any
failures, and complete real Windows/VPS acceptance.

Versions and publications are independent:
- Server/panel: `0.3.0-alpha`, tag `v0.3.0-alpha`, Linux archives and SHA256SUMS.
- Windows client: `0.1.0-alpha`, tag `namedp-v0.1.0-alpha`, archive
  `namedP-0.1.0-alpha-windows-x64.zip` and its own SHA256SUMS.

A main-branch commit containing `[release-v0.3.0-alpha]` publishes the server
after Linux validation. `[release-namedp-v0.1.0-alpha]` publishes the client
after its Linux and Windows jobs. These can be separate commits and releases.

## Required real-network acceptance

Use a non-production VPS and Windows 10/11 x64. Test numeric and all encrypted
profiles; HTTP(S) over TCP; DNS, UDP echo and QUIC; proxy and full VPN; DNS/IPv6
leaks; quota boundaries; wrong credentials and blocked ICMP; killed GUI/engine;
route/firewall restoration; adapter change; suspend/resume; and a throughput
comparison against the prior client. The app is unsigned and IPv4-only in VPN
mode. Existing more-specific LAN routes remain reachable. UDP larger than 8192
bytes is unsupported. This is not yet a validated production release.
