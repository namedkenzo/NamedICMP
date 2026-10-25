#!/usr/bin/env python3
"""Detect public interface addresses, with bounded public-IP lookup for NAT."""
import ipaddress
import json
import os
import subprocess
import sys

def public(value, version):
    try:
        address = ipaddress.ip_address(value)
        return address.version == version and address.is_global and not address.is_multicast
    except ValueError:
        return False

def detect(family):
    version = 6 if family == "ipv6" else 4
    result = subprocess.run(["ip", "-j", "address", "show", "up"], capture_output=True, text=True)
    if result.returncode == 0:
        for interface in json.loads(result.stdout):
            for address in interface.get("addr_info", []):
                value = address.get("local", "")
                if public(value, version):
                    return value
    # A proxy's egress address is not the public address of this VPS.
    if any(os.environ.get(key) for key in ("HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy")):
        return ""
    endpoint = "https://api6.ipify.org" if version == 6 else "https://api.ipify.org"
    result = subprocess.run(["curl", "-6" if version == 6 else "-4", "-fsS", "--max-time", "3", "--max-filesize", "64", endpoint], capture_output=True, text=True)
    value = result.stdout.strip()
    return value if result.returncode == 0 and public(value, version) else ""

if __name__ == "__main__":
    try:
        print(detect(sys.argv[1]))
    except (OSError, ValueError, IndexError):
        print("")
