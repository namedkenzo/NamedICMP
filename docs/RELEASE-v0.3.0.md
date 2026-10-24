# namedICMP 0.3.0 — Server and web panel

Stable publication of the server and web panel, promoted from 0.3.0-alpha.
The release is not a prerelease and is marked **Latest** on GitHub.

## Included

- Independent Go ICMP server for TCP/UDP and the English/Persian management panel.
- Correct user total-usage display (upload, download and recovery usage).
- System-aware default theme and explicit light/dark overrides.
- Dashboard user counts without copying every user and credential.
- Existing numeric/encryption-only credentials, quotas and outbounds remain compatible.
- Linux amd64 and arm64 server archives plus SHA256SUMS. No Windows executables.

The installer downloads the exact `v0.3.0` Linux asset for the server architecture,
verifies its checksum and executable version, and installs only the server/panel.
Use **Update** in the manager to preserve users, quotas, credentials and TLS.

## Separate client

The Windows companion **namedP remains 0.1.0-alpha**, under its own tag
`namedp-v0.1.0-alpha`, release notes and Windows ZIP. This server release does
not install the Windows client or promote its maturity status.

## Validation

Server publication requires Go race tests and vet, installer/update regression
checks, version consistency, panel DOM checks and both Linux builds. This version
changes release/version metadata rather than transport behavior. It adds no new
claim of throughput, bug-free operation or completed real-network Windows testing.
Existing protocol/security limitations remain documented in SECURITY.md.

## فارسی

پنل و هسته سرور با نسخه پایدار **0.3.0** و برچسب **Latest** منتشر می‌شوند؛
این ریلیز دیگر Pre-release نیست. اصلاح نمایش مصرف کاربران، تم مطابق سیستم
و بهینه‌سازی شمارش کاربران از نسخه قبلی حفظ شده‌اند. نصب‌کننده فقط برنامه
سرور لینوکس را دریافت و نصب می‌کند. کلاینت ویندوز namedP با نسخه مستقل
**0.1.0-alpha** در ریلیز جداگانه باقی می‌ماند.
