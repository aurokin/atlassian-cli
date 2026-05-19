# Phase 4: Confluence MVP Commands Implementation Plan

**Goal:** Add the first Confluence product command surface to `atl-conf` —
`space`, `page`, `search cql`, and `status` — backed by a typed Confluence API
client. This is the Confluence mirror of the Phase 3 Jira MVP and the first
time `atl-conf` does more than the shared `auth`/`api`/`resolve`/`browse`
commands.

**Why now:** Phase 3 completed the Jira MVP and proved the product-command
pattern: a typed `internal/jira` client plus an `internal/jiracmd` tree layered
onto the shared root. Phase 4 applies the same shape to Confluence, guided by
`docs/confluence-mvp.md`.

**Tech stack:** Go, Cobra, standard library only. No new dependencies. Live
calls go through `internal/httpclient`; tests use `net/http/httptest` and never
touch the network. `go test ./...` is the first gate.

---

## Source documents

Read before implementation:

1. `README.md`, `AGENTS.md`
2. `docs/command-contract.md` — current implemented surface
3. `docs/confluence-mvp.md` — the Confluence command tree and design notes
4. `docs/shared-architecture.md` — `internal/conf/` and `internal/atlconfcmd/`
5. `docs/access-error-model.md` — structured error and recovery model
6. `docs/phase-3-jira-mvp-plan.md` — the Jira MVP this phase mirrors
7. `docs/phase-4-confluence-mvp-plan.md` — this file
8. Confluence REST v2: https://developer.atlassian.com/cloud/confluence/rest/v2/intro/
9. Confluence REST v1: https://developer.atlassian.com/cloud/confluence/rest/v1/intro/

## Scope

Phase 4 ships in two halves, each its own PR with a review checkpoint between.

**Phase 4A — read-only commands (PR 1):**

```text
atl-conf space list [--limit N]
atl-conf space view <key>
atl-conf page list --space <key> [--limit N]
atl-conf page view <id>
atl-conf page children <id> [--limit N]
atl-conf search cql <cql> [--limit N]
atl-conf status
```

**Phase 4B — mutating commands (PR 2):**

```text
atl-conf page create --space <key> --title <text> --body <text> --body-format <storage|atlas_doc_format|wiki>
atl-conf page edit <id> [--title <text>] [--body <text> --body-format <fmt>]
```

Non-goals for Phase 4:

- Attachments, comments, labels, versions, properties, blogs, groups, users
  (Phase 5).
- Page deletion, move, and restore — the MVP is list/view/children plus
  create/edit only.
- Silent body-format conversion — writes carry an explicit `--body-format` and
  the content is passed through verbatim.
- Rich table rendering — human output is a compact per-command summary.
- Jira commands (done, Phase 3).
- `--jq`, `--trace`, OS-keychain token storage — still deferred foundation
  work.

## Quality bars

- `go test ./...`, `go vet ./...`, and `gofmt` pass after every task.
- The Confluence API client is tested against `httptest` servers with canned
  Confluence JSON; no test makes a real network call.
- Non-2xx responses surface as the existing structured `apperr.Error` values
  (`unauthorized`, `forbidden`, `not_found_or_not_visible`, `rate_limited`),
  already produced by `httpclient.Client.Do`.
- Every command has tests covering success, the structured-error path, and
  argument validation.
- Confluence-specific code lives in `internal/conf` and `internal/confcmd`. The
  shared `internal/cli` package needs no changes — the `SiteClient`/`Render`
  seam built in Phase 3 is already product-agnostic.

---

## Package layout

```text
internal/conf/client.go        # typed Confluence API client over httpclient.Client
internal/conf/client_test.go
internal/conf/models.go        # model structs for human rendering
internal/confcmd/confcmd.go    # AddCommands; shared command helpers
internal/confcmd/space.go      # space list|view
internal/confcmd/page.go       # page list|view|children (+ create|edit 4B)
internal/confcmd/search.go     # search cql
internal/confcmd/status.go     # status
internal/confcmd/*_test.go
```

`internal/atlconfcmd/root.go` calls `confcmd.AddCommands(root, info, g)` after
`cli.NewRoot`, mirroring how `atljiracmd` layers the Jira commands. This is the
one change outside `internal/conf`/`internal/confcmd`.

## Design decisions

- **Reuse the Phase 3 seam:** product commands resolve a `*httpclient.Client`
  via `cli.SiteClient(info, g)` and render via `cli.Render`. Both already key
  off `info.Product`, so Confluence needs no `internal/cli` change.
- **API version:** Confluence Cloud REST **v2** is the primary surface (the
  configured API base is `<site>/wiki/api/v2`). Two MVP needs are not covered
  by v2 and fall back to **v1**, as `docs/confluence-mvp.md` anticipates:
  - **CQL search** — v2 has no CQL endpoint; `search cql` uses the v1
    `/wiki/rest/api/search` endpoint.
  - **current user** — v2 has no current-user resource; `status` uses the v1
    `/wiki/rest/api/user/current` endpoint.
  Task 1 confirms each endpoint and pagination shape against current docs, and
  decides how the client addresses a v1 path from a v2-rooted base (likely an
  absolute URL built from the site root, allowed by `httpclient` origin rules).
