# Authentication Reference

Use this reference for login setup, token style selection, CI credentials,
status checks, and starter scope guidance.

## Contents

- Token styles and secret handling
- Jira and Confluence Cloud login examples
- Bitbucket Cloud login example
- CI/headless auth
- Verification and scopes

## Token Styles

| Situation | `--token-style` | Required |
|---|---|---|
| Atlassian Cloud legacy unscoped API token | `cloud-classic` | `--username <email>` |
| Atlassian Cloud scoped API token for Jira/Confluence | `cloud-scoped` | `--username <email>`, `--cloud-id <id>` |
| Server or Data Center PAT | `data-center-pat` | token only |
| Atlassian Cloud OAuth 3LO | `oauth-3lo` | `--client-id`, `--client-secret`, `--scopes` |
| Bitbucket Cloud API token with scopes | `cloud-classic` | `--username <email>` |

If unsure on Jira or Confluence Cloud, start with `cloud-classic` when the user
has a legacy unscoped token. Use `cloud-scoped` for scoped API tokens.

## Supplying Secrets

- Prefer `--token-stdin` for interactive use.
- Prefer `--token-env NAME` for CI/headless use. It stores only `env:NAME`; the value stays in the environment.
- Avoid `--token <value>` unless shell history and process-list exposure are acceptable.
- `config.json` stores profile metadata and `token_ref`, never the raw token.
- Stored tokens go to the OS keychain when available, otherwise a `0600` fallback credentials file.

## Jira Cloud Classic

```bash
printf '%s' "$ATLASSIAN_TOKEN" | atl-jira auth login \
  --site work \
  --url https://your-site.atlassian.net \
  --token-style cloud-classic \
  --username you@example.com \
  --token-stdin
```

## Confluence Cloud Classic

Use the site URL; the CLI handles the Confluence API base.

```bash
printf '%s' "$ATLASSIAN_TOKEN" | atl-conf auth login \
  --site work \
  --url https://your-site.atlassian.net \
  --token-style cloud-classic \
  --username you@example.com \
  --token-stdin
```

## Jira Or Confluence Scoped Token

Scoped API tokens use the `api.atlassian.com` gateway and require a cloud id.
Get the cloud id from:

```text
https://<your-site>.atlassian.net/_edge/tenant_info
```

```bash
printf '%s' "$ATLASSIAN_TOKEN" | atl-jira auth login \
  --site work \
  --url https://your-site.atlassian.net \
  --token-style cloud-scoped \
  --username you@example.com \
  --cloud-id 11111111-2222-3333-4444-555555555555 \
  --token-stdin
```

Substitute `atl-conf` for Confluence. Confluence is a mixed v1/v2 client, so
scoped or OAuth credentials may need both classic and granular Confluence
scopes for the command set being used.

## Bitbucket Cloud

Bitbucket Cloud uses Basic auth with an API token with scopes against the fixed
REST host. The username is the Atlassian account email, not a Bitbucket username.

```bash
printf '%s' "$BITBUCKET_TOKEN" | atl-bb auth login \
  --site work \
  --url https://api.bitbucket.org/2.0 \
  --token-style cloud-classic \
  --username you@example.com \
  --token-stdin
```

Do not use Bitbucket app passwords for `atl-bb`; use an Atlassian API token
with Bitbucket scopes.

## CI Or Headless Auth

```bash
atl-jira auth login \
  --site work \
  --url https://your-site.atlassian.net \
  --token-style cloud-classic \
  --username you@example.com \
  --token-env ATLASSIAN_TOKEN
```

The profile records `env:ATLASSIAN_TOKEN` and resolves the value at request time.

## Verify

```bash
atl-jira auth status --site work --json='*'
atl-jira status --site work --json='*'
```

`auth status` is offline and confirms whether the token is resolvable. `status`
makes a live authenticated request and confirms token, base URL, and access.

## Scopes

For starter scope sets, read `docs/token-scopes.md`.

- Jira: issue/project/JQL workflows generally need Jira read and write scopes.
- Confluence: mixed v1/v2 behavior means some workflows need both classic and granular scopes.
- Bitbucket: read scopes are explicit; write/admin scopes do not imply all matching reads.
