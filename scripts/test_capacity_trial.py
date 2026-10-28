#!/usr/bin/env python3
import json
from pathlib import Path
import stat
import tempfile
import unittest
from unittest.mock import patch

import capacity_trial as module


class CapacityTrialTests(unittest.TestCase):
    def setUp(self):
        self.scratch = tempfile.TemporaryDirectory(prefix="namedicmp-capacity-test-")
        self.addCleanup(self.scratch.cleanup)
        self.path = Path(self.scratch.name) / "config.json"
        self.data = {**module.DEFAULTS, "panel_listen": "[::]:8443",
                     "public_host": "vpn.example.test", "tls_key": "/private/path",
                     "icmp_packets_per_second": 2500, "unrecognized_future_field": True}
        self.path.write_text(json.dumps(self.data))
        self.path.chmod(0o640)

    def apply(self):
        module.trial(self.path, apply=True)
        backups = list(self.path.parent.glob(".capacity-before-*.json"))
        self.assertEqual(len(backups), 1)
        return backups[0]

    def test_dry_run_does_not_write(self):
        previous = self.path.read_bytes()
        self.assertIn("Dry run", " ".join(module.trial(self.path)))
        self.assertEqual(self.path.read_bytes(), previous)
        self.assertEqual(list(self.path.parent.iterdir()), [self.path])

    def test_apply_and_restore_only_capacity(self):
        original = self.path.read_bytes()
        info = self.path.stat()
        backup = self.apply()
        self.assertEqual(backup.read_bytes(), original)
        self.assertEqual(stat.S_IMODE(backup.stat().st_mode), 0o600)
        self.assertEqual(stat.S_IMODE(self.path.stat().st_mode), 0o640)
        self.assertEqual(self.path.stat().st_uid, info.st_uid)
        current = json.loads(self.path.read_text())
        self.assertEqual(current, {**self.data, **module.TRIAL})
        current["public_host"] = "edited.example.test"
        self.path.write_text(json.dumps(current))
        module.trial(self.path, apply=True, restore=backup)
        self.assertEqual(json.loads(self.path.read_text()), {**self.data, "public_host": "edited.example.test"})
        self.assertTrue(backup.exists())

    def test_missing_default_fields_preserved_on_restore(self):
        for key in module.DEFAULTS:
            self.data.pop(key)
        self.path.write_text(json.dumps(self.data))
        backup = self.apply()
        module.trial(self.path, apply=True, restore=backup)
        self.assertEqual(json.loads(self.path.read_text()), self.data)

    def test_custom_limits_refused_without_changes(self):
        for key in module.DEFAULTS:
            with self.subTest(key=key):
                self.path.write_text(json.dumps({**self.data, key: 100}))
                previous = self.path.read_bytes()
                with self.assertRaises(ValueError):
                    module.trial(self.path, apply=True)
                self.assertEqual(self.path.read_bytes(), previous)
        self.assertEqual(len(list(self.path.parent.iterdir())), 1)

    def test_restore_refuses_administrator_capacity_changes(self):
        backup = self.apply()
        current = json.loads(self.path.read_text())
        current["max_sessions_per_user"] = 80
        self.path.write_text(json.dumps(current))
        previous = self.path.read_bytes()
        with self.assertRaises(ValueError):
            module.trial(self.path, apply=True, restore=backup)
        self.assertEqual(self.path.read_bytes(), previous)

    def test_symlink_refused(self):
        link = self.path.parent / "link.json"
        link.symlink_to(self.path)
        with self.assertRaises(ValueError):
            module.trial(link, apply=True)
        self.assertEqual(json.loads(self.path.read_text()), self.data)

    def test_replace_failure_preserves_original_and_backup(self):
        previous = self.path.read_bytes()
        with patch.object(module.os, "replace", side_effect=OSError("simulated failure")):
            with self.assertRaises(OSError):
                self.apply()
        self.assertEqual(self.path.read_bytes(), previous)
        backups = list(self.path.parent.glob(".capacity-before-*.json"))
        self.assertEqual(len(backups), 1)
        self.assertEqual(backups[0].read_bytes(), previous)
        self.assertFalse(list(self.path.parent.glob(".capacity-new-*")))


if __name__ == "__main__":
    unittest.main()
