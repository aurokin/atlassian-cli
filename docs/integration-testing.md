# End-to-end testing

The ordinary `go test ./...` / `make check` suite includes the real-binary
[process contracts](../e2e/README.md). Those cases use local HTTP fixtures,
isolated configuration and dummy credentials. They establish process behavior;
they do not establish Atlassian compatibility.

The `integration` package builds the real binaries and exercises live API reads,
owned resource mutations, read-back assertions and cleanup. It is manual-only:
the `integration` build tag, `ATL_RUN_INTEGRATION=1`, and an unset `CI` are all
required. Missing configuration or permissions fail selected tests. They are
never converted to successful skips.

## Run the stored-profile matrix

Use dedicated test accounts and containers. Tests create and delete issues,
pages, comments, attachments, branches, tags, private repositories and pull
requests. Bitbucket needs permission to create/delete private repositories;
its owned repository tests seed commits through the official multipart API.
No test purchases capacity or changes subscriptions.

The Python 3 standard-library runner validates profile product/token-style
metadata, discovers the compiled test inventory, then runs each selected cell
serially. It reads `config.json` metadata only; the CLI resolves credentials
through its existing backend.

```bash
python3 scripts/e2e-matrix.py \
  --jira-classic jira-classic --jira-scoped jira-scoped \
  --conf-classic conf-classic --conf-scoped conf-scoped \
  --bb bb \
  --jira-project KAN --conf-space ATLE2E \
  --bb-workspace OhBizzle --bb-repo bb-cli-integration-primary \
  --output /tmp/atl-e2e-matrix-unique-run
```

The output directory must be new. Omit `--output` to create a private temporary
directory. Each selected product requires explicit container arguments. Select
fewer profile flags for a partial run; the report identifies exactly those
cells. The example targets the dedicated test setup, not a generic default.
Confluence `ATLE2E` is a regular space suitable for blogposts; the onboarding
`SD` space is not the fixture container for this suite.

Use `--jira-expected-account-id ACCOUNT_ID` and
`--conf-expected-account-id ACCOUNT_ID` to require the expected identities, and
`--bb-expected-account-id UUID` for Bitbucket's distinct identity shape. Without these options, preflight still requires a positive authenticated
identity and records it with the exact site/profile target.

Bitbucket scoped API tokens use Basic authentication (`cloud-classic` in the
CLI). The runner validates that transport; it cannot infer the server-issued
token's scopes from the profile. There is no second Bitbucket cloud-id gateway
cell. Jira and Confluence classic/scoped cells validate distinct token styles.

### OAuth workflows and forced refresh

Add `--jira-oauth work --conf-oauth smoke-conf` to select dedicated, previously
reauthorized OAuth profiles. This runs the complete respective product
workflows and then two separate serial refresh cells. Selecting either flag
also authorizes modifying that test profile's locally stored OAuth expiry to
force refresh. The test checks credential rotation/persistence and a second
process's authenticated identity. It does not revoke the grant or restore an
obsolete refresh token. Browser authorization must already have succeeded.

OAuth profile selection adds one refresh test per selected profile. A missing
or failed refresh does not disappear into a passing product result.

### Independent reviewer

`--bb-reviewer PROFILE` selects live approval/unapproval by a distinct Bitbucket
workspace member. The test grants that member write access only to its newly
created private repository. The owner's token needs `write:permission:bitbucket`
and repository administration; the reviewer's needs repository read and PR
read/write. A missing profile, same identity, or failed permission grant fails
the selected test. Without this option the report records the test as excluded,
not passed. Do not substitute the author's token for independent approval.

### Optional live capabilities and restricted scopes

`--bb-project-admin` adds owned project create/read/delete. `--bb-pipelines`
adds owned pipeline run/steps/log/stop and deployment/environment readback.
Verify at least three free build minutes before selecting it. The token needs
`admin:pipeline:bitbucket` for environment creation as well as pipeline read/write
and the repository/project scopes used by the base suite. Fixtures use `[skip ci]`
commits, one-minute size-1x steps, and stop-before-delete cleanup. No subscription
changes or external deployment credentials are needed. Unselected capabilities
are explicitly recorded as exclusions.

To test a readable resource with a write-restricted token, add:

```bash
--jira-readonly jira-readonly --jira-readonly-owner jira-classic \
--conf-readonly conf-readonly --conf-readonly-owner conf-classic \
--bb-readonly bb-readonly --bb-readonly-owner bb
```

These cells require separate stored credentials for the same account and site.
The owner creates and successfully edits a fixture; the read-only credential
must read it, fail to edit it, and leave the independently read-back state
unchanged. Jira/Confluence use scoped gateway tokens; Bitbucket uses Basic token
transport. This tests token scopes, not a different user's resource permissions
or independent PR approval. Never supply a full-access token as the read-only
profile.

## Results, deadlines and recovery

The runner writes `summary.json` incrementally, plus each cell's complete Go
JSON event log and separate stderr file. A cell fails for any selected test or
subtest failure/skip, missing expected test, malformed event stream, missing
package completion, missing build identity, nonzero process exit, or timeout.
Unstarted cells remain explicitly blocked after an interrupted run. The runner
returns nonzero if any selected cell fails. It does not equate a partial matrix
with full-release acceptance.

The default runner deadline is 1,200 seconds per cell (`--cell-timeout` changes
it); Go's package deadline expires 15 seconds earlier. Each CLI invocation has
a 90-second outer deadline and each build has a two-minute deadline. Cleanup
commands get fresh deadlines. The Unix runner stops and terminates every member
of its owned session after launcher exit, timeout, or interruption (including
SIGTERM), including CLI subprocesses in separate process groups. The live matrix runner refuses Windows execution until
equivalent process-tree cleanup is supported. Hermetic CLI contracts run in native Linux, macOS and Windows CI jobs.
Native credential persistence is enabled only on ephemeral macOS/Windows workers.

