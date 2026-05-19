# Continuation Handoff

> Last updated: 2026-05-19. Point-in-time handoff for continuing this work in the app or a fresh agent session.

## Current repository state

Repository: `/Users/auro/code/atlassian-cli`

Remote: `git@github.com:aurokin/atlassian-cli.git`

Branch: `phase-7-confluence-content` (Phase 7 work). Phases 1–6 and the
post-MVP roadmap are merged to `main`.

Status at handoff: Phases 1–6 are merged to `main` — both product CLIs have a
full MVP command surface plus the output and pagination polish (`--jq`,
`--all`) and secure token storage (OS keychain via `github.com/zalando/go-keyring`,
with a `0600` `credentials.json` fallback; `config.json` never holds a raw
token). `docs/post-mvp-roadmap.md` sequences the post-MVP work into Phases
5–8. Phase 7 — Confluence content depth — is implemented on the
`phase-7-confluence-content` branch per
`docs/phase-7-confluence-content-plan.md`: `atl-conf` gains `page comment`
(footer comments: list/view/create/edit/delete), `page label`
(list/add/remove, written through the v1 content-label surface), and
`attachment` (list, and download to a file or stdout). `go test ./...`
passes. See `docs/command-contract.md` for the implemented surface.

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
16. Product docs only after foundation work:
   - `docs/jira-mvp.md`
   - `docs/confluence-mvp.md`
17. Bitbucket future docs only when planning migration:
   - `docs/bitbucket-migration-roadmap.md`
   - `docs/bb-rewrite-plan.md`

## Next action

Phase 7 (Confluence content depth) is implemented on
`phase-7-confluence-content` and ready for its PR: `page comment`, `page
label`, and `attachment` commands.

Next: **Phase 8 — deeper Jira coverage** (assign, watchers, links, worklog)
per `docs/post-mvp-roadmap.md`.

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
