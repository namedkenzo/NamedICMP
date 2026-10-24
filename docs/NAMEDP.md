# namedP for Windows

namedP is the dedicated Windows client for namedICMP. Download
`namedP-0.1.0-alpha-windows-x64.zip` from the **namedP** release. Extract **the entire ZIP**
into a folder and launch `namedP.exe`. Windows requests Administrator permission
for the raw ICMP socket and network adapter. Windows 10/11 x64 is the initial target.
The .NET runtime is bundled. No Python, browser or upstream PingTunnel executable is required.

Paste a complete `pingtunnel://` link exported by the panel. The field masks your
credential and profiles are not saved to disk or passed in command-line arguments.
Select **Full-device VPN** or **Local proxy**, then Connect. The encrypted or numeric
credential is validated with an ICMP handshake before the GUI shows Connected.

- Full-device VPN sends IPv4 TCP and UDP through a Wintun adapter and tun2socks.
  A server /32 route stays on the physical gateway to avoid recursive tunneling.
  IPv6 is temporarily blocked rather than bypassing an IPv4-only tunnel. Physical
  interface DNS port 53 is blocked; the tunnel adapter uses 1.1.1.1 and 1.0.0.1.
  Existing more-specific LAN routes remain available; this is not a strict LAN kill switch.
- Local proxy opens SOCKS5 on `127.0.0.1:<lport>` with TCP CONNECT and UDP ASSOCIATE.
  Configure consuming apps to use it. HTTP proxy negotiation is not implemented;
  use SOCKS5. Server-side HTTP CONNECT outbounds are separate and still TCP-only.
- System theme is the initial default. Dark and Light override it. The companion
  breathes/blinks while idle, runs while connecting and wags its tail when connected.
  Windows' animation preference is respected. No polling animation assets or downloads.
- A separate PowerShell guardian removes only this session's routes/firewall rules
  on disconnect, engine failure or GUI exit. Original physical adapter DNS is not
  edited. Network changes require reconnecting. Sleep/resume is not seamless roaming.

The transport is original Go code sharing the server's independently written wire
codec and selective-repeat implementation. TCP and UDP use separate random IDs;
encrypted profiles omit the numeric password. TCP retries are ordered and deduplicated.
UDP has a bounded 8192-byte datagram limit and no retransmission. UDP availability
still depends on your server outbound, ICMP network and consuming application.

## Status and verification

The client version is **namedP 0.1.0-alpha**, independently versioned from
the namedICMP server (currently 0.3.0). Client tags use `namedp-v*`, while
server tags use `v*`. The client has a separate release and download archive.
This is not a claim of bug-free VPN operation. CI builds Windows
and Linux, tests protocol/profile behavior and renders both themes. A public-server
Windows test (TCP/UDP, DNS/IPv6 leak checks, throughput, forced crash cleanup,
reconnect and Windows Defender interaction) is required before recommending it
for sensitive use. The app is not Authenticode signed; the Wintun driver is signed
by its supplier. Release archives include SHA256 checksums and dependency licenses.

## Build

On Windows with Go 1.26+ and .NET SDK 8:

```
dotnet publish clients/namedP/namedP.csproj -c Release -r win-x64 --self-contained true -o dist/namedP
go build -o dist/namedP/namedp-engine.exe ./cmd/namedp-engine
go test ./internal/client ./internal/reliable ./internal/wire
dotnet run --project clients/namedP.Smoke -c Release
```

Bundle tun2socks v2.6.0, amd64 wintun.dll v0.14.1 and their licenses as in
`.github/workflows/namedp.yml`. Keep all files beside namedP.exe.

## فارسی

کل فایل ZIP را استخراج و `namedP.exe` را با دسترسی مدیر اجرا کنید. لینک کامل
پنل را بچسبانید و VPN کل سیستم یا پروکسی محلی را انتخاب کنید. حالت VPN ترافیک
TCP و UDP روی IPv4 را از تونل می‌فرستد و برای جلوگیری از نشت، IPv6 را موقتاً
مسدود می‌کند. مسیرهای محلی LAN باقی می‌مانند. حالت پروکسی فقط برنامه‌هایی را
پوشش می‌دهد که SOCKS5 آن را تنظیم کرده‌اند. تم اولیه از ویندوز گرفته می‌شود.
این نسخه آلفا است؛ آزمون واقعی اتصال، سرعت، DNS و پاک‌سازی پس از کرش روی
ویندوز هنوز باید انجام شود. بدون این آزمون‌ها ادعای «بدون باگ» نداریم.
