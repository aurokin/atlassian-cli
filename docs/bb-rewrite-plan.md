# Bitbucket Rewrite Plan (Phase B1.5)

> The new-standards modernization plan for `atl-bb`, written after Phase B0
> ([bb-inventory.md](bb-inventory.md)) and Phase B1 (the Bitbucket section of
> [shared-foundation-scorecard.md](shared-foundation-scorecard.md)). It says
> what to preserve, rewrite, or intentionally change when bringing legacy `bb`
> onto the `atl-*` foundation. It is a plan, not an import: no Bitbucket source
> moves until Phase B2 ([bb-compatibility-plan.md](bb-compatibility-plan.md))
> and B3 are agreed.

## Intent

Use legacy `bb` as the behavior oracle and working baseline, then bring
Bitbucket up to the standards `atl-jira` and `atl-conf` set: shared
foundation, structured errors, secret-store auth, and golden-tested output
contracts. Compatibility means preserving **user value and stable machine
contracts**, not freezing internal design.

## Repository shape (recommendation)

Adopt roadmap **option 1 — monorepo with rewrite**: import legacy `bb` into
`atlassian-cli` as the rewrite baseline for `atl-bb`, so all three binaries
build from one foundation. Rationale: B1 showed `atl-bb` reuses
`httpclient`, `output`/`cli.Render`, `restutil`, `apperr`, `secrets`, config
mechanics, and the resolve/browse frameworks — that reuse is only free inside
one module. The legacy `bb` repo stays the public home until the
rename/release transition (B2/B5).

> **D1 — resolved (Auro, 2026-05-20): monorepo-with-rewrite.** `atl-bb` is
> built inside `atlassian-cli` as the rewrite baseline; all three binaries
> share one module/foundation. The legacy `bb` repo stays the public home
> only until the release transition.

## Target package layout

Mirror the existing per-product split (`jira`+`jiracmd`+`atljiracmd`,
`conf`+`confcmd`+`atlconfcmd`):

```
cmd/atl-bb/main.go                      # entrypoint, like cmd/atl-jira
internal/bitbucket/                     # typed Bitbucket client over internal/httpclient
  client.go  (thin: wraps httpclient.Client; get/send/Decode helpers)
  workspaces.go projects.go repositories.go pullrequests.go pipelines.go
  issues.go commits.go refs.go deployments.go … (ported, reshaped to models)
internal/bbcmd/                         # the Bitbucket command tree (cobra)
  repo.go pr.go pipeline.go issue.go workspace.go project.go commit.go
  branch.go tag.go deployment.go search.go status.go bbcmd.go …
internal/atlbbcmd/root.go               # layers bbcmd onto the shared cli root
internal/bbresolve/ or internal/resolve/bitbucket.go  # Bitbucket URL parser/builder
internal/git/                           # Bitbucket-owned git integration (ported as-is)
```

Reused unchanged from the foundation: `internal/httpclient`, `internal/cli`
(root, `api`, `auth`, `browse`, `resolve` scaffolding, `GlobalFlags`,
`SiteClient`, `Render`), `internal/output`, `internal/restutil`,
`internal/apperr`, `internal/secrets`, `internal/config`, `internal/appinfo`,
`internal/browser`, `internal/auth`.

## HTTP, product, and auth model

- Add `ProductBitbucket` to `httpclient`/`appinfo`; API base
  `https://api.bitbucket.org/2.0`, overridable by env for tests (the existing
  `BB_API_BASE_URL` behavior, renamed to the `atl-*` convention).
