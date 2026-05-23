> **Historical / archived.** This document is a completed planning or design artifact, kept for project history. It is **not** maintained and may not reflect the current code. For the implemented behavior see [`docs/command-contract.md`](../command-contract.md) and [`docs/shared-architecture.md`](../shared-architecture.md).

# Phase 5: Output & Pagination Polish Implementation Plan

**Goal:** Make `--jq` real and add an `--all` follow-all-pages flag, so both
`atl-jira` and `atl-conf` are genuinely composable for agents and scripts.

**Why now:** `--jq` is a documented stub (`output.ErrJQNotImplemented`) and
`--all` has been deferred since Phase 3. Both are cross-cutting — landing them
before the Phase 7/8 product-depth work means every new command inherits them.
This is Phase 5 of `docs/post-mvp-roadmap.md`.

**Tech stack:** Go, Cobra. Phase 5 adds the project's first non-Cobra
dependency: `github.com/itchyny/gojq` for `--jq`. The stdlib-only-beyond-Cobra
rule is relaxed to "Cobra plus a small set of well-maintained dependencies,
added deliberately"; `gojq` is the first such addition. Tests use
`net/http/httptest` and never touch the network.

---

## Source documents

1. `README.md`, `AGENTS.md`
2. `docs/command-contract.md` — current implemented surface
3. `docs/post-mvp-roadmap.md` — Phase 5 scope contract
4. `docs/shared-architecture.md` — output renderer and pagination notes
5. gojq: https://github.com/itchyny/gojq
6. Jira REST v3 pagination — `/project/search` (offset), `/search/jql`
   (`nextPageToken`), `/issue/{key}/comment` (offset)
7. Confluence REST v2 pagination — cursor in the `Link` header / `_links.next`

## Scope

Phase 5 ships in two halves, each its own PR with a review checkpoint between,
mirroring Phases 3 and 4.

**Phase 5A — `--jq` filtering (PR 1):**

```text
atl-jira issue view ABC-1 --jq '.fields.status.name'
atl-conf search cql 'type = page' --jq '.results[].content.title'
atl-jira issue list --project DEV \
  --jq '.issues[] | select(.fields.status.name=="Done") | .key'
```

**Phase 5B — `--all` pagination (PR 2):**

```text
atl-jira project list --all
atl-jira issue list --project DEV --all
atl-jira search issues '<jql>' --all
atl-jira issue comment list ABC-1 --all
atl-conf space list --all
atl-conf page list --space DEV --all
atl-conf page children <id> --all
atl-conf search cql '<cql>' --all
```

Non-goals for Phase 5:

- A jq `--raw-output` mode — Phase 5A uses jq's default formatting; a raw mode
  can be added later if needed.
- Streaming output — `--all` collects pages in memory and emits one aggregate.
- Server-side cursor resumption / saved cursors.
- Phase 6–8 work (token storage, product depth).

## Quality bars

- `go test ./...`, `go vet ./...`, and `gofmt` pass after every task.
- `go mod tidy` is clean; `gojq` and its transitive deps are the only
  additions to `go.mod`.
- Every new flag has tests for success, the error path, and edge cases.
- No live Atlassian calls; HTTP tests use `httptest`.
- Multi-agent review wave on each PR branch until no findings remain.

---

## Design decisions

### `--jq` (Phase 5A)

- **Engine:** `github.com/itchyny/gojq` — full jq language (`select()`,
  pipes, functions, arithmetic). Chosen over a hand-written subset so `--jq`
  behaves exactly like jq with no surprising gaps.
- **Input:** `--jq` operates on the same JSON value a command would emit under
  `--json`. `internal/output` marshals the value and unmarshals it into a
  generic `any` (`map[string]any` / `[]any` / scalars) — the shape gojq
  consumes. This works whether the command passes a `json.RawMessage` (most
  commands) or a synthesized struct (e.g. `editResult`).
- **Output:** each query result is printed on its own line as compact JSON,
  matching jq's default (non-`-r`) behavior. An empty result stream prints
  nothing and is not an error.
- **Errors:** a query that fails to parse → `apperr.InvalidInput` (`invalid_input`).
  A runtime error from gojq (e.g. indexing a string) → a structured error.
- **Relationship to `--json`:** `--json` (bare → full, or a comma-separated
  top-level field list) and `--jq` are independent flags. When `--jq` is set
  it owns the output and runs against the full JSON value. Passing `--jq`
  together with a `--json` *field list* is rejected as ambiguous
  (`invalid_input`); bare `--json` with `--jq` is allowed and equivalent to
  `--jq` alone.

### `--all` (Phase 5B)

- **Three pagination styles** the helper must handle:
  - *Offset* — Jira `/project/search` and `/issue/{key}/comment`
    (`startAt` + `maxResults`, with `isLast` / `total`).
  - *Token* — Jira `/search/jql` (`nextPageToken`, `isLast`).
  - *Cursor* — Confluence v2 list endpoints and v1 `/search`
    (`_links.next`, a relative URL carrying an opaque `cursor`).
  Task 5B-1 confirms each against current API docs before coding.
- **Aggregate output is synthesized, not verbatim.** A multi-page result has
  no single API body, so `--all` emits a synthesized object that keeps the
  first page's top-level item key (`values` / `issues` / `comments` /
  `results`) holding every item, with per-page pagination cursors dropped.
  This is the documented exception to "`--json` is the verbatim API body",
  alongside the existing synthesized 204-mutation results.
