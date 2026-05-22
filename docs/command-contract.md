# Command Contract

> Implemented behavior as of Phase 6. This document describes what the
> `atl-jira` and `atl-conf` binaries actually do today, not the long-term
> design. Update it whenever the command surface changes.

## Scope

Phase 1 delivers the shared foundation: two binaries, global flags, config,
structured output and errors, auth signing, an HTTP client, the `auth` command
subtree, and the raw `api` escape hatch. Phase 2 adds offline URL/key
resolution: the `resolve` and `browse` commands. Phase 3A adds the read-only
Jira product commands: `project`, `issue` (view/list), `issue comment`
(list/view), `search issues`, and `status`. Phase 3B adds the Jira mutating
commands: `issue` (create/edit/transition) and `issue comment`
(create/edit/delete). Phase 4A adds the read-only Confluence product commands:
`space` (list/view), `page` (list/view/children), `search cql`, and `status`.
Phase 4B adds the Confluence mutating commands: `page` (create/edit). Phase 5A
makes the global `--jq` flag a real jq filter; Phase 5B adds the `--all`
follow-all-pages flag to the list and search commands. Phase 6 adds secure
token storage: `auth login` can store a token in the OS keychain (or a `0600`
fallback file), so commands no longer require `--token-env` on every run.
Phase 7 deepens the Confluence surface with `page comment`, `page label`,
and `attachment` commands. Phase 8 deepens the Jira `issue` surface with
`assign`, `watch`/`unwatch`/`watchers`, `link`/`link types`, and `worklog`
(list/add).

## Binaries

- `atl-jira` — Jira CLI (`product: jira`)
- `atl-conf` — Confluence CLI (`product: confluence`)
- `atl-bb` — Bitbucket Cloud CLI (`product: bitbucket`) — under construction
  (Phase B3b); `repo`, `pr`, `pipeline`, `issue`, `workspace`, `project`, `commit`, `branch`, `tag`, `deployment`, `environment`, `search`, and `status` are the command groups shipped so far.

All binaries share one command tree built in `internal/cli`; only product
identity and build metadata differ.

## Global flags

| Flag | Type | Behavior |
|---|---|---|
| `--json` | string | JSON output. Bare `--json` renders all fields; `--json=field1,field2` selects top-level fields. |
| `--jq` | string | Filter the JSON output through a jq expression. |
| `--site` | string | Names the configured site profile a command targets. |
| `--no-prompt` | bool | Forces non-interactive behavior. `browse` treats it as `--no-browser` (print the URL, never open one). No other command prompts yet. |
| `--trace` | bool | Accepted and reserved. Request tracing is not implemented yet. |

`--json` takes an *optional* value, so a value must be attached with `=`
(`--json=field1,field2`) — passing it space-separated (`--json field1`) leaves
`field1` as a stray argument. Bare `--json` already means "all fields"; the
explicit all-fields form is `--json=*`, which must be quoted in shells that
expand globs (`--json='*'` in zsh/bash).

