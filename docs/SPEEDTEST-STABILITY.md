# Speed-test disconnects: findings and limits

## What was verified in code

The investigation identified failure paths that can be reproduced without a
provider blocking ICMP:

- **Asynchronous sending:** reliability timers must reflect actual socket
  transmission, not merely admission to a paced writer queue. Otherwise a busy
  download can retransmit frames that have not left the queue yet, amplifying
  congestion and delaying control traffic.
- **Download completion:** an outbound TCP EOF does not mean the queued download
  has reached the client. Closing must allow reliable data to drain and consider
  acknowledgement progress, rather than expire a fixed short drain deadline
  while useful transfer is still happening.
- **Receive errors:** a transient socket receive error must not be treated as a
  permanent failure of the shared ICMP transport.
- **Client health checks:** one unanswered probe is insufficient evidence that a
  working tunnel should be disconnected. Health checks need bounded retry and
  recovery handling; they must not manufacture successful test results.

These are code-level findings and regression scenarios, **not proof of which
one occurred on a particular VPS**. Identical symptoms can also result from
upstream filtering, packet loss, resource exhaustion, or an application error.
An actual deployment needs before/after diagnostics and a matching timestamp.
Changes must preserve quotas, credential validation, flow isolation and bounded
resource use; increasing every timeout or disabling protection is not a fix.

## Does ICMP have a speed limit?

There is **no universal ICMP Mbps ceiling** in [RFC 792][icmp]. Networks and
implementations can impose their own policies: [RFC 1812 §4.3.2.8][rate] permits
rate-limiting generated ICMP messages, including Echo Replies.

NamedICMP's default application pacing is **2,500 outgoing packets/second**.
With its current **888-byte TCP data chunks**, a hypothetical budget containing
only full data frames gives `2500 × 888 × 8 = 17,760,000 bit/s`, approximately
**17.76 Mbit/s across all flows**. This is an upper bound, not a promised speed:
control packets, retransmissions, path capacity, RTT and loss reduce useful
throughput. Reaching the pacing limit should throttle traffic, not disconnect
the configuration. The number is an application setting, not a law of ICMP.

Linux's `icmp_msgs_per_sec`, `icmp_msgs_burst` and `icmp_ratelimit` apply according
to its rate mask; default masks exclude Echo Request/Reply. Kernel-generated
messages and application-written raw packets are also different send paths.
Raising these sysctls blindly therefore does not establish or remove the tunnel's
throughput limit. See [Linux IP sysctl documentation][linux] and the existing
[network-tuning guide](NETWORK-TUNING.md).

## What still depends on the path

- A path-MTU black hole can allow connection setup and small requests while
  bulk transfers stall. [RFC 2923][blackhole] describes this pattern;
  [RFC 4459][mtu] explains encapsulation and fragmentation hazards. A successful
  small ping alone does not validate a bulk-transfer path. The current small TCP
  chunks reduce ordinary fragmentation risk, but cannot prove every path works.
- Congestion and retransmission amplification need controlled sending and
  backpressure, not unbounded queues. [RFC 8085][congestion] provides relevant
  guidance for tunnels and protocols directly over IP as well as UDP.
- Guest settings cannot override a provider's policing, null route, DDoS
  mitigation or ICMP prohibition. Ask the provider about permitted traffic and
  rates. **Do not flush the firewall, disable DDoS protections, or remove all
  pacing as a workaround.** No release can promise to bypass every upstream
  restriction.

## Collect evidence safely

Run this block once immediately **before**, and again immediately **after** the
failure, without restarting the service between snapshots:

```bash
date -u '+%Y-%m-%dT%H:%M:%SZ'
sudo /opt/namedicmp/namedicmp version
sudo systemctl show namedicmp -p ActiveState -p NRestarts -p Result -p ExecMainCode -p ExecMainStatus -p MemoryCurrent -p TasksCurrent
sudo /opt/namedicmp/namedicmp diagnostics
```

Compare counter **deltas**, not just non-zero lifetime totals: queue drops,
socket errors, rejected/session-limit packets, outbound error categories and
session closures. Record the client version, whether its VPN state actually
disconnected, the test timestamp, and any provider mitigation event. A running
service does not prove the path is reachable; a historical outbound error does
not prove the service crashed. Do not share connection links, keys, passwords,
`state.json`, credential files or private packet payloads.

