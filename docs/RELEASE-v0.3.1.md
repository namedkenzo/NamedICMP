# namedICMP 0.3.1 — server stability and reversible network tuning

- Bounded, single-writer ICMP output with reserved ACK/probe/control capacity and fair data scheduling. Slow writes no longer run inline in the receive loop.
- Aggregate outgoing pacing: 2500 packets/sec, burst 32 by default; configurable. Raw-socket write deadline and detailed queue/socket counters.
- Automatic network profile during Install/Update: socket buffer ceilings, receive backlog and conservative ICMP error-message budget. Records originals, survives reboot, supports restore and preserves administrator changes. No firewall/security protections disabled.
- Diagnostics separate outbound DNS/policy/timeout/network/other errors and session-limit/session-queue/output-queue drops. No keys/destination logging.
- Regression tests: saturated writes, control priority, bounded rate/burst, parallel loss/retransmission transfers, exact accounting and tuning/update restoration.
- Server/panel remains stable and separate from client releases.

Kernel ICMP message limits do not pace raw tunnel Echo packets or control the
provider's anti-DDoS system. See [network tuning](NETWORK-TUNING.md). Controlled
tests do not establish real VPS/carrier speed-test acceptance. The reported
disconnect still requires before/after diagnostics on the affected VPS; this
release does not claim every possible underlying cause is fixed.

## فارسی

صف ارسال محدود و اولویت‌دار، تنظیم نرخ بسته‌های خروجی، تنظیمات امن و قابل‌بازگشت
شبکه در نصب/آپدیت، و جزئیات بیشتر خطاها در diagnostics. اطلاعات کاربران حفظ
می‌شوند؛ آپدیت یک بار اتصال‌ها را بازنشانی می‌کند. ضد‌دیداس شرکت میزبان با
sysctl داخل VPS قابل خاموش‌کردن یا تضمین نیست. تست روی همان سرور لازم است.
