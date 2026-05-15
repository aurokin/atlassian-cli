# Atlassian CLI Design Package

> Point-in-time design package derived from the combined Jira/Confluence spec committed in `bitbucket_cli` on 2026-05-14.

## Goal

Create two separate CLIs, `jira` and `confluence`, with shared foundations where useful but separate product vocabularies where necessary.

## Design docs

- [auth-design.md](auth-design.md) — Cloud classic/scoped tokens, Data Center PAT, OAuth later.
- [access-error-model.md](access-error-model.md) — permission-aware UX and structured recovery.
- [shared-architecture.md](shared-architecture.md) — shared packages, raw API, output, config, pagination.
- [jira-mvp.md](jira-mvp.md) — first Jira command surface.
- [confluence-mvp.md](confluence-mvp.md) — first Confluence command surface.
- [implementation-plan.md](implementation-plan.md) — phased build plan.
- [bitbucket-migration-roadmap.md](bitbucket-migration-roadmap.md) — long-term plan for possibly bringing `bb` into the shared Atlassian CLI ecosystem.

## Naming

`jira` and `confluence` are the current binary names. Avoid `jj`, `cc`, and `conf` because of collisions with Jujutsu, Claude Code, and config shorthand.
