# Command Contract

> Implemented behavior as of Phase 4A. This document describes what the
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
The Confluence page write commands are not implemented yet.

## Binaries

- `atl-jira` — Jira CLI (`product: jira`)
- `atl-conf` — Confluence CLI (`product: confluence`)

Both binaries share one command tree built in `internal/cli`; only product
identity and build metadata differ.

## Global flags

| Flag | Type | Behavior in Phase 1 |
|---|---|---|
| `--json` | string | JSON output. Bare `--json` renders all fields; `--json=field1,field2` selects top-level fields. |
| `--jq` | string | Reserved. Returns a clear "not yet implemented" error if used. |
| `--site` | string | Names the configured site profile a command targets. |
| `--no-prompt` | bool | Forces non-interactive behavior. `browse` treats it as `--no-browser` (print the URL, never open one). No other command prompts yet. |
| `--trace` | bool | Accepted and reserved. Request tracing is not implemented yet. |

`--json` takes an *optional* value, so a value must be attached with `=`
(`--json=field1,field2`) — passing it space-separated (`--json field1`) leaves
`field1` as a stray argument. Bare `--json` already means "all fields"; the
explicit all-fields form is `--json=*`, which must be quoted in shells that
expand globs (`--json='*'` in zsh/bash).

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
  [--username <email>] [--cloud-id <id>] [--token-env <ENV_VAR>]
```

Records a site profile under `--site`. Required: `--site`, `--url`,
`--token-style`. `--username` is required for `cloud-classic` and
`cloud-scoped`; `--cloud-id` is required for `cloud-scoped`.

`--url` must be an `http`/`https` URL with a host and no embedded credentials.
Cloud token styles (`cloud-classic`, `cloud-scoped`) require `https`;
`data-center-pat` also accepts `http` for internal instances.

No raw token is stored. `--token-env NAME` records the reference `env:NAME`;
the token value is read from that environment variable at request time.

### `auth status`

```
atl-jira auth status [--site <name>] [--json]
```

With `--site`, shows that profile; without it, lists every configured profile.
Output reports `token_status` — whether the referenced token is currently
resolvable — but never the token value itself.

### `auth logout`

```
atl-jira auth logout --site <name>
```

Removes exactly the named profile. Errors if the site is not configured.

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
```

`list` returns the pages in a space — `--space` is required and is resolved
from key to id. `view` returns one page by id, including its storage-format
body under `--json`. `children` lists a page's direct child pages.

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

## Config file

- Path: `$XDG_CONFIG_HOME/atlassian-cli/config.json`, or
  `~/.config/atlassian-cli/config.json` when `XDG_CONFIG_HOME` is unset.
- The file is written `0600` and its directory `0700`.
- Writes are atomic: a temporary file is renamed over the target, so a crash
  never leaves a partial file and a symlink at the path is replaced, not
  followed.
- A missing file is treated as empty config; a malformed file is a structured
  error.

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

`token_ref` holds an indirect reference (`env:NAME`), never a token value.

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
- No Bitbucket (`atl-bb`).
- Confluence page write commands (`page create`/`edit`) are not implemented
  yet; Phase 4A is the read-only Confluence surface.
- `issue create`/`edit` accept a plain-text `--description` (wrapped as ADF) or
  raw ADF via `--field description=<adf-json>`; richer markup helpers are not
  implemented.
- Jira and Confluence list commands fetch a single page bounded by `--limit`;
  there is no follow-all-pages flag yet.
- Jira and Confluence commands target the Atlassian **Cloud** REST APIs (Jira
  v3, Confluence v2 with a v1 fallback for CQL search and the current user).
  Against a Data Center instance the API base is the configured URL verbatim,
  so the Cloud paths these commands use will not match; use the raw `api`
  command there.
- `--jq` is a stub; `--trace` is accepted but inert. `--no-prompt` is honored
  only by `browse`.
- Tokens are referenced via `--token-env` only. Raw token storage and OS
  keychain support are not implemented.
- Human (non-`--json`) output is a compact per-command summary: single-item
  views print label/value lines, list and search commands print aligned
  columns. It is intentionally minimal — `--json` is the complete surface.
- URL resolution covers Atlassian **Cloud** canonical forms only. Confluence
  tiny links (`/wiki/x/<token>`), Data Center URL shapes, and blog-post or
  attachment URLs are not recognized; `browse` roots a URL input at `https`.
