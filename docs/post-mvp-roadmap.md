# Post-MVP Roadmap (Phases 5–8)

> Sequenced plan for the work after the Jira and Confluence MVPs. Phases 1–4
> are merged to `main`: the shared foundation, URL resolution, the Jira MVP
> (`project`/`issue`/`search`/`status` plus issue and comment mutations), and
> the Confluence MVP (`space`/`page`/`search cql`/`status` plus page writes).
> This document turns the loose "Phase 5: Broader coverage" sketch in
> `implementation-plan.md` into four concrete, ordered phases.

## Goal

Take both CLIs from "MVP that works" to "tool you would actually live in":
finish the deferred output and auth foundation work, then deepen each
product's command surface. Every phase keeps the project posture — true to the
official API, deterministic, structured output for agents, stdlib-only beyond
Cobra unless a phase explicitly decides otherwise.

## Phase sequencing

| Phase | Theme | Why this order |
|-------|-------|----------------|
| 5 | Output & pagination polish (`--jq`, `--all`) | Cross-cutting: every existing and future command benefits, and it clears the most visible documented stub. |
| 6 | Secure token storage | Finishes the auth foundation before more product surface is layered on; self-contained. |
| 7 | Confluence content depth (comments, labels, attachments) | Product depth; the Confluence client is freshest. |
| 8 | Deeper Jira coverage (assign, watchers, links, worklog) | Product depth; independent of Phase 7 and reorderable with it. |

Phases 5 and 6 are foundation work and should land first and in order.
Phases 7 and 8 are independent product-depth tracks and may be reordered or
interleaved. The monorepo / Bitbucket-migration review (the old "Phase 6" in
`implementation-plan.md`) moves out to **Phase 9** and is unchanged.

Each phase, like Phases 1–4, gets its own detailed `phase-N-*-plan.md` with a
task-by-task breakdown when it begins. This roadmap is the sequencing layer
and the per-phase scope contract.

## Cross-cutting rules (all phases)

- `go test ./...`, `go vet ./...`, and `gofmt` pass after every task.
- New API calls go through `internal/httpclient`; tests use
  `net/http/httptest` and never touch the network.
- Non-2xx responses surface as the existing structured `apperr.Error` values.
- `--json` emits the raw API response body verbatim; human output is a
  compact per-command summary.
- Every command ships with tests for success, the structured-error path, and
  argument validation.
- Multi-agent review wave on each phase branch until no findings remain, then
  one PR per phase.
- Docs (`command-contract.md`, `README.md`, `docs/README.md`,
  `continuation-handoff.md`) updated as part of each phase.

---

## Phase 5 — Output & pagination polish

**Goal:** Make `--jq` real and add an `--all` follow-all-pages flag, so both
CLIs are genuinely composable for agents and scripts.

**Why now:** `--jq` is currently a documented stub (`output.ErrJQNotImplemented`)
and `--all` has been deferred since Phase 3. Both are cross-cutting: doing them
before Phases 7–8 means every new command inherits them for free.

### Scope

- **`--jq` filtering.** Apply a jq-style expression to the raw JSON response
  before output. Replaces the stub in `internal/output`.
- **`--all` pagination.** A boolean flag on every list/search command
  (`project list`, `issue list`, `search issues`, `space list`, `page list`,
  `page children`, `search cql`) that follows pagination to completion and
  emits the aggregated result, bounded by a safety cap.

### Key design decisions (resolve at phase start)

1. **jq engine.** stdlib has no jq. Options: (a) implement a minimal,
   vendored path-filter subset (`.a.b`, `.a[]`, `.a[0]`, `|`), no dependency;
   (b) add a real jq library (`github.com/itchyny/gojq`) and relax the
   stdlib-only rule. Recommendation: start with (a) — a documented subset
   covers the agent use cases (field extraction, array iteration) without a
   dependency; revisit (b) only if the subset proves too thin.
2. **`--jq` vs `--json` field selector.** `--json` already takes an optional
   value; Phase 5 must define cleanly how `--json <selector>` and
   `--jq <expr>` relate (or fold one into the other) and document it.
3. **`--all` aggregation shape.** Jira (`startAt`/`isLast`/`nextPageToken`)
   and Confluence v2 (cursor in the `Link` header / `_links.next`) paginate
   differently. Decide the merged-output JSON shape and a hard page cap so a
   runaway query cannot loop forever.

### Task outline

1. Define and implement the jq subset in `internal/output`; replace the stub.
2. Wire `--jq` end-to-end through `cli.Render`; remove `ErrJQNotImplemented`.
3. Add cursor-following helpers to `internal/jira` and `internal/conf`.
4. Add the `--all` flag to every list/search command; aggregate and cap.
5. Tests; docs; review wave; PR.

### Done definition

- `--jq` evaluates the documented expression subset against any command's
  JSON output; no command reports jq as unimplemented.
- `--all` returns every page of a multi-page result, with a documented cap.
- `command-contract.md` documents the jq subset and `--all` semantics.

---

## Phase 6 — Secure token storage

**Goal:** Let users store a credential securely instead of requiring
`--token-env` for every invocation.

