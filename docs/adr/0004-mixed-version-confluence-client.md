# 0004 — Confluence is a mixed-version (v2 + v1) client

**Status:** Accepted

## Context

Atlassian's Confluence Cloud REST API is mid-migration. The v2 API
(`/wiki/api/v2`) is the modern, supported surface for pages, blogposts, spaces,
comments, and the like — but it does **not** cover everything. Several
operations the CLI needs have no v2 endpoint and exist only on the older v1 API
(`/wiki/rest/api`): CQL search, the current-user lookup, label writes, and
attachment creation.

A purist "v2 only" client could not implement those commands at all; a "v1
only" client would be built on the surface Atlassian is steering away from.

## Decision

`atl-conf` is a **mixed-version client**: v2 is the default and primary
surface, with **documented v1 fallbacks** for the specific operations v2 does
not expose — CQL search, current-user, label writes, and attachment create. The
client targets the right base per operation; the fallbacks are deliberate and
narrow, not a wholesale dual implementation.

The `api` escape hatch exposes `--api-version` so a caller can explicitly select
v1 or v2 when driving raw endpoints.

## Consequences

- Each v1 fallback is a known, recorded exception rather than an accident — when
  v2 grows an equivalent endpoint, that fallback can be migrated and dropped.
- Contributors adding a Confluence command should reach for v2 first and only
  fall back to v1 when v2 genuinely lacks the endpoint, documenting it where the
  command is described.
- The user-visible consequence (mixed version, with v1 used for CQL search,
  current-user, and label writes) is stated in the README and
  [command-contract.md](../command-contract.md); the base URLs for both versions
  are in [auth-design.md](../auth-design.md).
- Jira (v3, with v2 reachable via `--api-version`) and Bitbucket (v2.0) are not
  mixed in this way; this decision is Confluence-specific.
