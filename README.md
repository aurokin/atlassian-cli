# Atlassian CLI Workspace

> Provisional private workspace. Canonical binary family: `atl-*`.

This repository is for designing and implementing true-to-API Atlassian CLIs:

- `atl-jira` for Jira
- `atl-conf` for Confluence
- `atl-bb` for Bitbucket, when/if the existing `bb` CLI is brought into the shared shape

The posture is inherited from `bb`, Auro's existing Bitbucket Cloud CLI, with `atl-bb` as the intended unified-name successor: official API behavior first, deterministic targeting, structured output for agents, no fake parity when Atlassian does not expose a real API path.

## Current status

Phase 1 (shared foundation), Phase 2 (offline URL resolution), and Phase 3
(the Jira MVP — read-only `project`/`issue`/`search`/`status` plus the
mutating `issue` create/edit/transition and `issue comment`
create/edit/delete) are merged to `main`. Phase 4A adds the read-only
Confluence product commands — `space` (list/view), `page` (list/view/children),
`search cql`, and `status` — over a typed Confluence API client. The Confluence
page write commands are not implemented yet.

```bash
go test ./...
go run ./cmd/atl-jira --help
go run ./cmd/atl-jira resolve PROJ-123 --json
go run ./cmd/atl-jira project list --site work
go run ./cmd/atl-conf space list --site work
go run ./cmd/atl-conf search cql 'type = page' --site work
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
11. [docs/continuation-handoff.md](docs/continuation-handoff.md)

## Guardrails

- Keep Jira and Confluence as separate CLIs from the user's perspective.
- Do not over-abstract before implementation teaches us the real seams.
- Keep Bitbucket migration as a later roadmap item, not an early constraint.
- Start implementation from `docs/phase-1-foundation-plan.md`.
- Keep `docs/continuation-handoff.md` current when plans, status, or next actions change.
- Never store real tokens, passwords, OAuth refresh tokens, cookies, or private credential files in this repo.
