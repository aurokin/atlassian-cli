"""Hermetic checks for matrix result accounting; never starts live tests."""

import json
import os
from pathlib import Path
import runpy
import signal
import subprocess
import sys
import tempfile
import time
import unittest


RUNNER = runpy.run_path(str(Path(__file__).with_name("e2e-matrix.py")))
ACCOUNT = RUNNER["account_events"]


class AccountingTests(unittest.TestCase):
    def account(self, events, expected=("TestJiraStatus",), code=0, timeout=False):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "events.jsonl"
            path.write_text("".join(json.dumps(event) + "\n" for event in events))
            return ACCOUNT(path, expected, code, timeout)

    def events(self):
        return [
            {"Action": "output", "Test": "TestJiraStatus", "Output": "build identity: binary=fixture sha256=example\n"},
            {"Action": "output", "Test": "TestJiraStatus", "Output": "Go source tree sha256=" + "a" * 64 + "\n"},
            {"Action": "pass", "Test": "TestJiraStatus"},
            {"Action": "pass"},
        ]

    def test_complete_success(self):
        result = self.account(self.events())
        self.assertEqual(result["status"], "pass")
        self.assertEqual(result["passed"], ["TestJiraStatus"])

    def test_skip_in_subtest_fails_even_if_parent_passes(self):
        events = self.events() + [{"Action": "skip", "Test": "TestJiraStatus/capability"}]
        self.assertEqual(self.account(events)["status"], "fail")

    def test_missing_expected_test_fails(self):
        result = self.account(self.events(), expected=("TestJiraStatus", "TestJiraSearch"))
        self.assertEqual(result["missing"], ["TestJiraSearch"])
        self.assertEqual(result["status"], "fail")

    def test_cleanup_failure_fails(self):
        events = self.events() + [{"Action": "fail", "Test": "TestJiraStatus"}, {"Action": "fail"}]
        self.assertEqual(self.account(events, code=1)["status"], "fail")

    def test_timeout_or_failed_exit_cannot_pass(self):
        self.assertEqual(self.account(self.events(), timeout=True)["status"], "fail")
        self.assertEqual(self.account(self.events(), code=1)["status"], "fail")

    def test_missing_package_completion_or_identity_fails(self):
        self.assertEqual(self.account(self.events()[:-1])["status"], "fail")
        self.assertEqual(self.account(self.events()[1:])["status"], "fail")

    def test_source_identity_must_be_present_and_consistent(self):
        events = self.events()
        del events[1]
        self.assertEqual(self.account(events)["status"], "fail")
        events = self.events() + [{"Action": "output", "Output": "Go source tree sha256=" + "b" * 64}]
        self.assertEqual(self.account(events)["status"], "fail")


