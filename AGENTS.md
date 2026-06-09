# AGENTS.md

Agent instructions for this provisional Atlassian CLI workspace.

## Read order

1. `README.md`
2. `docs/README.md`
3. `docs/command-contract.md` for currently implemented command behavior
4. `docs/consuming.md` for the output/exit-code contract and how the CLIs are used
5. `docs/auth-design.md` and `docs/auth-runbook.md`
6. `docs/access-error-model.md`
7. `docs/shared-architecture.md`
8. `docs/adr/` for the standing decisions (the *why*); `docs/engineering-notes.md` for contributor conventions and gotchas
9. `docs/releasing.md` before cutting or changing a release
10. `docs/integration-testing.md` before touching the live suite
11. `CONTRIBUTING.md` for the development loop, PR workflow, and test-harness conventions

Completed phase plans, MVP specs, and the `atl-bb` rewrite arc are historical
records under `docs/archive/` — consult them for context on *why* something was
built, not for current behavior.

## Product posture

- Stay true to official Atlassian APIs.
- Prefer explicit site/resource targeting.
- Preserve agent paths: `--json`, `--jq`, `--no-prompt`.
- Raw `api` command is a first-class escape hatch.
- Unsupported or ambiguous behavior should be documented and surfaced clearly.
- Users have different access levels. Design for good behavior under low-access, scoped-token, service-account, contributor, and admin contexts.
- All three CLIs (`atl-jira`, `atl-conf`, `atl-bb`) are implemented; new work deepens an existing surface or shares a proven seam rather than bootstrapping. New code goes through the shared foundation in `internal/cli`.
- Destructive verbs require an explicit `--yes`, and input/confirmation validation must run *before* the product client is constructed. These are standing rules — see `docs/adr/0003-destructive-verbs-require-yes.md` and `docs/engineering-notes.md`.
- Don't over-abstract product semantics: promote a shared shape only once implementation proves the seam (`docs/adr/0002-shared-foundation.md`).
- Update `docs/command-contract.md` whenever a change alters the command surface or behavior, and add an ADR under `docs/adr/` when you make a new standing decision.
- Treat repo-local skills as consumer-facing artifacts. Do not put repo-maintainer workflow instructions into a skill when they belong in `AGENTS.md`, tests, or repo docs.
- Keep the repo-local skill installable via `npx skills add https://github.com/aurokin/atlassian-cli --skill atlassian-cli`.

## Secrets

Never commit API tokens, PATs, OAuth tokens, passwords, cookies, private keys, or raw credential JSON.
