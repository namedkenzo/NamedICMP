# Security reporting

Do not put live tunnel keys, panel passwords, proxy credentials, state files,
private certificate keys, or identifiable packet captures in public issues.

The project owner should enable GitHub private vulnerability reporting before
publishing the repository. Until then, use a private contact method chosen by
that owner. No maintainer address is invented in this source package.

This is an alpha release. The transport uses the PingTunnel IPv4 wire contract
with a positive 31-bit numeric access key. **Encryption defaults to none**,
including existing users. In that mode the access key and traffic are visible
on the network; the numeric key is an interoperability credential, not strong
encrypted user authentication.

Optional AES-128-GCM, AES-256-GCM and ChaCha20-Poly1305 authenticate and encrypt
the entire protobuf envelope using independently generated per-user random
keys and random nonces. Clients must use the matching mode and Base64 secret.
Plaintext downgrade for an encrypted user is rejected. The envelope agrees
with the reference client's raw-key format, but does not add forward secrecy
or a separate replay-defense protocol. UDP has no exactly-once delivery.

The requested connection URI contains only the numeric key; encrypted users
must separately configure their mode and encryption secret. Treat both keys
and copied CLI commands as secrets. Use end-to-end TLS for application traffic
and HTTPS or an SSH tunnel for the panel. Panel HTTPS does not enable ICMP
encryption automatically.

Admin HTTP is same-origin, size-limited and protected with session cookies and
CSRF. The management socket is local and 0600. Outbound credentials are stored
in the protected state file because proxy negotiation needs the originals.
Encryption secrets and update backups are also protected local state. Avoid
granting untrusted accounts read access to those files or the service account.
