# namedICMP v0.5.2 · Admission correctness & load diagnostics

## English

- Fix a reproducible saturated-table bug: late TCP ACK/data/CLOSE packets for an already-removed flow are ignored before admission limits are checked. They no longer produce false capacity rejections or unnecessary per-flow KICKs. Genuine opening requests remain bounded; existing flows retain their independent state.
- Separate global/TCP-user/UDP-user/shutdown admission counters and TCP local-queue retries/control drops/UDP evictions/new UDP rejections. The old aggregate counters remain for compatibility; they are **not measured network packet loss** or counts of unique failed connections.
- Show the busiest user's current TCP/UDP usage versus each cap without exposing identities. Add UDP idle-expiry and outbound read/write-error counters to diagnostics and Dashboard.
- Provide an **opt-in, reversible** capacity trial for the observed default configuration: TCP 32 → 64 and UDP 128 → 192 per user, with the global 256-flow cap and pacing unchanged. Dry run is the default; it never restarts services automatically or overrides custom limits. Updates do not silently apply this experiment.
- Add regression tests, including an admission test that demonstrably fails on v0.5.1, category accounting, UI labels, and safe configuration backup/restore tests.

This release fixes the verified admission bug and makes load failures distinguishable. It does **not** claim that a particular VPS Speedtest disconnect has been reproduced or completely resolved. A running healthy core, near-full UDP table and historical queue events alone cannot identify the failing path. See [the investigation and controlled trial](https://github.com/namedkenzo/NamedICMP/blob/v0.5.2/docs/SPEEDTEST-STABILITY.md).

Users, quotas, connection links, encryption and network protections are unchanged. Only Linux web-panel archives are distributed here; namedP clients remain in NamedClients. Updating restarts the service and reconnects active tunnels once.

## فارسی

- یک باگ قابل بازتولید در ظرفیت پر رفع شد: ACK، داده و CLOSE دیررسِ اتصال TCP حذف‌شده دیگر به‌اشتباه درخواست اتصال جدید محسوب نمی‌شوند و پیام قطع اضافی دریافت نمی‌کنند. سقف اتصال‌های جدید و جداسازی جریان‌ها حفظ شده است.
- علت‌های رد ظرفیت و فشار صف جدا شدند؛ تلاش مجدد TCP برای ورود به صف با حذف بستهٔ UDP یا خطای صف کنترل یکی شمرده نمی‌شود. آمار تجمیعی قدیمی همچنان وجود دارد، اما **افت بستهٔ مسیر شبکه یا تعداد اتصال‌های ناموفق مجزا نیست**.
- بیشترین مصرف فعلی TCP و UDP یک کاربر نسبت به سقف، بدون نمایش هویت کاربر، در داشبورد و diagnostics دیده می‌شود. آمار انقضای UDP غیرفعال و خطای خواندن/نوشتن خروجی UDP اضافه شد.
- یک آزمایش ظرفیت **اختیاری و برگشت‌پذیر** برای تنظیم پیش‌فرض اضافه شد: TCP هر کاربر از ۳۲ به ۶۴ و UDP از ۱۲۸ به ۱۹۲. سقف کل ۲۵۶ و نرخ ارسال تغییر نمی‌کند؛ اسکریپت در حالت پیش‌فرض فقط بررسی می‌کند، تنظیم سفارشی را تغییر نمی‌دهد و خودش سرویس را ری‌استارت نمی‌کند. آپدیت این آزمایش را خودکار اعمال نمی‌کند.
- تست رگرسیون شامل بازتولید باگ روی نسخهٔ ۰٫۵٫۱، تفکیک شمارنده‌ها، برچسب‌های داشبورد و پشتیبان‌گیری/بازیابی تنظیمات اضافه شد.

باگ پذیرش اتصال اصلاح شده، اما **رفع کامل قطعی اسپیدتست روی VPS مشخص هنوز تأیید نشده است**. نزدیک‌شدن UDP به سقف و رویدادهای تاریخی صف، به‌تنهایی علت دقیق قطعی را ثابت نمی‌کند. راهنمای بررسی و آزمایش کنترل‌شده در لینک بالا آمده است.

کاربران، حجم، کانفیگ، رمزنگاری و حفاظت شبکه حفظ می‌شوند. این ریلیز فقط وب پنل لینوکسی است؛ کلاینت‌ها همچنان در NamedClients هستند. آپدیت یک بار سرویس را ری‌استارت و تونل‌های فعال را بازنشانی می‌کند.
