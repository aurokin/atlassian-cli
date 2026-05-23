> **Historical / archived.** This document is a completed planning or design artifact, kept for project history. It is **not** maintained and may not reflect the current code. For the implemented behavior see [`docs/command-contract.md`](../command-contract.md) and [`docs/shared-architecture.md`](../shared-architecture.md).

# Phase 9 — Shared-foundation review (in-repo)

> Implementation plan for the in-repo half of the Phase 9 milestone in
> [implementation-plan.md](implementation-plan.md). Scope this phase:
> extract only the proven, low-risk duplicate helpers identified in
> [shared-foundation-scorecard.md](shared-foundation-scorecard.md). The
> Bitbucket `atl-bb` migration thread is **deferred to its own future
> phase** and is not touched here.

## Goal

Remove the byte-for-byte duplication between `internal/jira` and
`internal/conf` (and between `internal/jiracmd` and `internal/confcmd`) for
the helpers that are genuinely product-agnostic, without disturbing the
parts that legitimately differ per product. No behavior changes; this is a
pure refactor proven by the existing test suites.

## Tasks

### Task 1 — `internal/restutil`

Create a new leaf package `internal/restutil` with:

- `WithQuery(path string, q url.Values) string`
- `MaxFollowPages` (const, 100)
- `Decode[T any](raw json.RawMessage, product string) (T, error)`
- `DecodeError(product string, err error) error`

Rewire:

- `internal/jira`: add `const productName = "Jira"`; make `Decode` and
  `decodeError` thin wrappers delegating to `restutil`; replace
  `withQuery`/`maxFollowPages` references with `restutil.WithQuery`/
  `restutil.MaxFollowPages` and delete the local copies.
- `internal/conf`: the same with `const productName = "Confluence"`.

The error-message text ("could not decode the Jira/Confluence API
response: …") is preserved exactly. Add `internal/restutil` unit tests for
`WithQuery` (empty and non-empty query) and `Decode`/`DecodeError`
(success, malformed JSON, product label in the message). Commit:
`refactor: extract shared restutil client helpers`.

### Task 2 — `output.TabWriter`

Move the identical `tabWriter` into `internal/output` as exported
`TabWriter(w io.Writer) *tabwriter.Writer`. Rewire `jiracmd` and `confcmd`
to call `output.TabWriter`; delete both local copies. The existing
human-output tests already cover the column formatting. Commit:
`refactor: share TabWriter via internal/output`.

### Task 3 — docs, review, PR

Update `docs/shared-architecture.md` to note the `restutil` and
`output.TabWriter` shared helpers and the explicit non-extraction of the
pagination followers. Refresh `README.md` status, `docs/README.md` index
(scorecard + this plan), and `docs/continuation-handoff.md`. Run the
multi-agent review wave until clean. Commit: `docs: document Phase 9
shared-foundation extraction`. Open PR.

## Done definition

- The proven-identical helpers live in one place; `jira`/`conf` and
  `jiracmd`/`confcmd` no longer carry private copies.
- Per-product divergent code (pagination, `setLimit`, `get`/`send`,
  command wiring) is unchanged.
- `go test ./...`, `go vet ./...`, and `gofmt` are clean; no behavior
  change.
- The scorecard and this plan are indexed in `docs/README.md`.

## Out of scope

- The Bitbucket `atl-bb` migration (its own future phase).
- Any generic pagination/follow abstraction over the two protocols.
- Unifying the per-product `Client` constructors or command trees.
