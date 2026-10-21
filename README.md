# namedICMP

**A self-hosted web panel and independent Go core for TCP/UDP tunneling over IPv4 ICMP.**

[فارسی](README.fa.md) · [Releases](https://github.com/namedkenzo/NamedICMP/releases) · [Windows clients](docs/WINDOWS.md)

Manage users, traffic allowances, expiry dates and outbound proxies from one English/Persian dashboard. The panel and tunnel core run as a single Linux service; no Node.js, database server or separate PingTunnel server is needed.

## Features

- Independent TCP and UDP sessions over ICMP, with per-user usage accounting.
- Enable/disable users, set expiry dates and traffic quotas, and export connection links.
- Choose **one credential per user**: a numeric access key **or** AES-128-GCM, AES-256-GCM or ChaCha20-Poly1305 with a generated encryption key.
- Direct connections, SOCKS5 outbounds (TCP/UDP), and HTTP CONNECT outbounds (TCP).
- English/Persian interface, dark/light themes, server metrics and tunnel diagnostics.
- systemd management, optional HTTPS with Certbot, and updates with backups and binary rollback on failed health checks.

The server implements the PingTunnel wire format independently. It does not bundle a PingTunnel executable. Client capabilities and versions still determine end-to-end compatibility.

## Install

Use a **Debian/Ubuntu server with systemd**, root access, public IPv4 connectivity and Linux **amd64 or arm64**. Allow IPv4 ICMP and your chosen panel TCP port in the server/provider firewall.

Run in an interactive root shell:

```bash
curl -fsSL https://raw.githubusercontent.com/namedkenzo/NamedICMP/main/install.sh -o /tmp/namedicmp-install.sh
bash /tmp/namedicmp-install.sh
```

Select **1 — Install**, then enter the panel port, public server address and administrator credentials. The repository is already set to `namedkenzo/NamedICMP`; there is no repository prompt.

Remote installations prefer the matching release archive and verify its SHA-256 checksum and executable version. This avoids downloading Go and compiling on the server. If release downloads are unavailable, automatic mode builds from source and explains the fallback. Invalid checksums stop installation. Installation time depends on the network and APT; no fixed timing is promised.

The installer installs missing OS dependencies only. It disables ordinary kernel echo replies to prevent duplicate tunnel responses; removal restores the recorded setting when applicable. The selected HTTPS/HTTP address is displayed after setup. For HTTPS, the domain must resolve to the server and TCP port 80 must be free and reachable for Certbot. For localhost HTTP access, use the SSH tunnel printed by the installer.

Reopen the manager:

```bash
namedicmp
```

## Update

Run the **new installer from the command above**, then select **2 — Update**. This is also the upgrade path from an older installed manager. Users, usage, quotas, administrator credentials and TLS settings are preserved. Updating restarts active tunnels once; a failed health check restores the previous binary. Backups are stored in `/var/lib/namedicmp/backups`.

**Upgrading encrypted users from 0.2.1:** copy/import their connection profiles again. Encrypted connections now omit the numeric password. Old commands combining `-key` and encryption must be replaced. Numeric-only profiles remain valid.

## Connect a client

Create a user in **Users**, select the credential mode, then open **Config** and copy the connection link or CLI command.

| User mode | Client settings |
| --- | --- |
| Numeric key (`none`) | Server + numeric key; encryption off |
| Encrypted | Server + encryption mode + Base64 encryption key; no numeric password |

Changing the mode replaces the active credential method and reconnects that user's sessions. Encryption-only users have an internal routing ID in server storage; it is not a client password and is never exported as one. Numeric keys alone do **not** encrypt traffic. Panel HTTPS and tunnel encryption are separate settings.

- **Desktop CLI:** [upstream PingTunnel releases](https://github.com/esrrhs/pingtunnel/releases). Windows builds exist; run the client as Administrator. Encryption requires a build exposing `-encrypt` and `-encrypt-key`.
- **Android / desktop GUI:** [community PingTunnel Client](https://github.com/itismoej/pingtunnel-client/releases). Its encryption mode uses the encryption key instead of the numeric key.
- **iOS:** [Ping Tunnel](https://apps.apple.com/us/app/ping-tunnel-vpn-over-icmp/id6748279683). Check the installed app's supported credential modes before choosing encryption.

For Windows diagnosis, CLI examples, and the scope of a possible namedICMP client, see [Windows guide](docs/WINDOWS.md). SOCKS proxy mode affects apps configured to use that proxy; it does not automatically route the whole device. UDP also requires support from the client, consuming app and outbound proxy. HTTP CONNECT outbounds cannot carry UDP.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| Installation builds slowly | The matching release assets may be missing or unreachable. Publish both Linux archives and `SHA256SUMS`; source fallback is slower. |
| Download/checksum failure | Check access to GitHub release assets; retry after fixing the download. Do not bypass checksum verification. |
| Panel unavailable | Manager **Status** and **Recent logs**, panel port conflicts, bind address and firewall rules. |
| Certbot fails | Public DNS, inbound TCP 80, and another service already using port 80. Retry **SSL certificate** after correcting the cause. |
| Client cannot connect | Matching credential mode, client administrator/raw-socket permissions, IPv4 ICMP connectivity and cloud firewall. |
| TCP works but UDP fails | Check client UDP support and SOCKS5 UDP ASSOCIATE support; HTTP outbounds support TCP only. |

Configuration: `/etc/namedicmp/config.json`. User state: `/var/lib/namedicmp/state.json`. Avoid running two server instances against the same state file. Useful commands:

```bash
systemctl status namedicmp --no-pager
journalctl -u namedicmp -n 60 --no-pager
/opt/namedicmp/namedicmp diagnostics
```

## Build and release

Go **1.26+** is required for source builds. Node.js is only used for optional development UI tests.

```bash
go test -race ./...
go vet ./...
bash scripts/test_installer.sh
bash scripts/test_update.sh
bash scripts/build.sh
```

Archives appear in `dist/`. Local checkout installs build that checkout. Set `NAMEDICMP_INSTALL_MODE=source` to force a source build, `binary` to require release assets, or leave the default `auto`. Source installation can additionally run tests with `NAMEDICMP_RUN_TESTS=1`. `NAMEDICMP_REPO` and `NAMEDICMP_REF` are optional developer overrides; custom refs build from source in automatic mode.

The tag workflow tests and builds both architectures, then creates a **draft prerelease** with archives and checksums. Review and publish it to make fast downloads publicly available. See [release notes and checklist](docs/RELEASE-v0.2.2-alpha.md), [architecture](docs/ARCHITECTURE.md) and [security](SECURITY.md).

## Support the project

- **TRX (Tron):** `TYbWTzdjVM3HpqkGiUX7GGV2XgK6QDJQis`
- **TON:** `UQDPGgQWQb9j8pxC5_RIh2HbuTJjaIYSga7FXPLZWbyzTZJN`

[MIT License](LICENSE) · [Third-party notices](THIRD_PARTY_NOTICES.md)
