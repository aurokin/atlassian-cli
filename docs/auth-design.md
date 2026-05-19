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
  "version": 1,
  "sites": {
    "work": {
      "base_url": "https://example.atlassian.net",
      "deployment": "cloud",
      "product": "jira",
      "auth_type": "api-token-basic",
      "token_style": "cloud-scoped",
      "username": "user@example.com",
      "token_ref": "keyring",
      "cloud_id": "required-for-cloud-scoped-and-oauth",
      "api_base_url": "https://api.atlassian.com/ex/jira/<cloudId>"
    }
  }
}
```

`config.json` never holds a raw token. `token_ref` is an indirect pointer to
where the token actually lives; see Credential storage below.

## Credential storage

The raw token is never written to `config.json`. A profile's `token_ref`
records which backend holds it, and the value is fetched only at request time:

| `token_ref` | Backend | Set by |
|-------------|---------|--------|
| `env:NAME`  | Environment variable `NAME` (value never stored). | `auth login --token-env NAME` |
| `keyring`   | OS keychain — macOS Keychain, Linux Secret Service, Windows Credential Manager — service `atlassian-cli`, account = site name. | `auth login --token-stdin` / `--token`, keyring usable |
| `file`      | A `0600` `credentials.json` beside `config.json`, keyed by site name. | `auth login --token-stdin` / `--token`, no keyring available |

- The keychain is reached through `github.com/zalando/go-keyring`.
- `auth login` prefers the keychain. When the keychain cannot be written
  (CI, containers, minimal Linux) it falls back to the `0600` file and prints
  a warning that the token is not keychain-protected — storing a token always
  succeeds.
- `--token-env` is the headless/CI path and stores nothing; it remains fully
  supported. `--token-stdin` is the preferred interactive path because the
  token never enters the shell history.
- `auth logout` deletes the stored secret from its backend before removing the
  profile. `auth status` reports whether the token is currently resolvable,
  never the value.
- Guardrail: no real token is ever committed to the repo. Tests use the
  go-keyring in-memory mock and temp directories.

## Required commands

```bash
atl-jira auth login --token-style cloud-classic --site work --url https://example.atlassian.net --username user@example.com --token-stdin
atl-jira auth login --token-style cloud-scoped --site work --url https://example.atlassian.net --username user@example.com --cloud-id "$ATLASSIAN_CLOUD_ID" --token-stdin
atl-jira auth status --site work --json '*'

atl-conf auth login --token-style cloud-classic --site work --url https://example.atlassian.net/wiki --username user@example.com --token-stdin
atl-conf auth login --token-style cloud-scoped --site work --url https://example.atlassian.net/wiki --username user@example.com --cloud-id "$ATLASSIAN_CLOUD_ID" --token-stdin
atl-conf auth status --site work --json '*'
```

The token is read from stdin (`--token-stdin`); `--token-env NAME` is the
headless/CI alternative. A live credential check is the product `status`
command, distinct from the offline `auth status`.

## Recovery guidance requirements

Detect and explain:

- scoped token used against site URL
- classic token used against API gateway URL
- missing cloud ID for scoped token
- missing scope
- expired/revoked token
- invalid Data Center PAT
- Cloud account password attempted instead of API token
