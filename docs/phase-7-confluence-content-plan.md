# Phase 7 — Confluence Content Depth: Implementation Plan

> Detailed task breakdown for Phase 7 of `docs/post-mvp-roadmap.md`. Phases 1–6
> are merged to `main`. This phase extends `atl-conf` beyond pages and spaces
> to the content that lives on a page: comments, labels, and attachments.

## Goal

Add `page comment`, `page label`, and `attachment` commands to `atl-conf` so
the CLI covers the everyday content operations, not just page bodies.

## Resolved design decisions

These were open in the roadmap and are now settled:

1. **Comment scope — footer comments only.** Confluence v2 splits page
   comments into footer comments (the normal "comment on a page") and inline
   comments (anchored to a text selection). `page comment` addresses footer
   comments exclusively: inline-comment creation needs `inlineCommentProperties`
   (the selected text and surrounding markup), which cannot be expressed
   well on a CLI. Inline comments are out of scope.
2. **Attachment download — explicit `--out` required.** `attachment download`
   never writes a file implicitly. It requires `--out <path>` (or `--out -`
   to stream to stdout). This keeps the project's first binary-response
   command explicit and predictable.
3. **Label writes use the v1 surface.** Confluence v2 lists page labels
   (`GET /pages/{id}/labels`) but has no page-label write endpoint; label add
   and remove use the v1 `content/{id}/label` endpoints, the same
   v2-primary / v1-fallback pattern Phase 4 established for CQL search.

## Command surface

```text
atl-conf page comment list <page-id> [--limit N] [--all]
atl-conf page comment view <comment-id>
atl-conf page comment create <page-id> --body <text> --body-format <fmt>
atl-conf page comment edit <comment-id> --body <text> --body-format <fmt>
atl-conf page comment delete <comment-id>
atl-conf page label list <page-id> [--limit N] [--all]
atl-conf page label add <page-id> <label>
atl-conf page label remove <page-id> <label>
atl-conf attachment list <page-id> [--limit N] [--all]
atl-conf attachment download <attachment-id> --out <path>
```

`page comment` and `page label` are sub-groups under the existing `page`
command; `attachment` is a new top-level command group. Every new list command
gets `--all` (the Phase 5B follow-all-pages flag) for consistency, since the
roadmap intends new commands to inherit `--jq`/`--all` for free.

## API notes

- **Footer comments (v2):** `GET /pages/{id}/footer-comments` (cursor
  pagination via `_links.next`), `GET /footer-comments/{id}`,
  `POST /footer-comments` (body `{pageId, body:{representation,value}}`),
  `PUT /footer-comments/{id}` (a full replacement: body plus
  `version:{number}` = current + 1, like a page edit),
  `DELETE /footer-comments/{id}`.
- **Labels:** `GET /pages/{id}/labels` (v2, cursor pagination). Add via v1
  `POST /rest/api/content/{id}/label` with `[{"name":"<label>"}]`; remove via
  v1 `DELETE /rest/api/content/{id}/label/{name}`.
- **Attachments (v2):** `GET /pages/{id}/attachments` (cursor pagination),
  `GET /attachments/{id}` (metadata, including `downloadLink`). The binary is
  fetched from `downloadLink`, which is rooted at the Confluence context path
  (`/wiki` on Cloud), not the v2 API base — so the client resolves it against
  the context base (the API base with the `/api/v2` suffix removed), keeping
  any context-path segment. The response body is buffered as bytes and written
  to `--out`, consistent with every other response in the client.

## Tasks

### Task 1 — page comments

Add the footer-comment models (`Comment`, `CommentList`, reusing `PageBody`/
`PageVersion`) and client methods (`ListFooterComments` + `*All`,
`GetFooterComment`, `CreateFooterComment`, `UpdateFooterComment`,
`DeleteFooterComment`) to `internal/conf`. Add `internal/confcmd/comment.go`
with the `page comment` sub-group: list/view/create/edit/delete. `edit` is a
full replacement — GET the comment, merge the new body, PUT version + 1, the
same shape as `page edit`. Tests for each command and client method.
Commit: `feat: add atl-conf page comment commands`.

### Task 2 — page labels

Add `Label`/`LabelList` models and client methods (`ListLabels` + `*All` via
v2; `AddLabel`/`RemoveLabel` via v1). Add `internal/confcmd/label.go` with the
`page label` sub-group: list/add/remove. Tests.
Commit: `feat: add atl-conf page label commands`.

### Task 3 — attachments

Add `Attachment`/`AttachmentList` models and client methods
(`ListAttachments` + `*All`, `GetAttachment`, `FetchAttachmentData`, and the
`downloadLink` resolution helper). Add `internal/confcmd/attachment.go` with
the `attachment` group: list and download. `download` requires `--out`;
`--out -` streams to stdout; under `--json` it prints the attachment metadata
rather than the binary. Tests, including a binary round trip via `httptest`.
Commit: `feat: add atl-conf attachment commands`.

### Task 4 — docs, review, PR

Update `docs/command-contract.md` (the new commands and the binary-response
behavior), `docs/confluence-mvp.md` if it tracks the surface, `README.md`,
`docs/README.md`, and `docs/continuation-handoff.md`. Run the multi-agent
review wave until clean.
Commit: `docs: document Confluence content commands`. Open PR.

## Done definition

- Page footer comments can be listed, viewed, created, edited, and deleted.
- Labels can be listed, added, and removed.
- Attachments can be listed and downloaded to a file or stdout.
- `command-contract.md` documents the new commands and the binary-response
  behavior.

## Out of scope

- Inline comments (creation needs text-anchor properties unsuited to a CLI).
- Attachment upload (a multipart write; a later phase if wanted).
- True streaming download — the response body is buffered like every other
  response; large-file streaming would need an `internal/httpclient` change.
