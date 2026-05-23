> **Historical / archived.** This document is a completed planning or design artifact, kept for project history. It is **not** maintained and may not reflect the current code. For the implemented behavior see [`docs/command-contract.md`](../command-contract.md) and [`docs/shared-architecture.md`](../shared-architecture.md).

# Implementation Plan

> This is a planning scaffold, not final implementation instructions.

## Phase 0: Design freeze for foundation

- Confirm repo naming and binary names.
- Finalize auth config schema.
- Finalize structured error/recovery schema.
- Decide whether initial code lives in one repo with two binaries or separate repos.

## Phase 1: Shared foundation

- Initialize Go module.
- Add Cobra root commands for `atl-jira` and `atl-conf`.
- Implement config loading/saving with safe file permissions.
- Implement output renderer with table, `--json`, selected fields, and `--jq`.
- Implement auth request signing for `cloud-classic`, `cloud-scoped`, and `data-center-pat`.
- Implement `auth login/status/logout`.
- Implement raw `api` command with effective API base routing.
- Implement structured access/error model.

## Phase 2: URL resolution and browse

- Add Jira issue/project URL parser.
- Add Confluence page/space URL parser.
- Add `resolve <url> --json '*'`.
- Add deterministic `browse --no-browser` URL output.

## Phase 3: Jira MVP commands

- `project list/view`
- `issue list/view/create/edit/transition`
- `issue comment list/view/create/edit/delete`
- `search issues`
- `status`

## Phase 4: Confluence MVP commands

- `space list/view`
- `page list/view/create/edit/children`
- `search cql`
- `status`

## Phases 5–8: Post-MVP

Detailed and sequenced in [post-mvp-roadmap.md](post-mvp-roadmap.md):

- Phase 5: output & pagination polish (`--jq`, `--all`).
- Phase 6: secure token storage (OS keychain).
- Phase 7: Confluence content depth (comments, labels, attachments).
- Phase 8: deeper Jira coverage (assign, watchers, links, worklog).

OAuth 3LO stays deferred until token-based auth is proven robust.

## Phase 9: Monorepo/refactor review

- Identify code repeated across Jira and Confluence.
- Read [bitbucket-migration-roadmap.md](bitbucket-migration-roadmap.md) before making any legacy `bb` to `atl-bb` migration decision.
- Inventory the current Bitbucket CLI into `bb-inventory.md`, including legacy `bb` behavior and the desired `atl-bb` shape.
- Score candidate shared packages in `shared-foundation-scorecard.md`.
- Write `bb-rewrite-plan.md` defining how imported Bitbucket code will be brought up to the new `atl-*` standards for structure, tests, recovery UX, docs generation, and performance.
- Write `bb-compatibility-plan.md` covering `atl-bb` behavior, legacy `bb` alias/wrapper strategy, config path/migration, JSON fields, generated docs, repo-local skill, and release transition.
- Decide whether to import Bitbucket CLI as a rewrite baseline in the shared Atlassian monorepo, consume shared code as a module, or leave it separate for now.
- Extract only proven shared foundations.
- If Bitbucket source is imported, treat legacy `bb` as a behavior oracle and refactor incrementally onto the new foundation rather than preserving old internals by default.