- **`--all` + `--limit`:** `--limit` sets the per-request page size; `--all`
  follows pages until the API reports no next page. `--limit` does not cap the
  total under `--all`.
- **Safety cap:** a hard cap on pages followed (e.g. 100) guards against an
  unbounded loop from a malformed cursor; reaching it stops following and
  emits what was collected.
- **`--all` + `--jq`:** compose cleanly — `--jq` runs against the synthesized
  aggregate.

---

## Package layout

```text
internal/output/output.go        # gojq-backed --jq; replaces the stub
internal/output/output_test.go
internal/jira/client.go          # page-following + *All list methods
internal/conf/client.go          # page-following + *All list methods
internal/jira/models.go          # pagination cursor fields
internal/conf/models.go          # pagination cursor fields
internal/jiracmd/*.go            # --all flag on list/search commands
internal/confcmd/*.go            # --all flag on list/search commands
```

No `internal/cli` signature changes are expected: `Render` already carries
`Options{JSON, JQ}`, and `--all` is plumbed per command.

---

## Phase 5A — `--jq` filtering (PR 1)

### Task 1: add gojq and implement `--jq`

**Objective:** Replace the `ErrJQNotImplemented` stub with a gojq-backed filter.

- `go get github.com/itchyny/gojq`; `go mod tidy`.
- In `internal/output`, add a `renderJQ` path: marshal `v` → unmarshal into
  `any` → `gojq.Parse` + `gojq.Compile` → run → print each result as compact
  JSON, one per line.
- Map parse failures to `apperr.InvalidInput`; map runtime errors to a
  structured error.
- Remove `ErrJQNotImplemented`; update `Render` so the `JQ` branch filters
  instead of erroring.

**Verify:** `go test ./internal/output ./...`
**Commit:** `feat: implement --jq filtering with gojq`

### Task 2: wire `--jq` through the commands and update tests

**Objective:** Confirm `--jq` works end to end and fix the stub-era tests.

- The Jira and Confluence command tests that currently assert
  `ErrJQNotImplemented` (`TestSearchIssuesJQReachesRenderer`,
  `TestSearchCQLJQReachesRenderer`) are updated to assert a real filtered
  result.
- Add `--jq` success and error tests against representative commands.
- Reject `--jq` combined with a `--json` field list (`invalid_input`).

**Verify:** `go test ./...`
**Commit:** `test: cover --jq across the command surface`

### Task 3: Phase 5A docs and PR 1

- `docs/command-contract.md`: document `--jq` (gojq, default formatting, the
  `--json` field-list conflict). Update the "`--jq` is a stub" limitation.
- `README.md`, `docs/README.md`, `AGENTS.md`, `docs/shared-architecture.md`:
  note the relaxed dependency rule and the `gojq` addition.
- `docs/continuation-handoff.md`: status and next action.

**Verify:** `go test ./...`, `go vet ./...`, `gofmt -l`, `git diff --check`.
**Commit:** `docs: document --jq filtering`. Open and review PR 1.

---

## Phase 5B — `--all` pagination (PR 2)

### Task 4: confirm cursors and model them

- Confirm the pagination shape of every list/search endpoint against current
  API docs (offset / token / cursor).
- Add the needed cursor fields to `internal/jira` and `internal/conf` page
  models (`isLast`, `nextPageToken`, `_links.next`, `startAt`, `total`).

**Verify:** `go test ./...`
**Commit:** `feat: model list pagination cursors`

### Task 5: page-following client methods

- Add a page-following helper to each client that, given a first request and
  the endpoint's pagination style, follows to completion under the safety cap.
- Expose `*All` variants (or a follow parameter) for each list/search method.
- The helper returns the concatenated items so the command can synthesize the
  aggregate body.

**Verify:** `go test ./internal/jira ./internal/conf ./...`
**Commit:** `feat: add page-following to the Jira and Confluence clients`

### Task 6: `--all` flag on the commands

- Add `--all` to `project list`, `issue list`, `search issues`,
  `issue comment list`, `space list`, `page list`, `page children`, and
  `search cql`.
- With `--all`, emit the synthesized aggregate; human output renders every
  row; `--json`/`--jq` operate on the aggregate.

**Verify:** `go test ./...`
**Commit:** `feat: add --all follow-all-pages flag`

### Task 7: Phase 5B docs and PR 2

- `docs/command-contract.md`: document `--all`, the synthesized-aggregate
  exception, the `--limit` interplay, and the page cap. Update the
  no-follow-all-pages limitation.
- Refresh `README.md`, `docs/README.md`, `docs/continuation-handoff.md`.

**Verify:** full verification suite.
**Commit:** `docs: document --all pagination`. Open and review PR 2.

---

## Phase 5 done definition

- `--jq` evaluates any jq expression against any command's JSON output; no
  command reports jq as unimplemented.
- `--all` returns every page of a multi-page result for all list/search
  commands, bounded by a documented page cap.
- `--jq` and `--all` compose.
- `go.mod` cleanly carries `gojq`; `go test ./...`, `go vet ./...`, and
  `gofmt` are clean.
- Docs describe `--jq`, `--all`, the synthesized aggregate, and the relaxed
  dependency rule.