- **Output:** `--json` emits the raw Confluence API response body verbatim
  (true-to-API, consistent with `atl-jira`). Human output is a curated
  per-command summary built from typed model structs.
- **Identifiers:** v2 pages and spaces are addressed by numeric id. Users think
  in space *keys*, so `space view <key>` and `page list --space <key>` accept a
  key and resolve it to a space id via `GET /spaces?keys=<key>`. `page view`
  and `page children` take a page id directly (obtainable from `page list` or
  `resolve`).
- **Page bodies:** `page create`/`edit` take `--body` plus a required
  `--body-format` (`storage`, `atlas_doc_format`, or `wiki`). The body is sent
  to the API verbatim under the named representation — never converted.
- **Page editing is versioned:** `page edit` first GETs the page to read its
  current `version.number`, then PUTs with `number + 1`. A `409` conflict
  surfaces as the usual structured error. Title and body are both optional on
  edit, but at least one change is required.
- **Pagination:** a `--limit` flag bounds the result count. A single page is
  fetched; an `--all` follow-everything flag is deferred (same as Jira).

---

## Phase 4A — read-only commands (PR 1)

### Task 1: Confluence read API client

**Objective:** A typed read client over `httpclient.Client`.

**Files:** create `internal/conf/client.go`, `internal/conf/models.go`,
`internal/conf/client_test.go`.

- `conf.New(c *httpclient.Client) *Client`.
- Read methods returning the raw `json.RawMessage` body: `CurrentUser`,
  `ListSpaces`, `GetSpace`/`GetSpaceByKey`, `ListPages`, `GetPage`,
  `GetChildPages`, `SearchCQL`. Each takes a `context.Context`.
- `models.go`: structs (`Space`, `Page`, `User`, search/list wrappers) with
  `json` tags, decoding only the fields human output needs, plus a generic
  `Decode[T]` (or reuse the Jira pattern).
- Confirm the v2 endpoints/params and the v1 fallback paths for CQL search and
  current user against current docs in this task.

**Verify:** `go test ./internal/conf ./...`  **Commit:** `feat: add Confluence read API client`

### Task 2: confcmd skeleton and space commands

**Objective:** Register the Confluence tree and ship `space list|view`.

**Files:** create `internal/confcmd/confcmd.go`, `internal/confcmd/space.go`,
tests; modify `internal/atlconfcmd/root.go`.

- `confcmd.AddCommands(root, info, g)` registers `space`, `page`, `search`, and
  `status`.
- A shared helper resolves a `*conf.Client` from `cli.SiteClient`.
- `space list` / `space view <key>` render via `cli.Render` (human + `--json`).

**Verify:** `go test ./...`  **Commit:** `feat: add conf space commands`

### Task 3: page read commands

**Files:** create `internal/confcmd/page.go` and test.

- `page list --space <key> [--limit]`, `page view <id>`,
  `page children <id> [--limit]`.

**Verify:** `go test ./...`  **Commit:** `feat: add conf page read commands`

### Task 4: search cql and status

**Files:** create `internal/confcmd/search.go`, `internal/confcmd/status.go`,
tests.

- `search cql <cql> [--limit]` — raw CQL via the v1 search endpoint.
- `status` — current user via the v1 endpoint, reporting the account and the
  resolved API base.

**Verify:** `go test ./...`  **Commit:** `feat: add conf search and status`

### Checkpoint

After Tasks 1–4 the Phase 4A read-only surface is complete. Stop and review the
command shapes, the `internal/conf` client API, and the human-output format
before documenting and opening PR 1.

### Task 5: documentation and PR 1

**Files:** update `docs/command-contract.md`, `docs/continuation-handoff.md`,
`docs/README.md`, `README.md`.

**Verify:** `go test ./...`, `go vet ./...`, `gofmt -l`, `git diff --check`.
**Commit:** `docs: document Phase 4A Confluence read commands`. Open and review
PR 1.

---

## Phase 4B — mutating commands (PR 2)

### Task 6: Confluence write client and page create/edit

**Files:** extend `internal/conf/client.go`; create `internal/confcmd/page.go`
write commands (in the existing file) and tests.

- Write methods: `CreatePage`, `UpdatePage`.
- `page create --space <key> --title --body --body-format` — resolves the space
  key to an id, then POSTs.
- `page edit <id> [--title] [--body --body-format]` — GETs the current version,
  then PUTs with the incremented version number.

**Commit:** `feat: add conf page create and edit`

### Task 7: documentation and PR 2

Update the same doc set; open and review PR 2.

**Commit:** `docs: document Phase 4B Confluence write commands`

---

## Phase 4 done definition

- `atl-conf space list --json` and `space view <key> --json` work against a
  configured site.
- `atl-conf page view <id>` renders a human summary; `--json` is the raw API
  body.
- `atl-conf page list --space <key>` and `page children <id>` return matching
  pages.
- `atl-conf search cql "<cql>"` returns matching content.
- `atl-conf status` reports the authenticated account or a structured error.
- `page create` and `page edit` perform the corresponding API mutations with an
  explicit `--body-format`.
- Non-2xx responses surface as structured `apperr.Error` envelopes.
- The Confluence client and commands are tested against `httptest`; no test
  makes a real network call.
- `go test ./...`, `go vet ./...`, and `gofmt` are clean.
- Docs list the new commands and their known limitations.
