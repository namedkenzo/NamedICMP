#!/usr/bin/env python3
"""Small, reversible Linux tuning profile. Never disables host protections."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import tempfile

MINIMUMS = {
    "net.core.rmem_max": 4 * 1024 * 1024,
    "net.core.wmem_max": 4 * 1024 * 1024,
    "net.core.netdev_max_backlog": 4096,
    "net.ipv4.icmp_msgs_per_sec": 5000,
    "net.ipv4.icmp_msgs_burst": 200,
}
HEADER = "# Managed by namedICMP; originals in /etc/namedicmp/network-tuning.json\n"

def atomic(path, content):
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, name = tempfile.mkstemp(prefix=".namedicmp-", dir=path.parent)
    try:
        with os.fdopen(fd, "w") as stream:
            os.chmod(name, 0o600)
            stream.write(content)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(name, path)
    finally:
        if os.path.exists(name):
            os.unlink(name)

def read_sysctl(key):
    result = subprocess.run(["sysctl", "-n", key], capture_output=True, text=True)
    value = result.stdout.strip()
    return int(value) if result.returncode == 0 and value.isdecimal() else None

def write_sysctl(key, value):
    return subprocess.run(["sysctl", "-q", "-w", f"{key}={value}"], capture_output=True).returncode == 0

def tune(action, config_dir, sysctl_dir, read=read_sysctl, write=write_sysctl):
    state_path = Path(config_dir) / "network-tuning.json"
    profile_path = Path(sysctl_dir) / "91-namedicmp-network.conf"
    state = json.loads(state_path.read_text()) if state_path.exists() else {"version": 1, "values": {}, "profile": ""}
    if state.get("version") != 1 or any(key not in MINIMUMS for key in state["values"]):
        raise ValueError("Unrecognized tuning backup; refusing to change sysctls")
    messages = []
    if action == "restore":
        for key, pair in list(state["values"].items()):
            if read(key) != pair["applied"]:
                messages.append(f"Kept administrator change: {key}")
            elif not write(key, pair["original"]):
                messages.append(f"Could not restore {key}; backup retained")
                continue
            del state["values"][key]
        if profile_path.exists() and profile_path.read_text() == state["profile"]:
            if state["values"]:
                # Failed restores stay persisted and recoverable, not forgotten.
                state["profile"] = HEADER + "".join(f"{k} = {v['applied']}\n" for k, v in state["values"].items())
                atomic(profile_path, state["profile"])
            else:
                profile_path.unlink()
        elif profile_path.exists():
            messages.append("Kept manually edited sysctl profile; review it before reboot")
        atomic(state_path, json.dumps(state, indent=2) + "\n")
        return messages + ["Network tuning restore finished (no firewall changes)."]
    if profile_path.exists() and profile_path.read_text() != state["profile"]:
        raise ValueError("Sysctl profile was edited or is not owned by this manager; review it first")
    output = {}
    for key, minimum in MINIMUMS.items():
        current = read(key)
        if current is None or current < 0:
            messages.append(f"Unsupported or restricted kernel setting: {key}")
            continue
        previous = state["values"].get(key)
        if previous and current != previous["applied"]:
            messages.append(f"Kept administrator change: {key}")
            continue
        # A zero ICMP rate is already unlimited. Never replace it with a limit.
        target = current if current == 0 and key.startswith("net.ipv4.icmp_msgs_") else max(current, minimum)
        state["values"][key] = {"original": previous["original"] if previous else current, "applied": target}
        # Persist the original before modifying a running kernel value.
        atomic(state_path, json.dumps(state, indent=2) + "\n")
        if current == target or write(key, target):
            output[key] = target
        else:
            messages.append(f"Kernel denied {key}; not added to the boot profile")
    state["profile"] = HEADER + "".join(f"{k} = {v}\n" for k, v in output.items())
    atomic(state_path, json.dumps(state, indent=2) + "\n")
    atomic(profile_path, state["profile"])
    return messages + ["Network profile applied; originals retained for restore.",
        "ICMP error sysctls do NOT pace raw tunnel echo packets. namedICMP paces its own sends.",
        "Provider anti-DDoS rules are outside this VPS; arrange ICMP permission with the provider."]

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("apply", "restore"))
    args = parser.parse_args()
    if os.geteuid() != 0:
        parser.error("Run as root")
    try:
        print("\n".join(tune(args.action, "/etc/namedicmp", "/etc/sysctl.d")))
    except (OSError, ValueError, KeyError) as error:
        parser.exit(1, f"Network tuning stopped: {error}\n")
