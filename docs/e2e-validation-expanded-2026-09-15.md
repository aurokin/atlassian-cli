# Expanded E2E validation — 2026-09-15

Historical result: the second-account blocker below was resolved by the
[September 19 validation](e2e-validation-2026-09-19.md), which passed the complete
81-execution matrix including independent-user denial and PR approval/unapproval.

## Merge decision

**High confidence in the tested behavior; keep the PR draft until the remaining
independent-user gate is resolved.** All 80 selected live executions passed,
with no skips or missing tests. Native-platform CI and the full branch review
are green. Nothing has been merged and auto-merge is disabled.

The optional independent-user denial/approval workflow is implemented but was
explicitly excluded before execution because a second account is unavailable.
The owner confirmed on 2026-09-15 that no second disposable account is available
and instructed that these cases be reported as blocked. This is not a waiver
or permission to merge.
It is not part of the 80 selected executions and is not certified. Supplying
another token for the same account does not satisfy this gate.

## Source and evidence

- Live-tested code: `eb3255b2ed01842605f136de841e87f1ba0b18c3`, clean tracked tree.
- Main comparison: `50a50599bd8be089b3109afd721e893a9b89f193`.
- Go source-tree SHA-256: `3da2620ce268036b2933ce4457ef83d59c96fb92ab52dd74096788701b4480fa`.
- Runtime: Go 1.26.5, macOS arm64; execution 18:22:52–18:43:01 UTC.
- Matrix: `/tmp/atl-final-expanded-matrix-eb3255b/summary.json`; individual Go
  JSON event logs and stderr files are alongside it.
- [CI for the tested revision](https://github.com/aurokin/atlassian-cli/actions/runs/35007090162):
  all six jobs passed. This includes formatting, compile/vet, tagged harness
  tests, lint, race/coverage, 20 matrix-accounting tests, Linux/macOS/Windows CLI
  processes, native macOS/Windows credential persistence and shell completion.
- Local integration-tagged `golangci-lint` also passed with zero issues.
- Full branch review against main: zero findings, valid review result, no
  reviewer warnings; `/tmp/atl-final-eb3255b-review.json`.

This report and its documentation links are a documentation-only follow-up;
they do not change the tested Go source or harness.

| Binary | SHA-256 |
|---|---|
| atl-jira | `f17b0f7340ad7e948f01b0bc640544befd62cb7d77621cd68be522a52c5b6018` |
| atl-conf | `ca51d77d18527cae1dccf1ad150a86a4a08b6c907860c92fbec0975fa13c3457` |
| atl-bb | `5867668400705e0d74a9f554782e98f0d3a56eb5b513062e202c9c680bf36d78` |

## Live matrix

| Cell | Passed / selected |
|---|---:|
| jira-classic | 8 / 8 |
| jira-scoped | 8 / 8 |
| conf-classic | 13 / 13 |
| conf-scoped | 13 / 13 |
| bb | 12 / 12 |
| jira-oauth | 8 / 8 |
| conf-oauth | 13 / 13 |
| jira-readonly | 1 / 1 |
| conf-readonly | 1 / 1 |
| bb-readonly | 1 / 1 |
| jira-oauth-refresh | 1 / 1 |
| conf-oauth-refresh | 1 / 1 |

The Bitbucket cell includes owned project administration and bounded
Pipelines/deployments. Bitbucket scoped API tokens use the CLI's `cloud-classic`
Basic transport; there is no second Bitbucket cloud-ID gateway cell. Jira and
Confluence each exercise classic, scoped and OAuth routing.

Both OAuth refresh cells verify credential rotation/persistence and a subsequent
process's identity. Read-only cells prove successful reads, scope-specific write
denial, and unchanged owner readback. These are same-account scope tests, not
separate-user resource permissions.

## Failures found and corrected

The expansion caught production bugs in Confluence ancestor pagination and
Windows extension resolution. Ancestors now traverse the first ancestor's
ID/type upward, preserve top-to-bottom order, detect cycles, and retain the
request cap. Windows extension names resolve case-insensitively while retaining
the executable path. Process and live/native checks verify the fixes.

The first expanded matrix on `fae0662` passed 74/80; its six failures were CQL
and text search under all three Confluence auth modes. Direct content reads
worked while newly published content was absent from search. A controlled
fixture became searchable between 298 and 331 seconds under all three modes
and remained searchable through 600 seconds. Both diagnostic pages were purged
and deletion verified.

Atlassian [describes minute-scale search propagation](https://jira.atlassian.com/browse/CONFCLOUD-80582).
The old 45/60-second deadlines were too short for this measured behavior. The
suite now bounds readiness at eight minutes, polls every ten seconds with a
ten-second request timeout, and logs elapsed time/attempts. Exact results remain
required; unexpected results and HTTP/authentication errors fail immediately.
Only empty search reads retry; creation never retries. The final six searches
passed with observed delays from 42 seconds to 3 minutes 7 seconds.

Other harness fixes covered PowerShell completion environment handling, an
isolated unlocked macOS CI keychain, delayed Bitbucket environment listing, and
suppression of automatic fixture builds with `[skip ci]`. No selected failure
was changed into a skip or exclusion to obtain the passing matrix.

## Coverage and cleanup

All 152 canonical runnable commands require actual-binary process or live
evidence in [the manifest](../e2e/coverage.json). Shared workflows include auth,
target precedence, aliases, extensions, resolve/browse behavior, shell completion,
raw request bodies, output shapes and interrupted downloads. Live workflows
include owned mutations with independent readback, pagination, binary attachment
round trips, Jira email assignment/worklogs, Confluence children/ancestors and
Bitbucket PR search, projects, pipelines, logs, deployments and stop state.

All 63 resources in the final matrix's ten cleanup ledgers end in `deleted`,
with deletion verification performed by the suite. The prior failed matrix's
63 resources, 49 resources recorded in earlier pilot logs, and both diagnostic
pages were also cleaned up. Evidence logs and ledgers are retained for audit.

Pipeline fixtures were private, owned repositories with size-1x, one-minute
steps; cleanup verifies terminal runs before deleting the repository. The Free
plan had ample verified quota. No subscriptions, credit cards, paid capacity or
unrelated repositories were changed.

The replacement Bitbucket token is verified and its superseded token revoked.
The new Bitbucket and three read-only tokens expire September 22, 2026. They are
stored through the CLI credential backend; no raw credentials were committed.
Task-only token automation and portable PowerShell artifacts were removed, and
the task browser session was closed.

## Remaining acceptance

**Blocked: no second disposable account is available.** Resuming this gate
requires a Bitbucket workspace member without inherited access to new private
repositories. `--bb-reviewer PROFILE` verifies denied access
before an owned-repository grant, then independent approval/unapproval and
cleanup. The owner's token also needs `write:permission:bitbucket`. These live
prerequisites and the workflow itself remain unverified; do not mark them passed.

Mixed non-page Confluence children have process evidence rather than live
fixtures. The command inventory does not imply every flag, content type,
permission or auth permutation is covered. Browser OAuth authorization remains
a separately performed setup step. The broader ergonomics redesign remains
[backlogged](ergonomics-redesign-notes.md).

See [the merge gate](e2e-merge-gate.md) and [run instructions](integration-testing.md).