`--jq` runs a full [jq](https://jqlang.github.io/jq/) expression — via the
embedded `gojq` engine — against the JSON value a command would emit, and
prints each result as compact JSON on its own line:

```
atl-jira issue view ABC-1 --jq '.fields.status.name'
atl-conf search cql 'type = page' --jq '.results[].content.title'
atl-jira issue list --project DEV \
  --jq '.issues[] | select(.fields.status.name=="Done") | .key'
```

A malformed expression, or one that fails against the data, returns a
structured `invalid_input` error. `--jq` cannot be combined with a `--json`
field list (the two are different projections of the same data); bare `--json`
with `--jq` is allowed and equivalent to `--jq` alone.

## Pagination — `--limit` and `--all`

The list and search commands — `project list`, `issue list`, `search issues`,
`issue comment list`, `issue worklog list` (Jira) and `space list`,
`page list`, `page children`, `page comment list`, `page label list`,
`attachment list`, `search cql` (Confluence) — fetch a single page by
default, sized by `--limit`.

Passing `--all` follows pagination to completion and emits one aggregated
result; `--limit` then sets the per-request page size rather than a total cap.
Because a multi-page result has no single API response body, `--all` output is
a *synthesized* object — the top-level list key (`values`/`issues`/`comments`/
`results`) holding every item, with per-page cursors dropped — not a verbatim
API body. Each item is kept verbatim, so no per-item field is lost. `--jq`
runs against the synthesized aggregate. Following is bounded by a 100-page
safety cap; reaching it returns what was collected so far.

## Commands

### `version`

```
atl-jira version [--json]
```

Prints binary, product, and version. With `--json`, emits the `appinfo.Info`
object (`binary`, `product`, `version`, optional `commit`, `date`).

### `auth login`

```
atl-jira auth login --site <name> --url <url> --token-style <style> \
  [--username <email>] [--cloud-id <id>] \
  [--token-env <ENV_VAR> | --token-stdin | --token <value>]
```

Records a site profile under `--site`. Required: `--site`, `--url`,
`--token-style`. `--username` is required for `cloud-classic` and
`cloud-scoped`; `--cloud-id` is required for `cloud-scoped`.

`--url` must be an `http`/`https` URL with a host and no embedded credentials.
Cloud token styles (`cloud-classic`, `cloud-scoped`) require `https`;
`data-center-pat` also accepts `http` for internal instances.

The token is supplied by at most one of three mutually exclusive flags:

- `--token-env NAME` records the reference `env:NAME`; nothing is stored and
  the token value is read from that environment variable at request time.
  This is the headless/CI path.
- `--token-stdin` reads the token from standard input and stores it securely.
  Preferred for interactive use — the token never reaches the shell history.
- `--token <value>` stores the supplied value securely. Convenient for
  scripts, but the value is visible in the shell history and process list.

A stored token goes to the OS keychain (token reference `keyring`). When no
keychain is available — CI, containers, minimal Linux — it falls back to a
`0600` `credentials.json` beside `config.json` (token reference `file`) and
`auth login` prints a warning that the token is not keychain-protected.
`config.json` itself never holds a raw token; it records only the indirect
`token_ref`.

### `auth status`

```
atl-jira auth status [--site <name>] [--json]
```

With `--site`, shows that profile; without it, lists every configured profile.
Output reports `token_status` — whether the referenced token is currently
resolvable (from the environment, the OS keychain, or the fallback file) —
but never the token value itself.

### `auth logout`

```
atl-jira auth logout --site <name>
```

Removes exactly the named profile and deletes any token stored for it in the
OS keychain or the fallback file. A `--token-env` reference has nothing stored
to delete. Errors if the site is not configured.

### `api`

```
atl-jira api <path-or-url> --site <name> [-X <method>] [--data <body>]
```

Sends a signed request to the `--site` profile and renders the response.
`--method`/`-X` defaults to `GET`. With `--json` the full response body is
rendered; `--json=field1,field2` selects top-level fields.

### `resolve`

```
atl-jira resolve <url-or-key> [--json]
atl-conf resolve <url-or-id> [--json]
```

Parses an Atlassian URL or a bare key/id into a structured resource. Resolution
is **offline string parsing** — no network request is made. Human output is a
short label/value summary; `--json` emits the full resource object (`kind`,
`product`, `input`, and the populated subset of `site_host`, `key`, `id`,
`title`). An input matching no known form fails with a structured `unresolved`
error.

Recognized forms:

| Product | Input | Resolves to |
|---|---|---|
| Jira | `PROJ-123` | issue |
| Jira | `PROJ` | project |
| Jira | `<site>/browse/PROJ-123` or `/browse/PROJ` | issue / project |
| Jira | `<site>/jira/.../projects/PROJ` | project (an issue when a `selectedIssue=` or `/issues/PROJ-123` hint is present) |
| Confluence | `123456` | page (by id) |
| Confluence | `<site>/wiki/spaces/KEY/pages/<id>/<slug>` | page |
| Confluence | `<site>/wiki/spaces/KEY[/overview]` | space |

Each binary resolves only its own product's forms: `atl-jira resolve` rejects a
Confluence URL, and vice versa.

### `browse`

```
atl-jira browse <url-or-key> [--site <name>] [--no-browser]
```

Resolves the input, builds the canonical browser URL, and opens it in the
default browser. A full URL carries its own host; a bare key/id needs `--site`
to supply the site root — a bare key without `--site` is a structured error.
Canonical URLs are the stable `<site>/browse/<KEY>` (Jira) and
`<site>/wiki/spaces/<KEY>/pages/<id>` (Confluence) forms; a bare Confluence page
id with no known space resolves to `<site>/wiki/pages/viewpage.action?pageId=<id>`.

With `--no-browser`, or the global `--no-prompt`, the URL is printed to stdout
instead of opened — keeping the command safe in non-interactive and agent
contexts. Under `--json` it is emitted as a `{"url": "..."}` object. The
browser is launched through the platform handler (`open`, `xdg-open`, or
`rundll32`); only `http`/`https` URLs are ever passed to it.

## Jira commands

These commands exist only on `atl-jira`. Each needs `--site` to name a
configured profile and makes a live, authenticated request. Under `--json` the
raw Jira REST v3 response body is emitted verbatim; human output is a compact
per-command summary. A mutating command whose API call returns no body (e.g.
`issue edit`) instead emits a small synthesized result object under `--json`.
A non-2xx response is mapped to the structured error model below.

### `project`

```
atl-jira project list [--limit N]
atl-jira project view <key>
```

`list` returns projects visible to the authenticated account; `view` returns a
single project by id or key.

### `issue`

```
atl-jira issue view <issue>
atl-jira issue list --project <key> [--status <name>] [--assignee <id>] [--limit N]
atl-jira issue create --project <key> --type <name> --summary <text> [--description <text>] [--assignee <id>] [--field name=value]...
atl-jira issue edit <issue> [--summary <text>] [--description <text>] [--assignee <id>] [--field name=value]...
atl-jira issue transition <issue> [--to <name-or-id>]
atl-jira issue assign <issue> <account-id|->
atl-jira issue watch <issue>
atl-jira issue unwatch <issue>
atl-jira issue watchers <issue>
atl-jira issue link <inward> <outward> --type <link-type>
atl-jira issue link types
atl-jira issue worklog list <issue> [--limit N]
atl-jira issue worklog add <issue> --time <duration> [--comment <text>]
```

`view` returns one issue. `list` builds a JQL query from its flags — `--project`
is required, `--status` and `--assignee` are optional filters, and results are
ordered newest-first by creation date. `--assignee` takes an account id or the
literal `currentUser()`. Broader queries go through `search issues`.

`create` and `edit` set fields from typed flags plus a repeatable `--field
name=value` escape for any other field; a `--field` value is sent as parsed
JSON when it is valid JSON, otherwise as a string, and a plain `--description`
is wrapped as an ADF document. `edit` requires at least one change. `create`
reports the new key; `edit` reports a confirmation.

`transition` with no `--to` lists the transitions available from the issue's
current status; with `--to <name-or-id>` it resolves the target against that
list (by id, or case-insensitive name) and applies it. There is no universal
close/reopen abstraction — Jira transitions are workflow specific.

`assign` sets the issue's assignee to the given account id; passing the
literal `-` sends `accountId: null` and unassigns the issue. Setting a
project's default assignee from the CLI is intentionally out of scope.

`watch` adds the authenticated account to the issue's watchers; `unwatch`
removes it (the API requires an explicit account id, so `unwatch` first
looks up the caller via `/myself`). `watchers` lists the issue's watchers.

`link` creates a directional link between two issues. The first positional
is the inward issue and the second is the outward issue, matching the Jira
API field names: with `--type Blocks`, `issue link A B --type Blocks` means
A is blocked by B and B blocks A. `issue link types` lists the link types
configured on the site with their inward and outward phrases.

`worklog list` returns the worklog entries on an issue; pagination is
controlled by `--limit` and `--all`. `worklog add` appends a new entry:
`--time` is passed through verbatim as Jira's `timeSpent` (duration strings
like `3h 30m` or seconds with units; the CLI does not parse or convert),
and `--comment` is plain text wrapped as an ADF document. Editing or
deleting an existing worklog is intentionally out of scope.