**Why now:** `--token-env` is the only credential path today (by deliberate
Phase 1 design). With both MVPs done, finishing the auth foundation before
adding more product surface keeps the security model coherent.

### Scope

- `auth login` can accept and store a token (interactive prompt or `--token`),
  not just record an env-var reference.
- Stored credentials live in an OS keychain where available, never in
  plaintext config.
- `auth status` / `auth logout` understand stored credentials.
- `--token-env` remains fully supported and is the documented fallback for
  headless/CI environments.

### Key design decisions (resolve at phase start)

1. **Keychain backend.** stdlib has no keychain. Options: (a) shell out to
   per-OS tools (`security` on macOS, `secret-tool`/libsecret on Linux,
   DPAPI/`cmdkey` on Windows) — dependency-free but per-OS code and brittle;
   (b) add `github.com/zalando/go-keyring`. Decide explicitly, consistent with
   the stdlib-only posture.
2. **Headless fallback.** When no keychain is available (CI, containers),
   define the behavior: refuse to store, or fall back to a
   restricted-permission file with a clear warning. `--token-env` must keep
   working regardless.
3. **No secrets in the repo.** The existing guardrail holds: no real tokens in
   tests, fixtures, docs, or committed config. Keychain interaction is tested
   behind an interface with a fake backend.

### Task outline

1. Define a `CredentialStore` interface in `internal/auth` (or a new
   `internal/secrets`) with a keychain backend and a fake for tests.
2. Implement the chosen OS backend(s).
3. Extend `auth login/status/logout` to use the store; keep `--token-env`.
4. Tests against the fake backend; docs (update `auth-design.md`); review; PR.

### Done definition

- A user can `auth login`, store a token securely, and run commands without
  re-supplying it.
- No plaintext token is ever written to config or the repo.
- `--token-env` still works as the headless/CI path.
- `auth-design.md` and `command-contract.md` document the storage model.

---

## Phase 7 — Confluence content depth

**Goal:** Extend `atl-conf` beyond pages and spaces to the content that lives
on a page: comments, labels, and attachments.

### Scope

```text
atl-conf page comment list <page-id> [--limit N]
atl-conf page comment view <comment-id>
atl-conf page comment create <page-id> --body <text> --body-format <fmt>
atl-conf page comment edit <comment-id> --body <text> --body-format <fmt>
atl-conf page comment delete <comment-id>
atl-conf page label list <page-id>
atl-conf page label add <page-id> <label>
atl-conf page label remove <page-id> <label>
atl-conf attachment list <page-id> [--limit N]
atl-conf attachment download <attachment-id> [--out <path>]
```

### Key design decisions (resolve at phase start)

1. **Footer vs inline comments.** Confluence v2 splits page comments into
   footer comments and inline comments. Decide whether `page comment`
   addresses footer comments only (simplest, the common case) or both via a
   flag.
2. **v1 fallback for labels.** Confirm which label operations have a v2
   endpoint; label *writes* may still require the v1 surface, the same
   v2-primary/v1-fallback pattern Phase 4 established.
3. **Attachment download.** This is the first non-JSON response in the
   project: a binary body written to a file. Define the streaming write,
   `--out` default (the API filename), and how `--json` behaves (metadata
   only, not the binary).

### Task outline

1. Comment read/write methods on `internal/conf`; `page comment` commands.
2. Label methods; `page label` commands.
3. Attachment list + download; the binary-response path in `internal/conf`.
4. Tests; docs; review wave; PR.

### Done definition

- Page comments can be listed, viewed, created, edited, and deleted.
- Labels can be listed, added, and removed.
- Attachments can be listed and downloaded to disk.

---

## Phase 8 — Deeper Jira coverage

**Goal:** Extend `atl-jira` issue operations beyond create/edit/transition to
the day-to-day fields and relationships.

### Scope

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

### Key design decisions (resolve at phase start)

1. **Assignee semantics.** `PUT /issue/{key}/assignee` takes `accountId`;
   define how `-` (unassign) and a default-assignee request map to the API.
2. **Link direction.** `POST /issueLink` is directional (inward/outward issue
   plus a link type). Decide the argument order and how `issue link types`
   surfaces the valid type names.
3. **Worklog time format.** Jira accepts a duration string (`3h 30m`) or
   seconds. Pass the duration through verbatim and let the API validate,
   consistent with the project's no-magic-conversion stance.

### Task outline

1. Assignee + watcher methods on `internal/jira`; `issue assign`/`watch`/
   `unwatch`/`watchers` commands.
2. Issue-link methods; `issue link` and `issue link types`.
3. Worklog methods; `issue worklog list`/`add`.
4. Tests; docs; review wave; PR.

### Done definition

- An issue can be assigned/unassigned and watched/unwatched.
- Issue links can be created and the available link types listed.
- Worklogs can be listed and added.

---

## After Phase 8

`implementation-plan.md` Phase 9 (formerly Phase 6) — the monorepo / shared-
foundation review and the Bitbucket `atl-bb` migration question — remains the
next milestone, gated on `bitbucket-migration-roadmap.md`. OAuth 3LO is still
deferred until token-based auth (Phases 1 and 6) is proven robust.
