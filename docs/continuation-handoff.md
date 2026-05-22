# Continuation Handoff

> Last updated: 2026-05-20. Point-in-time handoff for continuing this work in the app or a fresh agent session.

## Current repository state

Repository: `/Users/auro/code/atlassian-cli`

Remote: `git@github.com:aurokin/atlassian-cli.git`

Branch: `bb-migration-clean-atl-bb` (Bitbucket migration docs cleanup, docs
only). Phases 1–9 and Phases B0–B2 are merged to `main`.

Status at handoff: Phases 1–9 are merged to `main` — both product CLIs
have a full MVP command surface, the output and pagination polish (`--jq`,
`--all`), secure token storage (OS keychain via
`github.com/zalando/go-keyring`, with a `0600` `credentials.json` fallback;
`config.json` never holds a raw token), the Confluence content depth
(`page comment`, `page label`, `attachment`), the deeper Jira coverage
(`issue assign`/`watch`/`unwatch`/`watchers`, `issue link` + `link types`,
`issue worklog`), and the in-repo shared-foundation extraction
(`internal/restutil` and `output.TabWriter`, scored in
`docs/shared-foundation-scorecard.md`). `go test ./...` passes. The
**Bitbucket `atl-bb` migration** is underway: Phase B0 (inventory of
legacy `bb`) is captured in `docs/bb-inventory.md`, and Phase B1
(shared-foundation comparison) extends `docs/shared-foundation-scorecard.md`
with the per-package reuse/adapt/keep/net-new decisions — the third
product validates the Phase 9 foundation rather than overturning it. See
`docs/command-contract.md` for the implemented Atlassian surface.

## Canonical CLI names

- `atl-jira` — Jira CLI
- `atl-conf` — Confluence CLI
- `atl-bb` — future Bitbucket CLI shape after import/rewrite

Do not revert to bare `jira`, bare `confluence`, `jj`, `cc`, or `conf`.

## What has been decided

- Build `atl-jira` and `atl-conf` first.
- Keep them separate from the user's perspective, with shared internal foundations only where real repetition exists.
- Support Atlassian auth modes that are true to the API:
  - Cloud classic API token: Basic auth, site URL
  - Cloud scoped API token: Basic auth, `api.atlassian.com/ex/{product}/{cloudId}`
  - Data Center PAT: Bearer auth, organization/Data Center URL
- OAuth 3LO is later, not Phase 1.
- Access-aware UX is required: permissions/scopes/product access are source of truth.
- JSON errors should include machine-readable recovery fields where known.
- Bitbucket is not migrated now. Legacy `bb` is a future behavior oracle for `atl-bb`, with an explicit import-and-rewrite period.

## Read order for continuation

1. `README.md`
2. `AGENTS.md`
3. `docs/README.md`
4. `docs/auth-design.md`
5. `docs/access-error-model.md`
6. `docs/shared-architecture.md`
7. `docs/implementation-plan.md`
8. `docs/phase-1-foundation-plan.md`
9. `docs/phase-2-resolve-browse-plan.md`
10. `docs/phase-3-jira-mvp-plan.md`
11. `docs/phase-4-confluence-mvp-plan.md`
12. `docs/post-mvp-roadmap.md`
13. `docs/phase-5-output-pagination-plan.md`
14. `docs/phase-6-secure-token-storage-plan.md`
15. `docs/phase-7-confluence-content-plan.md`
16. `docs/phase-8-jira-coverage-plan.md`
17. `docs/shared-foundation-scorecard.md`
18. `docs/phase-9-shared-foundation-plan.md`
19. Product docs only after foundation work:
   - `docs/jira-mvp.md`
   - `docs/confluence-mvp.md`
20. Bitbucket future docs only when planning migration:
   - `docs/bitbucket-migration-roadmap.md`
   - `docs/bb-rewrite-plan.md`

## Next action

