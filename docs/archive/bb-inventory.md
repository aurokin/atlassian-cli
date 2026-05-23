> **Historical / archived.** This document is a completed planning or design artifact, kept for project history. It is **not** maintained and may not reflect the current code. For the implemented behavior see [`docs/command-contract.md`](../command-contract.md) and [`docs/shared-architecture.md`](../shared-architecture.md).

# Legacy `bb` Inventory (Phase B0)

> Output of **Phase B0** in [bitbucket-migration-roadmap.md](bitbucket-migration-roadmap.md):
> a focused inventory of the existing Bitbucket CLI so the `atl-bb` migration
> can treat it as a behavior oracle, not an architecture to copy. Source
> analyzed: `~/code/bitbucket_cli` (`github.com/aurokin/bitbucket_cli`), at
> commit `e1a0071`. This is a point-in-time snapshot for planning; it does not
> change any code in either repo.

## Snapshot summary

`bb` is a mature, broad Bitbucket **Cloud** CLI — far larger than `atl-jira`
and `atl-conf` combined. It already shares the same core stack and the same
agent-facing output contract as the Atlassian CLIs, which makes it a strong
fit for the shared foundation. The largest deltas the migration must resolve
are **secret storage** (plaintext token in `config.json` vs. keychain/`0600`
fallback), the **error model** (guided prose vs. structured `apperr` codes),
and the **config schema / path** (`bb/config.json` host map vs.
`atlassian-cli/config.json` site map).

| Dimension | Legacy `bb` | atl-jira / atl-conf foundation |
|---|---|---|
| Language | Go 1.25 | Go 1.26 |
| Command lib | `spf13/cobra` 1.10 + `pflag` | `spf13/cobra` |
| jq | `itchyny/gojq` 0.12.18 | `itchyny/gojq` |
| Terminal | `golang.org/x/term` | (n/a yet) |
| Secret storage | **plaintext token in `config.json`** | OS keychain via `zalando/go-keyring` + `0600` fallback; never in config |
| Error model | guided prose strings + `*APIError` | structured `apperr.Error{Code, Message, Next, …}` |
| Output contract | `--json` (`*`/field list) + `--jq` + table | identical (`cli.Render`, `output`, `restutil`) |
| API base | `https://api.bitbucket.org/2.0` | per-product (`/rest/api/3`, `/wiki/api/v2`, gateway) |
| Auth signing | Basic `username:token` | Basic (cloud), Bearer (DC PAT) |
| Commit attribution | `Codex by OpenAI` | `Claude` |

## 1. Command tree and generated-docs pipeline

### 1.1 Command surface

Root is `bb`; one persistent flag: `--no-prompt`. The tree (≈150 leaf
commands) registered in `internal/cmd/root.go`:

```
bb
├── alias            get|set|list|delete
├── api              <path-or-url> [-X method] [--input file|-] [--jq] [--host]
├── auth             login|status|logout
├── branch           list|view|create|delete
├── browse           [target] [--no-browser] [--branch|--commit] …
├── commit           view|diff|statuses|approve|unapprove
│   ├── comment      list|view
│   └── report       list|view
├── completion       bash|zsh|fish|powershell
├── config           get|set|unset|list|path
├── deployment       list|view
│   └── environment  list|view
│       └── variable create|edit|delete|list|view
├── extension        list|exec                     (bb-<name> external binaries)
├── issue            create|edit|view|list|close|reopen
│   ├── attachment   list|upload
│   ├── comment      create|edit|delete|list|view
│   ├── component    list|view
│   └── milestone    list|view
├── pipeline         list|view|run|stop|log|test-reports
│   ├── cache        list|delete|clear
│   ├── runner       list|view|delete
│   ├── schedule     create|edit?|delete|enable|disable|list|view
│   └── variable     create|edit|delete|list|view
├── pr               list|view|create|merge|close|checkout|diff|commits|activity|checks|status
│   ├── comment      view|edit|delete|resolve|reopen
│   ├── review       approve|unapprove|request-changes|clear-request-changes
│   └── task         create|edit|delete|list|view|resolve|reopen
├── project          create|edit|delete|view|list
│   ├── default-reviewer  list
│   └── permissions  group (list|view) | user (list|view)
├── repo             view|list|create|edit|delete|clone|fork
│   ├── deploy-key   create|delete|list|view
│   ├── hook         create|edit|delete|list|view
│   └── permissions  group (list|view) | user (list|view)
├── resolve          <bitbucket-url>
├── search           repos|prs|issues
├── status           [--json authored_prs,review_requested_prs,your_issues]
├── tag              create|delete|list|view
├── version
└── workspace        list|view
    ├── member       list|view
    ├── permission   list|view
    └── repo-permission list
```

