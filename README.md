# Atlassian CLI Workspace

> Provisional private workspace. Canonical binary family: `atl-*`.

This repository is for designing and implementing true-to-API Atlassian CLIs:

- `atl-jira` for Jira
- `atl-conf` for Confluence
- `atl-bb` for Bitbucket, when/if the existing `bb` CLI is brought into the shared shape

The posture is inherited from `bb`, Auro's existing Bitbucket Cloud CLI, with `atl-bb` as the intended unified-name successor: official API behavior first, deterministic targeting, structured output for agents, no fake parity when Atlassian does not expose a real API path.

## Current status

Phase 1 foundation and Phase 2 URL resolution are implemented: both binaries
build, with shared global flags, config, structured output and errors, auth
signing, an HTTP client, the `auth login/status/logout` subtree, the raw `api`
command, and the offline `resolve`/`browse` commands. Product-specific Jira and
Confluence commands are not implemented yet.

```bash
go test ./...
go run ./cmd/atl-jira --help
go run ./cmd/atl-jira resolve PROJ-123 --json
go run ./cmd/atl-conf version --json
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
9. [docs/continuation-handoff.md](docs/continuation-handoff.md)

## Guardrails

- Keep Jira and Confluence as separate CLIs from the user's perspective.
- Do not over-abstract before implementation teaches us the real seams.
- Keep Bitbucket migration as a later roadmap item, not an early constraint.
- Start implementation from `docs/phase-1-foundation-plan.md`.
- Keep `docs/continuation-handoff.md` current when plans, status, or next actions change.
- Never store real tokens, passwords, OAuth refresh tokens, cookies, or private credential files in this repo.