### `search issues`

```
atl-jira search issues <jql> [--limit N]
```

Runs a raw JQL query. JQL is the stable, expressive query surface; `issue list`
is a convenience wrapper over the same endpoint.

### `issue comment`

```
atl-jira issue comment list <issue> [--limit N]
atl-jira issue comment view <issue> <comment-id>
atl-jira issue comment create <issue> --body <text>
atl-jira issue comment edit <issue> <comment-id> --body <text>
atl-jira issue comment delete <issue> <comment-id>
```

Lists, views, and manages comments on an issue. Comment bodies are stored as
Atlassian Document Format; human output renders a best-effort plain-text
extraction, while `--json` preserves the raw ADF body. `create` and `edit`
take a plain-text `--body` that is wrapped as an ADF document.

### `status`

```
atl-jira status
```

A live authentication check: it calls `/myself` with the `--site` credential
and reports the authenticated account. This is distinct from `auth status`,
which inspects local config offline and makes no request.

## Confluence commands

These commands exist only on `atl-conf`. Each needs `--site` to name a
configured profile and makes a live, authenticated request. They target the
Confluence Cloud REST **v2** API; CQL search and the current-user lookup have
no v2 resource, so `search cql` and `status` fall back to REST **v1**. Under
`--json` the raw response body is emitted verbatim; human output is a compact
per-command summary. A non-2xx response is mapped to the structured error
model below.