## فارسی

### نتیجهٔ مقایسهٔ آزمایشگاهی

در CI یک تست یکسانِ ۸ دانلود ۹۶ کیبی‌بایتی، با تأخیر ارسال یک میلی‌ثانیه، تأخیر ACK سی میلی‌ثانیه و حذف هر بیست‌ونهمین بسته اجرا شد. نسخهٔ ۰٫۵٫۰ این تست را هم کامل کرد، اما حدود ۱۴۹۳ تا ۱۶۴۰ ارسال مجدد داشت؛ نسخهٔ اصلاح‌شده در سه بار اجرا حدود ۹۵ تا ۲۰۵ ارسال مجدد داشت. این مقایسه، تقویت ارسال مجدد بر اثر صف را نشان می‌دهد، نه اینکه قطعی خاصِ مسیر شما بازتولید شده باشد. زمان‌های این شبیه‌ساز، تست سرعت اینترنت عمومی یا تضمین کارایی روی VPS نیستند.

در بررسی کد، چند مسیر خرابی قابل بازتولید پیدا شد: شروع زودهنگام تایمر ارسال
مجدد پیش از خروج واقعی بسته از صف، بستن زودهنگام دانلود پس از EOF در حالی که
داده هنوز تأیید نشده، تبدیل خطای موقت دریافت به قطع کل هسته، و قطع کلاینت با
یک تست پینگ بی‌پاسخ. تست‌های رگرسیون باید این حالت‌ها را پوشش دهند. این
یافته‌ها به‌تنهایی مشخص نمی‌کنند کدام مورد دقیقاً روی VPS شما رخ داده است؛
برای آن باید خروجی قبل و بعد از تست و زمان دقیق قطع مقایسه شوند.

ICMP سقف سرعت ثابت و جهانی ندارد. تنظیم پیش‌فرض خود برنامه ۲۵۰۰ بستهٔ خروجی
در ثانیه است؛ با محتوای ۸۸۸ بایتی، سقف فرضیِ فقط داده حدود **۱۷٫۷۶ مگابیت
بر ثانیه برای مجموع جریان‌ها** می‌شود. سربار کنترل، ارسال مجدد و شرایط مسیر
سرعت عملی را کمتر می‌کنند. رسیدن به این سقف باید باعث محدودشدن سرعت شود، نه
قطع کانفیگ. افزایش کورکورانهٔ `icmp_msgs_*` هم تضمین افزایش سرعت بسته‌های خام
تونل نیست؛ ماسک و مسیر ارسال کرنل با برنامه متفاوت‌اند.

مشکل MTU، افت بسته یا محدودیت ارائه‌دهنده همچنان ممکن است وجود داشته باشد؛
پینگ کوچک موفق به‌تنهایی سلامت دانلود را ثابت نمی‌کند. هیچ تنظیم داخل VPS
نمی‌تواند مسدودسازی یا ضد‌دیداس بالادست را تضمینی خنثی کند. فایروال و حفاظت
ضد‌دیداس را خاموش نکنید و برای رفع مشکل همهٔ محدودیت‌های ارسال را حذف نکنید.

بلوک دستور بالا را یک بار قبل و یک بار بلافاصله بعد از قطع، **بدون ری‌استارت
بین آن‌ها** اجرا کنید. تغییر شمارنده‌ها، وضعیت سرویس، نسخهٔ کلاینت، زمان دقیق
و این‌که خود VPN قطع شده یا فقط اینترنت متوقف شده را ثبت کنید. لینک کانفیگ،
کلید، رمز، فایل وضعیت کاربران و محتوای خصوصی بسته‌ها را منتشر نکنید.

[icmp]: https://www.rfc-editor.org/rfc/rfc792.html
[rate]: https://www.rfc-editor.org/rfc/rfc1812.html#section-4.3.2.8
[linux]: https://docs.kernel.org/networking/ip-sysctl.html
[blackhole]: https://www.rfc-editor.org/rfc/rfc2923.html#section-2.1
[mtu]: https://www.rfc-editor.org/rfc/rfc4459.html
[congestion]: https://www.rfc-editor.org/rfc/rfc8085.html
