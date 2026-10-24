# namedP 0.1.0 alpha — Windows client

First independent-version release of namedP. This client release does not
install, replace or update the namedICMP web panel or server core.

- Native Windows x64 desktop UI; system theme by default, plus dark/light overrides.
- Original animated dachshund: breathing/blinking idle, running while connecting,
  happy tail wag when connected. Respects Windows animation preferences.
- Paste an exported pingtunnel:// link; credentials stay masked and are not saved.
- Full-device IPv4 VPN for TCP/UDP or local SOCKS5 proxy mode.
- Independent Go transport, authenticated ICMP probe and separate TCP/UDP flows.
- Wintun routing with server bypass, temporary DNS/IPv6 guards and cleanup guardian.

Download `namedP-0.1.0-alpha-windows-x64.zip`, verify SHA256SUMS, extract the
whole archive and run namedP.exe as Administrator. The .NET runtime is included.
Windows 10/11 x64 is the initial target. No PingTunnel executable is bundled.

## Alpha limitations

The release workflow requires build/protocol/UI checks to pass before publishing.
It does not replace real Windows/VPS network acceptance. Throughput, DNS/IPv6
leak checks, forced-crash cleanup and sleep/resume testing remain pending.
The app is unsigned. VPN is IPv4-only with temporary IPv6 blocking; existing
more-specific LAN routes remain. UDP requires a compatible server outbound.

## فارسی

اولین نسخه کلاینت ویندوز namedP با شماره مستقل `0.1.0-alpha`؛ نسخه پنل
و سرور تغییری با نصب این کلاینت نمی‌کند. ZIP را کامل استخراج کنید و برنامه
را با دسترسی مدیر اجرا کنید. VPN کل سیستم روی IPv4 و پروکسی SOCKS5، تم
خودکار سیستم/روشن/تیره و داچهوند متحرک اضافه شده‌اند. این نسخه آلفا است
و آزمون واقعی شبکه ویندوز و VPS هنوز باید تکمیل شود.
