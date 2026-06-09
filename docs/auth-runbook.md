# Authentication runbook

A practical, end-to-end guide to authenticating the `atl-*` CLIs. For the
exhaustive flag reference see [command-contract.md](command-contract.md)
(`auth login`/`auth status`/`auth logout`, token styles, `api` URL resolution);
for the design rationale see [auth-design.md](auth-design.md).

The same model applies to all three binaries — `atl-jira`, `atl-conf`,
`atl-bb`. Substitute the binary for your product in every example below.

## How it works in one paragraph

You record a **site profile** with `auth login` (a name like `work`, a base
URL, and a token style). Commands then target it with `--site <name>`. The
profile is stored in `config.json`, but **the token value is never written
there** — `config.json` records only an indirect `token_ref`, and the secret
lives in your OS keychain, a `0600` fallback file, or an environment variable.
Every live command resolves the token at request time.

## Step 1 — pick a token style

| Your situation | `--token-style` | Also required |
|---|---|---|
| Atlassian **Cloud**, account email + a legacy *unscoped* API token (talks directly to the site URL) | `cloud-classic` | `--username <email>` |
| Atlassian **Cloud**, an **API token with scopes**, through the `api.atlassian.com` gateway with a tenant cloud id | `cloud-scoped` | `--username <email>`, `--cloud-id <id>` |
| **Server / Data Center**, authenticating with a Personal Access Token | `data-center-pat` | — |
| Atlassian **Cloud**, interactive browser sign-in with your own OAuth app | `oauth-3lo` | `--client-id`, `--client-secret`, `--scopes` |

If you are unsure on Cloud, start with **`cloud-classic`**: it talks directly to
`https://<your-site>.atlassian.net` and needs only your email and an API token.

## Step 2 — choose how the token is supplied

`auth login` accepts the token by at most one of three mutually exclusive
flags (supply one to authenticate). Pick by context:

| Flag | Stored where | Use for |
|---|---|---|
| `--token-stdin` | OS keychain (or `0600` file fallback) | **Interactive use.** The token never enters your shell history. Recommended. |
| `--token-env NAME` | nothing stored — read from `$NAME` at request time | **CI / headless / containers.** The reference `env:NAME` is recorded; the value stays in the environment. |
| `--token <value>` | OS keychain (or `0600` file fallback) | Scripts, when stdin is awkward. Note: the value is visible in shell history and the process list. |

When a stored token can't reach a keychain (CI, containers, minimal Linux),
`auth login` falls back to a `0600` `credentials.json` next to `config.json`
and prints a warning that the token is not keychain-protected.

> `oauth-3lo` does **not** use these flags — it obtains tokens through a browser
> consent flow and stores its own bundle. Skip to its section under Step 4.

## Step 3 — create the token

- **API token with scopes** (recommended; use with `cloud-scoped`): create one
  at <https://id.atlassian.com/manage-profile/security/api-tokens> →
  **Create API token with scopes**, pick the app (Jira, Confluence, or
  Bitbucket — one app per token) and grant the scopes your commands need. See
  [token-scopes.md](token-scopes.md) for product-neutral starter sets that stay
  within Atlassian's scope-count guidance. A
  scoped token authenticates as your **Atlassian account email** and **must** be
  used through the gateway, so it requires `cloud-scoped` with a `--cloud-id`;
  it will not work against the site URL with `cloud-classic`.
- **Legacy unscoped API token** (for `cloud-classic`): the older "Create API
  token" button on the same page. Atlassian is phasing these out (existing ones
  auto-expire through 2026), so prefer a scoped token.
- **Data Center PAT** (for `data-center-pat`): create one from your profile →
  **Personal Access Tokens** on the instance.
- **Cloud id** (for `cloud-scoped`): fetch it from
  `https://<your-site>.atlassian.net/_edge/tenant_info` — the `cloudId` field.

## Step 4 — log in

### Atlassian Cloud — `cloud-classic` (recommended)

Interactive (token read from stdin, stored in the keychain):

```bash
printf '%s' "$YOUR_API_TOKEN" | atl-jira auth login \
  --site work \
  --url https://your-site.atlassian.net \
  --token-style cloud-classic \
  --username you@example.com \
  --token-stdin
```

Confluence is identical with `atl-conf` and the same `--url` (the `/wiki`
segment is added automatically):

```bash
printf '%s' "$YOUR_API_TOKEN" | atl-conf auth login \
  --site work --url https://your-site.atlassian.net \
  --token-style cloud-classic --username you@example.com --token-stdin
```

### Atlassian Cloud — `cloud-scoped` (API token with scopes)