### `space`

```
atl-conf space list [--limit N]
atl-conf space view <key>
```

`list` returns spaces visible to the authenticated account. `view` takes a
space key; since v2 addresses a space by numeric id, the key is resolved to an
id first.

### `page`

```
atl-conf page list --space <key> [--limit N]
atl-conf page view <id>
atl-conf page children <id> [--limit N]
atl-conf page create --space <key> --title <text> --body <text> --body-format <fmt>
atl-conf page edit <id> [--title <text>] [--body <text> --body-format <fmt>]
```

`list` returns the pages in a space — `--space` is required and is resolved
from key to id. `view` returns one page by id, including its storage-format
body under `--json`. `children` lists a page's direct child pages.

`create` makes a new page: `--space` (resolved key to id), `--title`, `--body`,
and `--body-format` are all required. `edit` updates a page by id and needs at
least one of `--title` or `--body`. `--body-format` must be one of `storage`,
`atlas_doc_format`, or `wiki`; the body is sent verbatim under that
representation and is never converted. `--body` and `--body-format` are passed
together.

Confluence v2 treats a page update as a full replacement, so `edit` first GETs
the page to read its current title, body, status, and version, then PUTs the
merged state with the version number incremented by one. A title-only edit
re-sends the page's current body in storage representation; if the page has no
storage-format body to re-send it is refused with an `invalid_input` error
rather than risk clearing the body — pass `--body` with `--body-format` to set
the body explicitly. A version conflict surfaces as the structured error model
below.

### `page comment`

```
atl-conf page comment list <page-id> [--limit N]
atl-conf page comment view <comment-id>
atl-conf page comment create <page-id> --body <text> --body-format <fmt>
atl-conf page comment edit <comment-id> --body <text> --body-format <fmt>
atl-conf page comment delete <comment-id>
```

Operates on a page's **footer** comments. `list` returns the footer comments
on a page. `view` returns one comment by id with its storage-format body.
`create` adds a footer comment; `--body` and `--body-format` are required.
`edit` replaces a comment's body — like `page edit`, Confluence v2 treats the
update as a full replacement, so `edit` first GETs the comment for its version
and PUTs the body with the version incremented by one. `delete` removes a
comment. `--body-format` is one of `storage`, `atlas_doc_format`, or `wiki`,
and the body is sent verbatim. Inline comments are out of scope.

### `page label`

```
atl-conf page label list <page-id> [--limit N]
atl-conf page label add <page-id> <label>
atl-conf page label remove <page-id> <label>
```

`list` returns a page's content labels. `add` attaches a label and `remove`
detaches one. Confluence v2 has no page-label write endpoint, so `add` and
`remove` use the REST **v1** content-label surface.

### `attachment`

```
atl-conf attachment list <page-id> [--limit N]
atl-conf attachment download <attachment-id> --out <path>
```

`list` returns a page's attachments. `download` fetches an attachment's binary
content: `--out` is required and names the destination file, or `--out -`
streams the bytes to stdout. The v2 `downloadLink` is rooted at the Confluence
context path rather than the API base, so it is resolved against the API base
with the trailing `/api/v2` segment removed. Under `--json` or `--jq` the
attachment metadata is printed and no binary is fetched. The response body is
buffered in full like every other response — there is no streaming download.

### `search cql`

```
atl-conf search cql <cql> [--limit N]
```

Runs a raw CQL query against the v1 search endpoint. CQL is the stable,
expressive query surface for Confluence content.

### `status`