Build logs record binary SHA-256, Git HEAD, tracked working-diff SHA-256,
untracked file inventory, Go version and OS/architecture. A Go source-tree
digest includes tracked/untracked Go files, go.mod and go.sum; every cell must
report the same digest or the matrix fails. These identify the
binaries used; dirty-tree evidence is not evidence for a later changed tree.
Keep the logs and rerun the matrix after material changes.

Owned-resource cleanup records are private JSONL files outside `t.TempDir`;
their paths appear in the logs and summary. Entries include exact resource IDs,
profile/target metadata and pending/deleted state. Cleanup failures fail the
case. A killed process may leave pending resources: inspect that exact target
and ID, reconcile whether the creation/deletion succeeded, and clean only the
owned resource. Do not bulk-delete matching prefixes or retry creation blindly.
There is no automatic ledger replay command yet.

## Direct invocation and environment contract

For a focused debugging run:

```bash
ATL_RUN_INTEGRATION=1 ATL_IT_USE_STORED_PROFILES=1 \
  ATL_IT_JIRA_SITE=jira-scoped ATL_IT_JIRA_PROJECT=KAN \
  go test -tags=integration ./integration -run '^TestJira' -count=1 -v
```

Unlike the matrix runner, direct `go test` does not reject skips for you. Select
products explicitly with `-run`; an unconfigured selected product fails.
`make integration` selects all tests, including the optional OAuth test (which
skips unless explicitly configured). Use the matrix for strict acceptance.

| Variable | Meaning |
|---|---|
| `ATL_RUN_INTEGRATION=1` | Required live opt-in; `CI` must be unset. |
| `ATL_IT_USE_STORED_PROFILES=1` | Reuse explicitly named stored profiles. |
| `ATL_IT_<P>_SITE` | Stored profile for `JIRA`, `CONF`, or `BB`. |
| `ATL_IT_<P>_EXPECTED_ACCOUNT_ID` | Optional expected identity; Bitbucket uses UUID. |
| `ATL_IT_JIRA_PROJECT` / `ATL_IT_JIRA_ISSUE_TYPE` | Dedicated project; issue type defaults to `Task`. |
| `ATL_IT_CONF_SPACE` | Dedicated regular space key. |
| `ATL_IT_BB_WORKSPACE` / `ATL_IT_BB_REPO` | Dedicated workspace and existing repository for read checks; writes use newly created private repositories. |
| `ATL_IT_OAUTH_SITE` | Dedicated OAuth profile for explicit forced-refresh test. |
| `ATL_IT_ACCESS_SITE` / `ATL_IT_ACCESS_OWNER_SITE` | Read-only and owner stored profiles for explicit `TestRestrictedAccess`. |
| `ATL_IT_BB_REVIEWER=1` / `ATL_IT_BB_REVIEWER_SITE` | Select independent approval with a second workspace-member profile. |
| `ATL_IT_BB_PROJECT_ADMIN=1` | Select owned project administration. |
| `ATL_IT_BB_PIPELINES=1` | Select bounded pipeline/deployment workflows after verifying free quota. |

Without stored-profile mode, tests provision temporary profiles using
`auth login --token-env`. Supply `ATL_IT_<P>_BASE_URL`, `ATL_IT_<P>_TOKEN`, and
`ATL_IT_<P>_USERNAME` (or `_EMAIL`). Bitbucket defaults its URL to
`https://api.bitbucket.org/2.0`. Jira/Confluence `_CLOUD_ID` selects scoped gateway
routing; omit it for classic direct-site tokens. Tokens remain in environment
references and are not copied into config or a keychain. This is supported by
the Go harness; the matrix runner intentionally requires stored profiles.

## Delivered live coverage and limits

The base families contain eight Jira, thirteen Confluence and ten Bitbucket
tests. Two optional Bitbucket capabilities, OAuth product/refresh cells, and
three restricted-scope cells extend the matrix. The runner discovers selected
names from the compiled suite; test count does not prove completeness.

| Product | Delivered workflow assertions |
|---|---|
| Jira | Identity, projects, fields, issue search/list, owned pagination, create/edit, comments, assignment/unassignment, watchers, worklogs, transitions, issue links, binary attachment stdout/file round trips, cleanup verification. |
| Confluence | Identity/spaces, owned CQL search, page list/view, storage and ADF title/body preservation, versions, trash/purge, comment lifecycle, labels, child pagination and ancestor membership, binary attachments, blogpost lifecycle. |
| Bitbucket | Identity/workspace/projects/repos, owned private repositories and seeded commits, source/file bytes, branch/tag pagination and lifecycle, PR pagination/default size, PR views/diff/comments, merge/decline states and destination commit, repository cleanup. |
| OAuth | Deliberately expired dedicated grant, persisted token rotation, subsequent subprocess authentication and unchanged identity. |

The [command inventory](../e2e/coverage.json) accounts for 152 canonical runnable
commands and validates their evidence pointers. It does not claim complete
workflow or flag coverage for every command. Remaining release-level gaps include
restricted identities, independent-user PR approval/unapproval, and mixed
non-page Confluence child types. Pipelines/deployments and native-platform
coverage require their explicit live/CI execution evidence. Browser OAuth authorization remains a separately performed setup
step. These capabilities are not silently counted as passed. See the broader
[design](e2e-test-design.md) for the intended release suite and the
[process coverage](../e2e/README.md) for local-only assertions.

Runner accounting tests are hermetic:

```bash
python3 -m unittest discover -s scripts -p '*_test.py'
```
