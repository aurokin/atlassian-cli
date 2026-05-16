# Phase 1 Foundation Implementation Plan

> **For Hermes:** Use `subagent-driven-development` skill to implement this plan task-by-task after the user approves execution.

**Goal:** Build the first working foundation for `atl-jira` and `atl-conf`: Go module, Cobra binaries, config skeleton, output renderer, structured errors, auth command scaffolding, and raw API command shape.

**Architecture:** Two user-facing binaries (`atl-jira`, `atl-conf`) share small internal foundation packages. Product-specific command trees should stay thin and call shared helpers only where behavior is genuinely common. Do not implement broad Jira/Confluence product commands until auth, output, errors, and raw `api` are proven.

**Tech Stack:** Go, Cobra, standard `net/http`, standard `encoding/json`, optional `gojq` only after evaluating dependency weight, `go test ./...` as the first gate.

---

## Source documents

Read these before implementation:

1. `README.md`
2. `AGENTS.md`
3. `docs/atlassian-cli/README.md`
4. `docs/atlassian-cli/auth-design.md`
5. `docs/atlassian-cli/access-error-model.md`
6. `docs/atlassian-cli/shared-architecture.md`
7. `docs/atlassian-cli/implementation-plan.md`
8. `docs/atlassian-cli/phase-1-foundation-plan.md` — this file

Do not start Bitbucket migration work during Phase 1. `atl-bb` is documented as a future import/rewrite after the new foundation proves itself.

## Non-goals for Phase 1

- No OAuth 3LO.
- No Bitbucket code import.
- No full Jira issue/project command set.
- No full Confluence page/space command set.
- No live Atlassian calls in default tests.
- No secrets in repo, fixtures, logs, or docs.
- No fake cross-product behavior when Atlassian APIs differ.

## Quality bars

- `go test ./...` passes after each implementation task that adds code.
- Commands that mutate local config have tests using temporary directories.
- Human output can be basic, but JSON output must be stable and documented by tests.
- Error payloads should match `access-error-model.md` shape as early as practical.
- Product-specific command files should be small; shared behavior belongs under `internal/` packages.
- Performance starts with avoiding obvious waste: no shell-outs, no repeated config reads per command, no unnecessary API calls in `help`, `version`, or config-only commands.

---

## Proposed package layout

```text
cmd/atl-jira/main.go
cmd/atl-conf/main.go
internal/atljiracmd/root.go
internal/atlconfcmd/root.go
internal/appinfo/appinfo.go
internal/config/config.go
internal/config/config_test.go
internal/output/output.go
internal/output/output_test.go
internal/apperr/error.go
internal/apperr/error_test.go
internal/auth/auth.go
internal/auth/auth_test.go
internal/httpclient/client.go
internal/httpclient/client_test.go
internal/jira/client.go
internal/confluence/client.go
```

Package names are intentionally short and Go-friendly:

- `atljiracmd` and `atlconfcmd` avoid hyphens in Go package names.
- `apperr` avoids collision with standard `errors` package.
- `httpclient` avoids vague `httpx` naming until the abstraction proves itself.

---

## Task 1: Initialize Go module and dependency baseline

**Objective:** Create a buildable Go module with Cobra as the CLI framework.

**Files:**

- Create: `go.mod`
- Create: `go.sum`

**Steps:**

1. Run:

   ```bash
   go mod init github.com/aurokin/atlassian-cli
   go get github.com/spf13/cobra@latest
   go mod tidy
   ```

2. Verify:

   ```bash
   go test ./...
   ```

   Expected for empty module: no package failures.

3. Commit:

   ```bash
   git add go.mod go.sum
   git commit -m "chore: initialize Go module"
   ```

---

## Task 2: Add app metadata package

**Objective:** Centralize binary name, product, version, and build metadata.

**Files:**

- Create: `internal/appinfo/appinfo.go`
- Create: `internal/appinfo/appinfo_test.go`

**Implementation sketch:**

```go
package appinfo

type Product string

const (
	ProductJira       Product = "jira"
	ProductConfluence Product = "confluence"
)

type Info struct {
	Binary  string  `json:"binary"`
	Product Product `json:"product"`
	Version string  `json:"version"`
	Commit  string  `json:"commit,omitempty"`
	Date    string  `json:"date,omitempty"`
}

func New(binary string, product Product, version, commit, date string) Info {
	if version == "" {
		version = "dev"
	}
	return Info{Binary: binary, Product: product, Version: version, Commit: commit, Date: date}
}
```

**Tests:**

- empty version defaults to `dev`
- product and binary are preserved

**Verify:**

```bash
go test ./internal/appinfo ./...
```

**Commit:**

```bash
git add internal/appinfo
git commit -m "feat: add app metadata package"
```

---

## Task 3: Add root commands for both binaries

**Objective:** Make `go run ./cmd/atl-jira --help` and `go run ./cmd/atl-conf --help` work.

**Files:**

- Create: `cmd/atl-jira/main.go`
- Create: `cmd/atl-conf/main.go`
- Create: `internal/atljiracmd/root.go`
- Create: `internal/atlconfcmd/root.go`
- Create: `internal/atljiracmd/root_test.go`
- Create: `internal/atlconfcmd/root_test.go`