Notable surface features beyond the Atlassian CLIs:
- **Aliases** (`bb alias set`) — user-defined command aliases expanded in
  `normalizeCLIArgsWithAliases` (recursion-bounded, depth 8).
- **Extensions** (`bb <name>` → `bb-<name>` binary on `PATH`) — gh-style
  external command dispatch (`runExtensionCommand`).
- **Local git integration** — `branch`, `pr checkout`, `repo clone/fork`
  shell out to `git` and infer the repo from remotes.
- **Shell completion** generation built in (`completion <shell>`).

### 1.2 Generated-docs pipeline

`go run ./cmd/gen-docs` regenerates, from the live Cobra tree + payload
structs + recovery catalog:

- `docs/cli-reference.md`, `docs/examples.md`, `docs/flag-matrix.md`
- `docs/command-metadata.json` (machine-readable command tree)
- `docs/json-fields.md`, `docs/json-shapes.md` (response/payload schemas)
- `docs/error-index.md`, `docs/recovery.md` (recovery catalog)
- `docs/completions/*` (shell completions), `docs/man/*` (man pages)

Hand-written docs: `README.md`, `docs/workflows.md` (human), `docs/automation.md`
(agent), `PRODUCT_PLAN.md`, `ROADMAP.md`, `AGENTS.md`/`CLAUDE.md`.

The generator lives entirely in `internal/cmd/*_doc.go` files
(`doc_reference.go`, `flag_matrix_doc.go`, `json_fields_doc.go`,
`json_shapes_doc.go`, `error_index_doc.go`, `recovery_doc.go`,
`generated_assets.go`). The Atlassian CLIs have **no** generated-docs
pipeline yet — this is net-new capability `bb` brings.

## 2. Config schema and migration constraints

`internal/config/config.go`. Path: `$BB_CONFIG_DIR/config.json`, else
`os.UserConfigDir()/bb/config.json` (macOS: `~/Library/Application Support/bb/`,
Linux: `~/.config/bb/`). Dir `0700`, file `0600`, atomic-ish (`MarshalIndent`
then `WriteFile`).

```jsonc
{
  "default_host": "bitbucket.org",
  "hosts": {
    "bitbucket.org": {
      "username": "you@example.com",
      "token": "<PLAINTEXT API TOKEN>",   // ← stored in the clear
      "auth_type": "api-token",
      "token_type": "",                     // legacy, normalized/cleared
      "updated_at": "2026-01-01T00:00:00Z"
    }
  },
  "settings": { "prompt": true, "browser": "", "editor": "", "pager": "", "output_format": "table|json" },
  "aliases": { "myprs": "pr list --repo me/app" }
}
```

Host model: keyed by **host** (`bitbucket.org`), with `default_host` and
`ResolveHost(--host)`. The Atlassian config is keyed by **named site profile**
with an indirect `token_ref` (`env:`/`keyring`/`file`) and never a raw token.

**Migration constraints (hard):**
1. **Plaintext token → secret store.** A migration must read the legacy
   `token` and re-home it into the keychain (or `0600` `credentials.json`),
   then rewrite the entry as a `token_ref`. This is the single biggest
   compatibility/safety task.
2. **Path + filename change** (`bb/config.json` → `atlassian-cli/config.json`,
   `BB_CONFIG_DIR` → `XDG_CONFIG_HOME`). Need either an auto-migration on
   first `atl-bb` run or an explicit `atl-bb auth login`.
3. **Host-keyed → site-keyed.** Map `hosts["bitbucket.org"]` to a site
   profile (e.g. `bitbucket`/default) with a Bitbucket product + Basic auth.
4. **Settings/aliases** have no Atlassian-side equivalent yet — decide whether
   to carry `aliases`, `prompt`, `browser`, `editor`, `pager`, `output_format`
   forward or drop them.

## 3. Auth assumptions and Bitbucket-only limits

`internal/bitbucket/client.go` + `internal/cmd/auth.go`.

- `bb auth login --host bitbucket.org --username <email> --token <t>
  | --with-token (stdin) --default` → stores a `HostConfig`.
- Single auth type: **`api-token`** → HTTP **Basic** (`username:token`) where
  username is the Atlassian account email and token is an Atlassian API token.
  Legacy `token_type` values (`app-password`, `basic`, `bearer`, `oauth`, …)
  are all normalized to `api-token`; `token_type` is cleared on save.
