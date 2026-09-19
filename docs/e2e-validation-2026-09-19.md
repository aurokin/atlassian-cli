# E2E validation — 2026-09-19

## Merge assessment

**High confidence in the tested PR; the independent-user acceptance gate is
resolved.** The complete 12-cell live matrix passed all 81 selected executions,
with no failures, skips, missing tests or excluded capabilities. No production
code or test changes were needed. The PR remains unmerged with auto-merge disabled.

This supersedes the second-account blocker in the
[September 15 report](e2e-validation-expanded-2026-09-15.md). It establishes the
documented merge gate, not exhaustive coverage of every flag or permission
combination. Mixed non-page Confluence children still have process-only evidence.

## Source and checks

- Live-tested revision: `916bd521d19b09ac9bf5fb0b428ba7f556ff10e8`, clean tree.
- Main: `50a50599bd8be089b3109afd721e893a9b89f193`, unchanged since branch review.
- Go source-tree SHA-256: `3da2620ce268036b2933ce4457ef83d59c96fb92ab52dd74096788701b4480fa`.
- Go 1.26.5, macOS arm64; 16:27:08–16:36:20 UTC.
- Artifacts: `/tmp/atl-full-reviewer-matrix-2026-09-19/summary.json` and adjacent
  JSON event logs/stderr. The three binary hashes match the September 15 report.
- Local `make check` and `make lint` passed; lint reported zero issues.
- [All six CI jobs for this revision passed](https://github.com/aurokin/atlassian-cli/actions/runs/35010830731),
  including Linux/macOS/Windows process contracts, native credential persistence,
  shell completion, formatting/build/vet/tests, lint and race/coverage.
- The previously clean full branch review still covers the unchanged Go source
  and unchanged main comparison. This follow-up changes documentation only.

## Live matrix

| Cell | Passed / selected |
|---|---:|
| jira-classic | 8 / 8 |
| jira-scoped | 8 / 8 |
| conf-classic | 13 / 13 |
| conf-scoped | 13 / 13 |
| bb | 13 / 13 |
| jira-oauth | 8 / 8 |
| conf-oauth | 13 / 13 |
| jira-readonly | 1 / 1 |
| conf-readonly | 1 / 1 |
| bb-readonly | 1 / 1 |
| jira-oauth-refresh | 1 / 1 |
| conf-oauth-refresh | 1 / 1 |

Bitbucket includes project administration, bounded Pipelines/deployments and
`TestBitbucketIndependentReviewer`. Jira/Confluence cover classic, scoped and
OAuth routing. Bitbucket uses scoped API tokens over the CLI's `cloud-classic`
Basic transport. Both forced-refresh cells and all three scope-denial cells passed.

## Independent reviewer

The owner authorized a second identity on September 19. Its Bitbucket profile
is `bb-reviewer`, distinct from the owner profile `bb`. It joined the dedicated
`atl-e2e-reviewers` group without workspace administration or project creation.

The live test verified distinct authenticated UUIDs before creating a private
fixture. The owner could read it; the reviewer received concealed HTTP 404,
exit 6 and the structured `not_found_or_not_visible` error. After the owner
granted write access only to that fixture, the reviewer could read it, approve
the owner's PR, and unapprove it. Owner readback verified each approval state
and the exact reviewer identity. Repository cleanup removed the temporary grant.

The targeted pilot also passed in 16.90 seconds; evidence is retained at
`/tmp/atl-reviewer-live-2026-09-19.log`. The matrix independently repeated it.

## Cleanup and credentials

All 64 resources in the matrix's ten cleanup ledgers end in `deleted`, with
deletion verified by the suite. The pilot's additional repository was also
deleted and verified. No subscriptions, credit cards, paid capacity or unrelated
repositories were changed.

The reviewer token has identity/workspace/repository read and PR read/write
scopes. The replacement owner token includes the permission-management scope
needed for the fixture grant; its superseded token was revoked after validation.
Both new tokens expire September 26, 2026 and are stored through the CLI's OS
keychain backend. Existing read-only tokens still expire September 22. No raw
credentials were written to the repository. Task browser sessions and the
temporary LAN relay were closed.

See [the merge gate](e2e-merge-gate.md) and [run instructions](integration-testing.md).
