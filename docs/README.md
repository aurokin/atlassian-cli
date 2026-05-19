# Documentation

> Point-in-time design package derived from the combined Jira/Confluence spec committed in `bitbucket_cli` on 2026-05-14. This docs directory is already scoped to the `atlassian-cli` project, so docs live directly under `docs/`.

## Goal

Create two separate CLIs, `atl-jira` and `atl-conf`, with shared foundations where useful but separate product vocabularies where necessary. If Bitbucket is migrated later, its unified binary shape is `atl-bb`.

## Design docs

- [auth-design.md](auth-design.md) — Cloud classic/scoped tokens, Data Center PAT, OAuth later.
- [access-error-model.md](access-error-model.md) — permission-aware UX and structured recovery.
- [shared-architecture.md](shared-architecture.md) — shared packages, raw API, output, config, pagination.
- [jira-mvp.md](jira-mvp.md) — first Jira command surface.
- [confluence-mvp.md](confluence-mvp.md) — first Confluence command surface.
- [implementation-plan.md](implementation-plan.md) — phased build plan.
- [phase-1-foundation-plan.md](phase-1-foundation-plan.md) — implementation plan for the shared Go/Cobra foundation (Phase 1, complete).
- [phase-2-resolve-browse-plan.md](phase-2-resolve-browse-plan.md) — implementation plan for URL resolution and the `resolve`/`browse` commands (Phase 2, complete).
- [phase-3-jira-mvp-plan.md](phase-3-jira-mvp-plan.md) — implementation plan for the Jira MVP commands: `project`, `issue`, `search`, `status` (Phase 3, complete).
- [phase-4-confluence-mvp-plan.md](phase-4-confluence-mvp-plan.md) — implementation plan for the Confluence MVP commands: `space`, `page` (read plus create/edit), `search cql`, `status` (Phase 4, complete).
- [post-mvp-roadmap.md](post-mvp-roadmap.md) — sequenced plan for Phases 5–8: output & pagination polish, secure token storage, Confluence content depth, deeper Jira coverage.
- [phase-5-output-pagination-plan.md](phase-5-output-pagination-plan.md) — implementation plan for Phase 5: `--jq` filtering (5A) and the `--all` follow-all-pages flag (5B).
- [phase-6-secure-token-storage-plan.md](phase-6-secure-token-storage-plan.md) — implementation plan for Phase 6: secure token storage (OS keychain via go-keyring, with a `0600`-file fallback).
- [command-contract.md](command-contract.md) — implemented command behavior, config schema, and known limitations.
- [continuation-handoff.md](continuation-handoff.md) — point-in-time handoff for continuing this work in the app or a fresh agent session.
- [bitbucket-migration-roadmap.md](bitbucket-migration-roadmap.md) — long-term plan for possibly bringing legacy `bb` into the shared `atl-*` Atlassian CLI ecosystem as `atl-bb`.
- [bb-rewrite-plan.md](bb-rewrite-plan.md) — placeholder standards for the future Bitbucket import-and-rewrite period.

## Naming

`atl-jira`, `atl-conf`, and future `atl-bb` are the binary names. The `atl-` prefix avoids collisions with common packages and makes these feel like one CLI family. Avoid bare `jira`, bare `confluence`, `jj`, `cc`, and `conf` because of collisions or ambiguity.
