# E2E validation — 2026-09-15

The selected live matrix passed **72/72 top-level test executions**, with zero
skips, missing tests, or failures. This validates the core workflows below;
it does not establish full behavioral coverage of every command or flag.
At validation time, changes were uncommitted on `fix/api-sync`; no push or merge
had been performed.

## Live results

Run window: 15:06:32–15:12:47 UTC, Go 1.26.5 on macOS arm64.
Jira used the disposable `KAN` project on `aurotest.atlassian.net`;
Confluence used the regular `ATLE2E` space. Bitbucket used `OhBizzle`, with
all mutations confined to repositories created by the tests.

| Product / authentication | Profile | Passed / selected |
| --- | --- | --- |
| Jira classic API token | `jira-classic` | 8 / 8 |
| Jira scoped API token | `jira-scoped` | 8 / 8 |
| Confluence classic API token | `conf-classic` | 12 / 12 |
| Confluence scoped API token | `conf-scoped` | 12 / 12 |
| Bitbucket scoped API token over Basic transport | `bb` | 10 / 10 |
| Jira OAuth workflows | `work` | 8 / 8 |
| Confluence OAuth workflows | `smoke-conf` | 12 / 12 |
| Jira forced OAuth refresh and persistence | `work` | 1 / 1 |
| Confluence forced OAuth refresh and persistence | `smoke-conf` | 1 / 1 |

Bitbucket's `cloud-classic` configuration label describes its transport here;
the credential is a scoped Bitbucket API token. This is one tested Bitbucket
authentication mode, not two independent classic/scoped credential runs.

Jira scenarios cover discovery, search, issue edits, comments, assignment,
watching, worklogs, transitions, links, forced pagination, and binary attachment
round trips. Confluence covers discovery, CQL, page and blogpost lifecycles,
storage/ADF body preservation during title edits, versions, labels, comments,
child pagination, ancestor membership, and binary attachments. Bitbucket covers
repository lifecycles, branches, tags, source bytes, commits, PR pagination,
comments, decline, merge, and destination-commit readback.

OAuth refresh tests expired the dedicated local grant, verified access and
refresh token rotation and persistence, and checked identity from another CLI
process. Scope and permission failures cannot silently become passing skips in
selected matrix cells.

## Fixes and repeatable verification

Four production fixes are included, with regression tests:

- Accept `ssh.bitbucket.org` Git remotes for repository inference.
- Classify OAuth transport timeouts consistently, including response-body reads.
- Decode numeric Jira attachment IDs without losing precision, while preserving
  upstream raw JSON. Live attachment download exposed this failure.
- Preserve the trailing slash for Bitbucket source-root requests. Live source
  listing exposed the previous 404.

The new hermetic suite invokes all three real binaries with isolated config and
local HTTP fixtures. It checks error/exit contracts, timeout behavior, redaction,
JSON/jq output, pagination, destructive guards, credential persistence, and Git
inference. The command inventory accounts for 152 canonical runnable commands
with evidence references or explicit gaps; accounting is not a claim that all
152 commands have complete E2E coverage.

| Local check | Result |
| --- | --- |
| `make check` — formatting, compilation, tagged integration compilation, vet, hermetic tests | Passed |
| `go test -race ./...` | Passed |
| `go test -tags=integration ./integration -run '^TestHarness' -count=1` | Passed |
| `golangci-lint run --build-tags integration ./...` (v2.1.6) | Passed, zero issues |
| Python matrix accounting/cancellation tests | Passed, 8 tests |
| `make docs-check` | Passed |
| `git diff --check` | Passed |

Formatting targets now include `e2e/`. CI includes the hermetic harness and
matrix-runner tests, plus macOS/Windows process-contract jobs. Those new remote
CI jobs have **not** run for these uncommitted changes.

The runner rejects missing/skipped tests and inconsistent Go source digests.
Its Unix timeout cleanup also has a regression test for a CLI child in its own
process group and for preserving unrelated sessions. That cancellation-only
Python fix was completed during the live run; the live run did not time out or
exercise cancellation. Windows live matrix execution is explicitly unsupported
until equivalent process-tree cancellation exists.

## Cleanup and credentials

