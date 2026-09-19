# Expanded E2E merge gate

The branch stays a draft until required checks pass. A clean review or complete
command inventory alone does not authorize merging. Do not merge automatically.

## Required evidence

- Every canonical runnable command has a meaningful actual-binary process or
  live test. `TestCommandCoverageInventory` rejects package-only coverage and
  stale references. The 152-command manifest records remaining variants honestly.
- Linux, macOS and Windows process suites pass. Bash/Zsh/Fish completion callbacks
  execute on Unix CI; PowerShell executes on Windows CI. Missing required shells
  fail. Zsh callback candidates are checked before its interactive matcher.
- Ephemeral macOS/Windows CI verifies native credential persistence and isolated
  profile logout with `ATL_E2E_NATIVE_CREDENTIALS=1`. Never enable this on a
  personal machine; default local tests do not touch the native credential store.
- The final committed source passes formatting, compile/vet, lint, race tests,
  tagged harness tests, matrix accounting tests, and full branch review.
- The final live matrix passes required auth modes, owned writes with independent
  readback, representative forced pagination and binary downloads, and verified
  cleanup. A selected capability's missing credentials, permissions or fixtures
  fails; it cannot be changed to an exclusion after the failure.
- Restricted-user denial and independent PR approval require an additional live
  identity. Simulated identities in process tests do not establish live access.

## Live capabilities

`scripts/e2e-matrix.py --bb-project-admin` selects project create/read/delete.
`--bb-pipelines` selects `TestBitbucketPipelinesAndDeployments`, requiring at least
three available free build minutes verified before execution. Unselected tests
are listed in `excluded_tests` before the cell starts; an excluded capability is
not certified and needs an explicit acceptance decision before merge.

Pipeline fixtures are private, run-owned repositories. Pipelines is disabled
while two branch fixtures with `[skip ci]` messages are committed, then enabled
after the last upload. The messages also suppress delayed automatic triggers.
Only explicit CLI runs follow; jobs use size 1x and one-minute step limits.
One prints a marker in a test deployment; the other proves stop/readback. No
external deployment credentials or real deployments are involved. Cleanup stops
remaining runs before deleting and verifying the repository. Do not purchase
capacity, change subscriptions, or enable Pipelines on unrelated repositories.

The optional independent-reviewer test first proves a distinct user cannot
read an owner-verified private repository, then grants access only to that
repository and verifies approval/unapproval. It requires a workspace member
without inherited repository access; an unavailable identity remains excluded.

Separate read-only token cells prove readable owned resources reject mutation
without changing state. These use the same account and do not satisfy the
separate restricted-user or independent-reviewer requirement.

## Additional scenarios delivered

Shared process workflows cover auth login/status/default/logout, precedence,
aliases and cycles, resolve/browse suppression, extension argv/exit/PATH behavior,
completion callbacks, raw request bodies, output shapes, and interrupted downloads
that preserve existing destination files. Native Windows extension resolution
includes case-insensitive prefix/name matching and PATHEXT precedence.

Jira live scenarios now verify email assignment and three worklogs across
one-item pages. Confluence verifies indexed text search, multi-batch ancestor
traversal, and exclusion of grandchildren from direct-child results. Bitbucket
adds known-ID PR search and owned project administration.

The stronger ancestor fixture exposed a client bug: this endpoint traverses by
the first ancestor's ID/type rather than ordinary list cursors. The client now
walks upward and preserves top-to-bottom order, with cycle detection and the
existing request cap. See the [official ancestor contract](https://developer.atlassian.com/cloud/confluence/rest/v2/api-group-ancestors/).

The [earlier validation report](e2e-validation-2026-09-15.md) records the first
72-execution matrix. It predates this expansion and does not certify the new
production fixes or capability tests. Final execution results must be recorded
separately, including blocked account capabilities and native-platform results.

The [September 19 validation report](e2e-validation-2026-09-19.md) records
the passing 81-execution matrix, native CI and verified cleanup. The independent
reviewer now passes live denial, approval and unapproval; the former account
blocker is resolved. This evidence does not authorize automatic merging.
