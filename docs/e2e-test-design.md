# End-to-end test suite design

Status: core implementation delivered 2026-09-15; broader acceptance design
retained below. See [the validation report](e2e-validation-2026-09-15.md) for
measured results and exclusions, and [the runbook](integration-testing.md) for
commands that exist today. This design extends the
[integration harness](../integration/harness_test.go) and
[live runbook](integration-testing.md).

## Outcome and acceptance standard

A reviewer should be able to identify the exact binaries tested, which user
workflows succeeded against Atlassian, and which capabilities remain untested.
A successful process exit alone is not sufficient evidence.

Use two complementary suites:

1. **CLI process contracts:** build and invoke the actual binaries against local
   HTTP fixtures and temporary config, files, and Git repositories. Run without
   real credentials in ordinary CI on Linux, macOS, and Windows. Assert actual
   exit codes, stdout, stderr, requests, downloaded bytes, and persisted state.
2. **Live acceptance:** invoke those binaries against explicitly configured
   disposable Atlassian accounts. Prove authentication, scopes, gateway routing,
   current API compatibility, pagination, resource changes, and cleanup. Keep
   this manually invoked initially, consistent with the existing CI prohibition.

Retain focused package tests for parsing, rendering, credential providers, and
concurrency. Local fixtures cannot establish live API compatibility. Live
services cannot reliably produce timeouts, rate limits, or concurrent refresh
races on demand. Report these types of evidence separately.

For a full release, every runnable command must have an explicit coverage entry:
process contract, live scenario where applicable, and any unverified platform,
auth mode, or account capability. Generate the command inventory from the Cobra
roots; fail the inventory check when a newly added command has no entry. Count
canonical commands once and exercise aliases separately. This is a small test
manifest, not a general test framework.

## Initial evidence and deficiencies (2026-09-13)

The initial live suite had 22 top-level tests: six Jira, eight Confluence, and
eight Bitbucket. They cover basic reads and selected resource lifecycles.
They do not cover the full implemented command surface.

Concrete gaps include:

- Bitbucket PR listing requests five items; it does not prove `--all`, the
  endpoint-specific default page size, or crossing a page boundary.
- No live direct-children, attachment round-trip, or forced OAuth refresh cases.
- Several successful writes are checked using human output only; Confluence
  label removal and comment deletion do not verify their new JSON results.
- Missing configuration and credentials skip tests. Write errors containing
  strings such as `unauthorized`, `forbidden`, or `insufficient` also skip tests.
  That can conceal an authentication defect, wrong scope, or CLI regression.
- Empty list fixtures can pass without proving retrieval of a known object.
- Cleanup failures are logged, not reflected in the final acceptance result.
- Child processes lack a harness-level deadline, and built binaries are cached
  in a temporary directory without an explicit run identity in the report.

### Local verification snapshot

On 2026-09-13, the `fix/api-sync` working tree based on `d390c95`, including the
uncommitted SSH-host and OAuth-timeout fixes, passed `make check`,
`make docs-check`, `go test -race ./...`, and `git diff --check`.
The repository-pinned golangci-lint v2.1.6 was installed with Go 1.26.5;
`make lint` completed with zero issues. These checks are not live acceptance.

Read-only credential probes used freshly built binaries and the existing
profiles. Raw tokens and OAuth bundles were not printed:

| Profile | Local credential resolution | Live status outcome |
|---|---|---|
| `bb` | Keychain credential available | HTTP 401, `unauthorized` |
| `jira-scoped` | Keychain credential available | HTTP 404, `not_found_or_not_visible` |
| `conf-scoped` | Keychain credential available | HTTP 404, `not_found_or_not_visible` |
| `work` | File-backed OAuth bundle available; access token expired | Refresh HTTP 403, `unauthorized_client: refresh_token is invalid` |
| `smoke-conf` | File-backed OAuth bundle available; access token expired | Refresh HTTP 403, `unauthorized_client: refresh_token is invalid` |

Both configured Jira/Confluence site roots and their `/_edge/tenant_info` URLs
also returned 404. Site deactivation or migration is a hypothesis, not a
confirmed diagnosis. No live mutation suite ran. The owner confirmed these are
purpose-built test accounts, currently on free plans; their active entitlements
and usable fixtures have not been established.

