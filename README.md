# Atlassian CLI Workspace

> Provisional private workspace. Naming is not final.

This repository is for designing and eventually implementing true-to-API Atlassian CLIs:

- `atl-jira` for Jira
- `atl-conf` for Confluence
- `atl-bb` for Bitbucket, when/if the existing `bb` CLI is brought into the shared shape

The posture is inherited from `bb`, Auro's existing Bitbucket Cloud CLI, with `atl-bb` as the intended unified-name successor: official API behavior first, deterministic targeting, structured output for agents, no fake parity when Atlassian does not expose a real API path.

## Current status

Design scaffold only. No CLI implementation yet.

Start here:

1. [docs/atlassian-cli/README.md](docs/atlassian-cli/README.md)
2. [docs/atlassian-cli/auth-design.md](docs/atlassian-cli/auth-design.md)
3. [docs/atlassian-cli/access-error-model.md](docs/atlassian-cli/access-error-model.md)
4. [docs/atlassian-cli/shared-architecture.md](docs/atlassian-cli/shared-architecture.md)
5. [docs/atlassian-cli/implementation-plan.md](docs/atlassian-cli/implementation-plan.md)

## Guardrails

- Keep Jira and Confluence as separate CLIs from the user's perspective.
- Do not over-abstract before implementation teaches us the real seams.
- Keep Bitbucket migration as a later roadmap item, not an early constraint.
- Never store real tokens, passwords, OAuth refresh tokens, cookies, or private credential files in this repo.
