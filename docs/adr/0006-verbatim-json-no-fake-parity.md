# 0006 — Verbatim upstream JSON; no fake parity

**Status:** Accepted

## Context

A CLI wrapping a REST API has a recurring temptation: "improve" the API by
renaming fields, reshaping responses into a tidy unified envelope, inventing
cross-product abstractions, and adding flags that pretend a capability exists
when the underlying API has no path for it. That feels friendlier but it (a)
makes the tool a lossy, lagging proxy for the real API, (b) breaks every time
upstream adds a field, and (c) lies to the user about what the platform can do.

This project's whole premise — inherited from `bb`, the original Bitbucket CLI —
is "true to the official API."

## Decision

Two linked commitments:

1. **Verbatim JSON.** Under `--json`/`--jq`, commands emit the **raw upstream
   API response body, unmodified.** The typed clients return `json.RawMessage`
   and the render layer never renames, reshapes, or invents fields. Human output
   is a separate, compact per-type summary — explicitly *not* a stable contract.
   The structured projections (`--json`, `--jq`) are the contract, and that
   contract is Atlassian's JSON.

2. **No fake parity.** Where Atlassian exposes no real API path for something,
   the CLI does **not** fake it. A flag or subcommand that can't be backed by a
   genuine API call is not added; a no-op flag that misleads is removed (as the
   misleading Bitbucket `pr --draft` no-op was). Unsupported or ambiguous
   behavior is surfaced clearly rather than papered over. The raw `api` command
   stays a first-class escape hatch for anything a typed command doesn't yet
   cover.

## Consequences

- Scripts that consume `--json` are coupled to Atlassian's field names and
  shapes, so their stability tracks the upstream API's — stated for consumers in
  [consuming.md](../consuming.md#output-contract). New upstream fields appear
  automatically; we don't gate them behind a client release.
- Contributors don't get to "tidy up" responses. If a transformation is truly
  wanted, it belongs in `--jq` territory (the user's expression), not baked into
  the command.
- The tool can be trusted as a faithful view of the platform: if `atl-*` can't
  do something, it's because the API can't, and the tool says so rather than
  pretending.
- This is the source of the project's "no over-abstraction of product
  semantics" guardrail (see also [ADR 0002](0002-shared-foundation.md)).
