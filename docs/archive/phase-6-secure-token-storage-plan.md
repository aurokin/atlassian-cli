> **Historical / archived.** This document is a completed planning or design artifact, kept for project history. It is **not** maintained and may not reflect the current code. For the implemented behavior see [`docs/command-contract.md`](../command-contract.md) and [`docs/shared-architecture.md`](../shared-architecture.md).

# Phase 6 — Secure Token Storage: Implementation Plan

> Detailed task breakdown for Phase 6 of `docs/post-mvp-roadmap.md`. Phase 5
> (`--jq` and `--all`) is merged to `main`. This phase lets a user store a
> credential once instead of passing `--token-env` on every invocation.

## Goal

`auth login` can accept a token value and store it securely, so later
commands authenticate without re-supplying the credential. `--token-env`
remains the headless/CI path and is unchanged.

## Resolved design decisions

These were open in the roadmap and are now settled:

1. **Keychain backend — `github.com/zalando/go-keyring`.** The project's
   second deliberate dependency after `gojq`. It wraps the macOS Keychain, the
   Linux Secret Service (libsecret/D-Bus), and the Windows Credential Manager
   behind one cross-platform API, and ships a process-global mock
   (`keyring.MockInit`) for tests.
2. **Headless fallback — 0600 file, with a warning.** When the keyring is not
   usable (CI, containers, minimal Linux), `auth login` stores the token in a
   restricted-permission (`0600`) `credentials.json` beside `config.json` and
   prints a clear warning that the token is not keychain-protected. Storing a
   token always succeeds; it never silently fails.
3. **No secrets in the repo.** Unchanged guardrail. `config.json` never holds
   a raw token. Tests use `keyring.MockInit` and `t.TempDir()`; no real token
   touches tests, fixtures, docs, or committed config.

## Credential model

A profile's `token_ref` records *where* the token lives. Phase 6 adds two
forms to the existing one:

| `token_ref` | Meaning | Set by |
|-------------|---------|--------|
| `env:NAME`  | Token is in environment variable `NAME` (value never stored). | `--token-env NAME` |
| `keyring`   | Token is in the OS keychain, service `atlassian-cli`, account = site name. | `--token` / `--token-stdin`, keyring usable |
| `file`      | Token is in the `0600` `credentials.json` fallback, keyed by site name. | `--token` / `--token-stdin`, keyring not usable |

`config.json` stays secret-free; only `token_ref` (an indirect pointer) is
written there. The `file` backend's `credentials.json` is the one place a raw
token can land on disk, always at `0600`, and only when no keychain exists.

## New package: `internal/secrets`

```go
// Store is one credential backend keyed by site name.
type Store interface {
    Name() string                       // "keyring" or "file": the token_ref value
    Set(site, token string) error
    Get(site string) (string, error)    // apperr "token_unavailable" when absent
    Delete(site string) error           // no error when already absent
}
```

- `keyringStore` — wraps `go-keyring`; `Name() == "keyring"`.
- `fileStore` — `0600` `credentials.json` (atomic write, mirroring
  `config.Save`); `Name() == "file"`.
- `Writable(credPath string) (s Store, fellBack bool, err error)` — returns the
  keyring store when a probe write succeeds, otherwise the file store with
  `fellBack == true`. The caller emits the warning when `fellBack` is true.
- `ForRef(ref, credPath string) (Store, error)` — maps a recorded `keyring` /
  `file` ref back to its backend for read and delete.

`config` gains `CredentialsPath()` returning `<config-dir>/credentials.json`,
so the file store honors `XDG_CONFIG_HOME` exactly like `config.json`.

## Tasks

### Task 1 — `internal/secrets` package

Add the `go-keyring` dependency. Implement `Store`, `keyringStore`,
`fileStore`, `Writable`, and `ForRef`. Add `config.CredentialsPath()`.
Tests: `keyringStore` via `keyring.MockInit`; `fileStore` via `t.TempDir()`
(set/get/delete round trip, `0600` permission, absent-key error, atomic
write); `ForRef` dispatch.
Commit: `feat: add internal/secrets credential store`.

### Task 2 — `auth login` token capture

Add `--token` (value) and `--token-stdin` (read one line from stdin) to
`auth login`. `--token`, `--token-stdin`, and `--token-env` are mutually
exclusive; document `--token-stdin` as the no-shell-history path. When a
token value is supplied, store it through `secrets.Writable`, record
`token_ref` = the store's `Name()`, and on fallback print the `0600`-file
warning. `--token-env` keeps recording `env:NAME` with no value stored.
No token flag at all keeps the current behavior (profile saved, no ref).
Tests: keyring happy path, file-fallback path, mutual-exclusion errors.
Commit: `feat: store auth login tokens via the secrets package`.

### Task 3 — resolve, status, logout

Extend `resolveToken` (`internal/cli`) to dispatch `keyring` / `file` refs
through `secrets.ForRef` alongside the existing `env:` form. Extend
`tokenStatus` so `auth status` reports stored-credential availability
(present vs missing) without ever printing the value. Extend `auth logout`
to delete the stored secret for the site via `secrets.ForRef` before the
profile is removed. Tests for each path.
Commit: `feat: resolve, report, and clear stored credentials`.

### Task 4 — docs, review, PR

Update `docs/auth-design.md` (storage model, `token_ref` forms, the keychain
backend and fallback), `docs/command-contract.md` (the new `auth login`
flags and storage behavior), `README.md`, `docs/README.md`, and
`docs/continuation-handoff.md`. Run the multi-agent review wave until clean.
Commit: `docs: document secure token storage`. Open PR.

## Done definition

- A user can `auth login` with `--token`/`--token-stdin`, have the token
  stored in the OS keychain (or the `0600` fallback file with a warning), and
  run commands without re-supplying it.
- No raw token is ever written to `config.json` or committed to the repo.
- `--token-env` still works as the headless/CI path.
- `auth status` reports stored-credential availability; `auth logout` clears
  the stored secret.
- `auth-design.md` and `command-contract.md` document the storage model.

## Out of scope

- An interactive no-echo TTY prompt (would add `golang.org/x/term`);
  `--token-stdin` covers secure entry without it.
- `auth status --check` live token verification (a later phase).
- OAuth 3LO — still deferred until token auth is proven robust.
