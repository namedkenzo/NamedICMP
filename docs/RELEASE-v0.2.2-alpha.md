# namedICMP v0.2.2-alpha

Faster Linux installation, bilingual project documentation, and encryption-only client compatibility.

## Changes

- Default GitHub repository is `namedkenzo/NamedICMP`; installation no longer asks for owner/repository.
- Remote installs/updates prefer matching Linux amd64/arm64 release binaries, validate SHA-256 and executable version, and avoid local Go installation/compilation.
- Only missing system packages are installed. Source fallback remains available; source-install tests are opt-in, with automated tests retained in CI.
- Downloads have connection/transfer limits and retries. Invalid checksums and version mismatches stop deployment.
- Detect occupied panel ports before installation, validate administrator username length, and check service health after startup/TLS configuration.
- Fix encryption-only clients: authenticate by the encryption key, use protocol-default zero in encrypted envelopes, and keep routing IDs private to the server.
- Numeric-only and encrypted credentials are mutually exclusive in client profiles and wire authentication. Encryption links now include URL-encoded algorithm/key fields.
- English default README plus Persian README, migration guidance and Windows troubleshooting.
- Tag builds create a draft GitHub prerelease with Linux archives and `SHA256SUMS`.

## Upgrade note

Run the new `install.sh` and choose **Update (2)**. Existing users, counters, quotas, panel credentials and TLS settings are preserved. Re-import encrypted users' profiles: old commands that combine a positive `-key` and encryption are no longer accepted. Numeric-only profiles are unchanged.

The store's numeric field remains an internal identifier for encrypted accounts so ownership and accounting can be preserved. It is not exported or used as an additional encrypted-client password.

## Before publishing

1. Commit the source changes and push them to the desired release branch (normally `main`).
2. Create and push tag `v0.2.2-alpha` on that commit. The build workflow checks that tag and binary versions agree.
3. Review the generated **draft prerelease**. Confirm that `namedicmp-linux-amd64.tar.gz`, `namedicmp-linux-arm64.tar.gz` and `SHA256SUMS` are attached.
4. Publish the prerelease. Draft assets are not publicly downloadable; automatic installation falls back to a slower source build until publication.
5. On a clean Debian/Ubuntu VPS, test install, panel login, numeric and encrypted clients, update and Certbot where configured. Test an actual Windows client in SOCKS mode, then VPN mode.

Alternatively attach the same three files from `dist/` to a manually created release with tag `v0.2.2-alpha`. GitHub Actions build artifacts alone do not provide public release download URLs.

## Validation limits

Local automated checks cover the encryption-only wire, mixed users, TCP/UDP loopback integration, API/DOM exports, installer integrity checks and mocked update rollback. See `TEST_REPORT.md` for results. Clean-VPS systemd/Certbot deployment, Windows/Wintun and WAN interoperability still require the acceptance tests above. Installation speed has not been benchmarked on a VPS.

## فارسی

این نسخه نصب باینری آماده، مخزن پیش‌فرض، README انگلیسی و فارسی، و اصلاح ناسازگاری کلاینت‌های رمزنگاری‌شده را اضافه می‌کند. در حالت رمزنگاری، رمز عددی از کانفیگ حذف شده و فقط کلید رمزنگاری استفاده می‌شود. کانفیگ کاربران رمزنگاری‌شدهٔ قدیمی باید دوباره وارد کلاینت شود. برای نصب سریع، هر دو فایل لینوکس و `SHA256SUMS` را در ریلیز همین تگ منتشر کنید؛ فایل‌های draft برای عموم قابل دانلود نیستند.
