# Continuation Handoff

> Last updated: 2026-05-17. Point-in-time handoff for continuing this work in the app or a fresh agent session.

## Current repository state

Repository: `/Users/auro/code/atlassian-cli`

Remote: `git@github.com:aurokin/atlassian-cli.git`

Branch: `phase-1-foundation` (Phase 1 work; not yet merged to `main`).

Status at handoff: Phase 1 foundation is implemented. The Go module, both
binaries, shared global flags, config store, output renderer, structured
errors, auth signing, HTTP client, the `auth` subtree, and the raw `api`
command are in place with `go test ./...` passing. Product-specific Jira and
Confluence commands are not started. See `docs/command-contract.md` for the
implemented surface.

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
9. Product docs only after foundation work:
   - `docs/jira-mvp.md`
   - `docs/confluence-mvp.md`
10. Bitbucket future docs only when planning migration:
   - `docs/bitbucket-migration-roadmap.md`
   - `docs/bb-rewrite-plan.md`

## Next action

Phase 1 (`docs/phase-1-foundation-plan.md`) is complete on the
`phase-1-foundation` branch. Next:

1. Review and merge `phase-1-foundation` into `main`.
2. Plan Phase 2: the first product commands, guided by `docs/jira-mvp.md` and
   `docs/confluence-mvp.md`.
3. Consider deferred foundation items when they become relevant: JSON error
   envelope rendering, `--jq` filtering, `--trace`, and secure token storage.

One Phase 1 architecture note: the shared root/version/auth/api wiring lives in
`internal/cli` (a package not named in the original plan layout) so the
`atljiracmd` and `atlconfcmd` packages stay thin product wrappers.

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
