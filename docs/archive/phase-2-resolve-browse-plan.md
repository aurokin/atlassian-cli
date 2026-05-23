> **Historical / archived.** This document is a completed planning or design artifact, kept for project history. It is **not** maintained and may not reflect the current code. For the implemented behavior see [`docs/command-contract.md`](../command-contract.md) and [`docs/shared-architecture.md`](../shared-architecture.md).

# Phase 2: URL Resolution and Browse Implementation Plan

**Goal:** Add deterministic URL/key resolution and a `browse` command to
`atl-jira` and `atl-conf`: a shared `internal/resolve` parser framework with
product-specific parsers, the `resolve` command, and the `browse` command.

**Why now:** The MVP docs say to prefer resolved identifiers — `docs/jira-mvp.md`
("prefer JQL", account/issue keys) and `docs/confluence-mvp.md` ("prefer page
IDs after URL/title resolution"). Phase 3/4 product commands will accept a URL
or a bare key and need a tested resolver underneath. Phase 2 builds that seam
before the product commands depend on it.

**Tech stack:** Go, Cobra, standard library only (`net/url`, `regexp`,
`os/exec` for `browse`). No new dependencies. `go test ./...` is the first gate.

---

## Source documents

Read before implementation:

1. `README.md`
2. `AGENTS.md`
3. `docs/command-contract.md` — current implemented surface
4. `docs/shared-architecture.md` — names `internal/resolve/` and the
   `resolve` / `browse` shared commands
5. `docs/jira-mvp.md`, `docs/confluence-mvp.md` — product command trees
6. `docs/implementation-plan.md` — Phase 2 scope
7. `docs/phase-2-resolve-browse-plan.md` — this file

## Scope

Phase 2 resolves the **canonical, stable** URL and key forms and is explicitly
best-effort: an input that no parser recognizes returns a structured
`unresolved` error, never a guess.

In scope:

- Jira: bare issue key (`PROJ-123`), bare project key (`PROJ`), and
  `<site>/browse/<KEY>` URLs (the stable canonical form; `/browse/PROJ-123`
  resolves to an issue, `/browse/PROJ` to a project). Also the new-nav
  `<site>/jira/.../projects/<KEY>` project URLs and a `selectedIssue=` /
  `/issues/<KEY-123>` issue hint where present.
- Confluence: `<site>/wiki/spaces/<SPACEKEY>/pages/<id>/<slug>` page URLs,
  `<site>/wiki/spaces/<SPACEKEY>[/overview]` space URLs, and a bare numeric
  page id.
- `resolve <input>` — render the structured resolution.
- `browse <input>` — resolve, build the canonical browser URL, and open it
  (or print it with `--no-browser`).

Non-goals for Phase 2:

- Confluence tiny links (`/wiki/x/<token>`) — they require an API round trip
  to expand; defer to a later phase.
- Data Center URL shapes — Cloud canonical forms only; document the limitation.
- Confluence blog-post and attachment URLs.
- Any network call: resolution is pure string parsing. `browse` only opens a
  local browser process.
- `--jq`, `--trace`, and secure token storage remain deferred foundation work.

## Quality bars

- `go test ./...` passes after each task that adds code.
- Resolution is pure and deterministic: no network, no clock, no environment
  reads inside the parsers.
- Every parser has table-driven tests covering each recognized form, plus
  negative cases that must stay unresolved.
- An unrecognized input yields a structured `apperr.Error` (code
  `unresolved`), consistent with the Phase 1 error model.
- `browse` never opens a browser in non-interactive contexts: `--no-browser`,
  and the global `--no-prompt`, both force print-only.
- Product command files stay thin; resolution logic lives in `internal/resolve`.

---

## Proposed package layout

```text
internal/resolve/resolve.go        # Resource, ResourceKind, Parser, dispatch
internal/resolve/resolve_test.go
internal/resolve/jira.go           # Jira parser + canonical URL builder
internal/resolve/jira_test.go
internal/resolve/confluence.go     # Confluence parser + canonical URL builder
internal/resolve/confluence_test.go
internal/browser/browser.go        # cross-platform "open this URL" helper
internal/browser/browser_test.go
internal/cli/resolve.go            # resolve command
internal/cli/resolve_test.go
internal/cli/browse.go             # browse command
internal/cli/browse_test.go
```

`resolve` and `browse` commands live in `internal/cli`, parameterized by
`appinfo.Info.Product`, exactly like `auth`/`api`/`version`. The product
command packages stay thin.

---

## Task 1: resolve core

**Objective:** Define the resolved-resource model and the parser dispatch.

**Files:** create `internal/resolve/resolve.go`, `internal/resolve/resolve_test.go`.

**Shape:**

```go
type ResourceKind string

const (
	KindJiraIssue       ResourceKind = "jira_issue"
	KindJiraProject     ResourceKind = "jira_project"
	KindConfluencePage  ResourceKind = "confluence_page"
	KindConfluenceSpace ResourceKind = "confluence_space"
)

// Resource is a resolved Atlassian resource. It is safe to render as JSON.
type Resource struct {
	Kind     ResourceKind `json:"kind"`
	Product  string       `json:"product"`
	Input    string       `json:"input"`
	SiteHost string       `json:"site_host,omitempty"` // host when input was a URL
	Key      string       `json:"key,omitempty"`       // issue/project/space key
	ID       string       `json:"id,omitempty"`        // numeric page id
	Title    string       `json:"title,omitempty"`     // URL slug, best-effort
}

// Parser resolves an input string for one product and builds canonical URLs.
type Parser interface {
	Parse(input string) (Resource, bool)
	CanonicalURL(baseURL string, r Resource) (string, error)
}
```

- `Resolve(product, input string) (Resource, error)` trims the input, selects
  the product parser, and returns a structured `apperr.New("unresolved", ...)`
  when no form matches.
- `parserFor(product string) (Parser, error)` returns the Jira or Confluence
  parser, or a structured error for an unknown product.

**Tests:** unknown product yields a structured error; an empty/whitespace
input yields `unresolved`; dispatch routes to the right parser kind.

**Verify:** `go test ./internal/resolve ./...`

**Commit:** `feat: add URL resolution core`

---

## Task 2: Jira parser

**Objective:** Resolve Jira issue/project keys and URLs, and build canonical
`/browse/<KEY>` URLs.

**Files:** create `internal/resolve/jira.go`, `internal/resolve/jira_test.go`.

**Recognized forms:**

- Bare issue key `^[A-Z][A-Z0-9]+-[0-9]+$` → `KindJiraIssue`.
- Bare project key `^[A-Z][A-Z0-9]+$` → `KindJiraProject`.
- URL path `/browse/<KEY-123>` → issue; `/browse/<KEY>` → project; capture the
  URL host into `SiteHost`.
- URL path `/jira/.../projects/<KEY>` → project; if the URL also carries
  `selectedIssue=<KEY-123>` or a `/issues/<KEY-123>` segment, prefer the issue.

**CanonicalURL:** `<baseURL>/browse/<KEY>` for both issue and project (Jira
redirects a project key to its project page). `baseURL` is the site root.

**Tests:** table-driven over every form above; negative cases — a lowercase
key, a key with no number, a random URL path, an empty string — must return
`false`. CanonicalURL round-trips a parsed key.

**Verify:** `go test ./internal/resolve ./...`

**Commit:** `feat: add Jira URL and key resolver`

---

## Task 3: Confluence parser

**Objective:** Resolve Confluence page/space URLs and bare page ids, and build
canonical URLs.

**Files:** create `internal/resolve/confluence.go`, `internal/resolve/confluence_test.go`.

**Recognized forms:**

- URL path `/wiki/spaces/<SPACEKEY>/pages/<id>/<slug>` → `KindConfluencePage`
  (capture `ID`, `Key`=space key, `Title`=slug, `SiteHost`).
- URL path `/wiki/spaces/<SPACEKEY>` or `.../spaces/<SPACEKEY>/overview` →
  `KindConfluenceSpace`.
- Bare numeric `^[0-9]+$` → `KindConfluencePage` (page id).

**CanonicalURL:**

- Page: `<baseURL>/wiki/spaces/<spaceKey>/pages/<id>` when the space key is
  known, otherwise `<baseURL>/wiki/pages/viewpage.action?pageId=<id>`.
- Space: `<baseURL>/wiki/spaces/<spaceKey>`.
- `baseURL` may or may not already include the `/wiki` segment; normalize the
  same way `httpclient.APIBase` does so both inputs work.

**Tests:** table-driven over each form; negative cases — a Jira-style key, a
non-numeric token, an unrelated URL — stay unresolved. CanonicalURL handles a
base both with and without `/wiki`.

**Verify:** `go test ./internal/resolve ./...`

**Commit:** `feat: add Confluence URL and id resolver`

---

## Checkpoint

After Tasks 1-3 the `internal/resolve` package is complete and fully unit
tested with no command wiring. Stop and review the `Resource` shape and parser
behavior before building the commands, so the command layer is not designed
against the wrong model.

---

## Task 4: resolve command

**Objective:** Add `resolve <input>` to both binaries.

**Files:** create `internal/cli/resolve.go`, `internal/cli/resolve_test.go`;
register the command in `internal/cli/root.go`.

**Shape:**

```
atl-jira resolve <url-or-key> [--json]
atl-conf resolve <url-or-id> [--json]
```

- `cobra.ExactArgs(1)`; product comes from `appinfo.Info.Product`.
- Calls `resolve.Resolve(product, input)` and renders the `Resource` via the
  shared `render` helper.
- An unresolved input returns the structured `unresolved` error.

**Tests:** Jira issue/project and Confluence page/space inputs resolve and
render; `--json` produces a valid `Resource` envelope; an unrecognized input
returns an `*apperr.Error`.

**Verify:** `go test ./...`

**Commit:** `feat: add resolve command`

---

## Task 5: browse command

**Objective:** Add `browse <input>` plus a cross-platform browser-open helper.

**Files:** create `internal/browser/browser.go`, `internal/browser/browser_test.go`,
`internal/cli/browse.go`, `internal/cli/browse_test.go`; register in `root.go`.

**`internal/browser`:** `Open(url string) error` selects the platform opener
(`open` on darwin, `xdg-open` on linux, `rundll32 url.dll,FileProtocolHandler`
on windows) via `os/exec`. Keep it injectable for tests (a package-level
`opener` func variable, or an `OpenWith(runner, url)` seam) so tests never
spawn a real browser.

**Shape:**

```
atl-jira browse <url-or-key> [--site <name>] [--no-browser]
```

- Resolve the input. If it was a full URL, normalize it via the parser's
  `CanonicalURL`; if it was a bare key/id, look up the `--site` profile's
  `base_url` from config to build the canonical URL.
- A bare key/id without `--site` returns a structured error explaining that a
  site is required to build the URL.
- Default: open the URL in a browser. With `--no-browser` or the global
  `--no-prompt`: print the URL to stdout and do not open anything.

**Tests:** bare key + `--site` builds the expected URL; full-URL input
normalizes; `--no-browser` prints and does not invoke the opener (assert via
the injected opener); missing `--site` for a bare key returns a structured
error; `--no-prompt` implies `--no-browser`.

**Verify:** `go test ./...`

**Commit:** `feat: add browse command`

---

## Task 6: documentation

**Objective:** Keep the docs in sync with the Phase 2 surface.

**Files:** update `docs/command-contract.md`, `docs/continuation-handoff.md`,
`docs/README.md`, and `README.md`.

**Document:** the `resolve` and `browse` commands; recognized URL/key forms
and the explicit unresolved behavior; the `browse` open-vs-print rules; and
the known limitations (no tiny links, no Data Center URL shapes).

**Verify:** `go test ./...`, `go vet ./...`, `gofmt -l`, `git diff --check`.

**Commit:** `docs: document Phase 2 resolve and browse`

---

## Phase 2 done definition

- `go run ./cmd/atl-jira resolve PROJ-123 --json` resolves an issue.
- `go run ./cmd/atl-conf resolve <page-url> --json` resolves a page.
- `browse <key> --site <name> --no-browser` prints the canonical URL.
- Unresolved input returns a structured error.
- Parsers are pure, deterministic, and table-tested.
- `go test ./...`, `go vet ./...`, and `gofmt` are clean.
- Docs list the new commands and known limitations.
