#!/usr/bin/env python3
"""Detect public interface addresses, with bounded public-IP lookup for NAT."""
import ipaddress
import json
import os
from pathlib import Path
import subprocess
import sys

def public(value, version):
    try:
        address = ipaddress.ip_address(value)
        return (address.version == version and address.is_global
                and not address.is_multicast
                and not (version == 6 and (address.ipv4_mapped is not None or getattr(address, "scope_id", None) is not None)))
    except ValueError:
        return False

def interface_ready(name):
    if not name or name in (".", "..") or "/" in name or "\\" in name:
        return False
    try:
        flags = int(Path("/sys/class/net", name, "flags").read_text().strip(), 0)
        return bool(flags & 1) and not bool(flags & 8)  # UP, not loopback.
    except (OSError, ValueError):
        return False


def parse_proc_ipv6(contents, ready=None):
    """Linux's read-only address inventory works without an AF_NETLINK socket."""
    ready = ready or interface_ready
    values = set()
    for line in contents.splitlines():
        fields = line.split()
        if len(fields) != 6 or len(fields[0]) != 32:
            continue
        try:
            value = str(ipaddress.IPv6Address(bytes.fromhex(fields[0])))
            scope, flags = int(fields[3], 16), int(fields[4], 16)
        except (ValueError, ipaddress.AddressValueError):
            continue
        # DAD failure, deprecated, tentative: do not advertise an unusable IP.
        if scope == 0 and not flags & (0x08 | 0x20 | 0x40) and ready(fields[5]) and public(value, 6):
            values.add(value)
    return sorted(values)


def local_addresses(version):
    values = set()
    readiness = {}
    try:
        result = subprocess.run(["ip", "-j", "address", "show", "up"],
                                capture_output=True, text=True, timeout=2)
        interfaces = json.loads(result.stdout) if result.returncode == 0 else []
        if not isinstance(interfaces, list):
            interfaces = []
        for interface in interfaces:
            if not isinstance(interface, dict):
                continue
            flags = interface.get("flags", [])
            name = interface.get("ifname", "")
            if name and flags:
                readiness[name] = "UP" in flags and "LOOPBACK" not in flags
            if flags and ("UP" not in flags or "LOOPBACK" in flags):
                continue
            for address in interface.get("addr_info", []):
                if not isinstance(address, dict):
                    continue
                value = address.get("local", "")
                if (any(address.get(flag) for flag in ("tentative", "dadfailed", "deprecated"))
                        or any(flag in address.get("flags", []) for flag in ("tentative", "dadfailed", "deprecated"))
                        or address.get("preferred_life_time") == 0):
                    continue
                if public(value, version):
                    values.add(value)
    except (OSError, ValueError, TypeError, subprocess.TimeoutExpired):
        pass
    if version == 6:
        try:
            # Proc also works when AF_NETLINK is denied. Prefer known
            # namespace interface flags: sysfs can refer to the host mount.
            def ready(name):
                return readiness[name] if name in readiness else interface_ready(name)
            return parse_proc_ipv6(Path("/proc/net/if_inet6").read_text(), ready)
        except OSError:
            pass
    return sorted(values)


def detect(family):
    if family not in ("ipv4", "ipv6"):
        return ""
    version = 6 if family == "ipv6" else 4
    addresses = local_addresses(version)
    if addresses:
        return addresses[0]
    # A proxy's egress address is not the public address of this VPS.
    if any(os.environ.get(key) for key in ("HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy")):
        return ""
    if os.environ.get("NAMEDICMP_PUBLIC_IP_LOOKUP") == "0":
        return ""
    endpoint = "https://api6.ipify.org" if version == 6 else "https://api.ipify.org"
    try:
        result = subprocess.run(["curl", "-6" if version == 6 else "-4", "-fsS", "--max-time", "3", "--max-filesize", "64", endpoint], capture_output=True, text=True, timeout=4)
    except (OSError, subprocess.TimeoutExpired):
        return ""
    value = result.stdout.strip()
    return value if result.returncode == 0 and public(value, version) else ""

if __name__ == "__main__":
    try:
        print(detect(sys.argv[1]))
    except (OSError, ValueError, IndexError):
        print("")
