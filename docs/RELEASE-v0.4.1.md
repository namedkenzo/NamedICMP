# NamedICMP 0.4.1 — IPv6 discovery and uninstall fixes

- Fix interface/IPv6 discovery in the systemd service by allowing AF_NETLINK.
  This also restores local-address validation for the optional AF_PACKET reader.
- Add local IPv6 discovery fallbacks and regression tests; no remote public-IP
  lookup is needed when a usable local IPv6 is present.
- Stop and verify services individually during uninstall, including installations
  without the optional certificate-renewal timer. Fail safely instead of deleting
  files while the managed process is still running.
- Recover safely from partially removed installations. User data remains retained
  by default; permanent removal still requires explicit confirmation.
- Test IPv6 discovery inside an actual systemd address-family sandbox and isolated
  network namespace, plus uninstall failure and preservation regressions.

Run the current installer and select **2 — Update** to refresh the service profile,
binary and manager. Simply restarting an old installation does not install these
fixes. For removal, run the current installer and select **11 — Uninstall**; it can
also handle a prior incomplete removal. Unrelated processes are never killed just
because they use the former panel port.

Routed IPv6 and provider/client reachability remain necessary. These fixes cannot
create IPv6 connectivity where it is unavailable upstream.
