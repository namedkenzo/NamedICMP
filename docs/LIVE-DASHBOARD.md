# Live Network dashboard

The **Dashboard** replaces Home. The panel samples once per second and shares the same authenticated WebSocket stream (`/api/live`) with all open dashboards. It keeps at most 300 samples in memory, not a database: restarting the service clears the five-minute chart. Switching tabs/pages pauses the browser stream; reconnecting restores available history. System/user settings still use the existing REST API.

## Read the measurements correctly

| Measurement | What it actually measures |
| --- | --- |
| Tunnel upload / download | Unique payload forwarded by the managed core, from the client's perspective. No ICMP headers or TCP retransmission double-counting. Not an active speed test. |
| VPS interfaces | Byte-rate deltas across non-loopback interfaces, including other services. Virtual/bridge interfaces can count a packet more than once. |
| ICMP packets/s | Managed-core receive/send counter deltas, including control packets and retransmissions. |
| Active flows | TCP connections and UDP associations, **not a count of VPN devices**. Active users have at least one flow. |
| Ping / loss / jitter | Real ICMP echo checks **from the VPS to the displayed public target**, not client-to-VPS RTT or game ping. |
| Drops / rejects / outbound errors | Cumulative managed-core counters since start. These are **not** network packet-loss percentages. |
| Kernel ICMP / interface errors | Host-wide kernel counters since boot, including activity outside the panel. |

### Path checks

IPv4 target: `1.1.1.1`. IPv6 target: `2606:4700:4700::1111` (Cloudflare). Each enabled family sends one 32-byte random echo payload, waits up to two seconds, then waits ten seconds before the next check. The dashboard never probes client addresses or arbitrary user-supplied destinations. Probes operate outside tunnel queues/accounting and require raw-socket privileges already needed by the managed ICMP core. Panel-only mode disables them.

Loss is missing echo replies divided by **successfully sent** probes in the last 30 completed sends. A failed socket/filter/send operation is **unavailable**, not fabricated 100% loss. Jitter is mean absolute RTT difference between consecutive successful probes; a timeout breaks the pair. At least two consecutive replies are required. Min/average/max use successful replies in the same window. Latest ping is unavailable after a timeout or probe error; historical window values retain their meaning.

Public targets, provider policy, ICMP prioritisation and route differences affect these results. An echo timeout alone cannot prove that tunnel traffic is broken. This does not test client reachability, path MTU, DNS resolution or available bandwidth, and it cannot bypass a provider firewall or DDoS policy.

### Security and overhead

- Same-origin login cookie required, strict WebSocket Origin checking, no credentials in URLs.
- Logout, password changes and session expiry revoke streams; shutdown closes them.
- Maximum 16 streams. Each has one latest-frame mailbox, a bounded read limit and write deadline. Slow clients cannot grow a backlog.
- One shared sampler and two low-rate probe workers; no probe per visitor, no CDN/chart dependency, no third-party analytics.
- Linux echo-reply filters keep unrelated tunnel requests out of probe sockets.
- Queue counters, storage health, live listener families, pacing, process resident memory, RAM/swap/disk, file descriptors and read-only sysctl values are visible. No dashboard action alters firewall, pacing or sysctl configuration.
- Missing data is shown as `—`/unavailable. Disconnected streams are labelled stale and retry with backoff; browsers without WebSocket fall back to the existing 30-second refresh.

## فارسی

صفحهٔ **خانه** به **داشبورد** تغییر کرده است. نمودار ۵ دقیقهٔ اخیر و آمار هر ثانیه با WebSocket به‌روز می‌شوند؛ تاریخچه فقط در حافظه است و با ری‌استارت پاک می‌شود.

پینگ، افت بسته و جیتر از **VPS تا مقصد نمایش‌داده‌شده** هستند، نه پینگ گوشی یا بازی. افت بسته بر اساس ۳۰ ارسال موفق اخیر محاسبه می‌شود؛ خطای سوکت/ارسال به‌عنوان دادهٔ ناموجود نمایش داده می‌شود. جیتر میانگین اختلاف مطلق RTT پاسخ‌های موفق متوالی است؛ پس از یک بستهٔ بی‌پاسخ جفت‌سازی از نو آغاز می‌شود. فیلتر ICMP ممکن است پاسخ مقصد را مسدود کند و به‌تنهایی خرابی تونل را ثابت نمی‌کند.

نمودار «محتوای تونل» آپلود/دانلود از دید کلاینت است و ارسال مجدد TCP دوباره محاسبه نمی‌شود. ترافیک رابط‌های VPS شامل سرویس‌های دیگر است. اتصال فعال یعنی جریان TCP یا UDP، نه تعداد دستگاه. شمارندهٔ حذف بستهٔ هسته با افت مسیر شبکه متفاوت است.

داشبورد صف‌ها، سقف نرخ ارسال، خطاهای خروجی، IPv4/IPv6 فعال، سلامت ذخیرهٔ مصرف، مصرف منابع و شمارنده‌های ICMP کرنل را نشان می‌دهد. هیچ تنظیم فایروال یا sysctl را تغییر نمی‌دهد. دادهٔ قدیمی یا ناموجود مشخص است؛ نمونه‌برداری مشترک و صف محدود، سربار بازکردن چند تب را کنترل می‌کنند.
