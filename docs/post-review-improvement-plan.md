# Post-review improvement plan

This plan sequences the recommendations from the repo-wide subagent review
(May 2026) into executable, individually-reviewable PRs. The first wave of that
review already merged as PRs #57–#64 (cleanup, cross-product consistency, the
CI integration-tag gate, integration-harness fixes, the docs archive, the
`WantsStructured()` dedup, and the `--all` truncation fix). What remains is
captured below.

## Working rules (unchanged)

Every PR below follows the standard loop: topical branch → `make check` →
`codexrabbit-code-reviewer` wave (address findings, re-review until clean) →
open PR → CI green → `gh pr merge --squash --delete-branch` → sync `main`.
Keep each PR a vertical slice (client method → models → command → tests →
docs). Update `docs/command-contract.md` whenever the surface changes.

## Decision points (confirm before the flagged PRs)

These change behavior or override a prior design decision; the flagged PRs
should not merge without an explicit go-ahead:

- **D1 — Exit codes (PR 2).** Mapping `apperr` categories to distinct process
  exit codes is a contract change: scripts that today branch on "exit 1 = any
  error" will see 4/5/6/7. High value for agents, but semi-breaking.
- **D2 — Shared client base (PR 5).** Extracting the per-client request
  plumbing into `restutil` overrides the explicit "keep per-product" note in
  `internal/restutil/restutil.go`. Worth it, but it touches all three clients'
  core path.
- **D3 — Write commands / read-mostly policy (PR 22).** `repo create/delete`,
  `project delete`, etc. expand `atl-bb` beyond its current read-mostly stance.
  Decide the policy before adding destructive verbs.

---

## Phase R1 — Correctness (highest value, low risk)

### PR 1 — Confluence: title-only `page edit` on ADF pages
- **Problem.** `conf.GetPage` requests only `body-format=storage`. Pages
  authored in the modern editor are stored as `atlas_doc_format`, so their
  `storage.Value` is empty and a title-only `page edit` aborts with "no
  storage-format body to preserve" (`internal/confcmd/page.go:240`).
- **Approach.** The v2 `body-format` query param is `…Single` (no comma list),
  so add a fallback: when the storage body is empty, re-`GET` with
  `body-format=atlas_doc_format`, model `AtlasDocFormat` in `conf.PageBody`, and
  re-send whichever representation is populated on the `UpdatePage` PUT.
- **Files.** `internal/conf/client.go` (GetPage variant or a body-format arg),
  `internal/conf/models.go` (PageBody.AtlasDocFormat), `internal/confcmd/page.go`.
- **Verify.** Unit test with an ADF-only page fixture (storage empty,
  atlas_doc_format populated) asserting the title-only edit re-sends ADF; add an
  integration lifecycle assertion that creates an `atlas_doc_format` page and
  renames it.
- **Risk.** Low–medium. Needs a live-tenant check via the integration suite.

### PR 2 — Error model: categories, exit codes, and the code catalog  *(D1)*
- **Problem.** All failures exit 1; there is no `timeout`/`network` category, no
  `410 Gone` message, no `auth_expired` vs `auth_missing` distinction; several
  emitted codes (`request_failed`, `response_decode_failed`, `untrusted_url`,
  `http_error`, `result_truncated`) are undocumented; `access-error-model.md`'s
  JSON example shows a `permission_denied` code the implementation never emits
  (it is `forbidden`).