## Accounts, credentials, and fixture setup

### Credential restoration, 2026-09-15

Five freshly created API tokens were saved through `auth login --token-stdin`
to the local secret backend; no raw tokens were written to this repository.
The token manager shows expiry on 2026-12-13. The following profiles passed
live status checks with binaries built from the current working tree:

| Product | Profile | Credential and transport |
|---|---|---|
| Jira | `jira-classic` | Unscoped API token, direct site Basic auth |
| Jira | `jira-scoped` | Scoped API token, cloud-id gateway Basic auth |
| Confluence | `conf-classic` | Unscoped API token, direct site Basic auth |
| Confluence | `conf-scoped` | Scoped API token, cloud-id gateway Basic auth |
| Bitbucket | `bb` | Scoped API token, Basic auth (`cloud-classic` in the CLI) |

Jira and Confluence now target the recreated `aurotest.atlassian.net` site.
Confirmed containers are Jira project `KAN`, Confluence space `SD`, and
Bitbucket workspace `OhBizzle`. Bitbucket contains existing repositories;
do not assume the workspace is empty or mutate unrelated repositories.

The existing read-only integration cases passed with `-count=1`: five Jira
and five Confluence cases in each auth mode, plus six Bitbucket cases using
`bb-cli-integration-primary` (26 passes, no skips). These establish live
authentication, routing, and the existing read assertions, not full E2E
acceptance. Writes, cleanup, attachment round trips, and OAuth renewal were
not verified in this run. The old OAuth profiles still need reauthorization.

Start from empty test accounts. Do not require preexisting issues, pages, PRs,
or pipelines. A small setup step establishes the containers below, using an
existing command when supported or a documented official API/UI setup when it
is not. Setup operations must be recorded separately from tested operations.

| Product | Durable test containers | Per-run fixtures |
|---|---|---|
| Jira | Dedicated project, supported issue type and transition, primary user | At least three issues, two comments, worklog, binary attachment, issue link |
| Confluence | Dedicated space | Parent and child pages, multiple versions, comments, labels, blogpost, binary attachment; mixed-type children when available |
| Bitbucket | Workspace, primary user; second user for independent PR approval | Private repository with seeded commits, divergent branches, tags, separate merge/decline PRs, comments; pipeline and deployment fixtures when enabled |

Use a globally unique run ID in fixture names and an exact ID ledger. Do not
find arbitrary existing resources and mutate the first result. The current
Confluence label test should move from an arbitrary space page to an owned page.

The setup manifest records explicit product targets, credential references,
expected account identity, fixture IDs, and selected capabilities. Store tokens
in the existing secret backends or environment references, never in the manifest.
Do not auto-select a default personal site. Validate identity and target before
any fixture write, even when all current accounts are disposable.

Jira and Confluence must run the required live workflows in both unscoped
(`cloud-classic`) and scoped (`cloud-scoped`) API-token modes, with separately
reported results. Bitbucket uses a scoped API token with Basic auth, called
`cloud-classic` by the CLI; it has no corresponding cloud-id gateway mode.
Do not count that one transport as two auth modes or require a retired app
password for a second live cell. OAuth needs separate successful login and refresh
coverage for Jira and Confluence. A second, deliberately restricted credential
supports scope/permission denial cases. One administrator token does not prove
low-access behavior. If an auth mode becomes unavailable, disclose the gap
instead of silently dropping its required cases. Cloud product E2E does not
claim Data Center API support merely because a local test uses a PAT profile.

### Free plans and optional capabilities

Do not encode a guessed list of paid features. Record the active plan, account
roles, token scopes, and available capabilities during setup; confirm uncertain
restrictions against current official documentation or the account's settings.

Define a required core covering ordinary reads, owned resource writes,
attachments, comments, pagination, and outputs. Separate capability selections
cover OAuth, restricted users, mixed Confluence content, Bitbucket Pipelines,
deployments, and project/repository administration.

A missing required capability blocks acceptance. An optional capability may be
excluded before execution with a specific reason and evidence; it remains
unverified in the report. Once selected, its 401/403 responses fail the case.
Do not reinterpret a failed case as an optional exclusion. A free-plan result
cannot certify an unexercised capability on another plan.