```
atl-conf status
```

A live authentication check: it calls the v1 current-user endpoint with the
`--site` credential and reports the authenticated account and resolved API
base. Distinct from `auth status`, which inspects local config offline.

## Bitbucket commands

These commands exist only on `atl-bb` (`product: bitbucket`). Each needs
`--site` to name a configured Cloud credential. Bitbucket Cloud uses Basic
auth (account email + API token, the `cloud-classic` style) against the fixed
`https://api.bitbucket.org/2.0` base.

> **JSON shape — intentional change from legacy `bb`.** Under `--json`/`--jq`,
> `atl-bb` emits the **verbatim Bitbucket REST API body** (e.g. `full_name`,
> `is_private`, `project.key`, `mainbranch.name`), exactly like `atl-jira` and
> `atl-conf`. Legacy `bb` emitted hand-built payloads with renamed fields
> (`private`, `project_key`, `main_branch`, `https_clone`, …); those names are
> **not** preserved. This is a documented break, alongside the structured
> `apperr` error output and the `api` same-origin guard.

### Repository targeting (`--repo` / `--workspace`)

Repo-scoped commands identify a repository as `<workspace>/<repo>`, supplied as
a positional argument or via `--repo`. A bare `<repo>` is allowed when paired
with `--workspace`. A positional argument wins over `--repo`; a `--workspace`
that conflicts with the workspace in a qualified target is rejected.
Git-checkout inference (running with no target inside a clone) is deferred to a
later slice (B3c).

### `repo`

```
atl-bb repo view [<workspace>/<repo>] [--repo <workspace>/<repo>] [--workspace <slug>]
atl-bb repo list [<workspace>] [--workspace <slug>] [--limit N] [--all]
```

`repo view` shows one repository (`GET /repositories/{ws}/{repo}`); human
output lists full name, visibility, project, main branch, and description.
`repo list` lists a workspace's repositories (`GET /repositories/{ws}`),
honoring `--limit` (the Bitbucket `pagelen`) and `--all` (follow the API's
`next`-URL pagination to completion, capped at 100 pages).

### `pr`

```
atl-bb pr list [--repo <workspace>/<repo>] [--workspace <slug>] [--state <state>] [--limit N] [--all]
atl-bb pr view <id> [--repo <workspace>/<repo>] [--workspace <slug>]
atl-bb pr create [--repo <workspace>/<repo>] [--workspace <slug>] \
  --title <text> --source <branch> --destination <branch> \
  [--description <text>] [--draft] [--close-source-branch]
```

Pull requests are addressed by the repo target (`--repo`/`--workspace`) plus a
numeric id for `view`. `pr list` filters by `--state` (`OPEN` default;
`MERGED`, `DECLINED`, `SUPERSEDED`, or `ALL` to list every state — `ALL` omits
the API `state` filter) and follows pagination with `--limit`/`--all`. `pr
create` requires `--title`, `--source`, and `--destination`; human output
prints `created pull request #<id>: <title>`. Pull-request comments, tasks,
review, merge, and decline are later slices.

### `pipeline`

```
atl-bb pipeline list [--repo <workspace>/<repo>] [--workspace <slug>] [--status <name>] [--limit N] [--all]
atl-bb pipeline view <number-or-uuid> [--repo <workspace>/<repo>] [--workspace <slug>]
atl-bb pipeline run [--repo <workspace>/<repo>] [--workspace <slug>] --ref <branch-or-tag> [--ref-type branch|tag]
```

`pipeline list` returns runs newest-first (`GET …/pipelines/`), filtered by
`--status` (the pipeline state name, e.g. `PENDING`, `IN_PROGRESS`,
`COMPLETED`) and paged with `--limit`/`--all`. `pipeline view` accepts either a
numeric **build number** (found by paging newest-first) or a pipeline **UUID**
(brace-wrapping is added automatically). `pipeline run` triggers a run against
a ref (`--ref`, with `--ref-type` defaulting to `branch`) and prints `triggered
pipeline #<n> on <ref-type> <ref>`. Steps, logs, stop, schedules, runners,
caches, and variables are later slices.

### `issue`