The final run's seven cleanup ledgers contain **52 owned resources**, all with
verified deletion records: 12 Jira issues, 33 Confluence pages, 3 blogposts, and
4 private Bitbucket repositories. No pending record remains in those ledgers.
Page cleanup includes purge and subsequent not-found verification. Child data
is removed through explicit lifecycle operations or its owning container.
The durable test project, space, and existing read-only Bitbucket fixture remain
available for repeat runs.

Credentials and OAuth grants were restored for the tested free accounts. The
insufficient Confluence scoped token was replaced and revoked after verification;
the five API tokens display an expiry of December 13, 2026. No subscription was
purchased or changed. Temporary reauthorization helpers were removed, the browser
session was closed, and the temporary full-user Proton Pass session was logged
out. Credentials are outside the repository and are not included in this report.

## Evidence identity

Base commit: `d390c95e48eef68e48988b1a27af4a92223cfec9`, with working-tree changes.
Every final matrix cell recorded the same Go source digest (tracked and untracked
Go sources, `go.mod`, and `go.sum`):

```text
fa6a9d74e86d3b8d3c5bfba33817111f58aca4625504e00aa705f7faadde779f
```

| Binary | SHA-256 |
| --- | --- |
| `atl-jira` | `c8d70a51e963a6a77a9b23e7b9225ac1d42a4556a24e21a6265e4c168ac2c7bd` |
| `atl-conf` | `0df53abcefee38608f2dbfc275231f83839cec957de85b24bba11786c050e49a` |
| `atl-bb` | `4c32f0afb0a55a9e784142b3d3601a1e5cd55e0766fd41f9d378064487f942b1` |

Local raw evidence is in `/private/tmp/atl-final-matrix-20260915-1506/summary.json`
and adjacent per-cell JSONL/stderr logs. The summary references protected cleanup
ledgers. These temporary local artifacts are not committed and may expire.
Tracked-diff hashes changed during documentation/runner edits; the Go digest and
per-product binary hashes remained consistent across all cells.

## Remaining coverage limits

- Live restricted-user denials and independent-reviewer approve/unapprove need
  another identity with deliberately different permissions.
- Pipelines/deployments need an enabled fixture and explicit execution budget;
  optional Bitbucket project administration was not selected.
- Confluence mixed non-page children have process-fixture coverage, not live
  fixture coverage. Ancestor membership passed; a live multi-page ancestor chain
  and grandchild exclusion were not established.
- Native Linux/Windows results, broader keychain/extension workflows, and command
  gaps recorded in `e2e/coverage.json` remain unverified here.
- Cleanup ledgers support exact-ID manual recovery, not automatic replay. A
  successful creation whose response cannot yield an ID still needs manual
  reconciliation; the suite cannot prove cleanup for such an unknown resource.

See [the design](e2e-test-design.md), [runbook](integration-testing.md), and
[deferred ergonomics notes](ergonomics-redesign-notes.md). The main assumption
that may not generalize is that these free-account capabilities and permissions
represent the environments used by other consumers.

## Subsequent review corrections

The first Diffwarden pass found two valid P2 harness/runner defects. Stored
profiles referencing `env:ATL_API_TOKEN` lost that variable in CLI subprocesses;
the filter now removes only known CLI selectors, with a real-binary local-server
regression proving stored-profile authentication. The matrix runner also now
terminates remaining session members after an early Go exit, as well as after
its own timeout. A regression covers both successful and failed launchers leaving
a child in a separate process group, while preserving unrelated sessions.

The second pass found a further P2: default SIGTERM handling bypassed cleanup.
The runner now routes SIGTERM through cleanup and interrupted-result reporting,
ignoring repeat termination requests while handling the first. A real-runner
subprocess regression verifies child termination, preservation of unrelated
sessions, and a failed/interrupted summary without accessing live accounts.

The hermetic harness tests (also under the race detector) and ten Python runner
tests pass after those fixes; tagged golangci-lint reports zero issues.
These later changes affect the harness
and runner, not production CLI sources. The 72-test live results and source digest
above describe the earlier run; that full live matrix was not repeated for these
failure-path fixes.

The third unscoped Diffwarden working-tree review (`codex-gpt56-sol`) completed
with zero findings, one successful reviewer, and no warnings. All three P2
findings from the earlier rounds were accepted and fixed; none were dismissed.
Diffwarden used source inspection only; executed checks are reported separately
above. Local review artifacts are `/tmp/atl-loop-review-20260915-round1.json`
through `round3.json`.
