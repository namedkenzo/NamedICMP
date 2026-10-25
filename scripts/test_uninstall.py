#!/usr/bin/env python3
"""Uninstall regressions: mocked systemd/files plus exact-executable recovery."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]
MOCK = r'''
set -euo pipefail
export NAMEDICMP_INSTALLER_TEST=1
source "$1"
fixture=$2
test_mode=$3
# Deliberately absent executable/config: old partial uninstalls must recover.
PREFIX="$fixture/missing/namedicmp"
CONFIG="$fixture/missing/config.json"
STATE="$fixture/missing/state"
events="$fixture/events"
service_state=active
questions=0
yesno(){ questions=$((questions+1)); [[ $questions == 1 || $test_mode == purge ]]; }
systemctl(){
 printf '%s\n' "$*" >>"$events"
 case $1 in
  show)
   case $3 in
    --property=LoadState)
     if [[ $2 == namedicmp.service || $test_mode == static-renew && $2 == namedicmp-renew.service ]];then printf 'loaded\n';else printf 'not-found\n';fi;;
    --property=ActiveState)printf '%s\n' "$service_state";;
    --property=UnitFileState)if [[ $2 == namedicmp-renew.service ]];then printf 'static\n';else printf 'enabled\n';fi;;
    *)return 90;;
   esac;;
  stop)
   [[ $2 == namedicmp.service || $test_mode == static-renew && $2 == namedicmp-renew.service ]] || return 89
   [[ $test_mode != stop-fails ]] || return 1
   [[ $test_mode == stop-lies ]] || service_state=inactive;;
  disable)[[ $test_mode != disable-fails ]];;
  daemon-reload)return 0;;
  *)return 88;;
 esac
}
uninstall_stop_processes(){ printf 'process-check\n' >>"$events";[[ $test_mode != process-remains ]]; }
uninstall_restore_network(){ printf 'network-restore\n' >>"$events";[[ $test_mode != restore-fails ]]; }
uninstall_remove_files(){ printf 'remove-files\n' >>"$events"; }
uninstall_remove_data(){ printf 'purge-data\n' >>"$events"; }
if uninstall;then printf 'RESULT=success\n';else printf 'RESULT=failure\n';fi
'''


class UninstallTests(unittest.TestCase):
    def run_mock(self, mode):
        with tempfile.TemporaryDirectory(prefix="namedicmp-uninstall-test.") as fixture:
            result = subprocess.run(
                ["bash", "-s", "--", str(ROOT / "install.sh"), fixture, mode],
                input=MOCK, text=True, capture_output=True, timeout=10,
            )
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            events = Path(fixture, "events").read_text().splitlines()
            return result.stdout, events

    def test_missing_optional_timer_and_partial_install(self):
        output, events = self.run_mock("normal")
        self.assertIn("RESULT=success", output)
        self.assertIn("stop namedicmp.service", events)
        self.assertIn("disable namedicmp.service", events)
        self.assertIn("process-check", events)
        self.assertIn("remove-files", events)
        self.assertNotIn("purge-data", events)
        self.assertIn("Data retained", output)
        self.assertFalse(any("disable --now" in event for event in events))
        self.assertLess(events.index("process-check"), events.index("remove-files"))

    def test_explicit_data_purge(self):
        output, events = self.run_mock("purge")
        self.assertIn("RESULT=success", output)
        self.assertIn("purge-data", events)

    def test_static_renewal_service_does_not_need_disable(self):
        output, events = self.run_mock("static-renew")
        self.assertIn("RESULT=success", output)
        self.assertIn("stop namedicmp-renew.service", events)
        self.assertNotIn("disable namedicmp-renew.service", events)

    def test_stop_failure_or_live_state_never_deletes_files(self):
        for mode in ("stop-fails", "stop-lies", "disable-fails", "process-remains", "restore-fails"):
            with self.subTest(mode=mode):
                output, events = self.run_mock(mode)
                self.assertIn("RESULT=failure", output)
                self.assertNotIn("remove-files", events)
                self.assertNotIn("purge-data", events)
                self.assertNotIn("Uninstalled.", output)

    def test_exact_deleted_executable_only(self):
        # Real owned child, not a system service. Same-name unrelated executable
        # must survive; deleting the owned executable simulates the reported bug.
        if not Path("/proc/self/exe").exists():
            self.skipTest("Linux /proc required")
        import signal
        if not hasattr(os, "pidfd_open") or not hasattr(signal, "pidfd_send_signal"):
            self.skipTest("pidfd support required for safe orphan cleanup")
        with tempfile.TemporaryDirectory(prefix="namedicmp-uninstall-test.") as fixture:
            prefix = Path(fixture, "namedicmp")
            other = Path(fixture, "other")
            prefix.mkdir()
            other.mkdir()
            shutil.copy2("/bin/sleep", prefix / "namedicmp")
            shutil.copy2("/bin/sleep", other / "namedicmp")
            owned = subprocess.Popen([str(prefix / "namedicmp"), "60"])
            unrelated = subprocess.Popen([str(other / "namedicmp"), "60"])
            try:
                (prefix / "namedicmp").unlink()
                command = ["bash", "-c", 'export NAMEDICMP_INSTALLER_TEST=1; source "$1"; PREFIX="$2"; uninstall_stop_processes',
                           "uninstall-recovery-test", str(ROOT / "install.sh"), str(prefix)]
                # Hosted CI has passwordless sudo. Production uninstall also
                # runs as root; unprivileged /proc cannot prove other users'
                # executables are absent and must fail closed, not skip them.
                if os.geteuid() != 0 and shutil.which("sudo"):
                    command = ["sudo", "-n"] + command
                result = subprocess.run(
                    command,
                    text=True, capture_output=True, timeout=12,
                )
                if result.returncode and "Permission denied" in result.stderr:
                    self.assertIsNone(owned.poll())
                    self.assertIsNone(unrelated.poll())
                    self.skipTest("Restricted /proc: exact process integration requires root; safe refusal verified")
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIsNotNone(owned.poll(), "Owned deleted executable survived cleanup")
                self.assertIsNone(unrelated.poll(), "Same-name unrelated executable was killed")
            finally:
                for child in (owned, unrelated):
                    if child.poll() is None:
                        child.terminate()
                    child.wait(timeout=5)

    def test_service_allows_interface_discovery(self):
        text = (ROOT / "install.sh").read_text()
        families = next(line for line in text.splitlines() if line.startswith("RestrictAddressFamilies="))
        self.assertIn("AF_NETLINK", families.split("=", 1)[1].split())
        self.assertIn("AF_INET6", families)
        self.assertIn("AF_PACKET", families)

    def test_remaining_network_backup_or_edited_profile_blocks_uninstall(self):
        declaration = subprocess.run(
            ["bash", "-c", 'export NAMEDICMP_INSTALLER_TEST=1; source "$1"; declare -f uninstall_restore_network',
             "uninstall-network-test", str(ROOT / "install.sh")],
            text=True, capture_output=True, check=True,
        ).stdout
        for mode in ("failed-restore", "edited-profile", "restored"):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory(prefix="namedicmp-uninstall-test.") as fixture:
                directory = Path(fixture)
                config, sysctl = directory / "config", directory / "sysctl"
                config.mkdir()
                sysctl.mkdir()
                ledger = '{"values":{"unrestored":{}}}' if mode == "failed-restore" else '{"values":{}}'
                (config / "network-tuning.json").write_text(ledger)
                if mode == "edited-profile":
                    (sysctl / "91-namedicmp-network.conf").write_text("# Administrator edit\n")
                # Redirect fixed system paths only in this function fixture;
                # never invoke real sysctl writes or touch host /etc.
                isolated = declaration.replace("/etc/namedicmp", str(config)).replace("/etc/sysctl.d", str(sysctl))
                script = f'PREFIX="{directory / "missing"}"\nsay(){{ printf "%s\\n" "$*"; }}\n' + isolated + "\nuninstall_restore_network\n"
                result = subprocess.run(["bash"], input=script, text=True, capture_output=True, timeout=5)
                if mode == "restored":
                    self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                else:
                    self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                    self.assertTrue((config / "network-tuning.json").exists())


if __name__ == "__main__":
    unittest.main()