Pipelines need an explicit available-minutes budget, an enabled repository,
and bounded jobs that only print test markers. Deployment fixtures must have no
external credentials or real deploy steps. Report quota exhaustion as blocked;
never purchase capacity or change a subscription as part of the suite.

## Coverage matrix

Each row is a workflow family, with separately reported assertions. A matrix
entry can map to more than one test; avoid one giant lifecycle whose early
failure hides every later assertion. Dependents of a failed fixture setup are
reported as blocked, while independent cases continue.

| Family | Commands and required observations | Evidence |
|---|---|---|
| Shared discovery | `version`, help, completion; correct binary identity, runnable generated shell completion, malformed argument/flag exits | Process, all three binaries |
| Auth/config | `auth login/status/default/logout`; env and stdin token sources, overwrite, target precedence, missing credentials, isolated persistence and logout | Process; native keychain round-trip on supported OS; live positive status |
| OAuth | Authorization callback state and PKCE rejection; exchange, refresh, rotated-token persistence, invalid grant, concurrent refresh, no secret output | Local provider/child-process fixtures; manual browser login and live refresh with dedicated test profiles |
| Raw API | GET and body-bearing methods, relative and permitted absolute URLs, cross-origin rejection, response bodies/errors, trace redaction | Process plus live harmless GET and owned resource mutation |
| Output | Human, bare JSON, selected fields on objects and arrays, jq, empty lists, bodyless writes, pure stdout and actionable stderr | Process across shapes; live representative reads and every synthesized mutation result |
| Offline targeting | `resolve`, `browse --no-browser` and `--no-prompt`; known URLs/keys/IDs, invalid inputs, no unwanted browser launch | Process, no network |
| Aliases/extensions | Alias set/list/delete, expansion/quoting/cycles; extension list/exec/fallback, argv and exit propagation, PATH precedence, symlinks and Windows PATHEXT | Process and real child executables on native OS |
| Jira discovery/search | `status`, project list/view, field list, search issues, issue list/view with fields/expand; find known fixture IDs and correct query membership | Live and process |
| Jira issue writes | Create/edit, assignment by ID/email/self/unassign, allowed transition; read back changed fields and unchanged fields that matter | Live, capability-aware fixture configuration |
| Jira collaboration | Comment list/view/create/edit/delete; watch/unwatch/watchers; link and link types; worklog list/add | Live read-back, deletion verification, JSON/jq assertions |
| Jira attachments | Add/list/download; filename metadata and byte-for-byte checksum match, no destination corruption on failure | Live plus process faults |
| Confluence discovery/search | `status`, space list/view, text and CQL search; known fixture returned with correct ID | Live v1/v2 routing and process |
| Confluence content | Page list/view/create/edit/delete, ancestors, children, versions; title-only body preservation, storage and ADF, trash then purge | Live; process version-conflict and malformed payload cases |
| Confluence child types | Direct pages; optional folder/whiteboard/database/embed children with type/ID assertions and pagination | Live; use official API/UI fixture setup for types the CLI cannot create |
| Confluence blogposts | List/view/create/edit, title and body read-back, version increment; official API cleanup | Live |
| Confluence collaboration | Footer comment list/view/create/edit/delete; label list/add/remove; read-back plus synthesized JSON/jq results | Live |
| Confluence attachments | Upload/list/download, metadata and checksum equality through scoped/OAuth routing | Live and process faults |
| Bitbucket discovery | Status, workspace view, project list/view, repo list/view, search repos; exact fixture identity | Live |
| Bitbucket administration | Project create/delete, repo create/delete; confirm visibility and removal | Live when explicitly selected; process guard coverage always |
| Bitbucket Git content | Commit list/view, src and file; compare commit identity, directory entries, and exact file bytes | Live seeded repository |
| Bitbucket refs/targeting | Branch/tag create/list/view/delete; explicit repo/workspace and inference from real temporary Git checkouts including `ssh.bitbucket.org`, legacy and altssh hosts | Live refs; process Git inference cases |
| Bitbucket PRs | Create/list/view/diff, comments list/add, approve/unapprove with independent user, decline and merge on separate fixtures; verify states and destination commit | Live; isolate independent PR lifecycles |
| Bitbucket Pipelines | Run/list/view, steps/log, stop; poll known run IDs, assert log marker and terminal state | Live selected capability with bounded build usage |
| Bitbucket deployments | Environment list/view, deployment list/view; known fixture IDs and relationship to harmless pipeline | Live selected capability |
| Pagination | Every implemented `--all` list/search family, explicit small limits, native next-token/offset/link behavior, complete fixture membership without duplicates | Live multi-page fixtures plus process protocol cases |
| Failures/confirmation | Missing `--yes`, invalid input before auth; 400/401/403/404/410/429/5xx, deadlines, interrupted response bodies, malformed JSON, pagination cap | Process; live valid restricted identity and deliberately missing owned-resource ID |

