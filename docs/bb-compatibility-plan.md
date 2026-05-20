# Bitbucket Compatibility Plan (Phase B2)

> How to ship `atl-bb` without breaking current `bb` users and agents. Written
> after B0 ([bb-inventory.md](bb-inventory.md)), B1 (the Bitbucket section of
> [shared-foundation-scorecard.md](shared-foundation-scorecard.md)), and B1.5
> ([bb-rewrite-plan.md](bb-rewrite-plan.md)). Compatibility = preserve **user
> value and stable machine contracts**, not every internal detail. Decision
> IDs continue the B1.5 sequence (D6+).

## 1. Binary name and the legacy `bb` shim

- Canonical name: **`atl-bb`** (matches `atl-jira`, `atl-conf`).
- Keep a **`bb` compatibility shim** for a deprecation window: a tiny wrapper
  binary (or symlink) that forwards all args to `atl-bb` and, on a TTY, prints
  a one-line deprecation notice to **stderr** (never stdout, so `--json`/`--jq`
  pipelines stay clean).
- Remove `bb` only after the window closes and the notice has shipped.

> **D6 — deprecation window length.** Recommend two minor releases of dual
> availability before `bb` is removed. Flagged.

## 2. Config path and automatic migration

Legacy: `$BB_CONFIG_DIR/config.json` else `os.UserConfigDir()/bb/config.json`,
host-keyed, **plaintext token**. Target: `XDG_CONFIG_HOME/atlassian-cli/config.json`,
site-keyed, `token_ref` indirection + `internal/secrets`.

Migration (runs once, automatically, on first `atl-bb` invocation when an
`atlassian-cli` config does not yet exist and a legacy `bb` config does):

1. Read the legacy `bb/config.json`.
2. For each host, create a site profile: `ProductBitbucket`, Cloud Basic
   auth, `username` carried over.
3. Move each plaintext `token` into the secret store (keychain, else `0600`
   `credentials.json`); record a `token_ref` in the new config. **No raw token
   is ever written to the new `config.json`.**
4. Carry `default_host`→default site, and (per D5) `aliases`/settings if kept.
5. Write the new config (`0700` dir / `0600` file).
6. **Scrub the legacy plaintext token:** rewrite the old `bb/config.json` with
   the token removed (or rename it to `config.json.migrated`) so a plaintext
   credential is not left on disk, and print a one-line notice naming both
   paths.

Fallback if migration is skipped or fails: `atl-bb auth login` produces a
clean new credential (the always-available path).

> **D7 — legacy token scrubbing.** Recommend scrubbing/renaming the old file
> after a successful secret-store migration (security win) rather than leaving
> the plaintext token in place. This mutates legacy state, so it is flagged.
>
> **D8 — default site name.** Recommend mapping host `bitbucket.org` to a site
> profile named `bitbucket`. Flagged (alternative: keep the literal
> `bitbucket.org`).
>
> **D9 — env var aliases.** Recommend honoring `BB_CONFIG_DIR` and
> `BB_API_BASE_URL` as deprecated aliases (alongside `XDG_CONFIG_HOME` and the
> `atl-*` test-base override) during the window. Flagged.

## 3. Command behavior and JSON field guarantees

- Preserve command paths and flags for the agent-facing commands: `repo view`,
  `pr list`/`view`/`create`, `status`, `resolve`, `browse --no-browser`,
  `api`, `auth status`. Capture **golden fixtures from current `bb`** (human +
  `--json`/`--jq`) before the rewrite touches those paths; field additions are
  allowed, renames/removals are versioned and documented.
- Preserve `--json` (empty / `*` / field list), `--jq` (requires `--json`),
  `--no-prompt`, and the bare-`--json`→`--json=*` normalization. (B1 confirmed
  these already match the foundation.)
- Preserve `resolve` JSON shape for the known Bitbucket URL fixtures and
  `browse --no-browser` canonical URLs (golden + the existing fuzz corpus).

### Intentional, documented contract changes

1. **Error output shape.** Legacy `bb` writes guided **prose** to stderr.
   `atl-bb` emits the structured `apperr` envelope (`error` code + `message` +
   `next` + context) to stderr under `--json`, and a plain `Error: <code>:
   <message>` line otherwise. This is an **improvement** to the machine
   contract but a change for anything that scraped `bb`'s prose. Documented in
   release notes.