@unittest.skipIf(os.name == "nt", "Unix session termination contract")
class TerminationTests(unittest.TestCase):
    def assert_not_running(self, pid):
        deadline = time.monotonic() + 2
        while True:
            state = subprocess.run(["ps", "-o", "stat=", "-p", str(pid)],
                                   capture_output=True, text=True, timeout=2).stdout.strip()
            if not state or state.startswith("Z"):
                return
            if time.monotonic() >= deadline:
                self.fail(f"detached process-group descendant survived: {state}")
            time.sleep(0.02)

    def test_timeout_kills_descendant_in_separate_process_group(self):
        self.assert_session_cleanup(None)

    def test_exited_launcher_leaves_no_detached_descendant(self):
        for exit_code in (0, 1):
            with self.subTest(exit_code=exit_code):
                self.assert_session_cleanup(exit_code)

    def assert_session_cleanup(self, exit_code):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            pid_file = root / "child.pid"
            child = ("import os,time,json; from pathlib import Path; os.setpgrp(); "
                     f"Path({str(pid_file)!r}).write_text(json.dumps([os.getpid(), os.getpgrp(), os.getsid(0)])); "
                     "time.sleep(60)")
            launcher = ("import subprocess,sys,time\nfrom pathlib import Path\n"
                        f"p=subprocess.Popen([sys.executable, '-c', {child!r}])\n"
                        f"while not Path({str(pid_file)!r}).exists(): time.sleep(0.01)\n"
                        + ("time.sleep(60)" if exit_code is None else f"sys.exit({exit_code})"))
            pid = None
            unrelated = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(60)"],
                                         start_new_session=True)
            try:
                code, timed_out = RUNNER["run_command"](
                    [sys.executable, "-c", launcher], os.environ.copy(),
                    root / "stdout", root / "stderr", 0.5 if exit_code is None else 5)
                self.assertEqual(timed_out, exit_code is None)
                if exit_code is None:
                    self.assertNotEqual(code, 0)
                else:
                    self.assertEqual(code, exit_code)
                pid, group, session = json.loads(pid_file.read_text())
                self.assertEqual(pid, group)
                self.assertNotEqual(group, session)
                self.assert_not_running(pid)
                self.assertIsNone(unrelated.poll(), "terminated a process outside the owned session")
            finally:
                unrelated.kill()
                unrelated.wait(timeout=5)
                if pid is None and pid_file.exists():
                    pid = json.loads(pid_file.read_text())[0]
                if pid is not None:
                    try:
                        os.kill(pid, signal.SIGKILL)
                    except ProcessLookupError:
                        pass

    def test_runner_sigterm_cleans_session_and_reports_interruption(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            config = root / "atlassian-cli"
            config.mkdir()
            (config / "config.json").write_text(json.dumps({"sites": {"fixture": {
                "product": "jira", "token_style": "cloud-classic", "base_url": "http://fixture.invalid"}}}))
            bin_dir = root / "bin"
            bin_dir.mkdir()
            pid_file = root / "child.pid"
            child = ("import os,time,json; from pathlib import Path; os.setpgrp(); "
                     f"Path({str(pid_file)!r}).write_text(json.dumps([os.getpid(), os.getpgrp(), os.getsid(0)])); "
                     "time.sleep(60)")
            # Replace only the external Go command, so the real runner parses
            # arguments/config, starts a cell, and writes the actual report.
            fake_go = bin_dir / "go"
            fake_go.write_text(f"#!{sys.executable}\nimport subprocess,sys,time\n"
                               "if '-list' in sys.argv:\n print('TestJiraStatus'); sys.exit(0)\n"
                               f"subprocess.Popen([sys.executable, '-c', {child!r}])\n"
                               "time.sleep(60)\n")
            fake_go.chmod(0o700)
            env = dict(os.environ, XDG_CONFIG_HOME=str(root), PATH=str(bin_dir) + os.pathsep + os.environ["PATH"])
            env.pop("CI", None)
            output = root / "report"
            runner = subprocess.Popen(
                [sys.executable, str(Path(__file__).with_name("e2e-matrix.py")),
                 "--jira-classic", "fixture", "--jira-project", "TEST", "--output", str(output)],
                env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
            unrelated = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(60)"],
                                         start_new_session=True)
            try:
                deadline = time.monotonic() + 5
                while not pid_file.exists():
                    if runner.poll() is not None or time.monotonic() >= deadline:
                        self.fail("runner did not start the fixture descendant")
                    time.sleep(0.02)
                pid, group, session = json.loads(pid_file.read_text())
                self.assertEqual(pid, group)
                self.assertNotEqual(group, session)
                runner.send_signal(signal.SIGTERM)
                stdout, stderr = runner.communicate(timeout=10)
                self.assertEqual(runner.returncode, 1, (stdout, stderr))
                self.assert_not_running(pid)
                self.assertIsNone(unrelated.poll(), "terminated an unrelated session")
                report = json.loads((output / "summary.json").read_text())
                self.assertEqual(report["status"], "fail")
                self.assertEqual(report["error"], "interrupted")
                self.assertEqual(report["cells"]["jira-classic"]["status"], "blocked")
            finally:
                if runner.poll() is None:
                    runner.kill()
                runner.communicate(timeout=5)
                unrelated.kill()
                unrelated.wait(timeout=5)
                if pid_file.exists():
                    RUNNER["terminate_session"](json.loads(pid_file.read_text())[2])


if __name__ == "__main__":
    unittest.main()
