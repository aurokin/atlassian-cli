# Bitbucket Compatibility Plan (Phase B2)

> How to ship `atl-bb` while preserving the data and machine contracts current
> `bb` users and agents rely on. Written after B0
> ([bb-inventory.md](bb-inventory.md)), B1 (the Bitbucket section of
> [shared-foundation-scorecard.md](shared-foundation-scorecard.md)), and B1.5
> ([bb-rewrite-plan.md](bb-rewrite-plan.md)). Compatibility here = preserve
> **stable machine contracts and stored credentials**, not the legacy binary
> name. **Decision (Auro): no `bb` alias/shim — ship `atl-bb` directly as a
> clean break.** Decision IDs continue the B1.5 sequence (D6+).

## 1. Binary name — clean break to `atl-bb`

- Canonical and **only** name: **`atl-bb`** (matches `atl-jira`, `atl-conf`).
- **No `bb` compatibility shim, alias, symlink, or deprecation window.** The
  legacy `bb` binary is simply replaced; users invoke `atl-bb`.
- Legacy `bb` continues to exist only as the **source/behavior oracle** for
  the rewrite (B3), not as a shipped binary.

> **D6 — resolved (no deprecation window).** Per the clean-break decision,
> there is no dual-availability period and no `bb` shim to retire.

## 2. Config path and automatic migration

Legacy: `$BB_CONFIG_DIR/config.json` else `os.UserConfigDir()/bb/config.json`,
host-keyed, **plaintext token**. Target: `XDG_CONFIG_HOME/atlassian-cli/config.json`,
site-keyed, `token_ref` indirection + `internal/secrets`.

A clean break on the **binary name** does not mean forcing a re-login: a
one-time credential **import** spares existing users from re-authenticating.
It runs once, automatically, on first `atl-bb` invocation when an
`atlassian-cli` config does not yet exist and a legacy `bb` config does:

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
> **D9 — legacy env vars.** At **runtime** `atl-bb` honors only the `atl-*`
> conventions (`XDG_CONFIG_HOME`, the `atl-*` test-base override) — no
> deprecated `BB_*` runtime aliases (consistent with the clean break). The
> one-time **importer** may still read `BB_CONFIG_DIR` purely to *locate* the
> legacy file to migrate. Flagged (whether the importer should consult
> `BB_CONFIG_DIR` at all, or only the default `bb/` path).

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
3. **Credential flag.** `--site` selects the credential. There is no `--host`
   alias (clean break; ties to D2).

> **D10 — error-output compatibility.** Recommend shipping the structured
> error model as an intentional improvement (no `bb`-prose compatibility
> mode). Flagged in case any known consumer parses `bb` stderr prose.

## 4. Repo-local `bb-cli` skill

Legacy skill installs via
`npx skills add https://github.com/aurokin/bitbucket_cli --skill bb-cli`.

- Publish a new **`atl-bb` skill** in the monorepo describing the `atl-bb`
  surface and the deterministic agent flags (`--repo`, `--json`, `--jq`,
  `--no-prompt`).
- Retire the `bb-cli` skill (clean break): leave a short note in the legacy
  `bb` repo pointing to the `atl-bb` skill, but do not maintain `bb-cli`.

> **D11 — resolved (clean break).** New `atl-bb` skill; `bb-cli` is retired
> with a pointer note rather than maintained as an alias.

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

- [ ] `atl-bb auth login/status/logout` works with migrated config (no legacy
      `bb` shim — clean break).
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

| ID | Decision | Status / Recommendation |
|---|---|---|
| D6 | `bb` deprecation window | **Resolved — none.** Clean break: ship `atl-bb`, no `bb` shim/alias/window. |
| D7 | scrub legacy plaintext token after migration | Flagged — yes, scrub/rename the old file (security win). |
| D8 | default site name for `bitbucket.org` | Flagged — `bitbucket` (alt: literal `bitbucket.org`). |
| D9 | importer reads `BB_CONFIG_DIR` to locate legacy file | Flagged — runtime honors only `atl-*`; importer may read `BB_CONFIG_DIR` only to find the file to migrate. |
| D10 | ship structured errors with no `bb`-prose compat mode | Flagged — yes (improvement). |
| D11 | skill name/home | **Resolved — clean break.** New `atl-bb` skill; `bb-cli` retired with a pointer note. |
| D12 | legacy `bb` repo docs disposition | Flagged — freeze-with-pointer if public, else delete. |

(D1–D5 are in [bb-rewrite-plan.md](bb-rewrite-plan.md).)

## Next

This completes the **planning** arc of the Bitbucket migration (B0→B1→B1.5→B2).
**Phase B3** (extract shared libraries behind stable APIs, then port the
typed client and command tree in vertical slices) is the first
implementation phase and should not begin until the flagged decisions
(D1–D12) are confirmed, since several (repo shape, config migration, error
model) shape the very first PRs.
