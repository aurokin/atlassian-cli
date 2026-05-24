# 0005 — Keychain-first token storage; tokens never in config

**Status:** Accepted

## Context

The CLIs authenticate with long-lived secrets (API tokens, PATs, OAuth refresh
tokens). Where those secrets live is a security decision. Writing them into
`config.json` is the easy path and the wrong one — config files get committed,
copied, synced, and shared. But the tools also have to run in headless and CI
environments where no OS keychain exists, and an agent workflow can't tolerate
"storing a token failed."

## Decision

A raw token is **never written to `config.json`.** A site profile stores only a
`token_ref` — an indirect pointer to where the secret actually lives — and the
value is resolved at request time. Three backends, in preference order:

| `token_ref` | Backend |
|---|---|
| `keyring` | OS keychain (macOS Keychain, Linux Secret Service, Windows Credential Manager) via `go-keyring`, service `atlassian-cli`. **Preferred.** |
| `file` | A `0600` `credentials.json` beside `config.json`. Fallback when no keychain is writable. |
| `env:NAME` | An environment variable; the value is never stored at all. The headless/CI path. |

`auth login` prefers the keychain and **falls back to the `0600` file with a
warning** when the keychain can't be written (CI, containers, minimal Linux), so
storing a token always succeeds. `--token-stdin` is the preferred interactive
input (the token never enters shell history); `--token-env` stores nothing.
`auth logout` deletes the secret from its backend before removing the profile;
`auth status` reports whether the token is *resolvable*, never its value.

## Consequences

- Committing or syncing `config.json` never leaks a credential — the secret is
  not in it.
- Storing a token never hard-fails for lack of a keychain; the worst case is a
  warned-about `0600` file. Headless callers can avoid on-disk storage entirely
  with `--token-env`.
- Tests use the `go-keyring` in-memory mock and temp config dirs, so the suite
  never touches the real keychain and never needs a real token — which is also
  the project's standing security rule (no real credentials in source, tests,
  fixtures, or committed config).
- Mechanism details and the recovery-guidance requirements are in
  [auth-design.md](../auth-design.md); end-to-end setup is in
  [auth-runbook.md](../auth-runbook.md).
