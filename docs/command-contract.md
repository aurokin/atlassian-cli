# Command Contract

> Implemented behavior as of Phase 2. This document describes what the
> `atl-jira` and `atl-conf` binaries actually do today, not the long-term
> design. Update it whenever the command surface changes.

## Scope

Phase 1 delivers the shared foundation: two binaries, global flags, config,
structured output and errors, auth signing, an HTTP client, the `auth` command
subtree, and the raw `api` escape hatch. Phase 2 adds offline URL/key
resolution: the `resolve` and `browse` commands. Product-specific commands
(Jira issues/projects, Confluence pages/spaces) are not implemented yet.

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
- No product commands beyond raw `api` and `resolve`/`browse` — no Jira
  issue/project or Confluence page/space commands.
- `--jq` is a stub; `--trace` is accepted but inert. `--no-prompt` is honored
  only by `browse`.
- Tokens are referenced via `--token-env` only. Raw token storage and OS
  keychain support are not implemented.
- Human (non-`--json`) output is minimal: `resolve` prints a label/value
  summary, other commands fall back to indented JSON.
- URL resolution covers Atlassian **Cloud** canonical forms only. Confluence
  tiny links (`/wiki/x/<token>`), Data Center URL shapes, and blog-post or
  attachment URLs are not recognized; `browse` roots a URL input at `https`.
