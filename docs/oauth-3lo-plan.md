# OAuth 2.0 (3LO) implementation plan

Plan for adding an `oauth-3lo` token style to the `atl-*` CLIs: interactive,
user-consented, scoped Atlassian Cloud access with automatic token refresh.
This complements — it does not replace — the existing static-token styles
(`cloud-classic`, `cloud-scoped`, `data-center-pat`), which remain the path for
CI and automation.

Reserved in [auth-design.md](auth-design.md) since the original design; this
document turns it into a concrete, sequenced build.

## Goal

`atl-jira auth login --site work --token-style oauth-3lo …` opens a browser,
the user consents to a set of scopes, and the CLI stores the resulting tokens
securely. Subsequent commands authenticate as that user with a short-lived
access token that the CLI refreshes automatically, never requiring the user to
paste a token.

## Locked decisions

| # | Decision | Choice | Rationale |
|---|---|---|---|
| D1 | OAuth app ownership | **Bring-your-own app** | Atlassian 3LO is a confidential client (needs a `client_secret`). A distributable CLI cannot embed a real secret, so the user registers their own Atlassian OAuth 2.0 (3LO) app and supplies `client_id` + `client_secret`. |
| D2 | Redirect capture | **Loopback localhost server on a fixed port** | Standard native-app pattern: a loopback listener catches the redirect at `http://localhost:8976/callback`. Atlassian validates the `redirect_uri` against the app's registered callback **byte-for-byte**, so the port cannot be ephemeral — it is fixed (default `8976`, overridable with `--callback-port` to match the callback the user registered). |
| D3 | CI / headless | **Interactive-only; API tokens for CI** | 3LO needs a browser. CI keeps using `cloud-classic`/`cloud-scoped` API tokens (already covered by the [auth runbook](auth-runbook.md)). Avoids the fragile "rotating refresh token in an env var" anti-pattern. |
| D4 | Secret storage | **Keychain bundle** | `client_secret` + access/refresh tokens are stored together as one JSON secret in the OS keychain (or the existing `0600` file fallback). `config.json` keeps only `client_id`, scopes, `cloud_id`, and the `token_ref`. |

## Background: the Atlassian 3LO flow

Endpoints (fixed):

- Authorize: `https://auth.atlassian.com/authorize`
- Token: `https://auth.atlassian.com/oauth/token`
- Accessible resources: `https://api.atlassian.com/oauth/token/accessible-resources`
- API gateway: `https://api.atlassian.com/ex/<product>/<cloudId>/…` (the same
  base `cloud-scoped` already targets)

Flow:

1. **Authorize.** Open the browser at the authorize endpoint with `client_id`,
   `scope`, `audience=api.atlassian.com`,
   `redirect_uri=http://localhost:8976/callback` (the exact callback registered
   on the app — Atlassian requires a byte-exact match), a random `state` (CSRF), a
   PKCE `code_challenge`, and `response_type=code`. The CLI **always ensures
   `offline_access` is in the scope set** (deduped) — that is what makes
   Atlassian return a refresh token — so the user's `--scopes` need not include
   it. (`prompt=consent` is optional; omitting it allows a silent re-authorize
   when the user has already consented. To be decided in slice 4.)
2. **Consent + redirect.** The user approves; Atlassian redirects to the
   loopback URL with `code` and `state`. The CLI validates `state`.
3. **Exchange.** POST the `code` (with `client_id`, `client_secret`,
   `redirect_uri`, `grant_type=authorization_code`) to the token endpoint →
   `access_token`, `refresh_token`, `expires_in`.
4. **Resolve cloud id.** GET accessible-resources with the access token →
   the list of authorized sites; match the one whose URL equals `--url` and
   record its `id` as `cloud_id`. (Under `--no-prompt`, a non-unique match with
   no `--cloud-id` is an error.)
5. **Store** the bundle (`client_secret`, `access_token`, `refresh_token`,
   `expiry`) in the keychain.

At request time: if the access token is expired (or within a small skew
window), POST `grant_type=refresh_token` to the token endpoint → a new
`access_token` **and a rotated `refresh_token`**; persist the new bundle, then
sign `Authorization: Bearer <access_token>`. Atlassian rotates refresh tokens,
so write-back is mandatory.

## Architecture fit

The existing seams absorb 3LO with two real additions (refresh write-back and a
bundle-shaped secret); everything else is reuse.

- **`internal/auth`** — add `StyleOAuth3LO = "oauth-3lo"`, `AuthType()` →
  `"oauth-bearer"`, include it in `AllStyles`/`Valid`. `Credential.Sign` sends
  `Bearer <access_token>` (the access token is the `Token` field). `Validate`
  requires a token and a `cloud_id`.
- **`internal/httpclient`** — `Target.APIBase` for `oauth-3lo` returns the same
  `https://api.atlassian.com/ex/<product>/<cloudId>/…` base as `cloud-scoped`
  (and the gateway is already an allowed origin). No new request plumbing.
- **`internal/config`** — `SiteProfile` gains `ClientID string` and
  `Scopes []string` (neither is secret). `client_secret` is **never** stored in
  config; it lives in the keychain bundle. `token_ref` stays `keyring`/`file`.
