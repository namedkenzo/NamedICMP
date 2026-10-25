# namedICMP 0.3.0 alpha — Web management panel

## Separate Windows companion
namedP is released independently as **0.1.0-alpha**, under tag
`namedp-v0.1.0-alpha`, with its own Windows archive and release notes. This
panel release contains only the Linux panel packages and SHA256SUMS.
Installing or updating the panel does not install or run the Windows client.

## Panel fixes and maintenance
- Restore the missing total usage field in user responses (upload + download + crash recovery).
- Follow the system color preference until an explicit panel theme is saved.
- Avoid full user-list copies for dashboard user counts.
- Share the tested selective-repeat module between server and client without importing Linux-only state code into Windows.

## Upgrade
Use the manager's Update option. Existing users, credentials and configuration stay
compatible. Encrypted profiles use encryption credentials only.

## Validation and limits
Server publication is gated on Linux tests/build. Client publication separately
requires its Linux and Windows jobs. Windows network acceptance is still pending
for the companion client; see its separate release notes. HTTP outbounds cannot
carry UDP. No speed improvement is claimed without benchmarks.

## فارسی
- کلاینت namedP با نسخه مستقل 0.1.0-alpha در ریلیز جداگانه منتشر می‌شود.
- اصلاح نمایش مصرف کل کاربران در پنل و پیش‌فرض تم مطابق سیستم.
- حفاظت مسیر سرور، کنترل موقت DNS و IPv6 و پاک‌سازی تنظیمات هنگام قطع.
- نسخه آلفا؛ آزمون شبکه واقعی ویندوز و مقایسه سرعت هنوز تکمیل نشده است.