- `bb auth status [--check] [--host]` — `--check` validates against `GET /user`.
- `bb auth logout [--host]`.
- **Cloud only.** `resolveBaseURL` accepts only `""`/`bitbucket.org`/
  `api.bitbucket.org` → `https://api.bitbucket.org/2.0`; any other host errors
  with "only Bitbucket Cloud is implemented". `BB_API_BASE_URL` overrides the
  base (used by tests / future Server support).
- No OAuth, no browser login (deliberate, per `AGENTS.md`).

Maps cleanly onto the Atlassian `httpclient` Basic-auth path; the new
foundation would add a Bitbucket product/token-style and an
`api.bitbucket.org/2.0` API base. Bitbucket has **no scoped-token gateway**
analog (`api.atlassian.com/ex/...`) and **no Data Center variant** wired yet.

## 4. Output renderer and JSON field selector

`internal/output/render.go` + `table.go`. **The contract already matches the
Atlassian foundation:**

- `--json` accepts: empty (human table), `*` (all fields), or a comma-separated
  field list (projection). Root rewrites bare `--json` → `--json=*`.
- `--jq` (gojq) filters the JSON; **requires `--json`** (same rule as
  `atl-*`). `ParseFormatOptions` enforces this.
- `output.Render(w, opts, data, humanRenderer)`: human path when no structured
  output requested; else normalize → projectFields → ApplyJQ → WriteJSON.
- `WriteJSON`: 2-space indent, `SetEscapeHTML(false)`.
- Field projection works on objects and arrays-of-objects; selecting a field
  from a scalar errors.

This is effectively the same design as `internal/output` + `internal/cli`
`Render` in the Atlassian CLIs (Phase 9 already extracted `output.TabWriter`
and `restutil`). High-confidence "share now" candidate.

Per-resource JSON shapes are defined as typed structs in `internal/bitbucket/*`
and surfaced through `docs/json-shapes.md`/`json-fields.md`.

## 5. Raw `api` command behavior

`internal/cmd/api.go`. `bb api <path-or-url> [-X method] [--input file|-]
[--jq] [--host]`:

- Relative path is joined onto `…/2.0`; an absolute URL (`scheme://…`) passes
  through unchanged (`client.resolveURL`). **No same-origin guard** — unlike
  the Atlassian `httpclient.ResolveURL`, which rejects off-origin absolute
  URLs with `untrusted_url`. Migration should adopt the safer guard.
- Body via `--input <file>` or `--input -` (stdin); `Content-Type: application/json`
  is set only when a body is present; `Accept: application/json` always.
- Non-2xx → `*APIError` (status + parsed message). Success → raw body written
  verbatim (newline-terminated), or `--jq`-filtered when requested.
- `--jq` here re-parses the response JSON and reuses `output.ApplyJQ`.

## 6. URL resolver and browser URL builder

- **`bb resolve <url>`** (`internal/cmd/resolve.go`, `entity.go`): parses a
  Bitbucket web URL into a `resolvedEntity` JSON:
  `{host, workspace, repo, type, url, canonical_url, ref?, path?, line?, commit?, pr?, comment?, issue?}`.
  Entity types: `repository`, `pull-request`, `pull-request-comment`
  (`#comment-N`), `issue`, `commit`, `path` (`/src/<ref>/<path>` with
  `#lines-N`). Fuzz-tested (`FuzzParseBitbucketEntityURL`).
- **`bb browse [target]`** (`browse.go`, `browse_support.go`): builds canonical
  web URLs for repo/path/commit/PR/issue, with `--branch`/`--commit`/line
  support and repo-relative path resolution; `--no-browser` prints the URL
  (the agent-safe path) instead of opening it. `browseBaseURL`,
  `buildBrowsePathURL`, `escapePathSegments`, `openURLInBrowser`,
  `defaultBrowserCommand(goos,…)`.

Analogous to the Atlassian `resolve`/`browse`, but with Bitbucket-specific
URL grammar. The framework is shareable; the parser/builder are
product-specific.

## 7. Recovery catalog and error handling

`internal/cmd/errors.go` + `recovery_doc.go` + `internal/bitbucket/client.go`.

- `*bitbucket.APIError{StatusCode, Status, Message, Body}` with `Is`/`As`
  helpers; `parseAPIErrorMessage` digs `error.message`/`error.detail`/`error`
  out of Bitbucket's JSON error envelope.