**Implementation shape:**

- `cmd/atl-jira/main.go` calls `atljiracmd.NewRoot(...).Execute()`.
- `cmd/atl-conf/main.go` calls `atlconfcmd.NewRoot(...).Execute()`.
- Root commands define:
  - `Use: "atl-jira"` or `Use: "atl-conf"`
  - short description
  - global flags: `--json`, `--jq`, `--site`, `--no-prompt`, `--trace`
  - `version` subcommand

**Test expectations:**

- root command use is correct
- help contains the binary name
- `version --json` returns valid JSON with `binary`, `product`, `version`

**Verify:**

```bash
go test ./...
go run ./cmd/atl-jira --help
go run ./cmd/atl-conf --help
go run ./cmd/atl-jira version --json
go run ./cmd/atl-conf version --json
```

**Commit:**

```bash
git add cmd internal/atljiracmd internal/atlconfcmd
git commit -m "feat: add atl-jira and atl-conf root commands"
```

---

## Task 4: Add output renderer foundation

**Objective:** Provide a shared renderer for human text and JSON output.

**Files:**

- Create: `internal/output/output.go`
- Create: `internal/output/output_test.go`

**Required behavior:**

- Render full JSON when `--json '*'` or `--json` is requested.
- Render selected top-level fields for `--json field1,field2`.
- Leave `--jq` as a documented stub if dependency is not selected yet.
- Keep human output simple for now.

**Initial API sketch:**

```go
type Options struct {
	JSON string
	JQ   string
}

func Render(w io.Writer, v any, opts Options) error
```

**Tests:**

- full JSON renders valid JSON
- selected fields render only requested top-level fields
- unknown selected fields are omitted or reported consistently; choose one and document it in test name
- `--jq` returns a clear not-yet-implemented error until implemented

**Verify:**

```bash
go test ./internal/output ./...
```

**Commit:**

```bash
git add internal/output
git commit -m "feat: add shared output renderer"
```

---

## Task 5: Add structured application error package

**Objective:** Create the machine-readable error envelope used across commands.

**Files:**

- Create: `internal/apperr/error.go`
- Create: `internal/apperr/error_test.go`

**Fields to support now:**

```go
type Error struct {
	Code               string `json:"code"`
	Message            string `json:"message"`
	Status             int    `json:"status,omitempty"`
	Product            string `json:"product,omitempty"`
	Site               string `json:"site,omitempty"`
	TokenStyle         string `json:"token_style,omitempty"`
	APIBaseURL         string `json:"api_base_url,omitempty"`
	RequiredScope      string `json:"required_scope,omitempty"`
	RequiredPermission string `json:"required_permission,omitempty"`
	Next               string `json:"next,omitempty"`
}
```

**Tests:**

- JSON shape matches `access-error-model.md`
- `Error()` human string includes code and message
- status/code helpers for `unauthorized`, `forbidden`, and `not_found_or_not_visible`

**Verify:**

```bash
go test ./internal/apperr ./...
```

**Commit:**

```bash
git add internal/apperr
git commit -m "feat: add structured application errors"
```

---

## Task 6: Add config schema and file store

**Objective:** Load/save local CLI config without real secrets in tests.

**Files:**

- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Config shape:**

```go
type Config struct {
	Version int                    `json:"version"`
	Sites   map[string]SiteProfile `json:"sites"`
}

type SiteProfile struct {
	Product    string `json:"product"`
	Deployment string `json:"deployment"`
	BaseURL    string `json:"base_url"`
	APIBaseURL string `json:"api_base_url,omitempty"`
	CloudID    string `json:"cloud_id,omitempty"`
	Username   string `json:"username,omitempty"`
	TokenStyle string `json:"token_style"`
	AuthType   string `json:"auth_type"`
	TokenRef   string `json:"token_ref,omitempty"`
}
```

**Rules:**

- Default config path should be under the user's config directory, product-family scoped, for example `~/.config/atlassian-cli/config.json`.
- Tests must use temporary directories and never touch the real home directory.
- File permissions for new config should be `0600` where supported.
- Store token references or placeholders only in Phase 1; do not implement OS keychain yet unless explicitly chosen.

**Tests:**

- new config defaults to version 1 and empty sites
- save then load round trip
- missing config returns empty config, not an error
- malformed config returns a structured error

**Verify:**

```bash
go test ./internal/config ./...
```

**Commit:**

```bash
git add internal/config
git commit -m "feat: add config file store"
```

---

## Task 7: Add auth model and signing helpers

**Objective:** Represent Atlassian-supported auth modes before wiring login prompts.

**Files:**

- Create: `internal/auth/auth.go`
- Create: `internal/auth/auth_test.go`

**Token styles:**

- `cloud-classic`: Basic auth with username/email + API token against site URL.
- `cloud-scoped`: Basic auth with username/email + scoped token against `api.atlassian.com/ex/{product}/{cloudId}`.
- `data-center-pat`: Bearer auth against organization/Data Center URL.

**Tests:**