This is the style for an **API token with scopes** on Jira/Confluence. Scoped
tokens authenticate as your Atlassian account email and are only honored through
the `api.atlassian.com` gateway, so a `--cloud-id` is required — they do **not**
work against the site URL the way a legacy `cloud-classic` token does.

```bash
printf '%s' "$YOUR_API_TOKEN" | atl-jira auth login \
  --site work \
  --url https://your-site.atlassian.net \
  --token-style cloud-scoped \
  --username you@example.com \
  --cloud-id 11111111-2222-3333-4444-555555555555 \
  --token-stdin
```

Requests then go through `https://api.atlassian.com/ex/<product>/<cloud-id>/…`.
Confluence is identical with `atl-conf`. Make sure the token's scopes cover the
commands you run (the same per-endpoint rules as the `oauth-3lo` section below,
e.g. Confluence needs both classic and granular scopes for its v1/v2 mix).

### Atlassian Cloud — `oauth-3lo` (interactive browser sign-in)

`oauth-3lo` signs in through a real Atlassian OAuth 2.0 (3LO) consent flow
instead of a stored API token, and refreshes the access token automatically.
It is **interactive** — it opens a browser — so it is for desktop use, not CI
(use `cloud-classic`/`cloud-scoped` there).

**One-time app registration** (bring-your-own app, so no secret is embedded in
the CLI):

1. At <https://developer.atlassian.com/console/myapps/> create an **OAuth 2.0
   (3LO)** app.
2. Add the **callback URL** exactly: `http://localhost:8976/callback`.
   Atlassian matches it byte-for-byte, so it must be this value (or pass a
   different port with `--callback-port` and register that instead).
3. Add the Jira and/or Confluence **scopes** your commands need. Scopes are
   **per-endpoint**, so grant a scope for every command you intend to run,
   including `status` (the verify step), and enable each on the app's
   **Permissions** page before you authorize. If a command returns
   `unauthorized: scope does not match`, the token is missing that endpoint's
   scope — add it to the app and re-run `auth login`.
   - **Jira:** `read:jira-work write:jira-work read:jira-user`
     (`read:jira-user` is what makes `status` work).
   - **Confluence — note `atl-conf` is a mixed v1/v2 client, so it needs
     *both* classic and granular scopes** (Atlassian's classic scopes cover
     the REST v1 endpoints; the granular `:confluence` scopes cover REST v2):
     - Classic (v1): `read:confluence-user` (`status`),
       `search:confluence` (`search cql`), plus
       `read:confluence-content.all` / `write:confluence-content` for the v1
       fallbacks (e.g. `page label`).
     - Granular (v2): `read:space:confluence` (`space`),
       `read:page:confluence` (`page view`/`list`),
       `write:page:confluence` (`page create`/`edit`).

     A classic content/space scope does **not** cover the v2 `space`/`page`
     endpoints, and vice versa — that is why both flavors are required. In the
     developer console the Confluence API permission has separate **Classic
     scopes** and **Granular scopes** tabs; add from each.

   The CLI adds `offline_access` itself (that is what grants a refresh token).
4. Copy the app's **client ID** and **client secret**.

Then log in (secret read from stdin so it stays out of shell history):

```bash
printf '%s' "$YOUR_CLIENT_SECRET" | atl-jira auth login \
  --site work \
  --url https://your-site.atlassian.net \
  --token-style oauth-3lo \
  --client-id "$YOUR_CLIENT_ID" \
  --client-secret-stdin \
  --scopes 'read:jira-work,write:jira-work,read:jira-user'
```

Your browser opens to the Atlassian consent screen; after you approve, the CLI
captures the redirect on `localhost:8976`, exchanges the code, and resolves the
tenant `cloud_id` from the sites your authorization covers. If more than one
authorized site matches `--url`, pass `--cloud-id <id>` to disambiguate.

The token bundle (client secret, access token, refresh token, expiry) is stored
as one secret. `config.json` records only `client_id`, `scopes`, `cloud_id`,
and the `token_ref`. From then on, commands refresh the access token
transparently; you only re-run `auth login` if the refresh token is revoked or
expires.

> **macOS note:** the bundle is larger than the macOS keychain CLI's per-item
> size limit (the access token is a multi-kilobyte JWT), so on macOS it is
> stored in the `0600` `credentials.json` beside `config.json` rather than the
> keychain. `auth login` says so. The file is still user-only (`0600`); keep it
> off shared machines and out of version control.

### Server / Data Center — `data-center-pat`

PAT styles also accept `http` for internal instances:

