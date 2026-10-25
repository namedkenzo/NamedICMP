#!/usr/bin/env python3
"""Address discovery regression tests; no network calls or host changes."""
import importlib.util
import json
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("detect_addresses", Path(__file__).with_name("detect_addresses.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

VALID = "26064700470000000000000000001111 02 40 00 80 eth0\n"


class AddressDetectionTest(unittest.TestCase):
    def test_proc_inventory_survives_missing_ip_and_proxy(self):
        with patch.object(Path, "read_text", return_value=VALID), patch.object(module, "interface_ready", return_value=True), patch.object(module.subprocess, "run", side_effect=FileNotFoundError), patch.dict(module.os.environ, {"HTTPS_PROXY": "http://proxy.invalid"}, clear=True):
            self.assertEqual(module.detect("ipv6"), "2606:4700:4700::1111")

    def test_proc_rejects_unready_or_nonpublic_ipv6(self):
        text = VALID + VALID.replace(" 80 ", " 40 ").replace("1111", "1112")
        text += VALID.replace(" 80 ", " 08 ").replace("1111", "1113")
        text += VALID.replace(" 80 ", " 20 ").replace("1111", "1114")
        text += VALID.replace("eth0", "down0").replace("1111", "1115")
        text += "fe800000000000000000000000000001 02 40 20 80 eth0\n"
        text += "fd000000000000000000000000000001 02 40 00 80 eth0\n"
        text += "20010db8000000000000000000000001 02 40 00 80 eth0\nmalformed\n"
        self.assertEqual(module.parse_proc_ipv6(text, lambda name: name == "eth0"), ["2606:4700:4700::1111"])

    def test_malformed_ip_json_still_uses_reflection(self):
        replies = [subprocess.CompletedProcess([], 0, "broken-json", ""), subprocess.CompletedProcess([], 0, "8.8.8.8", "")]
        with patch.object(module.subprocess, "run", side_effect=replies) as run, patch.dict(module.os.environ, {}, clear=True):
            self.assertEqual(module.detect("ipv4"), "8.8.8.8")
            self.assertEqual(run.call_count, 2)

    def test_no_proxy_ip_reflection_and_no_lookup_when_disabled(self):
        for env in ({"HTTPS_PROXY": "http://proxy.invalid"}, {"NAMEDICMP_PUBLIC_IP_LOOKUP": "0"}):
            with patch.object(module, "local_addresses", return_value=[]), patch.object(module.subprocess, "run") as run, patch.dict(module.os.environ, env, clear=True):
                self.assertEqual(module.detect("ipv6"), "")
                run.assert_not_called()

    def test_ip_json_fallback_rejects_tentative_and_down_interfaces(self):
        data = [{"flags": ["UP"], "addr_info": [{"local": "2606:4700:4700::1111"}, {"local": "2606:4700:4700::1112", "tentative": True}]}, {"flags": ["BROADCAST"], "addr_info": [{"local": "2606:4700:4700::1113"}]}]
        with patch.object(Path, "read_text", side_effect=OSError), patch.object(module.subprocess, "run", return_value=subprocess.CompletedProcess([], 0, json.dumps(data), "")):
            self.assertEqual(module.local_addresses(6), ["2606:4700:4700::1111"])

    def test_local_ip_detection_works_without_curl(self):
        data = [{"flags": ["UP"], "addr_info": [{"local": "8.8.8.8"}]}]
        with patch.object(module.subprocess, "run", return_value=subprocess.CompletedProcess([], 0, json.dumps(data), "")) as run:
            self.assertEqual(module.detect("ipv4"), "8.8.8.8")
            self.assertEqual(run.call_count, 1)

    def test_namespace_interface_readiness_does_not_require_host_sysfs(self):
        data = [{"ifname": "eth0", "flags": ["UP"], "addr_info": []}]
        with patch.object(Path, "read_text", return_value=VALID), patch.object(module, "interface_ready", side_effect=AssertionError("host sysfs consulted")), patch.object(module.subprocess, "run", return_value=subprocess.CompletedProcess([], 0, json.dumps(data), "")):
            self.assertEqual(module.local_addresses(6), ["2606:4700:4700::1111"])

    def test_wrong_family_and_scoped_mapped_addresses(self):
        self.assertEqual(module.detect("wrong"), "")
        self.assertFalse(module.public("8.8.8.8", 6))
        self.assertFalse(module.public("::ffff:8.8.8.8", 6))
        self.assertFalse(module.public("2606:4700:4700::1111%eth0", 6))


if __name__ == "__main__":
    unittest.main()
