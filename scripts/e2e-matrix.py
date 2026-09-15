#!/usr/bin/env python3
"""Run explicitly selected stored-profile live suites with strict accounting."""

import argparse
import datetime
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import tempfile
import time


ROOT = Path(__file__).resolve().parents[1]
CELLS = {
    "jira-classic": ("Jira", "JIRA", "jira", "cloud-classic"),
    "jira-scoped": ("Jira", "JIRA", "jira", "cloud-scoped"),
    "conf-classic": ("Conf", "CONF", "confluence", "cloud-classic"),
    "conf-scoped": ("Conf", "CONF", "confluence", "cloud-scoped"),
    "bb": ("Bitbucket", "BB", "bitbucket", "cloud-classic"),
    "jira-oauth": ("Jira", "JIRA", "jira", "oauth-3lo"),
    "conf-oauth": ("Conf", "CONF", "confluence", "oauth-3lo"),
    "jira-oauth-refresh": ("OAuth", "OAUTH", "jira", "oauth-3lo"),
    "conf-oauth-refresh": ("OAuth", "OAUTH", "confluence", "oauth-3lo"),
    "jira-readonly": ("Restricted", "ACCESS", "jira", "cloud-scoped"),
    "conf-readonly": ("Restricted", "ACCESS", "confluence", "cloud-scoped"),
    "bb-readonly": ("Restricted", "ACCESS", "bitbucket", "cloud-classic"),
}


def terminate_session(session_id):
    """Stop forks before killing every member, including detached process groups.

    The runner owns this new session. Verify session membership immediately
    before signaling each PID; process-group membership is insufficient because
    the Go harness puts each CLI subprocess in its own group.
    """
    members = set()

    def signal_member(pid, sig):
        try:
            if os.getsid(pid) == session_id:
                os.kill(pid, sig)
        except ProcessLookupError:
            pass

    signal_member(session_id, signal.SIGSTOP)
    try:
        deadline = time.monotonic() + 5
        while True:
            listing = subprocess.run(["ps", "-axo", "pid="], capture_output=True,
                                     text=True, check=True, timeout=2)
            current = set()
            for value in listing.stdout.split():
                pid = int(value)
                try:
                    if os.getsid(pid) == session_id:
                        current.add(pid)
                except (ProcessLookupError, PermissionError):
                    # Some platforms hide other users' session metadata. All
                    # descendants we own retain our user and remain queryable.
                    pass
            for pid in current:
                signal_member(pid, signal.SIGSTOP)
            new = current - members
            members.update(current)
            if not new:
                break
            if time.monotonic() >= deadline:
                raise RuntimeError("could not quiesce the owned test session before deadline")
            time.sleep(0.01)
    finally:
        # Kill children before their session leader; this also runs if process
        # enumeration fails, with a visible error rather than false completion.
        for pid in members - {session_id}:
            signal_member(pid, signal.SIGKILL)
        signal_member(session_id, signal.SIGKILL)


def run_command(command, env, stdout_path, stderr_path, timeout):
    """Leave diagnostic files intact even after a timeout or interruption."""
    if os.name == "nt":
        raise ValueError("live matrix runner requires Unix session termination; Windows is not supported")
    with stdout_path.open("w") as stdout, stderr_path.open("w") as stderr:
        process = subprocess.Popen(
            command, cwd=ROOT, env=env, stdout=stdout, stderr=stderr,
            start_new_session=True,
        )
        timed_out = False
        try:
            try:
                process.wait(timeout=timeout)
            except subprocess.TimeoutExpired:
                timed_out = True
        finally:
            # Go can exit on its own package deadline before our outer timer,
            # leaving CLI descendants alive in separate process groups. Sweep
            # the owned session on every exit, even after a successful launcher.
            try:
                terminate_session(process.pid)
            finally:
                process.wait(timeout=10)
        return process.returncode, timed_out