```
atl-bb issue list [--repo <workspace>/<repo>] [--workspace <slug>] [--state <name>] [--limit N] [--all]
atl-bb issue view <id> [--repo <workspace>/<repo>] [--workspace <slug>]
atl-bb issue create [--repo <workspace>/<repo>] [--workspace <slug>] \
  --title <text> [--body <raw>] [--kind <kind>] [--priority <priority>]
```

`issue list`/`view`/`create` operate on a repository's issue tracker. Issue
states are lower-case (`new`, `open`, `resolved`, `on hold`, `invalid`,
`duplicate`, `wontfix`, `closed`); `--state` is passed through verbatim and
`ALL` lists every state. `issue create` requires `--title`; `--kind`
(`bug`/`enhancement`/`proposal`/`task`) and `--priority`
(`trivial`/`minor`/`major`/`critical`/`blocker`) are passed through for the API
to validate, and human output prints `created issue #<id>: <title>`.

If a repository's **issue tracker is disabled**, Bitbucket returns 404 with a
recognizable message; `atl-bb` surfaces this as the `feature_disabled` error
code (distinct from `not_found_or_not_visible`) so an agent can tell "enable
the tracker" from "the repo or issue is missing". Issue comments, attachments,
state changes, and taxonomy (milestones/components/versions) are later slices.

### `workspace`

```
atl-bb workspace list [--limit N] [--all]
atl-bb workspace view [<workspace>] [--workspace <slug>]
```

`workspace list` lists the workspaces the authenticated account is a member of
(`GET /workspaces?role=member`). `workspace view` shows one workspace by slug
(positional or `--workspace`). Members, permissions, and repo-permissions are
later slices.

### `project`

```
atl-bb project list [<workspace>] [--workspace <slug>] [--limit N] [--all]
atl-bb project view <project-key> --workspace <slug>
atl-bb project create <project-key> --workspace <slug> --name <text> [--description <text>] [--private]
```

`project list` lists a workspace's projects (`GET /workspaces/{ws}/projects`).
`project view`/`create` take the project key as the positional argument and the
workspace from `--workspace`. `project create` requires `--name`; `--private`
is forwarded only when set, so an unset flag leaves Bitbucket's default
visibility in place. Project permissions and default reviewers are later
slices.

### `commit`

```
atl-bb commit list [--repo <workspace>/<repo>] [--workspace <slug>] [--revision <branch|tag|hash>] [--limit N] [--all]
atl-bb commit view <revision> [--repo <workspace>/<repo>] [--workspace <slug>]
```

`commit list` lists a repository's commit history
(`GET /repositories/{ws}/{repo}/commits`); `--revision` scopes it to a branch,
tag, or commit (`GET .../commits/{revision}`) and defaults to the main branch.
Human output shows the short hash, the first line of the message, and the
author. `commit view` resolves a single commit by hash, branch, or tag
(`GET .../commit/{revision}`).

### `branch`

```
atl-bb branch list [--repo <workspace>/<repo>] [--workspace <slug>] [--limit N] [--all]
atl-bb branch view <name> [--repo <workspace>/<repo>] [--workspace <slug>]
atl-bb branch create [--repo <workspace>/<repo>] [--workspace <slug>] --name <branch> --target <hash|branch>
atl-bb branch delete <name> [--repo <workspace>/<repo>] [--workspace <slug>]
```

Branch refs live under `GET/POST/DELETE /repositories/{ws}/{repo}/refs/branches`.
`branch create` requires `--name` and `--target` (the commit hash or existing
branch the new branch points at); the request body is
`{"name":…, "target":{"hash":…}}`. `branch delete` returns no content on
success.

### `tag`

```
atl-bb tag list [--repo <workspace>/<repo>] [--workspace <slug>] [--limit N] [--all]
atl-bb tag view <name> [--repo <workspace>/<repo>] [--workspace <slug>]
atl-bb tag create [--repo <workspace>/<repo>] [--workspace <slug>] --name <tag> --target <hash> [--message <text>]
atl-bb tag delete <name> [--repo <workspace>/<repo>] [--workspace <slug>]
```

Tag refs live under `GET/POST/DELETE /repositories/{ws}/{repo}/refs/tags`.
`tag create` requires `--name` and `--target`; `--message` is forwarded only
when set (an annotated tag) and omitted otherwise. `tag delete` returns no
content on success.

### `deployment` / `environment`

