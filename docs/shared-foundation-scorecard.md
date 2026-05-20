# Shared-foundation scorecard

> Phase 9 review of code repeated across the Jira and Confluence
> implementations, scoring each candidate for extraction into a shared
> package. Grounded in the real tree after Phases 1–8, per the guardrail
> "extract only proven shared foundations; do not over-abstract before
> implementation teaches us the real seams."

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

## Out of scope (deferred to a later, dedicated phase)

The Bitbucket `atl-bb` migration — legacy `bb` inventory, the rewrite and
compatibility plans, and the import-vs-module-vs-separate decision — is its
own phase, to be taken up once Jira and Confluence are considered complete
and the legacy `bb` source is available as a behavior oracle. See
[bitbucket-migration-roadmap.md](bitbucket-migration-roadmap.md).
