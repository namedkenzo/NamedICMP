# Windows clients and diagnosis

**namedP is maintained in [NamedClients](https://github.com/namedkenzo/NamedClients).** Download and build client applications there. This page covers connecting third-party clients to the namedICMP server.


## Existing clients

There is already a standalone Windows CLI in [esrrhs/pingtunnel releases](https://github.com/esrrhs/pingtunnel/releases). The [community Flutter GUI](https://github.com/itismoej/pingtunnel-client/releases) is a separate desktop application built around a PingTunnel executable and, in VPN mode, tun2socks.

Run the chosen Windows executable from an **Administrator PowerShell**. Start with a local SOCKS proxy to isolate transport/credentials from VPN adapter and routing issues. Use a client version whose `-h` output includes the encryption flags if you choose encryption.

Numeric-only user (replace placeholders):

```powershell
.\pingtunnel.exe -type client -l 127.0.0.1:1080 -s SERVER_IPV4 -sock5 1 -key NUMERIC_KEY -tcp_gz 0
```

Encryption-only user:

```powershell
.\pingtunnel.exe -type client -l 127.0.0.1:1080 -s SERVER_IPV4 -sock5 1 -encrypt aes256 -encrypt-key 'BASE64_KEY_FROM_PANEL' -tcp_gz 0
```

Use the exact algorithm selected in the panel. Do not add `-key` to the encrypted command. The protocol default of zero is not a second password.

In another terminal, test an application through SOCKS:

```powershell
curl.exe --proxy socks5h://127.0.0.1:1080 https://example.com
```

This checks a TCP application path only. It does not verify UDP or system-wide VPN routing. PingTunnel's SOCKS flag is spelled `-sock5`.

## Confirmed server incompatibility fixed in 0.2.2

The inspected GUI runner uses **either** `-encrypt` plus `-encrypt-key`, **or** `-key`. In encrypted mode it leaves the numeric key at the CLI default, zero. namedICMP 0.2.1 instead required its generated positive numeric key inside every encrypted packet. That mismatch prevented an encryption-only GUI profile from authenticating.

0.2.2 authenticates the AEAD envelope first and maps it to the user's private routing ID. Encrypted request/response envelopes use zero; positive numeric credentials are accepted only with encryption off. The API and exported links no longer expose the routing ID as an encrypted user's password. Old combined numeric/encrypted commands must be re-exported.

This is a server compatibility fix. It does not establish the cause of every Windows connection failure.

## Separate transport from VPN failures

| Observation | Next check |
| --- | --- |
| Client exits or cannot open ICMP socket | Administrator privileges, executable architecture, and endpoint firewall/security logs. |
| `-encrypt` is unknown | The bundled executable is too old or lacks encryption support; use a compatible build. |
| No successful tunnel probes | Public IPv4 address, selected credential method, and IPv4 ICMP rules on both endpoints/provider. Ordinary OS ping is not decisive because the server installer suppresses ordinary echo replies. |
| SOCKS works but VPN does not | Wintun availability, adapter creation, route/DNS setup, and privileges in the GUI. |
| Tunnel disconnects after VPN starts | Check that traffic to the tunnel server still uses the physical interface. A default tunnel route without a physical route to the server can create a loop. This is a diagnostic possibility, not a reproduced fault on the user's PC. |
| TCP works but UDP does not | Client UDP support, consuming application's proxy mode, and SOCKS5 outbound UDP ASSOCIATE. HTTP CONNECT cannot forward UDP. |

The inspected GUI VPN path invokes tun2socks with a Wintun device and PowerShell scripts that change the default route and DNS. Its Windows route script does not itself add a physical host route for the tunnel server. Whether that causes a problem depends on the full route table and interface binding; it needs a real Windows test. Keep the server's panel/service logs and client error text when diagnosing, and remove credentials before sharing them.

## Dedicated namedP client

The dedicated Windows UI, engine and VPN guardian have moved to [NamedClients](https://github.com/namedkenzo/NamedClients). Its code and releases are separate from this server repository. namedP remains alpha and requires real-network Windows acceptance; the server's stable status does not promote the client to stable.

## Inspected primary sources

- [GUI runner](https://github.com/itismoej/pingtunnel-client/blob/cdff269f3ace289ef4bb7a8defa5967993dc2c33/templates/lib/src/desktop_runner.dart): mutually exclusive credential flags, Wintun and interface selection.
- [GUI URI parser](https://github.com/itismoej/pingtunnel-client/blob/cdff269f3ace289ef4bb7a8defa5967993dc2c33/templates/lib/src/config.dart): `encrypt`, `encrypt_key`, URL query decoding.
- [GUI Windows route setup](https://github.com/itismoej/pingtunnel-client/blob/cdff269f3ace289ef4bb7a8defa5967993dc2c33/templates/assets/scripts/windows/vpn_up.ps1).
- [CLI flag definitions](https://github.com/esrrhs/pingtunnel/blob/df16e52771ee67f1f9770ec68763b425672a36fc/cmd/main.go): numeric key defaults to zero, encryption flags.
- [CLI crypto framing](https://github.com/esrrhs/pingtunnel/blob/df16e52771ee67f1f9770ec68763b425672a36fc/crypto.go): raw Base64 keys and AEAD nonce/ciphertext layout.

## خلاصهٔ فارسی

کلاینت خط فرمان مستقل برای ویندوز موجود است. ابتدا آن را با **Run as Administrator** و حالت SOCKS امتحان کنید. برای کاربر رمزنگاری‌شده فقط الگوریتم و کلید رمزنگاری را وارد کنید و رمز عددی نگذارید. در نسخهٔ 0.2.2 لینک اتصال این اطلاعات را کامل دارد؛ کانفیگ‌های رمزنگاری قدیمی را دوباره وارد کنید.

اگر SOCKS کار کرد ولی VPN کار نکرد، بررسی Wintun، مجوز مدیر، DNS و مسیر شبکه لازم است. اصلاح این ریلیز ناسازگاری احراز هویت سرور را برطرف می‌کند؛ تست واقعی روی ویندوز انجام نشده است. ساخت رابط سادهٔ اختصاصی با واردکردن لینک، اتصال/قطع و نمایش لاگ ممکن است؛ VPN کل سیستم کار و تست جداگانه می‌خواهد.

