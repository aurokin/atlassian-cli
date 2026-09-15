"""Hermetic checks for matrix result accounting; never starts live tests."""

import contextlib
import io
import json
import os
from pathlib import Path
import re
import runpy
import signal
import subprocess
import sys
import tempfile
import time
import unittest
from unittest import mock


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


@unittest.skipIf(os.name == "nt", "Live matrix runner requires Unix")
class ReadOnlyMatrixTests(unittest.TestCase):
    """Exercise real argument/config/report handling; replace only external Go."""

    inventory = ["TestJiraStatus", "TestConfStatus", "TestBitbucketStatus",
                 "TestOAuthRefreshPersistence", "TestRestrictedAccess",
                 "TestBitbucketIndependentReviewer", "TestBitbucketProjectAdministration",
                 "TestBitbucketPipelinesAndDeployments"]
    fixtures = ["--jira-project", "TEST", "--conf-space", "SPACE",
                "--bb-workspace", "workspace", "--bb-repo", "repository"]

    def run_fixture(self, arguments, *, skip_profile=None, inventory=None, wrong_style=False):
        calls = []
        inventory = self.inventory if inventory is None else inventory
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            config = root / "atlassian-cli"
            config.mkdir()
            profiles = {}
            for name, product, style in (("jira", "jira", "cloud-scoped"),
                                         ("conf", "confluence", "cloud-scoped"),
                                         ("bb", "bitbucket", "cloud-classic")):
                profiles[name + "-reader"] = {
                    "product": product, "token_style": style,
                    "base_url": "https://fixture.invalid/" + name,
                    "token_ref": "never-copy-this-reference-to-report"}
                profiles[name + "-owner"] = {
                    "product": product, "token_style": "cloud-classic",
                    "base_url": "https://fixture.invalid/" + name}
            if wrong_style:
                profiles["jira-reader"]["token_style"] = "cloud-classic"
            (config / "config.json").write_text(json.dumps({"sites": profiles}))
            output = root / "report"

            def fake_go(command, env, stdout, stderr, timeout):
                calls.append((list(command), dict(env)))
                stderr.write_text("")
                if "-list" in command:
                    stdout.write_text("\n".join(inventory) + "\n")
                    return 0, False
                pattern = command[command.index("-run") + 1]
                names = [name for name in inventory if re.fullmatch(pattern, name)]
                events = [
                    {"Action": "output", "Output": "build identity: binary=fixture sha256=fixture\n"},
                    {"Action": "output", "Output": "Go source tree sha256=" + "a" * 64 + "\n"},
                ]
                for name in names:
                    action = "skip" if skip_profile and env.get("ATL_IT_ACCESS_SITE") == skip_profile else "pass"
                    events.append({"Action": action, "Test": name})
                events.append({"Action": "pass"})
                stdout.write_text("".join(json.dumps(event) + "\n" for event in events))
                return 0, False

            env = {"XDG_CONFIG_HOME": str(root), "ATL_IT_ACCESS_SITE": "stale-reader",
                   "ATL_IT_ACCESS_OWNER_SITE": "stale-owner", "ATL_IT_JIRA_SITE": "stale-jira",
                   "ATL_IT_BB_REVIEWER": "1", "ATL_IT_BB_REVIEWER_SITE": "stale-reviewer",
                   "ATL_RUN_INTEGRATION": "0", "ATL_SITE": "stale-site", "ATL_TIMEOUT": "1ns"}
            argv = ["e2e-matrix.py", *arguments, *self.fixtures, "--output", str(output)]
            old_umask = os.umask(0o077)
            try:
                with mock.patch.dict(os.environ, env, clear=True), mock.patch.object(sys, "argv", argv), \
                        mock.patch.dict(RUNNER["run_matrix"].__globals__, {"run_command": fake_go}), \
                        contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
                    code = RUNNER["run_matrix"]()
            finally:
                os.umask(old_umask)
            return code, json.loads((output / "summary.json").read_text()), calls

    def test_each_readonly_selection_requires_explicit_owner(self):
        for product in ("jira", "conf", "bb"):
            with self.subTest(product=product):
                argv = ["e2e-matrix.py", "--" + product + "-readonly", "reader"]
                stderr = io.StringIO()
                with mock.patch.dict(os.environ, {}, clear=True), mock.patch.object(sys, "argv", argv), \
                        mock.patch.dict(RUNNER["run_matrix"].__globals__, {"run_command": mock.Mock(side_effect=AssertionError("started Go before owner validation"))}), \
                        contextlib.redirect_stderr(stderr), self.assertRaises(SystemExit) as caught:
                    RUNNER["run_matrix"]()
                self.assertEqual(caught.exception.code, 2)
                self.assertIn("requires --" + product + "-readonly-owner", stderr.getvalue())

    def test_readonly_cells_target_correct_profiles_without_selecting_owner_workflows(self):
        arguments = []
        for product in ("jira", "conf", "bb"):
            arguments += ["--" + product + "-readonly", product + "-reader",
                          "--" + product + "-readonly-owner", product + "-owner"]
        code, report, calls = self.run_fixture(arguments)
        self.assertEqual(code, 0)
        self.assertEqual(report["status"], "pass")
        self.assertEqual(set(report["cells"]), {"jira-readonly", "conf-readonly", "bb-readonly"})
        self.assertEqual(len(calls), 4)  # Inventory plus exactly three requested cells.
        for product, (command, env) in zip(("jira", "conf", "bb"), calls[1:]):
            self.assertEqual(env["ATL_IT_ACCESS_SITE"], product + "-reader")
            self.assertEqual(env["ATL_IT_ACCESS_OWNER_SITE"], product + "-owner")
            self.assertNotIn("ATL_IT_JIRA_SITE", env)
            self.assertNotIn("ATL_SITE", env)
            self.assertNotIn("ATL_TIMEOUT", env)
            self.assertEqual(env["ATL_RUN_INTEGRATION"], "1")
            self.assertEqual(env["ATL_IT_USE_STORED_PROFILES"], "1")
            self.assertEqual(command[command.index("-run") + 1], "^(TestRestrictedAccess)$")
            self.assertEqual(report["cells"][product + "-readonly"]["expected"], ["TestRestrictedAccess"])
        self.assertEqual(calls[1][1]["ATL_IT_JIRA_PROJECT"], "TEST")
        self.assertEqual(calls[2][1]["ATL_IT_CONF_SPACE"], "SPACE")
        self.assertEqual(calls[3][1]["ATL_IT_BB_WORKSPACE"], "workspace")
        self.assertEqual(calls[3][1]["ATL_IT_BB_REPO"], "repository")
        self.assertNotIn("never-copy-this-reference-to-report", json.dumps(report))

    def test_core_and_readonly_selection_keep_families_separate(self):
        code, report, calls = self.run_fixture([
            "--jira-classic", "jira-owner", "--jira-readonly", "jira-reader",
            "--jira-readonly-owner", "jira-owner"])
        self.assertEqual(code, 0)
        self.assertEqual(report["cells"]["jira-classic"]["expected"], ["TestJiraStatus"])
        self.assertEqual(report["cells"]["jira-readonly"]["expected"], ["TestRestrictedAccess"])
        self.assertNotIn("ATL_IT_ACCESS_SITE", calls[1][1])
        self.assertNotIn("ATL_IT_ACCESS_OWNER_SITE", calls[1][1])

    def test_skipped_restriction_cannot_report_success(self):
        code, report, _ = self.run_fixture([
            "--conf-readonly", "conf-reader", "--conf-readonly-owner", "conf-owner"],
            skip_profile="conf-reader")
        self.assertEqual(code, 1)
        self.assertEqual(report["status"], "fail")
        self.assertEqual(report["cells"]["conf-readonly"]["skipped"], ["TestRestrictedAccess"])

    def test_missing_restricted_inventory_cannot_report_success(self):
        code, report, _ = self.run_fixture([
            "--bb-readonly", "bb-reader", "--bb-readonly-owner", "bb-owner"],
            inventory=["TestBitbucketStatus"])
        self.assertEqual(code, 1)
        self.assertEqual(report["status"], "fail")
        self.assertNotEqual(report["cells"]["bb-readonly"]["status"], "pass")

    def test_wrong_readonly_transport_blocks_before_starting_tests(self):
        code, report, calls = self.run_fixture([
            "--jira-readonly", "jira-reader", "--jira-readonly-owner", "jira-owner"],
            wrong_style=True)
        self.assertEqual(code, 1)
        self.assertEqual(report["cells"]["jira-readonly"]["status"], "blocked")
        self.assertEqual(calls, [])
        self.assertIn("jira/cloud-scoped", report["error"])

    def test_unselected_bitbucket_capabilities_are_explicitly_excluded(self):
        code, report, calls = self.run_fixture(["--bb", "bb-owner"])
        self.assertEqual(code, 0)
        cell = report["cells"]["bb"]
        self.assertEqual(cell["expected"], ["TestBitbucketStatus"])
        self.assertEqual(set(cell["excluded_tests"]), {
            "TestBitbucketIndependentReviewer", "TestBitbucketProjectAdministration",
            "TestBitbucketPipelinesAndDeployments"})
        self.assertIn("--bb-reviewer", cell["excluded_tests"]["TestBitbucketIndependentReviewer"])
        self.assertNotIn("ATL_IT_BB_REVIEWER", calls[1][1])
        self.assertNotIn("ATL_IT_BB_REVIEWER_SITE", calls[1][1])

    def test_selected_reviewer_receives_companion_profile_and_runs_in_owner_cell(self):
        code, report, calls = self.run_fixture([
            "--bb", "bb-owner", "--bb-reviewer", "independent-reviewer"])
        self.assertEqual(code, 0)
        self.assertEqual(set(report["cells"]), {"bb"})
        cell = report["cells"]["bb"]
        self.assertEqual(set(cell["expected"]), {"TestBitbucketStatus", "TestBitbucketIndependentReviewer"})
        self.assertNotIn("TestBitbucketIndependentReviewer", cell["excluded_tests"])
        self.assertEqual(calls[1][1]["ATL_IT_BB_SITE"], "bb-owner")
        self.assertEqual(calls[1][1]["ATL_IT_BB_REVIEWER"], "1")
        self.assertEqual(calls[1][1]["ATL_IT_BB_REVIEWER_SITE"], "independent-reviewer")

    def test_selected_reviewer_missing_from_inventory_blocks_before_live_cell(self):
        code, report, calls = self.run_fixture([
            "--bb", "bb-owner", "--bb-reviewer", "independent-reviewer"],
            inventory=["TestBitbucketStatus"])
        self.assertEqual(code, 1)
        self.assertEqual(report["cells"]["bb"]["status"], "blocked")
        self.assertIn("selected capability missing from compiled test inventory: TestBitbucketIndependentReviewer",
                      report["error"])
        self.assertEqual(len(calls), 1)  # Discovery only; no live subprocess.

    def test_reviewer_requires_explicit_bitbucket_owner_cell(self):
        argv = ["e2e-matrix.py", "--jira-classic", "jira-owner", "--bb-reviewer", "reviewer"]
        stderr = io.StringIO()
        with mock.patch.dict(os.environ, {}, clear=True), mock.patch.object(sys, "argv", argv), \
                mock.patch.dict(RUNNER["run_matrix"].__globals__, {"run_command": mock.Mock(side_effect=AssertionError("started Go without owner selection"))}), \
                contextlib.redirect_stderr(stderr), self.assertRaises(SystemExit) as caught:
            RUNNER["run_matrix"]()
        self.assertEqual(caught.exception.code, 2)
        self.assertIn("Bitbucket capabilities require --bb PROFILE", stderr.getvalue())


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
