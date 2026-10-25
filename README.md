# NamedICMP

**A clean control panel and independent server for TCP/UDP tunneling over ICMP.**

[![Latest release](https://img.shields.io/github/v/release/namedkenzo/NamedICMP?color=2ea44f)](https://github.com/namedkenzo/NamedICMP/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)
[![Platform](https://img.shields.io/badge/Linux-amd64%20%7C%20arm64-555555?logo=linux&logoColor=white)](#quick-install)

[فارسی](README.fa.md) · [Server releases](https://github.com/namedkenzo/NamedICMP/releases) · [Client apps → NamedClients](https://github.com/namedkenzo/NamedClients) · [Architecture](docs/ARCHITECTURE.md)

## What lives here?

**This repository is the web panel and Linux tunnel server.** The panel and independent Go core run as one systemd service, with no Node.js runtime, external database server or separate PingTunnel executable.

**Windows and Android client apps belong in [NamedClients](https://github.com/namedkenzo/NamedClients).** Client versions and downloads are managed there independently. The initial Android app is available as a debug-signed development APK; it remains alpha.

## Features

- **Users:** separate credentials and traffic accounting, quotas, expiry dates and live enable/disable.
- **Transport:** independent TCP/UDP sessions over IPv4 ICMP; direct, SOCKS5 and HTTP CONNECT outbounds.
- **Dashboard:** English/Persian, system-aware dark/light themes, server metrics and tunnel diagnostics.
- **Management:** interactive installer, optional Certbot HTTPS, backups, health checks and update rollback.
- **Credentials:** numeric key without encryption, or AES-128-GCM / AES-256-GCM / ChaCha20-Poly1305 with an encryption key.

## Quick install

Use **Debian/Ubuntu with systemd**, root access and public IPv4. Linux **amd64 and arm64** are supported. Allow ICMP and the chosen panel TCP port in your host/provider firewall.

```bash
curl -fsSL https://raw.githubusercontent.com/namedkenzo/NamedICMP/main/install.sh -o /tmp/namedicmp-install.sh
bash /tmp/namedicmp-install.sh
```

Choose **1 — Install** and enter the panel port, server address and administrator credentials. The repository is preset; no repository-name prompt is needed.

The installer selects the exact Linux release for the server architecture, verifies SHA-256 and the executable version, and installs **only the server and panel**. It never downloads the Windows ZIP/EXE or an Android APK. If binaries are unavailable, automatic mode can build the selected server source.

HTTPS uses Certbot; your domain must resolve to the server and TCP 80 must be free and reachable. HTTP can bind publicly or use localhost with the printed SSH-tunnel command. Panel HTTPS and tunnel encryption are independent.

Reopen management with `namedicmp`. To upgrade, run the latest installer above and choose **2 — Update**. Users, usage, credentials, quotas and TLS settings are preserved; active tunnels reconnect once. Failed health checks restore the previous binary.

## Choose a client

Create a user in **Users**, open **Config**, then copy the complete connection URI.

| Client | Where to get it | Notes |
| --- | --- | --- |
| **namedP — Windows** | [NamedClients](https://github.com/namedkenzo/NamedClients) · [Downloads](https://github.com/namedkenzo/NamedClients/releases) | Our dedicated app; VPN and SOCKS5 modes; currently alpha |
| **namedP — Android** | [Android guide](https://github.com/namedkenzo/NamedClients/tree/main/android) · [Development APK](https://github.com/namedkenzo/NamedClients/releases/tag/android-v0.1.0-alpha) | Initial alpha; debug-signed; real-phone/VPS acceptance pending |
| **Original PingTunnel CLI** | [esrrhs/pingtunnel](https://github.com/esrrhs/pingtunnel) · [Releases](https://github.com/esrrhs/pingtunnel/releases) | Upstream command-line client; choose the matching OS build |
| **Community PingTunnel Client** | [itismoej/pingtunnel-client](https://github.com/itismoej/pingtunnel-client) · [Releases](https://github.com/itismoej/pingtunnel-client/releases) | Android/desktop GUI; linked from the upstream PingTunnel project |
| **Ping Tunnel — iOS** | [App Store](https://apps.apple.com/us/app/ping-tunnel-vpn-over-icmp/id6748279683) | Check the installed version's supported modes |

Upstream/community clients are separate projects. Compatibility depends on the client version and features; encryption requires the selected algorithm and raw Base64-key support. [Windows troubleshooting](docs/WINDOWS.md).

| User mode | Client credential |
| --- | --- |
| Numeric / no encryption | Server + numeric key; encryption off |
| Encrypted | Server + algorithm + Base64 encryption key; **no numeric password** |

Changing credential mode reconnects that user's sessions. Numeric keys do not encrypt traffic. Encrypted users' internal routing IDs are never client passwords. Old 0.2.1 profiles combining numeric and encryption credentials must be exported again.

**UDP requires support from the client, application and outbound. HTTP CONNECT outbounds are TCP-only.** A SOCKS proxy only covers apps configured to use it; full-device VPN is a client-side feature.

## Operations & troubleshooting

Install/Update now apply reversible network tuning and the core paces outgoing ICMP bursts. [Settings, restore and provider limitations](docs/NETWORK-TUNING.md). These cannot guarantee protection from upstream/provider anti-DDoS shutdowns.

Upgrading from 0.3.0? Run the current installer command above and choose **2 · Update**. The older installed manager cannot install the new tuning helper itself. The current manager preserves users/settings and applies the profile automatically; active tunnels reconnect once.

```bash
systemctl status namedicmp --no-pager
journalctl -u namedicmp -n 60 --no-pager
/opt/namedicmp/namedicmp diagnostics
```

| Symptom | Check |
| --- | --- |
| Slow source build | Matching Linux release assets and SHA256SUMS must be available; source fallback is slower |
| Panel unavailable | Bind address, chosen port, conflicts, firewall and Recent logs |
| Certbot failure | Public DNS, free/reachable TCP 80 and existing web services |
| Client cannot connect | Credentials, ICMP reachability, client permissions and provider firewall |
| TCP works; UDP fails | Application/client UDP support and SOCKS5 UDP ASSOCIATE; HTTP outbounds cannot carry UDP |

Configuration: `/etc/namedicmp/config.json` · State: `/var/lib/namedicmp/state.json` · Backups: `/var/lib/namedicmp/backups`. Do not run two instances against the same state file. The installer records/restores kernel echo-reply suppression where applicable. Never bypass checksum validation.

## Development

Go **1.26+**; Node.js is only needed for optional panel DOM tests.

```bash
go test -race ./...
go vet ./...
node scripts/test_versions.cjs
bash scripts/test_installer.sh
bash scripts/test_update.sh
bash scripts/build.sh
```

Linux archives are written to `dist/`. Use `NAMEDICMP_INSTALL_MODE=source`, `binary` or the default `auto`. Source tests can be enabled with `NAMEDICMP_RUN_TESTS=1`; repository/ref overrides are for development. Windows/Android build workflows are maintained in **NamedClients**, not here.

[Security](SECURITY.md) · [Server release notes](docs/RELEASE-v0.3.1.md) · [Architecture](docs/ARCHITECTURE.md) · [Third-party notices](THIRD_PARTY_NOTICES.md)

## Support

- **TRX (Tron):** `TYbWTzdjVM3HpqkGiUX7GGV2XgK6QDJQis`
- **TON:** `UQDPGgQWQb9j8pxC5_RIh2HbuTJjaIYSga7FXPLZWbyzTZJN`

[MIT License](LICENSE)
