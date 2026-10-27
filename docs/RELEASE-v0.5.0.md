# namedICMP v0.5.0 · Live Network dashboard

## English

- Renamed Home to **Dashboard**, with a responsive English/Persian layout and dark/light themes.
- Added authenticated, same-origin **WebSocket** updates every second and a bounded, in-memory five-minute chart. Separate tunnel payload, host-interface and ICMP packet-rate views.
- Added real IPv4/IPv6 echo RTT, rolling loss, jitter and min/average/max, clearly scoped **VPS → displayed target**, not client or game ping. Unavailable/filtered probes are not shown as a fake zero ping.
- Added active TCP/UDP flows and users, queue pressure, pacing limits, rejection/drop reasons, outbound DNS/policy/timeout/network errors, socket errors and storage health.
- Added process RSS/heap, RAM/swap/disk/CPU, file descriptors, interface drops/errors, kernel ICMP counters and read-only network settings.
- Shared sampling, low-rate echo workers, bounded stream count/mailboxes, logout revocation, stale-state display and reconnect backoff. No changes to tunnel protocol, client apps or user data format.
- Installer/update retains users, usage, credentials, endpoint and TLS settings. Normal service restart reconnects active tunnels once.

Monitoring does not bypass provider filtering or guarantee DDoS exemption. Read [Live Dashboard](https://github.com/namedkenzo/NamedICMP/blob/main/docs/LIVE-DASHBOARD.md) for measurement scope.

## فارسی

- تغییر «خانه» به **داشبورد**، طراحی واکنش‌گرا و سازگار با فارسی/انگلیسی و تم روشن/تیره.
- آمار زندهٔ یک‌ثانیه‌ای با **WebSocket** و نمودار ۵ دقیقهٔ اخیر در حافظه؛ نمایش جداگانهٔ محتوای تونل، رابط‌های VPS و نرخ بستهٔ ICMP.
- پینگ واقعی IPv4/IPv6، افت بسته، جیتر و کمینه/میانگین/بیشینه؛ با توضیح روشن اینکه مسیر **VPS تا مقصد پایش** است، نه پینگ کلاینت یا بازی.
- سلامت ذخیرهٔ مصرف، جریان‌های TCP/UDP و کاربران فعال، فشار صف‌ها، محدودیت ارسال و جزئیات خطاهای خروجی و حذف بسته.
- مصرف CPU/RAM/Swap/دیسک، حافظهٔ فرایند، فایل‌های باز، خطاهای رابط و شمارنده‌های ICMP کرنل و تنظیمات فقط‌خواندنی شبکه.
- نمونه‌برداری مشترک، تست کم‌نرخ، ارتباط محدود و امن، قطع دسترسی پس از خروج، نمایش دادهٔ قدیمی و اتصال مجدد خودکار.
- پروتکل تونل و کلاینت‌ها تغییر نکرده‌اند. اطلاعات کاربران و تنظیمات در آپدیت حفظ می‌شوند؛ ری‌استارت عادی اتصال‌های فعال را یک بار بازنشانی می‌کند.
