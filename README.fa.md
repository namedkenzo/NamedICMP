# NamedICMP

**وب‌پنل مدیریت VPN over ICMP با مدیریت کاربران و پشتیبانی از TCP و UDP.**

[![آخرین نسخه](https://img.shields.io/github/v/release/namedkenzo/NamedICMP?color=2ea44f)](https://github.com/namedkenzo/NamedICMP/releases/latest)
[![مجوز MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

[English](README.md) · [ریلیزهای پنل](https://github.com/namedkenzo/NamedICMP/releases) · [کلاینت‌ها ← NamedClients](https://github.com/namedkenzo/NamedClients)

## این مخزن برای چیست؟

این مخزن مخصوص **وب‌پنل مدیریت VPN over ICMP** است: مدیریت کاربران، حجم و زمان، کانفیگ‌ها و خروجی‌ها. هستهٔ تونلِ مدیریت‌شدهٔ پنل همراه آن در یک سرویس systemd اجرا می‌شود و به اجرای جداگانهٔ PingTunnel، Node.js یا دیتابیس خارجی نیاز ندارد.

کلاینت‌های ویندوز و اندروید در مخزن مستقل **[NamedClients](https://github.com/namedkenzo/NamedClients)** نگهداری می‌شوند. نسخه و دانلود کلاینت‌ها مستقل از پنل است. نسخهٔ اولیهٔ اندروید namedP به‌صورت APK آزمایشی با امضای توسعه موجود است و همچنان آلفاست.

## امکانات

- **تنظیمات اتصال:** انتخاب IPv4/IPv6، آی‌پی یا دامنه، تشخیص آدرس و روش پیشرفتهٔ دریافت بسته. خروجی‌ها تب مستقل در تنظیمات دارند و IPهای خانه تا لمس مخفی هستند. [راهنمای IPv6 و Raw](docs/IPV6-READERS.md).

- **کاربران:** کلید و مصرف مستقل، محدودیت حجم، تاریخ انقضا و فعال/غیرفعال‌سازی زنده.
- **تونل:** جریان‌های جداگانهٔ TCP و UDP روی ICMPv4 یا ICMPv6.
- **خروجی‌ها:** مستقیم، SOCKS5 برای TCP/UDP و HTTP CONNECT برای TCP.
- **داشبورد:** فارسی/انگلیسی، تم روشن/تیره مطابق سیستم و نمایش وضعیت سرور و تونل‌ها.
- **مدیریت:** نصب تعاملی، HTTPS اختیاری با Certbot، پشتیبان‌گیری و بازگردانی در صورت شکست آپدیت.
- **احراز هویت:** رمز عددی بدون رمزنگاری، یا AES-128-GCM، AES-256-GCM و ChaCha20-Poly1305 با کلید رمزنگاری.

## نصب سریع

سرور **Debian/Ubuntu با systemd**، دسترسی root، IPv4 عمومی یا IPv6 مسیریابی‌شده و معماری **amd64 یا arm64** لازم است. ICMP و پورت TCP پنل را در فایروال سرور و ارائه‌دهنده باز کنید.

```bash
curl -fsSL https://raw.githubusercontent.com/namedkenzo/NamedICMP/main/install.sh -o /tmp/namedicmp-install.sh
bash /tmp/namedicmp-install.sh
```

گزینهٔ **۱ — Install** را انتخاب کنید و خانوادهٔ IPv4/IPv6، آدرس خودکار یا آی‌پی/دامنهٔ دلخواه، روش دریافت بسته، پورت پنل و مشخصات مدیر را انتخاب کنید. نام مخزن از قبل تنظیم شده و پرسیده نمی‌شود.

نصب‌کننده فایل مخصوص **سرور لینوکس** را مطابق معماری می‌گیرد و checksum و نسخهٔ فایل را بررسی می‌کند. **ZIP یا EXE ویندوز و APK اندروید را دریافت یا نصب نمی‌کند.** اگر فایل آماده موجود نباشد، حالت خودکار می‌تواند هستهٔ مدیریت‌شدهٔ پنل را از سورس بسازد.

برای HTTPS، دامنه باید به سرور اشاره کند و پورت ۸۰ آزاد و قابل دسترس باشد. HTTP می‌تواند عمومی باشد یا با localhost و SSH tunnel استفاده شود. HTTPS پنل و رمزنگاری تونل دو تنظیم مستقل‌اند.

برای بازکردن دوبارهٔ منوی مدیریت، دستور `namedicmp` را اجرا کنید. برای آپدیت، نصب‌کنندهٔ جدید بالا را بگیرید و **۲ — Update** را بزنید؛ کاربران، مصرف، حجم، مشخصات مدیر و TLS حفظ می‌شوند. اتصال‌های فعال یک بار دوباره برقرار می‌شوند و در صورت شکست بررسی سلامت، فایل اجرایی قبلی برمی‌گردد.

## انتخاب کلاینت

در بخش **Users** کاربر بسازید، **Config** را باز کنید و لینک کامل اتصال را کپی کنید.

| کلاینت | دریافت | توضیح |
| --- | --- | --- |
| **namedP ویندوز** | [NamedClients](https://github.com/namedkenzo/NamedClients) · [دانلود](https://github.com/namedkenzo/NamedClients/releases) | کلاینت اختصاصی با VPN و SOCKS5؛ فعلاً آلفا |
| **namedP اندروید** | [راهنما](https://github.com/namedkenzo/NamedClients/tree/main/android) · [APK آزمایشی](https://github.com/namedkenzo/NamedClients/releases/tag/android-v0.2.0-alpha) | آلفا با امضای توسعه؛ نیازمند تست گوشی و سرور واقعی |
| **خط فرمان اصلی PingTunnel** | [esrrhs/pingtunnel](https://github.com/esrrhs/pingtunnel) · [Releases](https://github.com/esrrhs/pingtunnel/releases) | کلاینت اصلی؛ فایل مناسب سیستم‌عامل را انتخاب کنید |
| **رابط گرافیکی جامعهٔ PingTunnel** | [itismoej/pingtunnel-client](https://github.com/itismoej/pingtunnel-client) · [Releases](https://github.com/itismoej/pingtunnel-client/releases) | اندروید و دسکتاپ؛ در پروژهٔ اصلی PingTunnel نیز معرفی شده است |
| **Ping Tunnel برای iOS** | [App Store](https://apps.apple.com/us/app/ping-tunnel-vpn-over-icmp/id6748279683) | پشتیبانی حالت‌های اتصال را در نسخهٔ نصب‌شده بررسی کنید |

این کلاینت‌های اصلی/جامعه، پروژه‌هایی مستقل‌اند. سازگاری به نسخه و قابلیت‌های کلاینت بستگی دارد؛ برای رمزنگاری، الگوریتم انتخاب‌شده و کلید خام Base64 باید پشتیبانی شوند. [راهنمای عیب‌یابی ویندوز](docs/WINDOWS.md).

| حالت کاربر | اطلاعات اتصال |
| --- | --- |
| رمز عددی، بدون رمزنگاری | آدرس سرور و رمز عددی؛ رمزنگاری خاموش |
| رمزنگاری‌شده | آدرس سرور، الگوریتم و کلید Base64؛ **بدون رمز عددی** |

تغییر حالت احراز هویت، اتصال‌های همان کاربر را بازنشانی می‌کند. رمز عددی ترافیک را رمزنگاری نمی‌کند. شناسهٔ داخلی کاربر رمزنگاری‌شده، رمز کلاینت نیست. کانفیگ‌های قدیمی 0.2.1 که رمز عددی و رمزنگاری را ترکیب می‌کردند باید دوباره از پنل کپی شوند.

**UDP به پشتیبانی برنامه، کلاینت و خروجی نیاز دارد؛ خروجی HTTP CONNECT فقط TCP است.** پروکسی SOCKS فقط برنامه‌هایی را پوشش می‌دهد که آن را تنظیم کرده‌اند؛ VPN کل سیستم قابلیت سمت کلاینت است.

## وضعیت و عیب‌یابی

نصب و آپدیت، تنظیمات قابل‌بازگشت شبکه را خودکار اعمال می‌کنند و هسته نرخ ارسال ICMP را کنترل می‌کند. [تنظیمات، بازگردانی و محدودیت‌های میزبان](docs/NETWORK-TUNING.md). این موارد تضمینی برای جلوگیری از مسدودسازی یا خاموش‌شدن از طرف ضد‌دیداس شرکت میزبان نیستند.

برای ارتقا از ۰.۳.۰، دستور نصب بالای صفحه را دوباره اجرا و **گزینهٔ ۲، Update** را انتخاب کنید تا مدیر جدید و ابزار تنظیم شبکه نصب شوند. منوی قدیمی به‌تنهایی ابزار جدید را نصب نمی‌کند. اطلاعات کاربران و تنظیمات حفظ می‌شوند؛ اتصال‌های فعال یک بار بازنشانی می‌شوند.

```bash
systemctl status namedicmp --no-pager
journalctl -u namedicmp -n 60 --no-pager
/opt/namedicmp/namedicmp diagnostics
```

| مشکل | بررسی |
| --- | --- |
| نصب از سورس کند است | دسترسی به فایل آمادهٔ لینوکس و SHA256SUMS؛ ساخت از سورس زمان بیشتری می‌برد |
| پنل باز نمی‌شود | آدرس bind، پورت، تداخل سرویس‌ها، فایروال و لاگ‌ها |
| خطای Certbot | DNS عمومی، آزاد و قابل‌دسترس‌بودن پورت ۸۰ |
| کلاینت وصل نمی‌شود | احراز هویت، دسترسی ICMP، مجوز کلاینت و فایروال ارائه‌دهنده |
| فقط UDP مشکل دارد | پشتیبانی UDP در برنامه/کلاینت و SOCKS5 UDP ASSOCIATE؛ HTTP خروجی UDP ندارد |

تنظیمات: `/etc/namedicmp/config.json` · دادهٔ کاربران: `/var/lib/namedicmp/state.json` · پشتیبان‌ها: `/var/lib/namedicmp/backups`. دو نمونهٔ سرور را هم‌زمان روی یک فایل state اجرا نکنید. بررسی checksum را دور نزنید.

## توسعه

Go نسخهٔ **1.26 یا جدیدتر** لازم است؛ Node.js فقط برای تست اختیاری رابط پنل استفاده می‌شود.

```bash
go test -race ./...
go vet ./...
node scripts/test_versions.cjs
bash scripts/test_installer.sh
bash scripts/test_update.sh
bash scripts/build.sh
```

خروجی لینوکس در `dist/` ساخته می‌شود. حالت نصب با `NAMEDICMP_INSTALL_MODE=source`، `binary` یا `auto` انتخاب می‌شود. تست نصب از سورس با `NAMEDICMP_RUN_TESTS=1` فعال می‌شود. فرایند ساخت ویندوز و اندروید در NamedClients قرار دارد.

[امنیت](SECURITY.md) · [یادداشت نسخهٔ پنل](docs/RELEASE-v0.4.1.md) · [معماری](docs/ARCHITECTURE.md) · [مجوز وابستگی‌ها](THIRD_PARTY_NOTICES.md)

## حمایت

- **TRX (Tron):** `TYbWTzdjVM3HpqkGiUX7GGV2XgK6QDJQis`
- **TON:** `UQDPGgQWQb9j8pxC5_RIh2HbuTJjaIYSga7FXPLZWbyzTZJN`

[مجوز MIT](LICENSE)
