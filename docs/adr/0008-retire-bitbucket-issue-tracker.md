# 0008 — Retire the Bitbucket issue-tracker surface

**Status:** Accepted

## Context

Bitbucket Cloud removed its native issue tracker and wiki — including their
REST endpoints — on 2026-08-20
([changelog CHANGE-3401](https://developer.atlassian.com/cloud/bitbucket/changelog)).
`GET /repositories/{workspace}/{repo}/issues` and its siblings now return HTTP
410, and Atlassian shipped no replacement endpoint.

`atl-bb` had a full surface built on those endpoints: `issue
list`/`view`/`create`/`update`, `search issues`, the `bitbucket_issue`
`resolve`/`browse` kind, and a dedicated `feature_disabled` error category
whose only job was to distinguish "this repository's issue tracker is switched
off" from "not found or not visible to this account".

The options were: keep the commands and let every invocation fail with `gone`;
re-point them at some other tracker (Jira); or remove them.

## Decision

Remove the surface. `atl-bb issue …`, `atl-bb search issues`, the
`bitbucket_issue` resource kind, and the `feature_disabled` error code are
deleted rather than kept as permanently-failing commands or emulated on top of
another product.

This follows from [ADR 0006](0006-verbatim-json-no-fake-parity.md): where
Atlassian exposes no real API path, the CLI does not fake one. Mapping `atl-bb
issue` onto Jira issues is exactly the fake parity that ADR forbids, and a
command whose only possible outcome is a 410 advertises a capability the
platform no longer has.

## Consequences

- Breaking change for callers: `atl-bb issue …` and `atl-bb search issues` now
  fail as unknown commands. No flag restores them; a script that tracked work
  in Bitbucket issues has to move to whatever tracker the team migrated to.
- A Bitbucket issue URL (`…/issues/{id}`) is no longer a distinct resolved
  kind. It falls through the parser's default branch and resolves to the
  enclosing repository, like any other unmodeled repository sub-page.
- `feature_disabled` leaves the error catalog in
  [access-error-model.md](../access-error-model.md); it had no other producer.
  It shared exit code `1`, so no exit code is freed or reassigned and
  [ADR 0001](0001-per-category-exit-codes.md)'s numbering is untouched — only
  its list of rare categories is superseded on that one entry.
- The same changelog removed the repository wiki. The CLI never had wiki
  commands, so nothing else is affected.
- This is the second Bitbucket endpoint withdrawal the CLI has absorbed, after
  the cross-workspace enumeration endpoint (`GET /2.0/workspaces`, CHANGE-3022,
  2026-04-14). The pattern is settled: when Atlassian withdraws an endpoint,
  the dependent commands go with it and the removal is documented where the
  commands used to be described.
