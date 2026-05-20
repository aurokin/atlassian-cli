# Shared-foundation scorecard

> Phase 9 review of code repeated across the Jira and Confluence
> implementations, scoring each candidate for extraction into a shared
> package. Grounded in the real tree after Phases 1–8, per the guardrail
> "extract only proven shared foundations; do not over-abstract before
> implementation teaches us the real seams."
>
> **Extended in Phase B1** (Bitbucket migration) with a third data point —
> the legacy `bb` CLI — to re-test the Phase 9 seams against a real third
> product. See [bb-inventory.md](bb-inventory.md) for the source analysis
> and [bitbucket-migration-roadmap.md](bitbucket-migration-roadmap.md) for
> the phase sequence. The Phase 9 sections below are unchanged; the
> Bitbucket comparison is added at the end.

## Method

Each candidate is scored on three axes:

- **Identical?** — is the code byte-for-byte the same across products, or
  does it diverge?
- **Coupling** — does it depend on product-specific types, API shapes, or
  configuration?
- **Churn** — how many call sites change if it is extracted?

The verdict is **extract now**, **keep thin per-product wrapper**, or
**leave per-product**.

## Candidates

| Candidate | Where | Identical? | Coupling | Verdict |
|---|---|---|---|---|
| `withQuery(path, q)` | `jira/client.go`, `conf/client.go` | Yes | None (pure) | **Extract now** |
| `maxFollowPages = 100` | `jira/client.go`, `conf/client.go` | Yes | None | **Extract now** |
| `tabWriter(w)` | `jiracmd/jiracmd.go`, `confcmd/confcmd.go` | Yes | None (presentation) | **Extract now** |
| `Decode[T](raw)` | `jira/models.go`, `conf/models.go` | Logic identical; message names the product | Generic over model type; product label only | **Share core, keep thin wrapper** |
| `decodeError(err)` | `jira/client.go`, `conf/client.go` | Logic identical; message names the product | Product label only | **Share core, keep thin wrapper** |
| `get` / `send` request helpers | both clients | Near-identical | Bound to each client's `http` field; `conf.get` also accepts absolute URLs | **Leave per-product** |
| `setLimit(q, limit)` | both clients | No — Jira sets `maxResults`, Confluence sets `limit` | API-specific param name | **Leave per-product** |
| `followAll`/`synthesize` vs `followList`/`nextPageURL` | `jira` (token + offset) vs `conf` (`_links.next` cursor) | No — different pagination protocols | Deep API-shape coupling | **Leave per-product** |
| `New(c *httpclient.Client) *Client` | both clients | Same shape | Returns the product's own `*Client` | **Leave per-product** (generics would obscure, not clarify) |
| `AddCommands` / `<product>Client` / test `exec*`/`login*Site` | `jiracmd`, `confcmd` | Parallel shape | Bound to each product's command tree | **Leave per-product** |

## Decisions

### Extract now → `internal/restutil`

A new leaf package `internal/restutil` holds the product-agnostic REST
client helpers:

- `WithQuery(path string, q url.Values) string` — identical in both
  clients; pure.
- `MaxFollowPages` (= 100) — the shared `--all` page cap.
- `Decode[T any](raw json.RawMessage, product string) (T, error)` — the
  generic decode plus structured-error wrap; `product` supplies the word in
  the error message so the per-product text is preserved.
- `DecodeError(product string, err error) error` — the structured
  decode/pagination failure error.

Each product package keeps **thin, unexported wrappers** so existing call
sites are untouched and the product label stays internal:

```go
// internal/jira
const productName = "Jira"
func Decode[T any](raw json.RawMessage) (T, error) { return restutil.Decode[T](raw, productName) }
func decodeError(err error) error                  { return restutil.DecodeError(productName, err) }
```

`withQuery` and `maxFollowPages` have only in-package call sites, so those
references are switched to `restutil.WithQuery` / `restutil.MaxFollowPages`
directly and the local copies are deleted.

### Extract now → `internal/output`

`tabWriter` is a pure presentation helper, identical in both command
packages. It moves to `internal/output` (the output-formatting package) as
`output.TabWriter(w io.Writer) *tabwriter.Writer`; `jiracmd` and `confcmd`
call that.

### Explicitly left per-product

