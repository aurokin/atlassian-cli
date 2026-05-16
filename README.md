# Atlassian CLI Workspace

> Provisional private workspace. Canonical binary family: `atl-*`.

This repository is for designing and implementing true-to-API Atlassian CLIs:

- `atl-jira` for Jira
- `atl-conf` for Confluence
- `atl-bb` for Bitbucket, when/if the existing `bb` CLI is brought into the shared shape

The posture is inherited from `bb`, Auro's existing Bitbucket Cloud CLI, with `atl-bb` as the intended unified-name successor: official API behavior first, deterministic targeting, structured output for agents, no fake parity when Atlassian does not expose a real API path.

## Current status

Design and implementation-plan scaffold only. No Go CLI implementation yet. The next implementation entry point is [docs/phase-1-foundation-plan.md](docs/phase-1-foundation-plan.md).

Start here:

1. [docs/README.md](docs/README.md)
2. [docs/auth-design.md](docs/auth-design.md)
3. [docs/access-error-model.md](docs/access-error-model.md)
4. [docs/shared-architecture.md](docs/shared-architecture.md)
5. [docs/implementation-plan.md](docs/implementation-plan.md)
6. [docs/phase-1-foundation-plan.md](docs/phase-1-foundation-plan.md)
7. [docs/continuation-handoff.md](docs/continuation-handoff.md)

## Guardrails

- Keep Jira and Confluence as separate CLIs from the user's perspective.
- Do not over-abstract before implementation teaches us the real seams.
- Keep Bitbucket migration as a later roadmap item, not an early constraint.
- Start implementation from `docs/phase-1-foundation-plan.md`.
- Keep `docs/continuation-handoff.md` current when plans, status, or next actions change.
- Never store real tokens, passwords, OAuth refresh tokens, cookies, or private credential files in this repo.