The **Bitbucket `atl-bb` migration** is underway per
`docs/bitbucket-migration-roadmap.md`. Phase B0 (inventory) →
`docs/bb-inventory.md`; Phase B1 (shared-foundation comparison) → the new
Bitbucket section in `docs/shared-foundation-scorecard.md`. B1's verdict:
`atl-bb` reuses `httpclient`, `output`/`cli.Render`, `restutil`, `apperr`,
`secrets`, the config mechanics, and the resolve/browse *frameworks*; keeps
Bitbucket models, command vocabulary, pagination, and git integration
product-specific; and brings net-new capabilities (generated docs, fuzz/
stability, aliases, extensions) for a deliberate monorepo decision. The
third product validated the Phase 9 seams rather than overturning them.

Phase B1.5 (`docs/bb-rewrite-plan.md`) is done: it sets the target
package layout (`cmd/atl-bb`, `internal/bitbucket` over `httpclient`,
`internal/bbcmd`, `internal/atlbbcmd`), the Bitbucket product/Basic-auth
model, the `apperr` recovery mapping, the docs-gen strategy, the required
test coverage, and the B3+ sequence. Its decisions are now **resolved (Auro,
2026-05-20)**: D1 monorepo-with-rewrite; D2 `--site` only (no `--host`); D3
add a `feature_disabled` apperr code; D4 generalize `gen-docs` now; D5 keep
aliases/extensions Bitbucket-only initially.

Phase B2 (`docs/bb-compatibility-plan.md`) is done and reflects the
**clean-break decision (Auro): no `bb` alias/shim/deprecation window — ship
`atl-bb` directly**. It covers the clean break on the binary name, the
one-time automatic config/credential **import** (host→site, plaintext
token→`secrets`, legacy file scrubbed) so existing users do not have to
re-login, JSON-field guarantees plus the documented intentional contract
changes (structured `apperr` error output; `api` same-origin guard; `--site`
with no `--host` alias), the `bb-cli`→`atl-bb` skill retirement (pointer
note), the live-test boundary, and the "migration done" checklist. Its
decisions D6–D12 are now **resolved (Auro, 2026-05-20)**: D6 no window; D7
scrub legacy token; D8 site name `bitbucket`; D9 importer-only `BB_CONFIG_DIR`;
D10 structured errors only; D11 clean skill break; D12 freeze-with-pointer.

This completes the **planning arc** of the Bitbucket migration
(B0→B1→B1.5→B2), and **all flagged decisions D1–D12 are resolved**.
**Phase B3 — extract + port** is underway:

- **B3a (merged):** `ProductBitbucket` + the Bitbucket Cloud Basic-auth API
  base in `internal/httpclient`/`internal/appinfo`, the new `feature_disabled`
  apperr code, and the typed `internal/bitbucket` client over `httpclient`
  (raw `json.RawMessage` returns, `Decode[T]`, `next`-URL pagination follower,
  the `feature_disabled` remap), with error-mapping tests. No commands.
