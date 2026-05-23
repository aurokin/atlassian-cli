# Access and Error Model

Different users and tokens have different access levels. The CLI should work well for the access the authenticated principal actually has.

## Principles

- Treat API-visible permissions as authoritative.
- Partial visibility is normal for list/search/status commands.
- Avoid admin-only assumptions.
- Distinguish `not found` from `not visible` when the API exposes enough signal.
- When Atlassian collapses unauthorized resources into 404/not-found, say `not found or not visible to this account`.
- Preflight permissions when cheap, but final mutation responses remain authoritative.

## First-class recovery cases

- `401`: bad token, expired token, wrong token style/base URL, missing cloud ID.
- `403`: authenticated but missing permission/scope/license.
- `404`: resource absent or hidden. Preserve ambiguity when needed.
- `410`: the endpoint has been removed — usually a withdrawn API version; upgrade the CLI.
- `429`: rate limited, include retry guidance when headers are present.
- Timeout / transport failure: the request never completed (deadline, connection refused, DNS).
- Product unavailable: a Jira/Confluence/Bitbucket feature not licensed or not enabled.

## Error code catalog

Every failure carries a stable, machine-readable code under the `error` key.
The full set the CLI emits:

| Code | Meaning | Typical origin |
|------|---------|----------------|
| `unauthorized` | bad/expired token, wrong style or base URL | HTTP 401 |
| `forbidden` | authenticated but missing permission/scope/license | HTTP 403 |
| `not_found_or_not_visible` | resource absent or hidden from this account | HTTP 404 |
| `feature_disabled` | capability exists but is switched off for the resource | e.g. Bitbucket 404 for a disabled issue tracker |
| `gone` | endpoint removed; upgrade the CLI | HTTP 410 |
| `rate_limited` | throttled by Atlassian | HTTP 429 |
| `http_error` | a non-2xx status no more specific category claimed | any other 4xx/5xx |
| `timeout` | request exceeded the deadline (retryable) | context deadline or client timeout, including one that fires mid-body-read |
| `request_failed` | non-timeout transport failure with no usable HTTP response | connection refused, DNS failure, non-deadline body-read error |
| `request_encode_failed` | the request body could not be marshaled to JSON before sending | client-side payload encoding |
| `invalid_input` | malformed or missing command input (no request made) | argument/flag validation |
| `untrusted_url` | absolute URL whose origin is neither the site nor the API gateway | `api` escape hatch guard |
| `response_decode_failed` | a response or aggregated page set could not be decoded | client decode / `--all` aggregation |
| `result_truncated` | `--all` hit the page-follow cap with more pages remaining | pagination cap |

## Process exit codes

Distinct exit codes let scripts and agents branch on the failure category
without parsing output. Categories without a dedicated code exit `1`.

| Exit | Category |
|------|----------|
| `0` | success |
| `1` | generic / uncategorized error (`http_error`, `request_failed`, `gone`, `feature_disabled`, `untrusted_url`, `response_decode_failed`, `result_truncated`) |
| `4` | `unauthorized` |
| `5` | `forbidden` |
| `6` | `not_found_or_not_visible` |
| `7` | `rate_limited` |
| `8` | `invalid_input` |
| `9` | `timeout` |

Exit `8` covers *semantic* input rejection raised by a command body
(`apperr.InvalidInput`): a missing required value, an unknown enum, a bad
combination of flags. *Usage* errors caught by the argument parser before the
command body runs — an unknown flag, the wrong number of positional args — print
the usage string and exit `1`, matching conventional CLI behavior.

## JSON error shape

```json
{
  "error": "forbidden",
  "message": "The authenticated account cannot edit this Confluence page.",
  "site": "work",
  "token_style": "cloud-scoped",
  "api_base_url": "https://api.atlassian.com/ex/confluence/<cloudId>",
  "required_scope": "write:page:confluence",
  "required_permission": "page edit permission",
  "next": "Ask a space admin for edit access, choose a token with the required scope, or retry with `atl-conf page view 123456 --json '*'`."
}
```

## Tests to require

- low-access user can list/view only visible resources
- scoped token missing write scope fails with clear recovery
- admin-only command under ordinary token fails clearly
- ambiguous 404 is represented honestly
- JSON includes stable machine-readable error fields