- Token style: Bitbucket Cloud **Basic** (`username:apiToken`), reusing the
  existing cloud-classic Basic signing path. No scoped-token gateway, no Data
  Center variant initially (design the seam, don't build it).
- Adopt the foundation's `httpclient.ResolveURL` same-origin guard for
  `atl-bb api` — an **intentional** behavior change vs. `bb api`, which lets an
  absolute URL target any host. Documented in the compatibility plan.
- `internal/bitbucket.Client` becomes a thin typed wrapper over
  `httpclient.Client` (like `jira`/`conf`), replacing `bb`'s bespoke transport,
  `applyAuthorization`, and `*APIError`.

> **Open decision (D2):** credential selection flag. The foundation uses
> `--site`; `bb` uses `--host`/`--repo`/`--workspace`. Recommending `atl-bb`
> keep `--repo <workspace>/<repo>` and `--workspace` for the **resource** and
> add `--site` for the **credential**. No `--host` alias (clean break — the
> legacy flag is not carried over). Flagged for confirmation.

## Output / JSON compatibility guarantees

The contract already matches (B1): `--json` (empty / `*` / comma field list),
`--jq` (requires `--json`), table default, bare-`--json`→`--json=*`
normalization, 2-space indent, no HTML escaping. `atl-bb` uses
`cli.Render`/`output` directly.

Guarantee: **JSON field names stay stable** for the commands agents use
(`repo view`, `pr list/view`, `pr create`, `status`, `resolve`,
`browse --no-browser`). Golden tests pin these before any internal change
(B2). Field *additions* are allowed; renames/removals are versioned and
documented.

## Recovery / error-model upgrade

Replace `bb`'s guided-prose `userFacingError` with the structured
`apperr.Error{Code, Message, Next, …}` model, preserving the helpful prose as
`Message`/`Next`. Mapping:

| `bb` case | `apperr` code | `Next` (preserve `bb`'s prose) |
|---|---|---|
| HTTP 401 | `unauthorized` | "token may be invalid/expired; run `atl-bb auth login`; rotate at <url>" |
| HTTP 403 | `forbidden` | "token may lack Bitbucket scopes or workspace/repo access; create a scoped token" |
| HTTP 404 | `not_found_or_not_visible` | "not found or token can't see it; check repo target / workspace / PR ID" |
| HTTP 429 | `rate_limited` | "retry after N seconds" (from `Retry-After`) |
| issue tracker disabled | `invalid_input` (or a new `feature_disabled`) | "enable the issue tracker in repo settings" |
| repo target / `--workspace` ambiguity, alias errors, repo-inference failure | `invalid_input` | preserve `bb`'s specific guidance text |

`httpclient.classify` already produces the 401/403/404/429 codes, so most of
this is wiring + Bitbucket-specific detail extraction (`error.message`/
`error.detail`).

> **D3 — resolved (Auro, 2026-05-20): add `feature_disabled`.** Introduce a
> dedicated `apperr` code for the issue-tracker-disabled (and similar
> repo-capability-off) case, for a clearer agent contract than overloading
> `invalid_input`/`forbidden`.

## Generated docs / man / completion strategy

`bb`'s `gen-docs` pipeline (CLI reference, examples, flag matrix,
command-metadata.json, json-fields/shapes, error-index/recovery, man pages,
shell completions) is **net-new** capability the Atlassian CLIs lack. Plan:
port it as `cmd/gen-docs` in the monorepo, generalized to take a root command
+ product name so it serves `atl-bb`, `atl-jira`, and `atl-conf`.

> **D4 — resolved (Auro, 2026-05-20): generalize now.** Port `gen-docs` as a
> monorepo-wide `cmd/gen-docs` that takes a root command + product name, so it
> serves `atl-bb`, `atl-jira`, and `atl-conf`. (Wiring Jira/Confluence into it
> can land in a follow-up slice; the generator is built product-agnostic from
> the start.)

## Aliases, extensions, git integration

- **Git integration** (`internal/git`: remote parse, repo inference, clone,
  checkout): port as Bitbucket-owned. `ParseRemoteURL` is generic enough to
  promote later if a second product ever needs git, but not now.
- **Aliases** (`bb alias`, recursive arg expansion) and **extensions**
  (`bb-<name>` dispatch): preserve in `atl-bb` to keep user value.

> **D5 — resolved (Auro, 2026-05-20): Bitbucket-only initially.** Keep aliases
> + extensions in `atl-bb` (smaller blast radius); revisit promoting them to
> the shared `cli` root if Jira/Confluence users ask.

## Performance opportunities and non-goals

Evaluate, with measurement, against representative read flows: startup time
for `version`/`help`/config-only commands; API-call count for `repo view`,
`pr view`, `pipeline status`, `issue view`; pagination for repo/PR/pipeline
lists (reuse the `--all` cap); memory for large lists; avoiding redundant
config reads and subprocess spawns. **Non-goal:** speculative perf work
without a measured baseline.

## Test coverage required before replacing legacy internals

1. **Golden output/JSON fixtures** for the agent-facing commands above
   (human and `--json`/`--jq`), captured from current `bb` and committed
   before the rewrite touches those paths.
2. **Resolve/browse fixtures** for the messy Bitbucket URL set `bb` already
   covers (repo/PR/comment/issue/commit/path, `#comment-N`, `#lines-N`),
   including the existing fuzz corpus.
3. **Config-migration tests** for `bb/config.json` (plaintext token, host map)
   → site-keyed `atlassian-cli/config.json` + `secrets`.
4. **Error-mapping tests** asserting the `apperr` code for each 401/403/404/429
   and Bitbucket-specific case.
5. Port `bb`'s `httptest`-based command tests; keep live integration tests
   manual-only (no live calls in `go test ./...`).

## Rewrite guardrails

- Use legacy `bb` only as the **source/behavior oracle** for the rewrite; it
  is not shipped alongside `atl-bb` (clean break — no dual-availability
  window). `atl-bb` replaces it on release.
- Never combine a mechanical source import with broad behavior changes in one
  PR.
- Prefer small rewrite PRs, each stating: compatible / intentionally breaking /
  internal-only.
- Golden tests must prove output compatibility before internal changes land.

## Sequencing into B3+

1. **B2** — compatibility plan (binary name, config migration, field
   guarantees, skill, live-test boundary). *(Next.)*
2. **B3a** — add `ProductBitbucket` + Basic path to the foundation; port the
   typed client over `httpclient`; golden + error-mapping tests. No commands.
3. **B3b** — port the command tree into `internal/bbcmd` + `internal/atlbbcmd`
   in vertical slices (repo → pr → pipeline → issue → workspace/project →
   commit/branch/tag/deployment → search/status → resolve/browse → api/auth),
   each slice green and golden-tested.
4. **B3c** — port git integration, aliases, extensions; generalize `gen-docs`.
5. **B4** — finalize the `bb`→`atl-bb` integration shape and config migration.
6. **B5** — release/docs/skill transition.

## Non-goals

- Do not fake Jira/Confluence parity in Bitbucket commands.
- Do not break stable JSON fields to match a new internal model.
- Do not make live Bitbucket tests mandatory in normal CI.
- Do not ship a `bb` shim/alias or a dual-availability window: `atl-bb` is a
  clean break that replaces `bb` on release (decision confirmed in B2). The
  one-time config/credential **import** is the only legacy concession.