- **B3b (in progress):** the command tree in `internal/bbcmd` +
  `internal/atlbbcmd` + `cmd/atl-bb`, ported in vertical slices. Shipped:
  `repo` (`view`, `list`) with the `--repo`/`--workspace` targeting helper,
  `pr` (`list`, `view`, `create`) with `--state` filtering, `pipeline`
  (`list`, `view` by build-number or UUID, `run`) with `--status` filtering,
  `issue` (`list`, `view`, `create`) — the first commands to exercise the
  `feature_disabled` remap (a repo with its issue tracker off) — `workspace`
  (`list`, `view`), `project` (`list`, `view`, `create`), `commit`
  (`list` with `--revision`, `view`), `branch` (`list`, `view`, `create`,
  `delete`), `tag` (`list`, `view`, `create`, `delete`), and `deployment` /
  `environment` (read-only `list`, `view`; deployment **variables** deferred
  as they hold secret values), `search` (`repos`, `prs`, `issues` taking a
  raw Bitbucket `q` query, mirroring atl-jira's raw-JQL search), and `status`
  (live `GET /user` auth check). The shared `resolve`/`browse` commands now
  recognize Bitbucket inputs via a `Bitbucket` parser in `internal/resolve`
  (bare `workspace/repo`, repo/PR/issue/commit web URLs; `CanonicalURL` maps
  the API host to `bitbucket.org`). The shared `api`/`auth` commands are
  inherited from the cli root and work for Bitbucket unchanged. Decision
  (Auro): under `--json`/`--jq`, `atl-bb` emits the **verbatim Bitbucket API
  body** like `atl-jira`/`atl-conf` — a documented break from legacy `bb`'s
  custom payload field names. B3b is functionally complete.
- **B3c (in progress):** ergonomics. **Git-checkout inference** is done — a
  new `internal/git` package infers `<workspace>/<repo>` from the local
  Bitbucket remote (offline, best-effort), wired into `resolveRepoTarget` as
  the no-target fallback (an explicit `--workspace` skips it). **Generalized
  `gen-docs`** is done — `cmd/gen-docs` builds any product's root via its
  `atl*cmd.NewRoot` and emits a Markdown tree with `cobra/doc`
  (`go run ./cmd/gen-docs --product all --out docs/cli`); adding a product is a
  one-line builder-map change. **Command aliases** are done — an
  `atl-bb alias set/list/delete` group stores shorthands in the shared
  config's top-level `aliases` map (atl-bb-only per D5), expanded before
  dispatch in `atlbbcmd.Run` (shell-style split, depth-8 recursion,
  cycle-safe). Remaining B3c: the extension mechanism (gh-style `bb-<name>`
  external binaries on PATH).

Standalone Jira/Confluence deepening remains available in parallel.

OAuth 3LO remains deferred until token-based auth is proven robust in
production use. Standalone Jira/Confluence deepening (issue link
edit/delete, worklog edit/delete, Confluence inline comments, attachment
upload) remains available as independent phases.

Architecture note: the shared command wiring (`root`, `version`, `auth`,
`api`, `resolve`, `browse`) lives in `internal/cli`, which also exports
`SiteClient` and `Render` as the seam for product commands. The Jira command
tree lives in `internal/jiracmd` over a typed `internal/jira` client, layered
onto the shared root by `atljiracmd`. The Confluence command tree lives in
`internal/confcmd` over a typed `internal/conf` client, layered onto the
shared root by `atlconfcmd` — the same shape as the Jira side, with no
`internal/cli` changes required.

## Implementation guardrails

- Commit after each small implementation task.
- Push after commits.
- Run `go test ./...` after each code task.
- Do not store raw tokens in tests, docs, fixtures, or committed config.
- Token storage (Phase 6): the OS keychain or a `0600` fallback file; tokens
  never go in `config.json`. Tests use the go-keyring in-memory mock.
- No live Atlassian API calls in default tests.
- Use local `httptest.Server` for HTTP command tests.
- Keep raw `api` command as the first real API integration point.
- Avoid broad product commands until foundation commands work.

## Documentation update rule

When implementation starts, update or create:

- `docs/command-contract.md` for implemented command behavior
- `README.md` current status
- `docs/README.md` design-doc index if new docs are added
- `AGENTS.md` if read order or guardrails change

## Verification commands

Use these before reporting success after implementation begins:

```bash
git status --short
go test ./...
go run ./cmd/atl-jira --help
go run ./cmd/atl-conf --help
go run ./cmd/atl-jira version --json
go run ./cmd/atl-conf version --json
git diff --check
```

## Known non-goals right now

- Do not import Bitbucket source yet.
- Do not add OAuth 3LO yet.
- Do not implement full Jira issue/project coverage yet.
- Do not implement full Confluence page/space coverage yet.
- Do not add browser login or cookie/session scraping.
- Do not fake parity across Atlassian products.
