# AGENTS.md

Agent instructions for this provisional Atlassian CLI workspace.

## Read order

1. `README.md`
2. `docs/atlassian-cli/README.md`
3. `docs/atlassian-cli/auth-design.md`
4. `docs/atlassian-cli/access-error-model.md`
5. `docs/atlassian-cli/shared-architecture.md`
6. `docs/atlassian-cli/implementation-plan.md`
7. `docs/atlassian-cli/phase-1-foundation-plan.md` before starting Go implementation
8. `docs/atlassian-cli/continuation-handoff.md` when resuming from the app or a fresh session
9. Product MVP docs before product-specific work:
   - `docs/atlassian-cli/jira-mvp.md`
   - `docs/atlassian-cli/confluence-mvp.md`

## Product posture

- Stay true to official Atlassian APIs.
- Prefer explicit site/resource targeting.
- Preserve agent paths: `--json`, `--jq`, `--no-prompt`.
- Raw `api` command is a first-class escape hatch.
- Unsupported or ambiguous behavior should be documented and surfaced clearly.
- Users have different access levels. Design for good behavior under low-access, scoped-token, service-account, contributor, and admin contexts.
- Start implementation from `docs/atlassian-cli/phase-1-foundation-plan.md`.
- Update `docs/atlassian-cli/continuation-handoff.md` whenever the next action, repo status, or implementation surface changes.

## Secrets

Never commit API tokens, PATs, OAuth tokens, passwords, cookies, private keys, or raw credential JSON.
