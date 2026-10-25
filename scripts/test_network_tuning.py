#!/usr/bin/env python3
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("network_tuning", Path(__file__).with_name("network_tuning.py"))
network = importlib.util.module_from_spec(spec)
spec.loader.exec_module(network)

class NetworkTuningTest(unittest.TestCase):
    def setUp(self):
        self.fixture = tempfile.TemporaryDirectory(prefix="namedicmp-network-test-")
        self.addCleanup(self.fixture.cleanup)
        self.config = Path(self.fixture.name) / "config"
        self.sysctls = Path(self.fixture.name) / "sysctl.d"
        self.values = {key: 100 for key in network.MINIMUMS}
        self.initial = dict(self.values)
        self.writes = []

    def write(self, key, value):
        self.values[key] = value
        self.writes.append((key, value))
        return True

    def run_tuning(self, action):
        return network.tune(action, self.config, self.sysctls, self.values.get, self.write)

    def test_apply_idempotent_persistent_and_restore(self):
        self.run_tuning("apply")
        self.assertEqual(self.values, network.MINIMUMS)
        state = json.loads((self.config / "network-tuning.json").read_text())
        self.run_tuning("apply")
        self.assertEqual(state, json.loads((self.config / "network-tuning.json").read_text()))
        self.assertEqual(len(self.writes), len(network.MINIMUMS))
        self.run_tuning("restore")
        self.assertEqual(self.values, self.initial)
        self.assertFalse((self.sysctls / "91-namedicmp-network.conf").exists())

    def test_never_lowers_existing_values_or_disables_security(self):
        self.values = {key: minimum * 2 for key, minimum in network.MINIMUMS.items()}
        self.values["net.ipv4.icmp_msgs_per_sec"] = 0
        expected = dict(self.values)
        self.run_tuning("apply")
        self.assertEqual(self.values, expected)
        self.assertEqual(self.writes, [])
        self.assertFalse(any("ratemask" in key or "firewall" in key or "rp_filter" in key for key in network.MINIMUMS))

    def test_restore_keeps_operator_changes(self):
        self.run_tuning("apply")
        self.values["net.core.rmem_max"] = 12345678
        self.run_tuning("apply")
        self.run_tuning("restore")
        self.assertEqual(self.values["net.core.rmem_max"], 12345678)

    def test_unsupported_and_denied_sysctls_are_not_persisted(self):
        self.values.pop("net.core.rmem_max")
        def denied(key, value):
            return False if key == "net.core.wmem_max" else self.write(key, value)
        network.tune("apply", self.config, self.sysctls, self.values.get, denied)
        text = (self.sysctls / "91-namedicmp-network.conf").read_text()
        self.assertNotIn("rmem_max", text)
        self.assertNotIn("wmem_max", text)

    def test_keeps_edited_profile_and_backup(self):
        self.run_tuning("apply")
        profile = self.sysctls / "91-namedicmp-network.conf"
        profile.write_text("# edited by administrator\n")
        with self.assertRaises(ValueError):
            self.run_tuning("apply")
        self.run_tuning("restore")
        self.assertEqual(profile.read_text(), "# edited by administrator\n")

if __name__ == "__main__":
    unittest.main()