def account_events(path, expected, returncode, timed_out):
    terminals = {}
    skipped, failed, malformed = [], [], []
    identities, ledgers, source_digests = [], set(), set()
    package_pass = False
    with path.open() as stream:
        for number, line in enumerate(stream, 1):
            try:
                event = json.loads(line)
            except json.JSONDecodeError:
                malformed.append(number)
                continue
            action, test = event.get("Action"), event.get("Test")
            if test and action in ("pass", "fail", "skip"):
                terminals[test] = action
            if action == "skip":
                skipped.append(test or "package")
            if action == "fail":
                failed.append(test or "package")
            if not test and action == "pass":
                package_pass = True
            output = event.get("Output", "")
            if "build identity:" in output:
                identities.append(output.strip())
            source_digests.update(re.findall(r"Go source tree sha256=([0-9a-f]{64})", output))
            for ledger in re.findall(r"cleanup ledger (.+?\.jsonl)", output):
                ledgers.add(ledger)
    missing = sorted(set(expected) - terminals.keys())
    passed = sorted(test for test in expected if terminals.get(test) == "pass")
    ok = (returncode == 0 and not timed_out and package_pass and bool(expected)
          and not skipped and not failed and not missing and not malformed
          and len(passed) == len(expected) and bool(identities) and len(source_digests) == 1)
    return {
        "status": "pass" if ok else "fail", "exit_code": returncode,
        "timed_out": timed_out, "expected": sorted(expected), "passed": passed,
        "failed": sorted(set(failed)), "skipped": sorted(set(skipped)),
        "missing": missing, "malformed_json_lines": malformed,
        "package_pass": package_pass, "build_identity": identities,
        "source_tree_sha256": sorted(source_digests),
        "cleanup_ledgers": sorted(ledgers),
    }


def profile_metadata(selected):
    # Config holds profile metadata and references, not credential material.
    # Never open credentials.json or resolve token_ref in the runner.
    config_home = Path(os.environ.get("XDG_CONFIG_HOME", str(Path.home() / ".config")))
    config = json.loads((config_home / "atlassian-cli" / "config.json").read_text())
    profiles = config.get("sites", {})
    targets = {}
    for cell, profile in selected.items():
        metadata = profiles.get(profile)
        product, style = CELLS[cell][2:]
        if not metadata or metadata.get("product") != product or metadata.get("token_style") != style:
            raise ValueError(f"{cell}: profile {profile!r} must be {product}/{style}")
        targets[cell] = {key: metadata.get(key, "") for key in
                         ("product", "token_style", "base_url", "api_base_url", "cloud_id")}
    return targets


def write_report(directory, report):
    temporary = directory / "summary.json.tmp"
    temporary.write_text(json.dumps(report, indent=2) + "\n")
    temporary.replace(directory / "summary.json")


