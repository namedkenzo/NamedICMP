# namedICMP v0.5.1 · Download stability

## English

- Separate local writer-queue delay from actual network retransmission timing. A reliable frame has at most one queued dispatch; failed local admission does not fabricate packet loss or reduce the congestion window.
- Keep finite downloads draining after outbound EOF until queued data is acknowledged; ACKed download progress refreshes the inactivity deadline.
- Retry known transient ICMP receive errors with bounded, cancellable backoff. Permanent failures remain visible.
- Add deterministic queue/early-ACK/concurrent-completion tests and pure-download load tests alongside existing TCP/UDP accounting tests.
- Connection profiles, quotas and wire format are unchanged. Client watchdog hardening is released separately in NamedClients.

See [investigation and limitations](https://github.com/namedkenzo/NamedICMP/blob/main/docs/SPEEDTEST-STABILITY.md). These fixes do not bypass provider ICMP filtering or guarantee bandwidth on every route. Update preserves users/settings; restarting the service reconnects tunnels once.

## فارسی

- زمان انتظار در صف ارسال دیگر به‌اشتباه زمان انتظار پاسخ شبکه محسوب نمی‌شود؛ برای هر فریم فقط یک ارسالِ در صف نگه داشته می‌شود.
- پس از پایان خواندن دانلود، دادهٔ در انتظار تأیید صرفاً به‌دلیل گذشت ۳۰ ثانیه حذف نمی‌شود؛ پیشرفت واقعی ACK زمان بی‌فعالیتی را تازه می‌کند.
- خطاهای موقت شناخته‌شدهٔ دریافت ICMP با مکث محدود و قابل توقف دوباره امتحان می‌شوند؛ خطاهای دائمی پنهان نمی‌شوند.
- تست‌های رگرسیون برای فشار دانلود، صف، ACK زودهنگام و حسابداری حجم اضافه شدند.
- کانفیگ‌ها، حجم کاربران و پروتکل اتصال تغییر نکرده‌اند. اصلاح تشخیص قطعی کلاینت به‌صورت مستقل در NamedClients منتشر می‌شود.

محدودسازی ICMP از طرف ارائه‌دهنده یا مسیر شبکه با این تغییرات دور زده نمی‌شود؛ تضمین سرعت یا رفع همهٔ علت‌های ممکن روی VPS شما نیست. کاربران و تنظیمات در آپدیت حفظ می‌شوند؛ ری‌استارت یک بار تونل‌ها را بازنشانی می‌کند.