- **`internal/secrets`** — the `Store` interface stays string-keyed; the
  `oauth-3lo` secret value is a JSON-encoded bundle. Keyring/file backends are
  untouched. A small typed helper marshals/parses the bundle.
- **Request-time refresh** — the one genuinely new capability; see the
  dedicated section below.

### Request-time refresh: the credential-provider seam

Today `httpclient.Client` holds a static `auth.Credential` and `Do` signs with
it directly; `cli.SiteClient` has no `context.Context` and the request path
never writes to the secret store. A naive "refresh inside SiteClient" would
need a `ctx` parameter and changes at all four call sites (`api.go`,
`jiracmd.go`, `confcmd.go`, `bbcmd.go`), and would still refresh only once even
for a long, multi-page command.

Instead, introduce a **credential provider** seam on `httpclient.Client`:

- `Client` holds a `credentials func(ctx context.Context) (auth.Credential, error)`
  rather than a static `auth.Credential`. `Do` calls it (it already has `ctx`),
  then signs — so refresh happens lazily, per request, with a real context, and
  **no call site or `SiteClient` signature changes**.
- For the static styles, `SiteClient` supplies a constant provider that returns
  the fixed credential.
- For `oauth-3lo`, `SiteClient` supplies a provider that: loads the bundle,
  returns the access token if still valid (outside a small expiry skew),
  otherwise refreshes via `internal/oauth`, **persists the rotated bundle**, and
  returns a `Bearer` credential carrying the fresh access token.

This resolves the ordering contract: the provider always yields a non-empty,
fresh access token *before* `Sign`/`Validate` run, so `Validate` for
`oauth-3lo` (which requires a token + `cloud_id`) never sees an empty token.
`Credential.Token` is never a stale captured value.

**Concurrency.** Atlassian rotates the refresh token on every refresh, so two
concurrent `atl-*` runs on the same site could each refresh and invalidate the
other's token. The provider takes a best-effort **advisory file lock** (on the
credentials path) around the read→refresh→write-back critical section, so
concurrent runs serialize rather than race. The single-flight limitation and
the "refresh failed → re-run `auth login`" recovery are documented; a failed
refresh maps to a clear re-auth `apperr`.

### Config schema (addition)

```json
{
  "sites": {
    "work": {
      "product": "jira",
      "deployment": "cloud",
      "base_url": "https://your-site.atlassian.net",
      "api_base_url": "https://api.atlassian.com/ex/jira/<cloudId>",
      "cloud_id": "<cloudId>",
      "token_style": "oauth-3lo",
      "auth_type": "oauth-bearer",
      "client_id": "<your-app-client-id>",
      "scopes": ["read:jira-work", "offline_access"],
      "token_ref": "keyring"
    }
  }
}
```

The keychain secret for `work` holds
`{"client_secret": "…", "access_token": "…", "refresh_token": "…", "expiry": "…"}` —
never written to `config.json`. `scopes` here is the full set sent to Atlassian
(so it includes `offline_access`). `api_base_url` remains informational —
`Target.APIBase()` recomputes it from product + `cloud_id` at request time, as
it already does for `cloud-scoped`.

## Slices (each its own reviewed PR)

1. **Auth model + config.** `StyleOAuth3LO`, Bearer signing, gateway APIBase
   reuse, `SiteProfile.ClientID`/`Scopes`. Pure; unit-tested (sign, APIBase,
   parse, validate). No flow yet.
2. **`internal/oauth` package.** `AuthorizeURL` builder (+`state` and a PKCE
   `code_challenge`), `Exchange(code, verifier)`, `Refresh(refreshToken)`,
   `AccessibleResources(token)` — all over an injectable base URL +
   `*http.Client` and an injectable `now func() time.Time` (clock seam for
   expiry tests). `TokenBundle` type. Because these calls bypass
   `httpclient.Client`, the package owns **its own redaction and error
   mapping**: the token POST body carries `client_secret`, `code`, and
   `refresh_token` in the form body (not headers), so the existing header-only
   `--trace` redaction does not cover them — never log the request body, and
   map an `invalid_grant`/`invalid_token` token-endpoint error to a re-auth
   `apperr` rather than a generic failure. Tests hit `httptest` servers and
   assert request bodies/headers and response/error parsing.
3. **Bundle storage.** Marshal/parse the keychain bundle; `auth status` and
   `auth logout` handle the `oauth-3lo` secret (status reports token presence +
   access-token expiry, never values).