No removed Bitbucket issue commands need tests merely to prove their absence.
Inventory follows the currently shipped command tree and the command contract.

### Assertions that prevent false confidence

- Seed three known objects and request pages of one or two, so pagination must
  cross a boundary. Verify exact fixture IDs and uniqueness rather than only
  checking that JSON parses. For PRs, also invoke `--all` without `--limit` to
  exercise the endpoint's accepted default size, then separately force paging.
- Live lists in shared containers may contain unrelated objects. Compare the
  run-owned subset, not the whole tenant count. Tests in per-run repositories
  can assert the entire expected set.
- Verify every write with an independent read. Assert deletion is invisible or
  trashed according to the endpoint contract. Assert merges advance the target
  commit and that decline does not merge. A printed success string is insufficient.
- Check structured stdout against required fields and values, preserving unknown
  upstream fields; avoid brittle snapshots of mutable timestamps or whole API
  responses. Test synthesized results against their exact documented contract.
- Compare attachment bytes using a small binary fixture containing non-text
  bytes. Validate stdout download and destination-file behavior separately.
- Search can be eventually consistent. Retry only the read assertion, with a
  deadline and recorded attempts; never blindly retry a resource creation.
- Invalid-input and confirmation process cases must observe zero requests.
  Local error fixtures assert both JSON category and actual process exit code.
- Inject 429 and timeouts locally. Do not exhaust a tenant quota or depend on
  internet slowness to validate those paths. Scope denial is tested with a valid
  restricted credential, distinct from a deliberately invalid-token 401.

## Harness changes

Keep Go and the existing per-product test organization. Add narrowly scoped
helpers for process execution, run-owned fixtures, polling, and reporting.

1. **Preflight once per selected product/auth mode.** Validate all required
   configuration, credential resolution, positive authentication, expected
   identity, target visibility, and enabled capabilities. Configuration failures
   produce a nonzero acceptance result and explicit blocked case IDs.
2. **Build once per run.** Use a run-owned temporary directory, record full Git
   SHA, dirty diff digest, Go/tool versions, OS/architecture, and each executable's
   SHA-256. A dirty-tree report describes only that working tree. Final merge
   evidence must correspond to the final committed tree.
3. **Bound subprocesses.** Use `exec.CommandContext`, an outer case deadline,
   and a process-tree termination strategy per OS. A hung child with
   `--timeout 0` must not hang the runner. Allow cleanup its own deadline.
4. **Use explicit environments.** Sanitize inherited `ATL_SITE`, `ATL_TIMEOUT`,
   config roots and extension PATH behavior. Inject only selected settings;
   process tests must never touch the real keychain or personal config.
5. **Replace permission-based skips.** Unselected capabilities are declared
   before execution. Authentication errors, missing scopes for selected cases,
   expired credentials, unexpected empty fixtures, and cleanup failures are
   visible failures or blocks, never converted to passes.
6. **Persist cleanup intent.** Record created IDs immediately, clean dependents
   before parents, and verify cleanup. Preserve a ledger after crashes and
   provide an explicit cleanup operation restricted to exact owned IDs and
   targets. Unknown creation outcomes require reconciliation, not repeat POSTs.
