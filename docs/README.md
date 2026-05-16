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
- [phase-1-foundation-plan.md](phase-1-foundation-plan.md) — concrete next implementation plan for the shared Go/Cobra foundation.
- [continuation-handoff.md](continuation-handoff.md) — point-in-time handoff for continuing this work in the app or a fresh agent session.
- [bitbucket-migration-roadmap.md](bitbucket-migration-roadmap.md) — long-term plan for possibly bringing legacy `bb` into the shared `atl-*` Atlassian CLI ecosystem as `atl-bb`.
- [bb-rewrite-plan.md](bb-rewrite-plan.md) — placeholder standards for the future Bitbucket import-and-rewrite period.

## Naming

`atl-jira`, `atl-conf`, and future `atl-bb` are the binary names. The `atl-` prefix avoids collisions with common packages and makes these feel like one CLI family. Avoid bare `jira`, bare `confluence`, `jj`, `cc`, and `conf` because of collisions or ambiguity.