The pagination followers, `setLimit`, `get`/`send`, `New`, and the command
wiring are **not** extracted. They look superficially similar but are
coupled to product-specific API shapes (pagination protocol, param names,
absolute-URL handling) or to each product's own types. Unifying them would
require a generic abstraction that hides real differences — exactly the
over-abstraction the guardrails warn against. They are revisited only if a
third product (Bitbucket `atl-bb`) makes the shared shape concrete.

---

# Phase B1 — Bitbucket (`atl-bb`) comparison

> Scores each legacy `bb` internal package/concern against the Atlassian
> foundation that exists today (after Phase 9), deciding what `atl-bb` should
> reuse, adapt, keep product-specific, or treat as net-new. This is the
> Phase B1 output called for by
> [bitbucket-migration-roadmap.md](bitbucket-migration-roadmap.md). It is a
> planning decision record, not an extraction PR — no code moves here.

## Method

Each `bb` concern gets one of four verdicts:

- **Reuse** — adopt the existing Atlassian foundation package as-is; `atl-bb`
  is a new caller. (The shape already matches.)
- **Adapt** — the foundation covers the concern but needs a Bitbucket-shaped
  addition (a product enum, an API base, a parser) before reuse.
- **Keep separate** — genuinely product-specific; do not try to share.
- **Net-new** — `bb` has a capability the foundation lacks; the foundation
  should adopt it (monorepo-wide) or consciously scope it out.

## Package-by-package

| `bb` concern | Source | Atlassian counterpart | Verdict |
|---|---|---|---|
| HTTP transport (`Do`/`NewRequest`/`resolveURL`, Basic auth) | `internal/bitbucket/client.go` | `internal/httpclient` (`Client.Do`, `ResolveURL`, `classify`) | **Adapt** — add a Bitbucket product + `api.bitbucket.org/2.0` base; reuse the same-origin guard and Basic signing |
| Output render + `--json`/`--jq`/table | `internal/output`, root `--json` normalize | `internal/output` + `internal/cli` `Render`, `GlobalFlags` | **Reuse** — contract is already identical |
| Field projection / `ApplyJQ` / `WriteJSON` | `internal/output/render.go` | `internal/output` + `gojq` wiring | **Reuse** |
| Structured decode | inline `json.NewDecoder` per service | `internal/restutil.Decode`/`DecodeError` | **Adapt** — route Bitbucket decodes through `restutil.Decode[T](raw, "Bitbucket")` |
| `WithQuery` / `MaxFollowPages` | (built ad hoc) | `internal/restutil` | **Reuse** for query building and the `--all` cap |
| Raw `api` command | `internal/cmd/api.go` | `internal/cli/api.go` | **Adapt** — reuse scaffolding; add the same-origin guard (intentional behavior change vs `bb`, flag in compat plan) |
| Error/recovery model | `errors.go` (guided prose) + `*APIError` | `internal/apperr` (`Code`/`Message`/`Next`) + `httpclient.classify` | **Adapt** — map `bb`'s 401/403/404/issue-tracker prose onto `apperr` codes + `Next`, preserving the helpful text |
| URL `resolve` framework | `internal/cmd/resolve.go` | `internal/resolve` + `internal/cli/resolve.go` | **Adapt** framework; the Bitbucket URL grammar (`entity.go`) is **keep separate** |
| `browse` framework | `internal/cmd/browse*.go` | `internal/cli/browse.go` + `internal/browser` | **Adapt** framework; Bitbucket web-URL builder is **keep separate** |
| Config mechanics (path, perms, load/save) | `internal/config/config.go` | `internal/config` (site model) | **Adapt** — reshape host-keyed → site-keyed; reuse the `0700`/`0600` discipline |
| Token storage | plaintext in `config.json` | `internal/secrets` (keychain + `0600`) | **Adapt** — `atl-bb` stores via `secrets`; add a one-time `bb`→`atl-bb` token migration |
| Auth login/status/logout | `internal/cmd/auth.go` | `internal/cli/auth.go` | **Adapt** — same command shape; Bitbucket Basic credential + `auth status --check` |
| Typed resource models + service methods | `internal/bitbucket/*.go` | `internal/jira`, `internal/conf` (analogous, not shared) | **Keep separate** — Bitbucket workspace/repo/PR/pipeline/issue semantics |
| Command vocabulary (~150 cmds) | `internal/cmd/*` | `internal/jiracmd`, `internal/confcmd` | **Keep separate** |
| Pagination follower | Bitbucket `{values, next}` top-level URL cursor | `conf` `_links.next`, `jira` token/offset | **Keep separate** (see re-evaluation below) |
| Limit param | Bitbucket `pagelen` | `jira` `maxResults`, `conf` `limit` | **Keep separate** — third distinct param name confirms the Phase 9 call |
| Git integration (remote parse, infer, clone, checkout) | `internal/git` | none | **Net-new, keep Bitbucket-owned** (`ParseRemoteURL` is reusable if a second product ever needs git) |
| Aliases (`bb alias`, arg expansion) | `internal/cmd/root.go`, `alias` | none | **Net-new** — decide in B2 whether to promote to all `atl-*` or keep Bitbucket-only |
| Extensions (`bb-<name>` dispatch) | `internal/cmd/root.go` | none | **Net-new** — same B2 decision |
| Generated-docs pipeline (`gen-docs`, `*_doc.go`, metadata, man, completions, recovery/error index) | `cmd/gen-docs`, `internal/cmd/*_doc.go` | none | **Net-new** — strong candidate to adopt monorepo-wide for all three CLIs |
| Test harness (`httptest`, golden, JSON-field asserts) | `internal/cmd/*_test.go` | `internal/*` tests | **Reuse conventions** — already aligned |
| Fuzz targets + stability gate | `FuzzParse*`, `make stability` | none | **Net-new** — adopt for parser-heavy code (URL/remote/selector) |
| Version | `internal/version` | `internal/appinfo` + `version` cmd | **Adapt** — fold into `appinfo` |

