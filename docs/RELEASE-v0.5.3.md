# namedICMP v0.5.3 · Connection cleanup & efficient queues

## English

- **Prompt proxy cleanup:** SOCKS5 and HTTP negotiations now honor cancellation and an earlier caller deadline even after the TCP socket connects. SOCKS UDP relay lookup/dial uses the same bounded context. Successful tunnels are detached from the completed handshake context.
- **Avoid redundant queued traffic:** discard a queued retry if its original data has already been acknowledged; invalidate queued sends when their TCP stream closes. This saves unnecessary ICMP writes under queue pressure without changing the wire protocol, pacing or payload accounting. A write already in progress cannot be recalled.
- **HTTP CONNECT compatibility:** accept valid three-digit `2xx` success responses, not only `200`. Preserve tunnel bytes already buffered after the headers; continue rejecting failed or malformed responses.
- **Clear diagnostics:** add `obsolete_queued_packets` to diagnostics and Dashboard's detailed pressure section. It counts avoided obsolete sends, not network packet loss, and is not added to `dropped`.
- **Regression coverage:** real TCP sockets exercise cancellation during HTTP headers and SOCKS negotiation; CONNECT response tests preserve buffered data; a blocked-writer test checks that acknowledged retries do not reach the socket. Proxy regressions are also run against v0.5.2 to demonstrate the old failures.

Users, quotas, links, encryption, existing settings and capacity/rate defaults are preserved. This release contains only Linux web-panel archives; namedP Android and Windows remain independently distributed in [NamedClients](https://github.com/namedkenzo/NamedClients). Updating restarts the service once and interrupts active tunnels briefly.

These are verified implementation fixes, **not a claim that the deployment-specific Speedtest disconnect is fully resolved**. Network/provider filtering and capacity pressure still require before/after diagnostics on the actual path. [Research and validation scope](https://github.com/namedkenzo/NamedICMP/blob/v0.5.3/docs/MAINTENANCE-v0.5.3.md).

## فارسی

- **آزادسازی سریع اتصال معطل:** مذاکرهٔ SOCKS5 و HTTP پس از اتصال TCP نیز لغو درخواست و مهلت کوتاه‌تر آن را رعایت می‌کند. پیدا کردن و اتصال به رلهٔ UDP ساکس هم محدود به همان مهلت است. اتصال موفق با پایان مهلت مذاکره، بی‌دلیل بسته نمی‌شود.
- **ارسال تکراری کمتر:** اگر دادهٔ اصلی تأیید شده باشد، ارسال مجددِ منتظر در صف کنار گذاشته می‌شود. بسته‌های صف‌شدهٔ جریان TCP بسته‌شده هم دیگر بیهوده ارسال نمی‌شوند. پروتکل، محدودسازی نرخ و حسابداری حجم تغییر نمی‌کند؛ نوشتنی که شروع شده قابل بازگرداندن نیست.
- **سازگاری HTTP CONNECT:** همهٔ پاسخ‌های موفق معتبر `2xx` پذیرفته می‌شوند، نه فقط `200`. دادهٔ تونل که همراه هدر دریافت شده حفظ می‌شود؛ پاسخ خطا یا نامعتبر همچنان رد می‌شود.
- **آمار روشن‌تر:** شمارندهٔ `obsolete_queued_packets` در diagnostics و جزئیات فشار داشبورد اضافه شد. این عدد ارسال‌های غیرضروریِ جلوگیری‌شده است، نه افت بستهٔ شبکه؛ به `dropped` اضافه نمی‌شود.
- **تست رگرسیون:** لغو مذاکره با سوکت واقعی، پاسخ‌های CONNECT و حفظ داده، و عدم ارسال مجددِ تأییدشده آزمایش می‌شوند. تست‌های پروکسی روی نسخهٔ ۰٫۵٫۲ نیز اجرا می‌شوند تا وجود باگ قبلی مشخص باشد.

کاربران، حجم، کانفیگ‌ها، رمزنگاری، تنظیمات قبلی و سقف اتصال و نرخ ارسال حفظ می‌شوند. ریلیز فقط شامل وب پنل لینوکسی است و کلاینت‌ها دست نخورده‌اند. آپدیت یک بار سرویس را ری‌استارت می‌کند و تونل‌های فعال برای لحظاتی قطع می‌شوند.

این اصلاح‌ها به معنی **تأیید رفع کامل قطعی اسپیدتست روی VPS شما نیست**؛ برای آن، مقایسهٔ تست و diagnostics روی همان مسیر لازم است.
