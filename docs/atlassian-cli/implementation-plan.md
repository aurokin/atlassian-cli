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

## Phase 5: Broader coverage

- Jira boards/sprints/filters/versions/components/attachments/worklogs/watchers.
- Confluence attachments/comments/labels/versions/properties/blogs/groups/users.
- Consider OAuth 3LO only after token-based auth is robust.

## Phase 6: Monorepo/refactor review

- Identify code repeated across Jira and Confluence.
- Read [bitbucket-migration-roadmap.md](bitbucket-migration-roadmap.md) before making any legacy `bb` to `atl-bb` migration decision.
- Inventory the current Bitbucket CLI into `bb-inventory.md`, including legacy `bb` behavior and the desired `atl-bb` shape.
- Score candidate shared packages in `shared-foundation-scorecard.md`.
- Write `bb-compatibility-plan.md` covering `atl-bb` behavior, legacy `bb` alias/wrapper strategy, config path/migration, JSON fields, generated docs, repo-local skill, and release transition.
- Decide whether to migrate Bitbucket CLI into a shared Atlassian monorepo, consume shared code as a module, or leave it separate.
- Extract only proven shared foundations.