2. **`api` same-origin guard.** `atl-bb api <absolute-url>` rejects off-origin
   hosts (`untrusted_url`), unlike `bb api`. Relative paths are unaffected.
3. **Credential flag.** `--site` selects the credential; `--host` is kept as a
   hidden deprecated alias for one release (ties to D2).

> **D10 — error-output compatibility.** Recommend shipping the structured
> error model as an intentional improvement (no `bb`-prose compatibility
> mode). Flagged in case any known consumer parses `bb` stderr prose.

## 4. Repo-local `bb-cli` skill

Legacy skill installs via
`npx skills add https://github.com/aurokin/bitbucket_cli --skill bb-cli`.

- Publish a new **`atl-bb` skill** describing the `atl-bb` surface and the
  deterministic agent flags (`--repo`, `--json`, `--jq`, `--no-prompt`).
- During the window, keep the `bb-cli` skill installable but updated to point
  at `atl-bb` and note the rename.

> **D11 — skill name + home.** Recommend a new `atl-bb` skill in the monorepo
> while leaving `bb-cli` as a thin pointer for the window. Flagged (whether to
> retire the `bb-cli` name entirely).

## 5. Generated docs

`bb`'s generated docs live in-repo (`docs/`), not on a hosted site, so there
are no public URLs to redirect. Plan: regenerate under the monorepo via the
generalized `gen-docs` (B1.5/D4). If the legacy `bb` repo stays public, freeze
its `docs/` with a pointer to the new home.

> **D12 — legacy docs disposition.** Recommend freeze-with-pointer if the `bb`
> repo remains public; delete if the repo is archived. Flagged (depends on D1
> repo-shape and whether the `bb` repo stays public).

## 6. Manual live-test boundary

Carry the `integration/` harness and its conventions unchanged: live Bitbucket
Cloud tests stay **manual-only**, env/credential-gated, never in
`go test ./...` or CI. Reuse the existing fixture workspace/project; use
sacrificial fixtures for destructive flows.

## 7. Compatibility checklist (gate for "migration done")

- [ ] `atl-bb auth login/status/logout` works with migrated config; legacy
      `bb` still works (shim) or has a documented transition.
- [ ] `atl-bb api` behavior matches intended Bitbucket API behavior; the
      same-origin guard delta is documented.
- [ ] `atl-bb resolve` outputs compatible JSON for the known URL fixtures.
- [ ] `atl-bb browse --no-browser` outputs compatible URLs.
- [ ] `--json`, `--jq`, `--no-prompt` behavior preserved.
- [ ] Config auto-migration tested: host→site, plaintext token→secret store,
      legacy token scrubbed, default carried over.
- [ ] Golden output/JSON tests pass for the agent-facing commands.
- [ ] Generated docs regenerated and reviewed.
- [ ] `bb-cli`/`atl-bb` skill points to valid install/use instructions.
- [ ] Live tests remain manual-only.
- [ ] `make check` (or the monorepo equivalent) passes in the new location.
- [ ] Core read-path startup/API-call performance is no worse than `bb`, or
      any regression is documented and accepted.

## 8. Open decisions carried forward

| ID | Decision | Recommendation |
|---|---|---|
| D6 | `bb` deprecation window length | two minor releases of dual availability |
| D7 | scrub legacy plaintext token after migration | yes, scrub/rename the old file |
| D8 | default site name for `bitbucket.org` | `bitbucket` |
| D9 | honor `BB_CONFIG_DIR`/`BB_API_BASE_URL` as deprecated aliases | yes, during the window |
| D10 | ship structured errors with no `bb`-prose compat mode | yes (improvement) |
| D11 | skill name/home | new `atl-bb` skill; `bb-cli` as a pointer for the window |
| D12 | legacy `bb` repo docs disposition | freeze-with-pointer if public, else delete |

(D1–D5 are in [bb-rewrite-plan.md](bb-rewrite-plan.md).)

## Next

This completes the **planning** arc of the Bitbucket migration (B0→B1→B1.5→B2).
**Phase B3** (extract shared libraries behind stable APIs, then port the
typed client and command tree in vertical slices) is the first
implementation phase and should not begin until the flagged decisions
(D1–D12) are confirmed, since several (repo shape, config migration, error
model) shape the very first PRs.