4. **`auth login --token-style oauth-3lo`.** New flags `--client-id`,
   `--client-secret`/`--client-secret-stdin`, `--scopes`, `--callback-port`
   (default `8976`, to match the registered callback), and `--cloud-id`
   (optional override when accessible-resources matching is ambiguous). Runs the
   loopback callback + browser-open (reusing `internal/browser`'s `runner` seam)
   + exchange + accessible-resources cloud-id resolution + store. Loopback
   hardening: the `redirect_uri` is the **exact registered value**
   `http://localhost:<callback-port>/callback` (Atlassian matches it
   byte-for-byte, so the port is fixed, not ephemeral). Because the registered
   host is the literal `localhost`, the listener binds the fixed port on the
   loopback interface and must accept whichever family `localhost` resolves to
   (`127.0.0.1` **or** `::1`) rather than assuming one; the callback handler
   **rejects a missing/mismatched `state`**; carry a PKCE verifier through to the
   exchange.
   Cloud-id resolution **normalizes** the comparison (case-insensitive host,
   scheme- and trailing-slash-insensitive) when matching accessible-resources to
   `--url`, falls back to `--cloud-id`, and errors under `--no-prompt` when the
   match is not unique. Honors `--no-prompt` overall (fails: a browser is
   required). Tested by stubbing the browser opener to hit the local callback
   with a fake `code` + correct `state`, with the OAuth endpoints pointed at
   `httptest`.
5. **Request-time refresh + `SiteClient` wiring.** Implement the
   credential-provider seam on `httpclient.Client` (see above): `Client` calls a
   `func(ctx) (auth.Credential, error)` before signing; `SiteClient` supplies a
   constant provider for static styles and a refreshing provider for
   `oauth-3lo`. The refreshing provider refreshes on expiry (with an injectable
   clock + skew), takes the advisory file lock around read→refresh→write-back,
   persists the rotated bundle, and returns a Bearer credential; a failed
   refresh (revoked grant, `invalid_grant`) maps to a clear re-auth apperr.
   Tested with a stored bundle + an `httptest` token endpoint: refresh fires
   only when expired, the rotated bundle is persisted, a fresh token is reused
   without a refresh, and static styles are unaffected.
6. **Docs.** Token-styles table, `auth login` flags, an `auth-runbook` 3LO
   section (including registering the app and the loopback callback URL), and
   marking 3LO implemented in `auth-design.md`.

## Test strategy & security

- **Hermetic.** Every OAuth HTTP call goes through an injectable base URL +
  `*http.Client`, so all tests run against `httptest` servers — no live
  Atlassian calls (per the standing constraint). The browser-open and the
  loopback flow are driven through the existing package-var stub pattern
  (`internal/browser`'s `runner`), so `auth login` is tested with no real
  browser.
- **No committed secrets.** `client_secret` and tokens live only in the keychain
  (or `0600` file); `config.json` never holds them. Tests use fake values and
  the mocked keyring (`keyring.MockInit` in `TestMain`), never real credentials.
- **Redaction.** The `--trace` path already redacts `Authorization`; the OAuth
  token-exchange/refresh requests must likewise never log the `client_secret`,
  `code`, or tokens.

## Non-goals (explicitly out of scope)

- A **bundled OAuth app** (embedded client_id/secret) — rejected per D1.
- **Headless/CI 3LO** (device flow, refresh-token-in-env) — rejected per D3;
  API tokens remain the automation path.
- **OAuth for Bitbucket/Data Center** — this plan is Atlassian Cloud
  (Jira/Confluence) 3LO. Bitbucket Cloud has its own OAuth model and Data
  Center has none; both can follow later if wanted.

## Open risks

- **App registration friction (D1).** Users must register an Atlassian OAuth
  2.0 (3LO) app and allowlist the loopback callback. The runbook (slice 6) must
  walk through this; without it the feature is unusable. Acceptable, but it is
  the main UX cost.
- **Loopback port/firewall.** The callback binds a **fixed** localhost port
  (default `8976`, overridable with `--callback-port`); the port can't be
  ephemeral because Atlassian matches the registered `redirect_uri` exactly. A
  port already in use, or a rare sandbox that blocks loopback, will fail the
  flow. Manual-paste fallback is a possible later addition (deferred, not in
  this plan).
- **End-to-end validation.** Default tests are hermetic and cannot exercise the
  real Atlassian endpoints; a one-time manual validation against a real
  registered app is needed before declaring the feature done.

## Resolved design questions (plan review)

A skeptical plan review cross-checked the architecture against the code and
raised the following, now folded into the slices above:

- **Refresh placement** → a credential-provider seam on `httpclient.Client`
  (lazy refresh on `Do`, which has `ctx`); `SiteClient` and the four call sites
  are unchanged.
- **Validate ordering** → the provider yields a fresh, non-empty access token
  before `Sign`/`Validate`, so `oauth-3lo` validation never sees an empty token.
- **Concurrent refresh** → advisory file lock around read→refresh→write-back;
  single-flight limitation documented; failed refresh → re-auth apperr.
- **OAuth-package redaction/errors** → `internal/oauth` owns its own redaction
  (form-body `client_secret`/`code`/`refresh_token` are not header-redacted) and
  maps `invalid_grant` to a re-auth apperr.
- **Loopback hardening** → fixed registered `redirect_uri`
  (`http://localhost:8976/callback`, port from `--callback-port`); the listener
  binds the fixed loopback port and accepts both IPv4/IPv6 loopback; validate
  `state`; add PKCE.
- **Clock seam** → injectable `now` for hermetic expiry/refresh tests.
- **cloud-id matching** → normalized comparison + `--cloud-id` override flag.
