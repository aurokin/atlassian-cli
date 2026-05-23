> **Historical / archived.** This document is a completed planning or design artifact, kept for project history. It is **not** maintained and may not reflect the current code. For the implemented behavior see [`docs/command-contract.md`](../command-contract.md) and [`docs/shared-architecture.md`](../shared-architecture.md).

# Phase 3: Jira MVP Commands Implementation Plan

**Goal:** Add the first Jira product command surface to `atl-jira` — `project`,
`issue`, `issue comment`, `search issues`, and `status` — backed by a typed
Jira API client. These are the first commands that make live, authenticated
calls beyond the raw `api` escape hatch.

**Why now:** Phases 1–2 delivered the shared foundation (auth, HTTP client,
config, output, errors) and offline URL resolution. The MVP docs
(`docs/jira-mvp.md`) define a concrete Jira command tree; Phase 3 builds the
read and write commands for issues and projects on top of the Phase 1 client.

**Tech stack:** Go, Cobra, standard library only. No new dependencies. Live
calls go through `internal/httpclient`; tests use `net/http/httptest` and never
touch the network. `go test ./...` is the first gate.

---

## Source documents

Read before implementation:

1. `README.md`, `AGENTS.md`
2. `docs/command-contract.md` — current implemented surface
3. `docs/jira-mvp.md` — the Jira command tree and design notes
4. `docs/shared-architecture.md` — `internal/jira/` and `internal/atljiracmd/`
5. `docs/access-error-model.md` — structured error and recovery model
6. `docs/implementation-plan.md` — Phase 3 scope
7. `docs/phase-3-jira-mvp-plan.md` — this file
8. Jira REST v3: https://developer.atlassian.com/cloud/jira/platform/rest/v3/intro/

## Scope

Phase 3 ships in two halves, each its own PR with a review checkpoint between.

**Phase 3A — read-only commands (PR 1):**

```text
atl-jira project list [--limit N]
atl-jira project view <KEY>
atl-jira issue list --project <KEY> [--status S] [--assignee A] [--limit N]
atl-jira issue view <KEY>
atl-jira search issues <JQL> [--limit N]
atl-jira issue comment list <ISSUE> [--limit N]
atl-jira issue comment view <ISSUE> <COMMENT-ID>
atl-jira status
```

**Phase 3B — mutating commands (PR 2):**

```text
atl-jira issue create --project K --type T --summary S [--description D] [--assignee A] [--field name=value]...
atl-jira issue edit <KEY> [--summary S] [--description D] [--assignee A] [--field name=value]...
atl-jira issue transition <KEY> [--to <name-or-id>]
atl-jira issue comment create <ISSUE> --body <text>
atl-jira issue comment edit <ISSUE> <COMMENT-ID> --body <text>
atl-jira issue comment delete <ISSUE> <COMMENT-ID>
```

Non-goals for Phase 3:

- Boards, sprints, filters, versions, components, attachments, worklogs,
  watchers (Phase 5).
- A universal close/reopen abstraction — Jira transitions are workflow
  specific, so `transition` lists and applies real transitions only.
- Rich table rendering — human output is a compact per-command summary;
  a full table renderer stays deferred.
- Confluence commands (Phase 4).
- `--jq`, `--trace`, OS-keychain token storage — still deferred foundation
  work.

## Quality bars

- `go test ./...`, `go vet ./...`, and `gofmt` pass after every task.
- The Jira API client is tested against `httptest` servers with canned Jira
  JSON; no test makes a real network call.
- Non-2xx responses surface as the existing structured `apperr.Error` values
  (`unauthorized`, `forbidden`, `not_found_or_not_visible`, `rate_limited`),
  already produced by `httpclient.Client.Do`.
- Every command has table-driven tests covering success, the structured-error
  path, and argument validation.
- Jira-specific code lives in `internal/jira` and `internal/jiracmd`; the
  shared `internal/cli` package gains only a small, deliberate exported seam.

---

## Package layout

