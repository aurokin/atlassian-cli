# 0002 — Shared foundation, and what is deliberately not shared

**Status:** Accepted (referred to during the build as "D2")

## Context

Three CLIs (`atl-jira`, `atl-conf`, `atl-bb`) live in one module. They share
obvious infrastructure (HTTP, auth, config, output rendering, the root command
tree) but each wraps a different Atlassian API with its own pagination model,
field names, and resource semantics. The standing risk in a multi-product
monorepo is **over-abstraction**: hoisting code into a shared package because it
*looks* similar, then contorting both callers to fit a unified shape that hides
real differences.

The build deliberately developed Jira and Confluence as separate CLIs first and
only extracted shared code once duplication was *proven* (the Phase 9
shared-foundation scorecard, now archived). Bitbucket later joined on the same
foundation.

## Decision

Extract into shared packages only the seams that are genuinely duplicated, and
**explicitly keep product-coupled code separate** even when it rhymes.

Shared (in `internal/`): `cli` (root, global flags, render dispatch, the shared
subcommands, the `SiteClient`/`Render` seam), `restutil` (the byte-for-byte
identical HTTP helpers — `Base`, `WithQuery`, `Decode`/`DecodeError` taking a
product label, `MultipartFile`, the pagination scaffolding), `output`,
`httpclient`, `apperr`, `config`, `secrets`, `auth`, `resolve`, `browse`. The
`alias` and `extension` commands were promoted to the shared root once all three
binaries wanted them.

**Deliberately not shared:** the pagination *followers* (Jira's token+offset
`followAll`/`synthesize` vs Confluence's `_links.next` cursor
`followList`/`nextPageURL`), the `setLimit` parameter name (`maxResults` vs
`limit`), the per-product `get`/`send` request helpers, the typed `Client`
constructors, and the command-tree wiring. These look alike but are coupled to
product-specific API shapes; unifying them would hide the differences that
matter.

The rule: **promote a shared shape only after implementation has proven the
seam**, not in anticipation of one.

## Consequences

- New product code goes through `internal/cli` and reuses the shared helpers
  (see the inventory in [engineering-notes.md](../engineering-notes.md#reach-for-the-shared-helpers)).
- When two product clients have similar-looking code, the default is to **leave
  it duplicated** unless it is identical and product-agnostic. A third concrete
  caller is the trigger to reconsider, not visual similarity.
- The shared layer stays thin and the product clients stay readable against
  their own API docs, at the cost of some intentional, eyes-open duplication.
- Rationale and the current package map are summarized in
  [shared-architecture.md](../shared-architecture.md); the original evidence is
  the archived Phase 9 scorecard.
