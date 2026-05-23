> **Historical / archived.** This document is a completed planning or design artifact, kept for project history. It is **not** maintained and may not reflect the current code. For the implemented behavior see [`docs/command-contract.md`](../command-contract.md) and [`docs/shared-architecture.md`](../shared-architecture.md).

# Phase 8 — Deeper Jira coverage

> Implementation plan for the next post-MVP track per
> [post-mvp-roadmap.md](post-mvp-roadmap.md). Extends `atl-jira` `issue`
> operations beyond create/edit/transition to the day-to-day fields and
> relationships.

## Goal

`atl-jira` covers a working day on an issue without leaving the CLI: assign
and watch the issue, link it to another issue, and log work against it.

## Scope (resolved at phase start)

```text
atl-jira issue assign <issue> <account-id|->        # "-" unassigns
atl-jira issue watch <issue>
atl-jira issue unwatch <issue>
atl-jira issue watchers <issue>
atl-jira issue link <inward> <outward> --type <link-type>
atl-jira issue link types
atl-jira issue worklog list <issue> [--limit N]
atl-jira issue worklog add <issue> --time <duration> [--comment <text>]
```

### Resolved design decisions

1. **Assignee semantics.** `issue assign <issue> <accountId>` sends
   `{"accountId": "<id>"}`. The literal value `-` sends `{"accountId": null}`
   (unassigned). The project-default assignee is **not supported** from the
   CLI this phase; passing `-1` would work as a raw accountId but is not
   advertised. Keep the surface minimal.
2. **Link direction.** `issue link <inward> <outward> --type <link-type>`
   matches the Jira API field names verbatim: the first positional becomes
   `inwardIssue.key`, the second becomes `outwardIssue.key`. `issue link
   types` (`GET /issueLinkType`) prints the available type names plus the
   inward and outward phrases so a caller can see which direction is which.
3. **Worklog time format.** `--time` is passed through verbatim as
   `timeSpent`; Jira accepts duration strings (`3h 30m`) or seconds with
   units, and validates server-side. No CLI-side conversion or parsing.
4. **Worklog comment.** `--comment` is plain text wrapped as ADF via
   `jira.DocOf`, consistent with `issue comment create --body` and `issue
   create --description`. Raw ADF is not exposed on this flag.

## Out of scope

- Setting a project default assignee from the CLI (`-1` sentinel).
- Editing or deleting existing issue links (`DELETE /issueLink/{id}`).
- Editing or deleting an existing worklog
  (`PUT/DELETE /issue/{key}/worklog/{id}`).
- Worklog visibility/restrictions (the `visibility` and
  `properties[*]` fields on a worklog).

## Task outline

### Task 1 — assignee + watchers

Add client methods on `internal/jira`:

- `AssignIssue(ctx, idOrKey, accountID string)` → `PUT
  /issue/{idOrKey}/assignee`, body `{"accountId": <string|null>}`. The
  command layer maps the literal `-` to a JSON `null`.
- `AddWatcher(ctx, idOrKey, accountID string)` → `POST
  /issue/{idOrKey}/watchers`, body is the accountID as a JSON string. An
  empty accountID means the authenticated user (the API default).
- `RemoveWatcher(ctx, idOrKey, accountID string)` → `DELETE
  /issue/{idOrKey}/watchers?accountId=<id>`.
- `ListWatchers(ctx, idOrKey string)` → `GET /issue/{idOrKey}/watchers`.

Add `internal/jiracmd` commands:

- `issue assign <issue> <accountId|->` — required positional; `-` unassigns.
- `issue watch <issue>` — adds the authenticated user as a watcher.
- `issue unwatch <issue>` — removes the authenticated user (looked up via
  `Myself`) as a watcher.
- `issue watchers <issue>` — lists watchers; under `--json`/`--jq` raw
  passthrough, otherwise aligned account-id/display-name rows.

Tests at both layers. Commit: `feat: add atl-jira issue assign and watcher
commands`.

### Task 2 — issue link

Add client methods:

- `CreateIssueLink(ctx, inward, outward, linkType string)` → `POST
  /issueLink`, body
  `{"type":{"name":<type>},"inwardIssue":{"key":<inward>},"outwardIssue":{"key":<outward>}}`.
  Jira returns no body on success.
- `ListIssueLinkTypes(ctx)` → `GET /issueLinkType`, returns
  `{"issueLinkTypes":[...]}` verbatim.

Add `internal/jiracmd` commands:

- `issue link <inward> <outward> --type <link-type>` — all three required.
- `issue link types` — lists available link types; human output is aligned
  rows of `name / inward-phrase / outward-phrase`.

Models: `LinkType{ID, Name, Inward, Outward}` and `LinkTypeList`. Tests.
Commit: `feat: add atl-jira issue link commands`.

### Task 3 — worklog

Add client methods:

- `ListWorklogs(ctx, idOrKey string, limit int)` → `GET
  /issue/{idOrKey}/worklog`, `maxResults=<limit>`. Returns the page body
  with a `worklogs` array.
- `ListWorklogsAll(ctx, idOrKey, limit int)` → follows offset pagination
  (the endpoint returns `startAt`/`maxResults`/`total`/`worklogs`),
  synthesizing `{"worklogs": [...]}`.
- `AddWorklog(ctx, idOrKey, timeSpent string, commentADF json.RawMessage)`
  → `POST /issue/{idOrKey}/worklog`, body `{"timeSpent":<string>,
  "comment":<ADF doc>}` (the comment key is omitted when ADF is nil).
  Returns the created worklog.

Add `internal/jiracmd` commands:

- `issue worklog list <issue> [--limit N] [--all]`.
- `issue worklog add <issue> --time <duration> [--comment <text>]` — wraps
  `--comment` via `jira.DocOf`.

Models: `Worklog{ID, Author User, TimeSpent, TimeSpentSeconds, Started}`
and `WorklogList{Worklogs []Worklog}`. Tests.
Commit: `feat: add atl-jira issue worklog commands`.

### Task 4 — docs, review, PR

Update `docs/command-contract.md` (new commands and the assign/watcher/link
semantics), `docs/jira-mvp.md` if it tracks the surface, `README.md`,
`docs/README.md`, `docs/continuation-handoff.md`. Add `worklog list` to the
`--all` list. Run the multi-agent review wave until clean. Commit: `docs:
document Jira coverage commands`. Open PR.

## Done definition

- An issue can be assigned, unassigned, watched, unwatched, and its
  watchers listed.
- A directional issue link can be created, and the available link types
  can be listed.
- Worklogs can be listed (with pagination) and added with a duration and
  optional comment.
- `command-contract.md` documents the new commands.

## Test posture

- HTTP command tests go through a local `httptest.Server`; no live
  Atlassian calls.
- Each client method has a direct test in `internal/jira/client_test.go`
  parallel to the existing pattern, including method, path, query, and
  request-body shape.
- The `--all` follow on `worklog list` is covered by `internal/jiracmd/
  all_test.go`.
