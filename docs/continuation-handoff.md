# Continuation Handoff

> Last updated: 2026-05-19. Point-in-time handoff for continuing this work in the app or a fresh agent session.

## Current repository state

Repository: `/Users/auro/code/atlassian-cli`

Remote: `git@github.com:aurokin/atlassian-cli.git`

Branch: `phase-5b-pagination` (Phase 5B work). Phases 1–4, the post-MVP
roadmap, and Phase 5A are merged to `main`.

Status at handoff: Phases 1–4 are merged to `main` — both product CLIs have a
full MVP command surface — and `docs/post-mvp-roadmap.md` sequences the
post-MVP work into Phases 5–8. Phase 5A — `--jq` filtering backed by
`github.com/itchyny/gojq` (the project's first dependency beyond Cobra) — is
merged to `main`. Phase 5B — the `--all` follow-all-pages flag — is
implemented on the `phase-5b-pagination` branch per
`docs/phase-5-output-pagination-plan.md`: a page-following helper added to the
Jira and Confluence clients handles the offset, token, and cursor pagination
styles, and `--all` is wired onto every list/search command, emitting a
synthesized aggregate under a 100-page cap. `go test ./...` passes. See
`docs/command-contract.md` for the implemented surface.

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
14. Product docs only after foundation work:
   - `docs/jira-mvp.md`
   - `docs/confluence-mvp.md`
15. Bitbucket future docs only when planning migration:
   - `docs/bitbucket-migration-roadmap.md`
   - `docs/bb-rewrite-plan.md`

## Next action

Phase 5B (`--all`) is implemented on `phase-5b-pagination` and ready for
PR 2. Phase 5 (5A + 5B) completes the output and pagination polish.

Next: **Phase 6 — secure token storage**, per `docs/post-mvp-roadmap.md`. It
moves credential handling beyond `--token-env` toward OS-keychain-backed
secure storage.

Phases 7–8 follow: Confluence content depth, deeper Jira coverage. Phases 7
and 8 are independent and may be reordered.

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
- Use `--token-env` first; defer raw token prompts and secure storage until explicitly designed.
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