def run_matrix():
    parser = argparse.ArgumentParser(description=__doc__)
    for cell in CELLS:
        if not cell.endswith("-refresh"):
            parser.add_argument("--" + cell, metavar="PROFILE")
    parser.add_argument("--jira-project")
    parser.add_argument("--jira-issue-type", default="Task")
    parser.add_argument("--conf-space")
    parser.add_argument("--bb-workspace")
    parser.add_argument("--bb-repo")
    parser.add_argument("--bb-reviewer", metavar="PROFILE", help="select independent live approval with a second workspace member")
    parser.add_argument("--bb-project-admin", action="store_true", help="select owned live project administration")
    parser.add_argument("--bb-pipelines", action="store_true", help="select bounded live Pipelines/deployments; requires verified free build quota")
    for product in ("jira", "conf", "bb"):
        parser.add_argument("--" + product + "-readonly-owner", help="full-access companion profile for the read-only scope test")
    parser.add_argument("--jira-expected-account-id", help="require this Jira account ID in preflight")
    parser.add_argument("--conf-expected-account-id", help="require this Confluence account ID in preflight")
    parser.add_argument("--bb-expected-account-id", help="require this Bitbucket UUID in Bitbucket preflight")
    parser.add_argument("--output", type=Path, help="new directory for private logs/report (default: temporary)")
    parser.add_argument("--cell-timeout", type=int, default=1200, metavar="SECONDS")
    args = parser.parse_args()
    if os.name == "nt":
        parser.error("live matrix runner requires Unix session termination; Windows is not supported")
    selected = {cell: getattr(args, cell.replace("-", "_")) for cell in CELLS
                if not cell.endswith("-refresh") and getattr(args, cell.replace("-", "_"))}
    for product in ("jira", "conf"):
        if product + "-oauth" in selected:
            selected[product + "-oauth-refresh"] = selected[product + "-oauth"]
    if not selected:
        parser.error("select at least one explicit profile")
    if (args.bb_project_admin or args.bb_pipelines or args.bb_reviewer) and not args.bb:
        parser.error("Bitbucket capabilities require --bb PROFILE")
    for product in ("jira", "conf", "bb"):
        if product + "-readonly" in selected and not getattr(args, product + "_readonly_owner"):
            parser.error("read-only scope test requires --" + product + "-readonly-owner")
    if os.environ.get("CI"):
        parser.error("live integration is manual-only; CI must be unset")
    for prefix, flags in {"jira": ("jira_project",), "conf": ("conf_space",),
                          "bb": ("bb_workspace", "bb_repo")}.items():
        if any(cell.startswith(prefix) for cell in selected):
            for flag in flags:
                if not getattr(args, flag):
                    parser.error("selected product requires --" + flag.replace("_", "-"))
    if args.cell_timeout < 60:
        parser.error("--cell-timeout must be at least 60 seconds")

    os.umask(0o077)
    directory = args.output.resolve() if args.output else Path(tempfile.mkdtemp(prefix="atl-e2e-matrix-"))
    if args.output:
        directory.mkdir(parents=True, exist_ok=False)
    report = {"started_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
              "status": "running", "selected_profiles": selected,
              "cells": {cell: {"status": "not_run", "profile": profile} for cell, profile in selected.items()},
              "scope": "selected live test families; not a complete CLI command inventory"}
    write_report(directory, report)
    print(f"Artifacts: {directory}", flush=True)
    try:
        targets = profile_metadata(selected)
        # Eliminate stale test targeting and timeout/trace selectors. The CLI
        # alone resolves stored credentials; the runner never resolves them.
        env = {key: value for key, value in os.environ.items()
               if not key.startswith("ATL_IT_") and key not in ("ATL_RUN_INTEGRATION", "ATL_SITE", "ATL_TIMEOUT")}
        env.update(ATL_RUN_INTEGRATION="1", ATL_IT_USE_STORED_PROFILES="1")
        if args.bb_reviewer:
            env["ATL_IT_BB_REVIEWER"] = "1"
            env["ATL_IT_BB_REVIEWER_SITE"] = args.bb_reviewer
        if args.bb_project_admin:
            env["ATL_IT_BB_PROJECT_ADMIN"] = "1"
        if args.bb_pipelines:
            env["ATL_IT_BB_PIPELINES"] = "1"
        fixtures = {"JIRA_PROJECT": args.jira_project, "JIRA_ISSUE_TYPE": args.jira_issue_type,
                    "CONF_SPACE": args.conf_space, "BB_WORKSPACE": args.bb_workspace, "BB_REPO": args.bb_repo}
        env.update({"ATL_IT_" + key: value for key, value in fixtures.items() if value})
        for product in ("jira", "conf", "bb"):
            identity = getattr(args, product + "_expected_account_id")
            if identity:
                env["ATL_IT_" + product.upper() + "_EXPECTED_ACCOUNT_ID"] = identity

        code, timeout = run_command(
            ["go", "test", "-tags=integration", "./integration", "-list", "^Test(Jira|Conf|Bitbucket|OAuth|Restricted)"],
            env, directory / "inventory.txt", directory / "inventory.stderr", 120)
        if code != 0 or timeout:
            raise ValueError("test inventory failed; see inventory.stderr")
        inventory = [line.strip() for line in (directory / "inventory.txt").read_text().splitlines()
                     if re.fullmatch(r"Test(?:Jira|Conf|Bitbucket|OAuth|Restricted)\w+", line.strip())]
        if not inventory:
            raise ValueError("compiled test inventory is empty")

        for cell, profile in selected.items():
            family, prefix = CELLS[cell][:2]
            expected = [test for test in inventory if test.startswith("Test" + family)]
            excluded = {}
            for test, enabled, reason in (
                ("TestBitbucketIndependentReviewer", bool(args.bb_reviewer), "requires --bb-reviewer with a distinct live workspace member"),
                ("TestBitbucketProjectAdministration", args.bb_project_admin, "requires --bb-project-admin"),
                ("TestBitbucketPipelinesAndDeployments", args.bb_pipelines, "requires --bb-pipelines and verified free build quota"),
            ):
                if cell == "bb" and enabled and test not in expected:
                    raise ValueError("selected capability missing from compiled test inventory: " + test)
                if test in expected and not enabled:
                    expected.remove(test)
                    excluded[test] = reason
            report["cells"][cell] = {"status": "running", "profile": profile, "target": targets[cell], "excluded_tests": excluded}
            write_report(directory, report)
            print(f"Running {cell}: {len(expected)} tests using {profile}", flush=True)
            cell_env = dict(env, **{"ATL_IT_" + prefix + "_SITE": profile})
            if cell.endswith("-readonly"):
                cell_env["ATL_IT_ACCESS_OWNER_SITE"] = getattr(args, cell.split("-")[0] + "_readonly_owner")
            output, errors = directory / (cell + ".jsonl"), directory / (cell + ".stderr")
            code, timeout = run_command(
                ["go", "test", "-tags=integration", "./integration", "-count=1", "-json",
                 "-timeout", str(args.cell_timeout - 15) + "s", "-run", "^(" + "|".join(expected) + ")$"],
                cell_env, output, errors, args.cell_timeout)
            outcome = account_events(output, expected, code, timeout)
            for digest in outcome["source_tree_sha256"]:
                if "source_tree_sha256" not in report:
                    report["source_tree_sha256"] = digest
                elif report["source_tree_sha256"] != digest:
                    outcome["status"] = "fail"
                    outcome["source_tree_changed"] = True
            report["cells"][cell].update(outcome)
            write_report(directory, report)
            print(f"{cell}: {outcome['status']} ({len(outcome['passed'])}/{len(expected)} passed, "
                  f"{len(outcome['skipped'])} skipped, {len(outcome['missing'])} missing)", flush=True)
        report["status"] = "pass" if all(cell["status"] == "pass" for cell in report["cells"].values()) else "fail"
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError, KeyboardInterrupt) as error:
        report["status"] = "fail"
        report["error"] = "interrupted" if isinstance(error, KeyboardInterrupt) else str(error)
        for cell in report["cells"].values():
            if cell["status"] in ("running", "not_run"):
                cell["status"] = "blocked"
        print(report["error"], file=sys.stderr)
    finally:
        report["finished_at"] = datetime.datetime.now(datetime.timezone.utc).isoformat()
        write_report(directory, report)
    print(f"Result: {report['status']}; {directory / 'summary.json'}", flush=True)
    return 0 if report["status"] == "pass" else 1


def main():
    def interrupted(_signum, _frame):
        # A second termination request must not interrupt session cleanup or
        # leave the report half-written while handling the first request.
        signal.signal(signal.SIGTERM, signal.SIG_IGN)
        raise KeyboardInterrupt

    previous = signal.signal(signal.SIGTERM, interrupted)
    try:
        return run_matrix()
    finally:
        signal.signal(signal.SIGTERM, previous)


if __name__ == "__main__":
    sys.exit(main())
