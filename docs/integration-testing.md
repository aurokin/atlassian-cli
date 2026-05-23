# Live integration testing

The default test suite (`make test` / `make check`) is **hermetic**: it never
touches the network and runs in CI on every change. That proves the CLIs'
internal logic but not that they actually work against a real Atlassian tenant —
authentication, OAuth refresh, gateway routing, request shaping, and output
rendering all working together end to end.

The **live integration suite** under [`integration/`](../integration) closes
that gap. It builds the real `atl-jira`, `atl-conf`, and `atl-bb` binaries and
drives them against a real tenant, exactly as a user would. This mirrors the
manual-only integration suite the original `bb-cli` carried.

It is **manual-only by design** and is excluded from normal runs three ways:

1. The `//go:build integration` tag keeps the files out of `go test ./...`.
2. Every test skips unless `ATL_RUN_INTEGRATION=1` is set.
3. Every test skips outright when `CI` is set, so it can never run in CI.

> ⚠️ These tests make real API calls and create/delete real (throwaway) data.
> Run them against a **sandbox/personal tenant you don't mind mutating**, never
> a production site. Write tests create uniquely-named resources and delete them
> in a `t.Cleanup`, but a crash mid-run can leave a stray issue/page/branch.

## Running

```bash
# All products (each is skipped unless its env vars are present):
ATL_RUN_INTEGRATION=1 make integration

# One product, verbose:
ATL_RUN_INTEGRATION=1 go test -tags=integration ./integration -run Jira -v

# A single test:
ATL_RUN_INTEGRATION=1 go test -tags=integration ./integration -run TestJiraIssueLifecycle -v
```

## Authentication: two modes

### Stored profiles (override) — required for `oauth-3lo`

Set `ATL_IT_USE_STORED_PROFILES=1` and name an already-configured site per
product. The suite uses your real config/keychain and the site's existing
credentials, so it works with any token style — including `oauth-3lo`, whose
tokens cannot be passed through an environment variable.

```bash
ATL_RUN_INTEGRATION=1 ATL_IT_USE_STORED_PROFILES=1 \
  ATL_IT_JIRA_SITE=work ATL_IT_JIRA_PROJECT=KAN \
  go test -tags=integration ./integration -run Jira -v
```

### Environment-variable credentials (default)

With `ATL_IT_USE_STORED_PROFILES` unset, the suite provisions a throwaway site
profile in a temp config dir by running `auth login --token-env`, so **nothing
is written to your keychain and no token touches disk**. This path uses the
static `cloud-classic` token style (email/username + API token).

```bash
ATL_RUN_INTEGRATION=1 \
  ATL_IT_JIRA_BASE_URL=https://your-site.atlassian.net \
  ATL_IT_JIRA_EMAIL=you@example.com \
  ATL_IT_JIRA_TOKEN="$YOUR_API_TOKEN" \
  ATL_IT_JIRA_PROJECT=KAN \
  go test -tags=integration ./integration -run Jira -v
```

## Environment-variable contract

Common:

| Variable | Meaning |
|---|---|
| `ATL_RUN_INTEGRATION=1` | Required. Opt in to the live suite. |
| `ATL_IT_USE_STORED_PROFILES=1` | Use already-configured site profiles instead of env credentials. |

Per product (`<P>` is `JIRA`, `CONF`, or `BB`). A product's tests skip unless
its required variables are present, so you can run just one product.

| Variable | Mode | Meaning |
|---|---|---|
| `ATL_IT_<P>_SITE` | stored | Name of the configured site profile to target. |
| `ATL_IT_<P>_BASE_URL` | env | Site URL (Bitbucket defaults to `https://api.bitbucket.org/2.0`). |
| `ATL_IT_<P>_EMAIL` / `ATL_IT_<P>_USERNAME` | env | Your **Atlassian account email** (used as the Basic-auth username for all three products, including Bitbucket scoped API tokens). |
| `ATL_IT_<P>_TOKEN` | env | API token. Never stored to disk. |
| `ATL_IT_JIRA_CLOUD_ID` / `ATL_IT_CONF_CLOUD_ID` | env | **Required for a scoped API token.** When set, the suite logs Jira/Confluence in with `cloud-scoped` (the `api.atlassian.com/ex/{product}/{cloudId}` gateway) instead of `cloud-classic`. Leave unset only for a legacy unscoped token that authenticates against the site URL directly. |

> **API tokens with scopes.** Atlassian is replacing both Jira/Confluence
> classic tokens and Bitbucket app passwords with scoped API tokens. A scoped
> token for Jira/Confluence **must** go through the gateway, so set
> `ATL_IT_JIRA_CLOUD_ID`/`ATL_IT_CONF_CLOUD_ID` to exercise that path. Bitbucket
> scoped tokens use plain Basic auth against `api.bitbucket.org` — no cloud id.
> In every case the Basic-auth username is your Atlassian account email.

Fixtures (used in both modes):

| Variable | Product | Meaning |
|---|---|---|
| `ATL_IT_JIRA_PROJECT` | Jira | Project key to create/list issues in (e.g. `KAN`). |
| `ATL_IT_JIRA_ISSUE_TYPE` | Jira | Optional issue type to create; defaults to `Task`. |
| `ATL_IT_CONF_SPACE` | Confluence | Space key to create/list pages in (e.g. `SD`). |
| `ATL_IT_BB_WORKSPACE` | Bitbucket | Workspace slug. |
| `ATL_IT_BB_REPO` | Bitbucket | Repository slug for repo-scoped tests. |

## What the suite covers

Each product has read checks (status, list, view, search) plus a reversible
write lifecycle that creates a uniquely-named resource, exercises edit/comment/
label/transition-style mutations, and deletes it (commands without a delete verb
are cleaned up through the raw `api` command).

Write steps that fail **only** because the credential lacks the required scope
or permission are treated as a tenant/app configuration gap and **skipped**, not
failed — so a partially-scoped app still produces a meaningful pass/skip report
rather than a red build.