```text
internal/jira/client.go        # typed Jira API client over httpclient.Client
internal/jira/client_test.go
internal/jira/models.go        # model structs for human rendering
internal/jira/adf.go           # plain-text <-> minimal ADF (Phase 3B)
internal/jiracmd/jiracmd.go    # AddCommands; shared command helpers
internal/jiracmd/project.go    # project list|view
internal/jiracmd/issue.go      # issue list|view (+ create|edit|transition 3B)
internal/jiracmd/comment.go    # issue comment list|view (+ create|edit|delete 3B)
internal/jiracmd/search.go     # search issues
internal/jiracmd/status.go     # status
internal/jiracmd/*_test.go
```

`internal/atljiracmd/root.go` calls `jiracmd.AddCommands(root, info, g)` after
`cli.NewRoot`, so the Jira commands are registered only on `atl-jira`.
`atl-conf` is untouched.

The Jira commands reuse the existing auth/HTTP machinery through a small
exported seam on `internal/cli` (Task 1): `cli.Render` and `cli.SiteClient`.

## Design decisions

- **Output:** `--json` emits the raw Jira API response body verbatim
  (true-to-API, consistent with the `api` command). Human output is a curated
  per-command summary built from typed model structs. Native pagination
  metadata is preserved in the `--json` body, never hidden.
- **`issue list` vs `search issues`:** `issue list` is a convenience that
  builds JQL from flags (`--project` required, optional `--status`,
  `--assignee`). `search issues <JQL>` takes raw JQL. Both call the same
  search endpoint.
- **Create/edit input:** typed flags for the common fields (`--project`,
  `--type`, `--summary`, `--description`, `--assignee`) plus a repeatable
  `--field name=value` escape for any other field. `--field` values are parsed
  as JSON when they parse, else treated as a string.
- **Rich text (ADF):** Jira Cloud v3 issue descriptions and comments use the
  Atlassian Document Format. Plain `--description`/`--body` text is wrapped in
  a minimal ADF document by `internal/jira/adf.go`; a caller needing raw ADF
  uses `--field description=<adf-json>`.
- **`status`:** a live health check distinct from the offline `auth status`.
  It calls `/myself` with the `--site` credential and reports the
  authenticated account plus the resolved API base, mapping any auth failure
  to the usual structured error.
- **Identifiers:** account IDs are the stable user identifier; `--assignee`
  takes an account ID (issue-list filters also accept the literal
  `currentUser()`).
- **Pagination:** a `--limit` flag bounds the result count (maps to the API
  `maxResults`). A single page is fetched; an `--all` follow-everything flag
  is deferred.

---

## Phase 3A — read-only commands (PR 1)

### Task 1: shared command seam

**Objective:** Let product commands reuse the auth/HTTP/render machinery
without duplicating it.

**Files:** modify `internal/cli/root.go`, `internal/cli/api.go`; add
`internal/cli/client.go` and a test.

- Export `Render(cmd *cobra.Command, g *GlobalFlags, v any) error` (a thin
  wrapper over the existing `render`).
- Add `SiteClient(info appinfo.Info, g *GlobalFlags) (*httpclient.Client, error)`,
  extracted from `runAPI`: it enforces `--site`, loads the profile, parses the
  token style, resolves the token, and builds the `httpclient.Client`.
- Refactor `runAPI` to call `SiteClient`, proving the extraction is
  behavior-preserving.

**Verify:** `go test ./...`  **Commit:** `refactor: add cli.SiteClient/Render seam for product commands`

### Task 2: Jira API client

**Objective:** A typed read client over `httpclient.Client`.

**Files:** create `internal/jira/client.go`, `internal/jira/models.go`,
`internal/jira/client_test.go`.

- `jira.New(c *httpclient.Client) *Client`.
- Read methods returning the raw `json.RawMessage` body: `Myself`,
  `GetProject`, `SearchProjects`, `GetIssue`, `SearchIssues`, `ListComments`,
  `GetComment`. Each takes a `context.Context`.
- `models.go`: structs (`Issue`, `Project`, `Comment`, `User`, search-result
  wrappers) with `json` tags, decoding only the fields human output needs,
  plus `Parse*` helpers.
- The search endpoint and its pagination shape are confirmed against current
  Jira API docs in this task before the methods are finalized.