```
atl-bb deployment list [--repo <workspace>/<repo>] [--workspace <slug>] [--limit N] [--all]
atl-bb deployment view <uuid> [--repo <workspace>/<repo>] [--workspace <slug>]
atl-bb environment list [--repo <workspace>/<repo>] [--workspace <slug>] [--limit N] [--all]
atl-bb environment view <uuid> [--repo <workspace>/<repo>] [--workspace <slug>]
```

`deployment list`/`view` read deployments
(`GET /repositories/{ws}/{repo}/deployments/` and `/deployments/{uuid}`);
`environment list`/`view` read deployment environments
(`.../environments/` and `/environments/{uuid}`). Both listing endpoints
require the trailing slash. A bare UUID is brace-wrapped (`{…}`) before the
request, matching `pipeline view`. These are read-only; deployment **variables**
hold secret values and are intentionally deferred to keep credential material
out of scope.

### `search`

```
atl-bb search repos  <query> --workspace <slug> [--sort <field>] [--limit N] [--all]
atl-bb search prs    <query> [--repo <workspace>/<repo>] [--workspace <slug>] [--sort <field>] [--limit N] [--all]
atl-bb search issues <query> [--repo <workspace>/<repo>] [--workspace <slug>] [--sort <field>] [--limit N] [--all]
```

Each subcommand takes a **raw Bitbucket query expression** (the `q` filter)
as its positional argument and passes it through verbatim — the same raw-API
philosophy as `atl-jira search issues <jql>`. `search repos` is workspace-scoped
(`GET /repositories/{workspace}?q=…`); `search prs`/`search issues` are
repo-scoped and render with the same human/JSON output as `pr list`/`issue
list`. `--sort` (e.g. `-updated_on`) is optional and omitted by default, leaving
the Bitbucket API's own ordering. `search issues` on a repository with its
issue tracker disabled surfaces the `feature_disabled` error.

### `status`

```
atl-bb status
```

A **live** authentication check against the configured site (`GET /user`),
distinct from the offline `auth status`. Human output reports `authenticated`,
the site name, the account (display name + account id), the username, and the
resolved API base. Mirrors `atl-jira`/`atl-conf status`.

### `resolve` / `browse` (Bitbucket forms)

The shared `resolve` and `browse` commands recognize Bitbucket inputs via the
`internal/resolve` Bitbucket parser:

- bare `workspace/repo` → repository
- `https://bitbucket.org/{ws}/{repo}` → repository
- `.../pull-requests/{id}` → pull request
- `.../issues/{id}` → issue
- `.../commits/{hash}` → commit
- any other repository sub-page (`/src/…`, `/branches`, …) falls back to the
  repository

`resolve` is offline string parsing. `browse` builds the canonical
`bitbucket.org` web URL: a URL input carries its own host, while a bare
`workspace/repo` uses the `--site` profile's base URL with the API host
(`api.bitbucket.org/2.0`) mapped to the web host (`bitbucket.org`).

## Config file

- Path: `$XDG_CONFIG_HOME/atlassian-cli/config.json`, or
  `~/.config/atlassian-cli/config.json` when `XDG_CONFIG_HOME` is unset.
- The file is written `0600` and its directory `0700`.
- Writes are atomic: a temporary file is renamed over the target, so a crash
  never leaves a partial file and a symlink at the path is replaced, not
  followed.
- A missing file is treated as empty config; a malformed file is a structured
  error.
- `credentials.json` beside it is the `0600` fallback secret store, written
  with the same atomic-rename discipline and used only when no OS keychain is
  available. It is the one place a raw token can land on disk.

Schema:

```json
{
  "version": 1,
  "sites": {
    "work": {
      "product": "jira",
      "deployment": "cloud",
      "base_url": "https://example.atlassian.net",
      "api_base_url": "https://example.atlassian.net/rest/api/3",
      "cloud_id": "",
      "username": "user@example.com",
      "token_style": "cloud-classic",
      "auth_type": "api-token-basic",
      "token_ref": "env:ATLASSIAN_API_TOKEN"
    }
  }
}
```

`token_ref` holds an indirect reference, never a token value: `env:NAME` for
an environment variable, `keyring` for a token in the OS keychain, or `file`
for a token in the `0600` `credentials.json` fallback.

## Token styles