- **Approach.**
  1. Add `apperr.Error.ExitCode()` (e.g. auth=4, forbidden=5, not-found=6,
     rate-limited=7, invalid-input=8, else 1); wire `Execute`/`Run` to use it.
  2. In `httpclient.classify`: add `case http.StatusGone` (clear "endpoint
     removed; upgrade the CLI") and detect `context.DeadlineExceeded`/timeout →
     a `timeout` category with a retry hint.
  3. Centralize all code string literals as `apperr.Code*` constants so the doc
     has one source of truth; document the full catalog (incl. `result_truncated`)
     in `access-error-model.md` and fix the `permission_denied` → `forbidden`
     example.
- **Files.** `internal/apperr/error.go`, `internal/httpclient/client.go`,
  `internal/cli/root.go` (Execute/Run), `docs/access-error-model.md`.
- **Verify.** Table tests over status→category→exit-code; doc updated.
- **Risk.** Medium (D1 contract change). Land exit codes only after sign-off;
  the new categories/catalog are safe to do regardless.

---

## Phase R2 — Refactors, dedup, elegance

### PR 3 — Shared `status` command
- Collapse the three near-identical `writeStatus`/`newStatusCommand` copies
  (`internal/{jiracmd,confcmd,bbcmd}/status.go`) into a `cli.RunStatus` taking a
  small per-product adapter (CurrentUser call + display/contact fields).
- **Risk.** Low. Pure dedup with existing tests as the guardrail.

### PR 4 — Generic render-decode helper
- Beyond the already-merged `WantsStructured()` predicate, add a
  `cli.RenderDecoded[T]` (render raw when structured; else decode→writeHuman) to
  remove the decode-then-write boilerplate repeated in every list/view command.
- **Risk.** Low–medium (many call sites; mechanical). Do after PR 3.

### PR 5 — Shared client base  *(D2)*
- Lift the byte-identical `get`/`send`/`decodeError`/`New`/`APIBase` and a
  param-named `setLimit` out of `internal/{jira,conf,bitbucket}/client.go` into a
  `restutil` base the clients embed, with an injectable error-remap hook so
  Bitbucket's `remapError` (feature-disabled signal) still applies.
- **Risk.** Medium–high; touches core plumbing. Sequence this **before** the
  feature-gap PRs (R5–R7) so new client methods are built on the base, not
  retrofitted.

### PR 6 — Unify the pagination followers
- Replace `bitbucket.followValues` / `conf.followList` / `jira.followAll` with a
  single `restutil.FollowAll(ctx, fetch, extract)` (Jira's callback shape
  generalizes the others). Concentrates the `result_truncated` logic (already
  added per-follower) in one place. Optionally fix the Jira offset-followers to
  derive the offset locally rather than trusting the response `startAt`.
- **Risk.** Medium. Depends on PR 5's seam.

### PR 7 — Misc dedup
- Merge `httpclient.extractMessage` and `bitbucket.errorMessage` (same body
  shape, two parsers). Let attachment downloads send `Accept: */*` instead of
  the unconditional `application/json`.
- **Risk.** Low.

---

## Phase R3 — Cross-product UX & progressive disclosure

### PR 8 — Default site & env override
- Add a `default_site` config key + `ATL_SITE` env override, resolved in the one
  chokepoint (`cli.SiteClient`), so networked commands stop needing `--site`
  every time.
- **Risk.** Low.

### PR 9 — Discoverability
- Group subcommands with `cobra.Group`/`AddGroup` (Core / Auth / Advanced);
  register shell-completion (`ValidArgsFunction`, flag completions) for sites,
  projects, etc.
- **Risk.** Low.

### PR 10 — Human-output `LabelWriter`
- Replace the ad-hoc per-command `%-Ns` label padding (and the literal-spaces in
  `cli/resolve.go`) with a shared `output.LabelWriter` for consistent detail
  rendering across products.
- **Risk.** Low.

### PR 11 — Bitbucket workspace inference & pagination polish
- `resolveWorkspace` should fall back to the git-checkout-inferred workspace the
  way `resolveRepoTarget` already does, so `repo list` / `project list` /
  `search repos` work in-repo without `--workspace`.
- Default the page size to the API max when `--all` is set without `--limit`
  (fewer requests, less chance of hitting the cap); preserve a `truncated`/next
  marker in the synthesized `--all` JSON.
- **Risk.** Low–medium.

---

## Phase R4 — Auth hardening

### PR 12 — `auth login` validation
- Reject incompatible flags up front for `oauth-3lo` (it silently ignores
  `--token*`/`--username`); warn (or fail) when a static style is logged in with
  no token source (currently persists an unusable profile); add an
  overwrite confirmation (or note) when re-logging an existing site; add
  `secrets.Store.Has` so `auth status` need not read the secret value.
- **Risk.** Low.

---

## Phase R5 — Feature gaps: Jira  *(product scope)*

- **PR 13 — `issue view --fields/--expand`**: pass-through to `GetIssue` so
  scripted `--json` can pull custom fields/comments/changelog.
- **PR 14 — field discovery**: `issue createmeta` (or `field list`) so users can
  find field ids/types instead of guessing.
- **PR 15 — user resolution**: accept email / `@me` for `--assignee` and
  `issue assign`, resolved via `GET /user/search`.
- **PR 16 — attachments**: `issue attachment list/download/add` (parity with the
  Confluence attachment surface).
- **PR 17 — read polish**: `--order`/`--since` on `comment list`/`worklog list`;
  emit `--json` results for `assign`/`watch`/`unwatch`; render the worklog
  comment.

## Phase R6 — Feature gaps: Bitbucket  *(product scope)*

- **PR 18 — PR review workflow**: `pr approve` / `pr decline` (+ DELETE approve),
  `pr merge`.
- **PR 19 — `pr diff` + `pr comments`** (list/add).
- **PR 20 — source browsing**: `src`/`file` over `GET …/src/{rev}/{path}` (tree
  + file read), pairing with the existing git inference.
- **PR 21 — pipelines**: `pipeline stop`, step listing, `pipeline log`.
- **PR 22 — write parity** *(D3)*: `repo create/delete`, `project delete`, issue
  state transitions — only after the read-mostly policy decision.

## Phase R7 — Feature gaps: Confluence  *(product scope)*

- **PR 23 — `page delete`** (decide trash vs `?purge`), closing the CRUD gap.
- **PR 24 — navigation**: `page ancestors`, `page versions` (history).
- **PR 25 — blogposts**: `blogpost` read (+ create/edit) over `/blogposts`.
- **PR 26 — read polish**: `search text` shorthand that builds `text ~ "…"` CQL
  with `--space`/`--type`; drop the redundant second round-trip in `space view`;
  attachment upload (multipart).

## Phase R8 — Tooling & CI

- **PR 27 — `golangci-lint`**: add `.golangci.yml`, point `make lint` at it, add
  a CI job (staticcheck/errcheck/ineffassign/unused beyond `go vet`).
- **PR 28 — race + coverage**: `go test -race ./...` in CI; optional
  `-coverprofile`.
- **PR 29 — docs-gen & release**: a CI step that runs `gen-docs` into a temp dir
  to catch a broken doc walker; a `goreleaser` config + tag-triggered release
  workflow reusing the existing ldflags.

---

## Suggested order

1. **R1** (PR 1, then PR 2's safe parts) — correctness first.
2. **R8 PR 27–28** — get the lint/race net under everything before large refactors.
3. **R2** (PR 3 → 4 → 5 → 6 → 7) — land the shared base before feature work.
4. **R3 + R4** — UX and auth hardening.
5. **R5 → R6 → R7** — feature gaps per product, each command its own slice.
6. **R8 PR 29** — release automation last.

Confirm **D1**, **D2**, **D3** before the PRs that carry them.