## Re-evaluation of the Phase 9 "leave per-product" seams

The Phase 9 scorecard left the pagination followers, `setLimit`, `get`/`send`,
and `New` per-product, noting they would be "revisited only if a third product
makes the shared shape concrete." Bitbucket is that third product, and the
verdicts hold:

- **`get`/`send` → confirmed shareable, but at the `httpclient` layer, not the
  typed-client layer.** The real shared HTTP surface is already
  `internal/httpclient.Client` (product-agnostic: it takes a `Target` +
  `Credential`). `atl-bb` should wrap that same client exactly as `jira`/`conf`
  do, replacing `internal/bitbucket/client.go`'s bespoke transport. The
  per-product `get`/`send` stay trivially thin wrappers — no change to the
  Phase 9 call.
- **`setLimit` → confirmed keep-separate.** Bitbucket adds a *third* limit
  param name (`pagelen`), so a shared setter would just be a switch on product.
- **Pagination follower → confirmed keep-separate, with one shared core
  candidate.** Bitbucket's `{values, next}` top-level full-URL cursor is closest
  to Confluence's `_links.next`. A future "follow `next` URL until empty"
  helper in `restutil` *could* serve both, but the response keys differ
  (`next` vs `_links.next`, `values` vs `results`) and Jira's token/offset
  scheme would not fit. Worth a small spike during B3, not a blocker.
- **`New(c)` → confirmed keep-separate** — each product returns its own client
  type.

**Conclusion:** the third product validates the Phase 9 foundation rather than
overturning it. `atl-bb` reuses `httpclient`, `output`/`cli.Render`,
`restutil`, `apperr`, `secrets`, the config mechanics, and the resolve/browse
*frameworks*; it keeps Bitbucket models, command vocabulary, pagination, and
git integration product-specific; and it brings net-new capabilities
(generated docs, fuzz/stability, aliases, extensions) the monorepo should
decide on deliberately.

## What B1 hands to the next phases

- **B1.5 → `bb-rewrite-plan.md`:** target package layout for `atl-bb` over the
  shared foundation, the `apperr` recovery mapping, and whether to adopt the
  generated-docs pipeline monorepo-wide.
- **B2 → `bb-compatibility-plan.md`:** the plaintext-token→secrets migration,
  the `bb/config.json`→site-keyed reshaping, the `bb`→`atl-bb` rename/alias
  decision, the fate of `aliases`/`extensions`, and golden tests pinning
  `resolve` JSON / `browse` URLs / `--json`/`--jq`/`--no-prompt` before any
  change.
