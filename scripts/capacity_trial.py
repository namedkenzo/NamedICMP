#!/usr/bin/env python3
"""Opt-in capacity experiment for the observed 256/32/128 configuration.

Never changes pacing, queues, firewall rules, credentials or the global limit.
Run without --apply first. Restarting the service is deliberately a separate step.
"""
import argparse
import json
import os
from pathlib import Path
import stat
import tempfile

DEFAULTS = {"max_sessions": 256, "max_sessions_per_user": 32,
            "max_udp_sessions_per_user": 128}
TRIAL = {"max_sessions_per_user": 64, "max_udp_sessions_per_user": 192}


def load(path):
    info = path.lstat()
    if not stat.S_ISREG(info.st_mode):
        raise ValueError("Configuration must be a regular file, not a symlink")
    raw = path.read_bytes()
    data = json.loads(raw)
    if not isinstance(data, dict):
        raise ValueError("Expected a JSON configuration object")
    for key, default in DEFAULTS.items():
        value = data.get(key, default)
        if type(value) is not int or not 1 <= value <= 10000:
            raise ValueError("Invalid capacity value: " + key)
    return data, raw, info


def check(data, expected):
    if any(data.get(key, DEFAULTS[key]) != value for key, value in expected.items()):
        raise ValueError("Capacity differs from the expected baseline; kept custom settings")


def write_file(fd, raw, info, mode):
    with os.fdopen(fd, "wb") as stream:
        if os.geteuid() == 0:
            os.fchown(stream.fileno(), info.st_uid, info.st_gid)
        os.fchmod(stream.fileno(), mode)
        stream.write(raw)
        stream.flush()
        os.fsync(stream.fileno())


def sync_directory(path):
    fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def replace(path, raw, previous, info):
    fd, name = tempfile.mkstemp(prefix=".capacity-new-", dir=path.parent)
    try:
        write_file(fd, raw, info, stat.S_IMODE(info.st_mode))
        current = path.lstat()
        if (current.st_dev, current.st_ino, current.st_mtime_ns) != (info.st_dev, info.st_ino, info.st_mtime_ns) or path.read_bytes() != previous:
            raise ValueError("Configuration changed concurrently; no replacement performed")
        os.replace(name, path)
        sync_directory(path.parent)
    finally:
        if os.path.exists(name):
            os.unlink(name)


def trial(config, apply=False, restore=None):
    path = Path(config).absolute()
    data, previous, info = load(path)
    if restore is not None:
        backup = Path(restore).absolute()
        if backup.parent != path.parent or not backup.name.startswith(".capacity-before-"):
            raise ValueError("Restore requires the backup created beside this configuration")
        original, _, _ = load(backup)
        check(original, DEFAULTS)
        check(data, {"max_sessions": 256, **TRIAL})
        for key in TRIAL:
            if key in original:
                data[key] = original[key]
            else:
                data.pop(key, None)
        description = "Restore TCP 32 / UDP 128 per user; preserve all other current settings."
    else:
        check(data, DEFAULTS)
        data.update(TRIAL)
        description = "Trial TCP 64 / UDP 192 per user; global 256 and pacing unchanged."
    if not apply:
        return [description, "Dry run only. No files changed and no service restarted."]
    if restore is None:
        fd, name = tempfile.mkstemp(prefix=".capacity-before-", suffix=".json", dir=path.parent)
        # The full configuration can contain secrets. Never print its contents.
        write_file(fd, previous, info, 0o600)
        # Make the backup directory entry durable before replacing config.
        sync_directory(path.parent)
        backup = Path(name)
    replace(path, (json.dumps(data, indent=2) + "\n").encode(), previous, info)
    return [description, "Backup: " + str(backup),
            "Config saved. Run: systemctl restart namedicmp",
            "That restart disconnects active tunnels once. Reconnect before testing."]


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", default="/etc/namedicmp/config.json")
    parser.add_argument("--apply", action="store_true")
    parser.add_argument("--restore", metavar="BACKUP")
    args = parser.parse_args()
    if args.apply and os.geteuid() != 0:
        parser.error("Run as root to apply/restore")
    try:
        print("\n".join(trial(args.config, args.apply, args.restore)))
    except (OSError, ValueError) as error:
        # Do not echo JSON source, which may contain credentials.
        parser.exit(1, "Capacity trial stopped: " + ("invalid JSON" if isinstance(error, json.JSONDecodeError) else str(error)) + "\n")