- `userFacingError` turns raw errors into **guided prose** with a trailing
  "Bitbucket said: <detail>" and a concrete next step:
  - `401` → "token may be invalid/expired… run `bb auth login`… rotate at <url>"
  - `403` → "missing Bitbucket scopes or no access… create a token with scopes…"
  - `404` → "not found or token can't see it… check target/workspace/PR ID"
  - issue-tracker-disabled, alias errors, repo-target ambiguity, missing
    `--workspace`/`--repo`, repo-inference failure — each gets specific text.
- A recovery catalog drives `docs/recovery.md` + `docs/error-index.md`.

**Key delta:** `bb` recovery is human-prose-first; the Atlassian model is a
**structured** `apperr.Error{Code, Message, Next, status, product, site, …}`
that serializes machine-readable codes (`unauthorized`, `forbidden`,
`not_found_or_not_visible`, `rate_limited`, …) and is the agent contract.
Migration should map `bb`'s guidance text onto `apperr` codes + `Next`,
preserving the helpful prose as the `Message`/`Next` while gaining the code.

## 8. Test helpers and live-smoke conventions

- ~52 `_test.go` in `internal/cmd`; tests use an in-process HTTP test server
  and golden output, asserting the stdout/stderr split, exit codes, JSON
  field sets, and human field order / `Next:` guidance.
- **Fuzz** targets: `FuzzParseBitbucketEntityURL`, `FuzzParseRemoteURL`
  (run via `make fuzz-short`).
- **Integration** (`integration/`): live Bitbucket Cloud tests, **manual-only**
  — never in `go test ./...` or CI (gated by env / build conventions), reusing
  a fixture workspace/project and sacrificial repos for destructive flows.
- `Makefile`: `make check` (= `test lint complexity`), `make race`,
  `make coverage`, `make fuzz-short`, `make stability`
  (= `test-shuffle test-repeat`), `make tools`. `.golangci.yml` committed.

The Atlassian CLIs use the same `httptest.Server` + golden approach and the
same "no live calls in default tests" rule, so the harness is largely
compatible. Net-new from `bb`: fuzz targets, the stability/shuffle gate, and
golangci-lint config.

## 9. Migration-relevant findings (feeds Phase B1 scorecard)

**Share now (high confidence):** output renderer + `--json`/`--jq` contract;
`api` command scaffolding (after adding the same-origin guard); test harness
shape; docs-generation pipeline (net-new, adopt wholesale into the monorepo).

**Adapt then share:** config mechanics (need site model + secret-ref
indirection); auth framework (add Bitbucket product + Basic path; reuse
keychain store); resolve/browse framework (Bitbucket parser/builder stay
product-specific); recovery catalog (re-express as `apperr` codes).

**Keep product-specific:** the Bitbucket resource models and the entire
command vocabulary (workspace/project/repo/pr/pipeline/issue/deployment/
commit/tag/branch semantics), Bitbucket permission/scope maps, git
shell-out integration, alias + extension dispatch.

**Hard compatibility tasks (must not regress users):**
1. Plaintext token → keychain/`0600` migration with a tested fallback.
2. Config path/filename + host→site reshaping with auto-migration or a clean
   re-login path.
3. Binary rename `bb` → `atl-bb` with an alias/wrapper decision (Phase B2).
4. Preserve `resolve` JSON, `browse --no-browser` URLs, and `--json/--jq/
   --no-prompt` behavior under golden tests before any change.
5. Decide the fate of `aliases`, `extension`, settings, and the generated
   `bb-cli` skill.

## 10. Open questions for B1/B2

- Adopt `bb`'s generated-docs pipeline into the monorepo as the standard for
  all three CLIs, or keep it Bitbucket-only at first?
- Carry `aliases` and the `bb-<name>` extension mechanism into `atl-bb`
  (and possibly all `atl-*`), or drop them as out-of-scope?
- Is the `bb` config auto-migrated on first `atl-bb` run, or is re-login
  required (simpler, but a worse upgrade experience)?
- Should the same-origin URL guard from the Atlassian `httpclient` be applied
  to `atl-bb api`, given some users may rely on `bb api <absolute-url>`?
- Bitbucket Data Center: design the product/token-style seam now (cheap) or
  defer entirely (Cloud-only, as today)?

## Next phase

**Phase B1 — shared-foundation comparison** → `docs/shared-foundation-scorecard.md`
already exists from Phase 9 (the Jira/Confluence extraction). Extend or
cross-reference it with the per-package decisions above before B1.5
(`docs/bb-rewrite-plan.md`, currently a placeholder).
