# 0003 — Destructive verbs require `--yes`

**Status:** Accepted (referred to during the build as "D3")

## Context

Adding write parity meant adding commands that *destroy* server state:
`atl-bb repo delete`, `atl-bb project delete`, `atl-conf page delete`, and
similar. These are irreversible (or nearly so) and are routinely invoked by
agents and scripts, where there is no human to catch a mistake and no
interactive "are you sure?" that a non-interactive caller could even answer.

The options considered were: an interactive confirmation prompt (useless to the
`--no-prompt` agent path, our primary audience), a global `--force`-style flag,
or a per-command required confirmation flag.

## Decision

Every command that destroys or irreversibly mutates server state requires an
explicit **`--yes`** flag. Without it, the command fails with `invalid_input`
and makes **no** API call.

- `--yes` is a confirmation, not a prompt — it works identically interactively
  and headlessly, which suits the agent-first posture.
- The guard is **validated before the client is constructed** (see the
  invariant in [engineering-notes.md](../engineering-notes.md#validate-input-before-constructing-a-client)),
  so a missing `--yes` is reported as bad input, not as an auth/site error.
- Where a delete has degrees of destructiveness, the safer default is implicit
  and the dangerous form is opt-in: `atl-conf page delete` trashes by default
  and only permanently removes with `--purge`, and **`--purge` still requires
  `--yes`**.

## Consequences

- A destructive command can never run by accident from a partial command line;
  the caller must opt in every time.
- This is a standing convention: **any new destructive verb must follow it** —
  required `--yes`, validated before auth, safer default with the dangerous
  variant gated. Reviewers enforce it.
- A small ergonomic cost (callers type `--yes`) bought in exchange for not
  silently deleting a repository, project, or page. Worth it.
- Per-command specifics are documented in
  [command-contract.md](../command-contract.md) alongside each verb.
