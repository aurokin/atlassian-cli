# Automation Reference

Use this reference when an agent needs deterministic output, pagination,
exit-code handling, URL resolution, browse behavior, or raw API fallback.

## Output

- Human output is for people and is not a stable parsing contract.
- `--json` emits all JSON fields.
- `--json=field1,field2` selects top-level fields.
- In shells that expand globs, quote the explicit all-fields form: `--json='*'`.
- `--jq '<expr>'` runs a jq expression over the same JSON.
- `--jq` and a `--json` field list cannot be combined.
- JSON is the verbatim upstream Atlassian API response unless a command has no response body and documents a small synthesized result.

Examples:

```bash
atl-jira issue view PROJ-123 --site work --jq '.fields.summary'
atl-conf search cql 'type = page' --site work --jq '.results[].content.title'
atl-bb repo view workspace/repo --site work --json='*'
```

## Pagination

- List and search commands return one page by default.
- Use `--limit N` to cap results.
- Use `--all` to follow pages.
- If `--all` reaches the internal page-follow cap, the command fails with `result_truncated`; narrow the query or page explicitly.

## Exit Codes

Branch on exit codes instead of scraping stderr:

| Exit | Meaning |
|---|---|
| `0` | success |
| `4` | unauthorized |
| `5` | forbidden |
| `6` | not found or not visible |
| `7` | rate limited |
| `8` | invalid input |
| `9` | timeout |
| `1` | generic or uncategorized error |

Under `--json`, structured failures include an error envelope with fields such
as `error`, `message`, `site`, `token_style`, `required_scope`, and `next`.

## Site Selection

Networked commands choose a target profile in this order:

1. `--site <name>`
2. `ATL_SITE`
3. configured `default_site`

Set a default with:

```bash
atl-jira auth default work
```

## Resolve And Browse

Use `resolve` before inferring from URLs or compact identifiers:

```bash
atl-jira resolve PROJ-123 --json='*'
atl-conf resolve https://your-site.atlassian.net/wiki/spaces/DEV/pages/123456/Notes --json='*'
atl-bb resolve https://bitbucket.org/workspace/repo/pull-requests/7 --json='*'
```

Use `browse` with `--no-browser` or global `--no-prompt` to print canonical URLs:

```bash
atl-jira browse PROJ-123 --site work --no-browser --json
atl-conf browse 123456 --site work --no-prompt --json
atl-bb browse workspace/repo --site work --no-browser --json
```

## Raw API Fallback

Use typed commands first. When no typed command exists for an official endpoint,
use the product binary's `api` command:

```bash
atl-jira api /myself --site work --json='*'
atl-conf api /spaces --site work --jq '.results[] | {id, key, name}'
atl-bb api /repositories/workspace/repo/pullrequests --site work --jq '.values[] | {id, title, state}'
```

Absolute URLs are guarded: the origin must match the configured site or the
Atlassian API gateway. Relative paths resolve against the effective product API
base.