- classic token signs with `Authorization: Basic ...`
- scoped token signs with `Authorization: Basic ...` and requires `cloud_id`
- Data Center PAT signs with `Authorization: Bearer ...`
- missing required fields return structured errors

**Verify:**

```bash
go test ./internal/auth ./...
```

**Commit:**

```bash
git add internal/auth
git commit -m "feat: add Atlassian auth signing model"
```

---

## Task 8: Add HTTP client wrapper and API path resolver

**Objective:** Resolve relative API paths and parse HTTP errors consistently.

**Files:**

- Create: `internal/httpclient/client.go`
- Create: `internal/httpclient/client_test.go`

**Required behavior:**

- Absolute URL allowed only when it matches configured site or Atlassian API gateway for that profile.
- Relative `/...` paths resolve against the product's effective API base.
- Jira default base:
  - Cloud classic: `<site>/rest/api/3`
  - Cloud scoped: `https://api.atlassian.com/ex/jira/<cloudId>/rest/api/3`
- Confluence default base:
  - Cloud classic: `<site>/wiki/api/v2`
  - Cloud scoped: `https://api.atlassian.com/ex/confluence/<cloudId>/wiki/api/v2`
- Parse 401, 403, ambiguous 404, and rate-limit responses into `apperr.Error`.

**Tests:**

- URL resolution for Jira classic/scoped
- URL resolution for Confluence classic/scoped
- Data Center URL resolution uses configured base
- 401/403/404 error mapping
- no network calls outside test HTTP server

**Verify:**

```bash
go test ./internal/httpclient ./...
```

**Commit:**

```bash
git add internal/httpclient
git commit -m "feat: add Atlassian HTTP client foundation"
```

---

## Task 9: Add auth commands without secret prompts

**Objective:** Provide non-interactive auth login/status/logout scaffolding that writes config profiles.

**Files:**

- Modify: `internal/atljiracmd/root.go`
- Modify: `internal/atlconfcmd/root.go`
- Add tests under both command packages

**Command shape:**

```bash
atl-jira auth login --site work --url https://example.atlassian.net --username user@example.com --token-style cloud-classic --token-env ATLASSIAN_API_TOKEN
atl-jira auth status --site work --json '*'
atl-jira auth logout --site work

atl-conf auth login --site work --url https://example.atlassian.net/wiki --username user@example.com --token-style cloud-scoped --cloud-id "$ATLASSIAN_CLOUD_ID" --token-env ATLASSIAN_API_TOKEN
atl-conf auth status --site work --json '*'
atl-conf auth logout --site work
```

**Important:** Do not store raw token values by default. Accept `--token-env` in Phase 1 and store only the environment variable reference. `--with-token` can be added later with secure handling.

**Tests:**

- login creates a site profile with product-specific product field
- status reports configured site without exposing token values
- logout removes only the requested site
- missing token style or URL gives structured error

**Verify:**

```bash
go test ./...
```

**Commit:**

```bash
git add internal/atljiracmd internal/atlconfcmd
git commit -m "feat: add auth command scaffolding"
```

---

## Task 10: Add raw API command against test server

**Objective:** Implement `api` command enough to validate config, auth signing, URL resolution, HTTP execution, and output rendering.

**Files:**

- Modify: `internal/atljiracmd/root.go`
- Modify: `internal/atlconfcmd/root.go`
- Add tests under both command packages

**Command shape:**

```bash
atl-jira api /myself --site work --json '*'
atl-conf api /pages --site work --json '*'
```

**Tests:**

- command calls a local test HTTP server
- Authorization header is present and uses expected style
- JSON response is rendered unchanged with `--json '*'`
- non-2xx response maps to `apperr.Error`
- absolute URL outside configured site is rejected

**Verify:**

```bash
go test ./...
```

**Commit:**

```bash
git add internal cmd
git commit -m "feat: add raw API command foundation"
```

---

## Task 11: Add documentation for current command contract

**Objective:** Keep repo docs in sync with the implemented Phase 1 surface.

**Files:**

- Create or update: `docs/atlassian-cli/command-contract.md`
- Update: `docs/atlassian-cli/README.md`
- Update: `README.md`

**Document:**

- current implemented commands
- config file path and schema
- supported token styles
- `--token-env` behavior and why raw token storage is deferred
- `api` command URL resolution rules
- known Phase 1 limitations

**Verify:**

```bash
go test ./...
git diff --check
```

**Commit:**

```bash
git add README.md docs/atlassian-cli
git commit -m "docs: document Phase 1 command contract"
```

---

## Phase 1 done definition

Phase 1 is done when all are true:

- `go run ./cmd/atl-jira --help` works.
- `go run ./cmd/atl-conf --help` works.
- `go run ./cmd/atl-jira version --json` works.
- `go run ./cmd/atl-conf version --json` works.
- `auth login/status/logout` works without storing raw token values.
- `api` can call a local test server and render JSON.
- structured errors are tested.
- config round trip is tested.
- `go test ./...` passes.
- docs list implemented commands and known limitations.

## First execution checkpoint

After Tasks 1-3, stop and review the command architecture before implementing config/auth. This prevents early coupling if the root command pattern feels wrong.
