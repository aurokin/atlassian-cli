# Authentication Design

## Auth modes to support

### Cloud classic/general API token

- Token style: `cloud-classic`
- Auth type: `api-token-basic`
- Signing: `Authorization: Basic base64(email:token)`
- Base URL: product site URL
  - Jira: `https://<site>.atlassian.net/rest/api/3`
  - Confluence v2: `https://<site>.atlassian.net/wiki/api/v2`
  - Confluence v1 fallback: `https://<site>.atlassian.net/wiki/rest/api`

### Cloud scoped API token

- Token style: `cloud-scoped`
- Auth type: `api-token-basic`
- Signing: `Authorization: Basic base64(email:token)`
- Requires `cloud_id`
- Base URL:
  - Jira: `https://api.atlassian.com/ex/jira/<cloudId>` plus API path
  - Confluence: `https://api.atlassian.com/ex/confluence/<cloudId>` plus API path
- Service accounts can only create scoped API tokens.

Important: Atlassian says integrations generally cannot distinguish scoped from non-scoped tokens from the token value alone. The CLI must make token style explicit and give recovery guidance for URL/token mismatches.

### Data Center / Server PAT

- Token style: `data-center-pat`
- Auth type: `pat-bearer`
- Signing: `Authorization: Bearer <token>`
- Base URL: provided instance URL
- Do not assume Cloud `/wiki` prefix for Confluence Data Center.

### OAuth 2.0 3LO

- Token style: `oauth-3lo`
- Later feature, not MVP.
- Must use a real Atlassian app flow and token refresh, not copied browser/session tokens.

## Config sketch

```json
{
  "default_site": "work",
  "sites": {
    "work": {
      "base_url": "https://example.atlassian.net",
      "deployment": "cloud",
      "product": "jira",
      "auth_type": "api-token-basic",
      "token_style": "cloud-scoped",
      "username": "user@example.com",
      "token": "...",
      "cloud_id": "required-for-cloud-scoped-and-oauth",
      "api_base_url": "https://api.atlassian.com/ex/jira/<cloudId>",
      "updated_at": "2026-05-14T18:53:00Z"
    }
  }
}
```

## Required commands

```bash
jira auth login --token-style cloud-classic --site work --url https://example.atlassian.net --username user@example.com --with-token
jira auth login --token-style cloud-scoped --site work --url https://example.atlassian.net --username user@example.com --cloud-id "$ATLASSIAN_CLOUD_ID" --with-token
jira auth status --check --json '*'

confluence auth login --token-style cloud-classic --site work --url https://example.atlassian.net/wiki --username user@example.com --with-token
confluence auth login --token-style cloud-scoped --site work --url https://example.atlassian.net/wiki --username user@example.com --cloud-id "$ATLASSIAN_CLOUD_ID" --with-token
confluence auth status --check --json '*'
```

## Recovery guidance requirements

Detect and explain:

- scoped token used against site URL
- classic token used against API gateway URL
- missing cloud ID for scoped token
- missing scope
- expired/revoked token
- invalid Data Center PAT
- Cloud account password attempted instead of API token