```bash
printf '%s' "$YOUR_PAT" | atl-jira auth login \
  --site dc \
  --url https://jira.internal.example.com \
  --token-style data-center-pat \
  --token-stdin
```

Data Center API paths are not pinned, so the configured URL is used verbatim —
reach endpoints through the raw `api` command with the full path, e.g.
`atl-jira api /rest/api/2/myself --site dc`.

### Bitbucket Cloud — `atl-bb`

Bitbucket Cloud uses the `cloud-classic` (Basic) style; the fixed REST host is
filled in automatically. Authenticate with an **API token with scopes** (create
one at <https://id.atlassian.com/manage-profile/security/api-tokens> → **Create
API token with scopes** → select **Bitbucket**). For a maintained broad starter
set, use [token-scopes.md](token-scopes.md#bitbucket); for a narrow token,
grant only the Bitbucket permissions needed by the `atl-bb` commands you run.

The username is your **Atlassian account email**, not your Bitbucket username:

```bash
printf '%s' "$YOUR_API_TOKEN" | atl-bb auth login \
  --site work \
  --url https://api.bitbucket.org/2.0 \
  --token-style cloud-classic \
  --username you@example.com \
  --token-stdin
```

> **App passwords are removed.** Bitbucket stopped issuing new app passwords
> (Sept 9, 2025) and removes existing ones for good (July 28, 2026, after
> brownouts begin June 9, 2026). Use an API token with scopes as above
> (Basic auth with your Atlassian account email as the username) — that is the
> only supported credential for `atl-bb`.

## Step 5 — verify

Two complementary checks:

```bash
# Offline: is a token resolvable for this profile? (never prints the token)
atl-jira auth status --site work

# Live: make a real authenticated call and report the account.
atl-jira status --site work
```

`auth status` reports `token_status` (resolvable from env / keychain / file or
not) without contacting the API. `status` (no `auth`) performs a live
authenticated request and reports the account, so it confirms the token,
style, and base URL all actually work together.

## Managing multiple sites

`--site` selects the profile, so several can coexist:

```bash
atl-jira auth login --site work ...      # production tenant
atl-jira auth login --site sandbox ...   # test tenant
atl-jira auth status                     # lists every profile
atl-jira issue view ABC-1 --site sandbox
atl-jira auth logout --site sandbox      # removes the profile + its stored token
```

`auth logout` deletes the named profile and any token stored for it in the
keychain or fallback file. A `--token-env` profile has nothing stored to
delete (the value lives in your environment).

## Headless / CI

Use `--token-env` so nothing secret is ever written to disk:

```bash
export ATL_API_TOKEN="$SECRET_FROM_CI"     # injected by the CI secret store
atl-jira auth login --site ci \
  --url https://your-site.atlassian.net \
  --token-style cloud-classic --username ci-bot@example.com \
  --token-env ATL_API_TOKEN
atl-jira status --site ci
```

`config.json` records `token_ref: env:ATL_API_TOKEN`; the value is read from
the environment on each call. **Never commit a real token** — see the security
note below.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `unauthorized` (401) | Wrong token, wrong style, or wrong base URL for this credential | Re-check email/token and that `--token-style` matches the instance; re-run `auth login`. |
| `unauthorized` on an `oauth-3lo` site that worked before | The refresh token was revoked or expired | Re-run `auth login --token-style oauth-3lo …` to re-authorize. |
| `forbidden` (403) | Token is valid but the account lacks the permission/scope | Use an account or token with the needed permission/scope. |
| `not_found_or_not_visible` (404) | Resource doesn't exist *or* isn't visible to this account | Confirm the key/id and that the account can see it. |
| `feature_disabled` (Bitbucket) | The repo's issue tracker or wiki is turned off | Enable it in repo settings, or target a repo that has it. |
| `untrusted_url` | An absolute URL passed to `api`/`browse` doesn't match the site or gateway | Use a relative path, or an absolute URL on the configured host. |
| `auth login` warns the token isn't keychain-protected | No OS keychain available | Expected in CI/containers; the token is in a `0600` file. Prefer `--token-env` there. |
| Token seems resolvable but calls fail | Style/URL mismatch | Run with `--trace` to see the exact request line, redacted headers, and status on stderr. |

`--trace` is the first debugging tool: it prints `[trace] > METHOD URL`, the
request headers (with the credential redacted), and `[trace] < STATUS` to
stderr, leaving stdout clean.

## Security

`config.json` never holds a raw token — only the indirect `token_ref`. Tokens
live in the OS keychain, a `0600` fallback file, or an environment variable.
**Never commit API tokens, PATs, app passwords, or `credentials.json`** to this
or any repository.