| Style | Auth | Signing | API base |
|---|---|---|---|
| `cloud-classic` | `api-token-basic` | `Authorization: Basic base64(email:token)` | Jira `<site>/rest/api/3`; Confluence `<site>/wiki/api/v2` |
| `cloud-scoped` | `api-token-basic` | `Authorization: Basic base64(email:token)` | `https://api.atlassian.com/ex/<product>/<cloudId>/...` (requires `cloud_id`) |
| `data-center-pat` | `pat-bearer` | `Authorization: Bearer <token>` | the configured URL verbatim |

For Confluence cloud-classic, the `/wiki` segment is added only if the
configured URL does not already include it.

## `api` URL resolution

- A **relative path** is appended to the product API base; a leading slash is
  cosmetic (`/myself` and `myself` resolve identically).
- An **absolute URL** is allowed only when its host matches the configured
  site or the Atlassian API gateway for that profile; otherwise it is rejected
  with an `untrusted_url` error.
- Data Center API paths are not pinned in Phase 1: the configured URL is used
  verbatim, so the caller supplies the full path (e.g. `/rest/api/2/myself`).

## Error model

Errors are structured values (`internal/apperr.Error`) whose JSON form follows
[access-error-model.md](access-error-model.md): the machine-readable code
serializes under the `error` key, alongside `message` and optional
`status`, `product`, `site`, `token_style`, `api_base_url`, `required_scope`,
`required_permission`, and `next`. HTTP `401`, `403`, `404`, and `429`
responses are mapped to `unauthorized`, `forbidden`, `not_found_or_not_visible`,
and `rate_limited`.

When `--json` is set and the error carries a structured `apperr.Error`, the
full JSON envelope is written to stderr; otherwise errors are written as a
plain `Error: <code>: <message>` line.

## Known limitations

- No OAuth 3LO; no browser/cookie login.
- `atl-bb` is still under construction (Phase B3b): git-checkout inference,
  aliases/extensions, and deployment-variable commands are not yet implemented.
- `issue create`/`edit` accept a plain-text `--description` (wrapped as ADF) or
  raw ADF via `--field description=<adf-json>`; richer markup helpers are not
  implemented.
- `page create`/`edit` take a `--body` plus an explicit `--body-format`; the
  content is sent verbatim and never converted between representations. A
  title-only `page edit` re-sends the page's current storage body; if the page
  has no storage-format body the edit is refused, so pass `--body` explicitly.
- Confluence page delete, move, and restore are not implemented; the page
  surface is list/view/children, create/edit, and the comment/label
  sub-groups.
- Jira `issue assign` does not set a project's default assignee (the `-1`
  sentinel is not exposed); pass `-` to unassign and an explicit account id
  to assign.
- Jira `issue link` creates a link but does not edit or delete existing
  links. Worklogs can be listed and added but not edited or deleted; worklog
  visibility/restriction is not modeled.
- `issue worklog add --time` is forwarded verbatim as Jira's `timeSpent`;
  the CLI does not parse or convert the duration. `--comment` is plain text
  wrapped as ADF.
- Confluence comment support is footer comments only — inline comments need
  text-anchor properties unsuited to a flag-based CLI. Attachment support is
  list and download only; attachment upload (a multipart write) is not
  implemented.
- Without `--all`, list and search commands fetch a single page bounded by
  `--limit`. `--all` follows every page but caps at 100 pages.
- Jira and Confluence commands target the Atlassian **Cloud** REST APIs (Jira
  v3, Confluence v2 with a v1 fallback for CQL search and the current user).
  Against a Data Center instance the API base is the configured URL verbatim,
  so the Cloud paths these commands use will not match; use the raw `api`
  command there.
- `--trace` is accepted but inert. `--no-prompt` is honored only by `browse`.
- A stored token is read at request time from the OS keychain or the `0600`
  fallback file; on macOS the keychain may prompt to authorize access the
  first time. There is no interactive no-echo token prompt — `--token-stdin`
  covers secure entry. `auth status` does not perform a live token check.
- Human (non-`--json`) output is a compact per-command summary: single-item
  views print label/value lines, list and search commands print aligned
  columns. It is intentionally minimal — `--json` is the complete surface.
- URL resolution covers Atlassian **Cloud** canonical forms only. Confluence
  tiny links (`/wiki/x/<token>`), Data Center URL shapes, and blog-post or
  attachment URLs are not recognized; `browse` roots a URL input at `https`.