**Verify:** `go test ./internal/jira ./...`  **Commit:** `feat: add Jira read API client`

### Task 3: jiracmd skeleton and project commands

**Objective:** Register the Jira tree and ship `project list|view`.

**Files:** create `internal/jiracmd/jiracmd.go`, `internal/jiracmd/project.go`,
tests; modify `internal/atljiracmd/root.go`.

- `jiracmd.AddCommands(root, info, g)` registers the `project`, `issue`,
  `search`, and `status` commands.
- A shared helper resolves a `*jira.Client` from `cli.SiteClient`.
- `project list` / `project view` render via `cli.Render` (human + `--json`).

**Verify:** `go test ./...`  **Commit:** `feat: add jira project commands`

### Task 4: issue view and list

**Files:** create `internal/jiracmd/issue.go` and test.

- `issue view <KEY>` — `GetIssue`, curated human summary.
- `issue list --project <KEY> [--status] [--assignee] [--limit]` — builds JQL
  and calls `SearchIssues`.

**Verify:** `go test ./...`  **Commit:** `feat: add jira issue view and list`

### Task 5: search and status

**Files:** create `internal/jiracmd/search.go`, `internal/jiracmd/status.go`,
tests.

- `search issues <JQL> [--limit]` — raw JQL via `SearchIssues`.
- `status` — `Myself`, reporting account and resolved API base.

**Verify:** `go test ./...`  **Commit:** `feat: add jira search and status`

### Task 6: issue comment read commands

**Files:** create `internal/jiracmd/comment.go` and test.

- `issue comment list <ISSUE> [--limit]` and `issue comment view <ISSUE> <ID>`.

**Verify:** `go test ./...`  **Commit:** `feat: add jira issue comment read commands`

### Checkpoint

After Tasks 1–6 the Phase 3A read-only surface is complete. Stop and review the
command shapes, the `internal/jira` client API, and the human-output format
before documenting and opening PR 1.

### Task 7: documentation and PR 1

**Files:** update `docs/command-contract.md`, `docs/continuation-handoff.md`,
`docs/README.md`, `README.md`.

**Verify:** `go test ./...`, `go vet ./...`, `gofmt -l`, `git diff --check`.
**Commit:** `docs: document Phase 3A Jira read commands`. Open and review PR 1.

---

## Phase 3B — mutating commands (PR 2)

### Task 8: ADF helper and Jira write client

**Files:** create `internal/jira/adf.go`; extend `internal/jira/client.go`.

- `adf.go`: wrap plain text into a minimal ADF document.
- Write methods: `CreateIssue`, `EditIssue`, `GetTransitions`, `DoTransition`,
  `CreateComment`, `EditComment`, `DeleteComment`.

**Commit:** `feat: add Jira write API client and ADF helper`

### Task 9: issue create and edit

- `issue create` / `issue edit` with typed flags + repeatable `--field`.

**Commit:** `feat: add jira issue create and edit`

### Task 10: issue transition

- `issue transition <KEY>` lists available transitions; `--to <name-or-id>`
  applies one. No universal close/reopen.

**Commit:** `feat: add jira issue transition`

### Task 11: issue comment write commands

- `issue comment create|edit|delete`.

**Commit:** `feat: add jira issue comment write commands`

### Task 12: documentation and PR 2

Update the same doc set; open and review PR 2.

**Commit:** `docs: document Phase 3B Jira write commands`

---

## Phase 3 done definition

- `atl-jira project list --json` and `project view <KEY> --json` work against a
  configured site.
- `atl-jira issue view <KEY>` renders a human summary; `--json` is the raw API
  body.
- `atl-jira search issues "<JQL>"` and `issue list --project <KEY>` return
  matching issues.
- `atl-jira status` reports the authenticated account or a structured error.
- `issue create|edit|transition` and `issue comment create|edit|delete`
  perform the corresponding API mutations.
- Non-2xx responses surface as structured `apperr.Error` envelopes.
- The Jira client and commands are table-tested against `httptest`; no test
  makes a real network call.
- `go test ./...`, `go vet ./...`, and `gofmt` are clean.
- Docs list the new commands and their known limitations.