7. **Keep setup independent where possible.** Use documented official API setup
   for resources missing CLI creation support, then exercise the CLI itself.
   Where setup uses a CLI write, report that separately so its failure blocks
   dependents rather than masquerading as their successful execution.
8. **Report assertions and coverage.** Emit Go test JSON plus a small summary
   artifact: case ID, command family, evidence type, auth mode, capability,
   expected result, pass/fail/blocked/excluded, elapsed time, attempts, and cleanup
   outcome. Attach sanitized diagnostics and IDs, never raw secret stores or
   token-bearing request data. Missing expected cases fail report validation.

Local process fixtures can use isolated local API profiles for product request
and output behavior; this does not prove the cloud gateway origin. Keep gateway
routing checks in the live suite. For mocked OAuth endpoints, use the existing
package test seams and a test-only child-process entry point for the shared API
command; do not add production flags that redirect secret-bearing OAuth calls.

## OAuth-specific acceptance

Use dedicated test grants. Do not log out or invalidate a shared grant to force
an error. Interactive authorization remains a separately recorded manual case
until a secure browser test flow is established; it cannot be silently included
in unattended acceptance.

After reauthorization, run a harmless read. For forced refresh, use the dedicated
profile and alter only the locally recorded expiry through a test helper,
retaining the actual stored token in its protected backend. Let the CLI refresh,
confirm the persisted bundle changed without printing token values, then launch
a second process and confirm it can authenticate. Serialize live refresh tests
for a grant. Test races and interrupted write-back with local fixtures, where
failures can be induced safely and deterministically.

## Execution policy and delivery order

Suggested targets below are proposals, not commands available today:

| Target | When | Success condition |
|---|---|---|
| `e2e-contract` | Every PR on Linux/macOS/Windows | All CLI process cases pass without live credentials |
| `e2e-preflight` | Before a live run | Selected identities, fixtures, capabilities and budgets are usable |
| `e2e-live-core` | Manual before merge of API/auth changes | Required core workflows pass for selected products and auth modes; zero unexpected skips or cleanup failures |
| `e2e-live-extended` | Before releases / changes to optional features | Selected OAuth, restricted-user, admin, mixed-content and pipeline workflows pass |
| `e2e-report` | Every run, including failed preflight | Complete case accounting, build identity, diagnostics and residual coverage gaps |

A product-scoped run is useful evidence, but cannot be reported as a full-suite
pass. A full-suite claim requires all declared required products, auth modes,
and capabilities. Native Windows evidence is required for Windows-specific
extension behavior; cross-compilation alone is not enough.

Implement in this order:

1. Restore reachable test sites and usable credentials; establish fixtures and
   a capability manifest. This is currently the prerequisite for any live proof.
2. Add strict preflight/result accounting, bounded subprocesses, and fixture
   ownership/cleanup. Preserve the existing manual-only live gate.
3. Add process contracts and live cases for the current branch: PR pagination,
   direct children, structured mutation output, confirmation guards, SSH-host
   inference, and timeout/refresh behavior.
4. Complete Jira and Confluence workflow families, including attachments and
   body-preserving edits; complete Bitbucket refs, Git content and PR lifecycles.
5. Add OAuth, restricted identities and optional Bitbucket capabilities; run
   the native OS contract matrix and enforce command-inventory coverage.

The current branch's merge decision should require the relevant cases from step
3 on its final tree, plus the existing core lifecycle tests without concealed
skips. The broader suite should be delivered as dedicated test changes, with
its remaining gaps explicit, rather than declaring the current 22 tests a full
acceptance suite.

## Remaining extended acceptance prerequisites

- The core test credentials, per-run fixtures, cleanup, and both OAuth grants
  have been restored and exercised; see the validation report.
- Decide which optional capabilities are required for a full release claim;
  verify free-plan entitlements and availability of a second test user.
- Provision an independent Bitbucket reviewer and bounded, harmless Pipeline
  and deployment fixtures before selecting those extended workflows.
- Obtain native Windows/Linux evidence from the configured CI jobs. Local
  macOS results and cross-compilation cannot stand in for those runs.

The assumption most likely to be wrong is that one empty, free account per
product can exercise every implemented capability. The suite must expose where
additional identities, scopes, setup, or plan capabilities are needed.
