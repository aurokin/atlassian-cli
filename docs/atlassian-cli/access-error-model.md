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
- `429`: rate limited, include retry guidance when headers are present.
- Product unavailable: Jira Software/Confluence feature not licensed or not enabled.

## JSON error shape

```json
{
  "error": "permission_denied",
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
