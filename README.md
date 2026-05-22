# Atlassian CLI Workspace

> Provisional private workspace. Canonical binary family: `atl-*`.

This repository designs and implements true-to-API Atlassian CLIs:

- `atl-jira` for Jira
- `atl-conf` for Confluence
- `atl-bb` for Bitbucket Cloud

All three are implemented and share one command tree built in `internal/cli`.
The posture is inherited from `bb`, Auro's original Bitbucket Cloud CLI, with `atl-bb` as the unified-name successor: official API behavior first, deterministic targeting, structured output for agents, no fake parity when Atlassian does not expose a real API path.

## Current status

All three CLIs are implemented and merged to `main`. The shared foundation
(root command, global flags, output rendering, config, auth, the raw `api`
escape hatch, offline `resolve`/`browse`) plus the per-product surfaces below
are complete; see [docs/command-contract.md](docs/command-contract.md) for the
exact command behavior and known limitations.

- **`atl-jira`** — `project`, `issue` (view/list, create/edit/transition,
  `assign`/`watch`/`unwatch`/`watchers`, `link` + `link types`,
  `worklog` list/add, `comment` create/edit/delete), `search issues` (JQL),
  and `status`.
- **`atl-conf`** — `space`, `page` (read plus create/edit), `page comment`,
  `page label`, `attachment` (list/download), `search cql`, and `status`. It
  is a mixed-version client (REST v2, with documented v1 fallbacks for CQL
  search, the current-user lookup, and label writes).
- **`atl-bb`** — `repo`, `pr`, `pipeline`, `issue`, `workspace`, `project`,
  `commit`, `branch`, `tag`, `deployment`, `environment`, `search`, and
  `status`, with built-in git-checkout repository inference.

Shared across **every** binary: `version`, `auth`, `api`, `resolve`, `browse`,
plus `alias` (command shorthands) and `extension` (gh-style `<binary>-<name>`
external commands). Output is human-readable by default and verbatim API JSON
under `--json`/`--jq`; list/search commands take `--limit` and `--all`
(follow all pages); tokens are stored in the OS keychain (or a `0600`-mode
fallback file).

The phased build history (Phases 1–9 for the shared foundation and the
Jira/Confluence MVPs and depth, then B0–B3c for the Bitbucket rewrite) is
recorded in [docs/post-mvp-roadmap.md](docs/post-mvp-roadmap.md) and the
phase plans under `docs/`.

```bash
go test ./...
go run ./cmd/atl-jira project list --site work
go run ./cmd/atl-conf space list --site work
go run ./cmd/atl-conf search cql 'type = page' --site work
go run ./cmd/atl-conf page create --space DEV --title Notes \
  --body '<p>hi</p>' --body-format storage --site work
go run ./cmd/atl-jira issue view PROJ-1 --site work --jq '.fields.status.name'
go run ./cmd/atl-bb repo view acme/widgets --site work --json
```

See [docs/command-contract.md](docs/command-contract.md) for the implemented
command surface and known limitations.

Start here:

1. [docs/README.md](docs/README.md)
2. [docs/command-contract.md](docs/command-contract.md)
3. [docs/auth-design.md](docs/auth-design.md)
4. [docs/access-error-model.md](docs/access-error-model.md)
5. [docs/shared-architecture.md](docs/shared-architecture.md)
6. [docs/implementation-plan.md](docs/implementation-plan.md)
7. [docs/phase-1-foundation-plan.md](docs/phase-1-foundation-plan.md)
8. [docs/phase-2-resolve-browse-plan.md](docs/phase-2-resolve-browse-plan.md)
9. [docs/phase-3-jira-mvp-plan.md](docs/phase-3-jira-mvp-plan.md)
10. [docs/phase-4-confluence-mvp-plan.md](docs/phase-4-confluence-mvp-plan.md)
11. [docs/post-mvp-roadmap.md](docs/post-mvp-roadmap.md)
12. [docs/bb-inventory.md](docs/bb-inventory.md), [docs/bb-rewrite-plan.md](docs/bb-rewrite-plan.md), [docs/bb-compatibility-plan.md](docs/bb-compatibility-plan.md) — the Bitbucket (`atl-bb`) rewrite
13. [docs/continuation-handoff.md](docs/continuation-handoff.md)

## Guardrails

- Keep Jira, Confluence, and Bitbucket as separate CLIs from the user's perspective.
- Do not over-abstract; promote shared shapes only once implementation has proven the seam (as was done for `internal/restutil` and the shared `alias`/`extension` commands).
- Keep `docs/continuation-handoff.md` current when plans, status, or next actions change.
- Never store real tokens, passwords, OAuth refresh tokens, cookies, or private credential files in this repo.
