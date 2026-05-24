# Contributing

This document covers the development loop, the pull-request workflow, the
test-harness conventions, and the non-negotiable security rules for the
`atlassian-cli` monorepo. Read [`AGENTS.md`](AGENTS.md) for product posture and
[`docs/command-contract.md`](docs/command-contract.md) for the implemented
command surface.

## Prerequisites

- Go **1.26+** (the module targets `go 1.26.3`).
- No other toolchain is required: the only non-stdlib dependencies are
  `spf13/cobra`, `itchyny/gojq`, and `zalando/go-keyring`.

## Repository layout

| Path | What lives there |
|---|---|
| `cmd/atl-jira`, `cmd/atl-conf`, `cmd/atl-bb` | Thin binary `main` packages; each calls its `atl*cmd.Run`. |
| `cmd/gen-docs` | Generates the Markdown command reference for any product. |
| `internal/cli` | Shared foundation: root command, global flags, output rendering, the shared subcommands (`version`, `auth`, `api`, `resolve`, `browse`, `alias`, `extension`), and the `Run`/`Execute` entry points. |
| `internal/atljiracmd`, `internal/atlconfcmd`, `internal/atlbbcmd` | Per-binary roots: build `cli.NewRoot`, layer the product commands, delegate to `cli.Run`. |
| `internal/jiracmd`, `internal/confcmd`, `internal/bbcmd` | Product command trees. |
| `internal/jira`, `internal/conf`, `internal/bitbucket` | Typed API clients (return raw `json.RawMessage` bodies). |
| `internal/{apperr,appinfo,auth,config,httpclient,output,restutil,secrets,resolve,browse,git}` | Shared support packages. |

## Development loop

Run these from the repo root before every commit. All four must be clean:

```bash
gofmt -l internal/ cmd/   # prints nothing when formatting is correct
go build ./...
go vet ./...
go test ./...
```

`gofmt -l` lists files that need formatting; it should print nothing. Use
`gofmt -w` (or `make fmt`) to fix. The shortcut for the whole gate is
`make check` (fmt-check + compile + compile-integration + vet + test); see `make help` for all
targets.

When a change touches the command tree, flags, or `cmd/gen-docs`, also run
`make docs-check` — it generates the Markdown reference into a throwaway dir to
catch a broken command walker, and is the same smoke test CI runs in the
`check` job.

[`docs/engineering-notes.md`](docs/engineering-notes.md) collects the
conventions and gotchas below the command-contract level — the
validation-before-auth ordering, the nil-body marshaling trap, the
reusable-helper inventory, and the destructive-verb rule. Read it before adding
a command. The standing design decisions are recorded as
[ADRs](docs/adr/).

`make lint` additionally runs [`golangci-lint`](https://golangci-lint.run/)
(errcheck, staticcheck, ineffassign, unused — the static-analysis net layered
on top of `go vet`; config in `.golangci.yml`). It is a separate CI job and is
skipped locally when the tool is not installed. Install it to match CI:

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.6
```

## Pull-request workflow

1. **Branch.** Never commit directly to `main`. Use a topical branch
   (`feat/...`, `fix/...`, `docs/...`).
2. **Implement in small slices.** Prefer a vertical slice (client method →
   models → command → tests → docs) over a broad sweep.
3. **Verify.** The four-command loop above must be green. GitHub Actions runs
   the same gate (`make check`) plus a `golangci-lint` job and a `race +
   coverage` job (`go test -race -coverprofile`) on every push to `main` and
   every pull request (`.github/workflows/ci.yml`). Reproduce those locally
   with `make lint`, `make test-race`, and `make cover`.
4. **Review.** Every PR — code *or* docs — passes a subagent code review
   (`codexrabbit-code-reviewer`) before merge. Address findings and re-review
   until clean.
5. **PR + merge.** Open a PR, then merge with
   `gh pr merge <n> --squash --delete-branch` and sync `main`.
6. **Commit trailer.** End commit messages with the project's
   `Co-Authored-By:` trailer; end PR bodies with the Claude Code generation
   line.

Keep [`docs/command-contract.md`](docs/command-contract.md) current whenever a
change alters command behavior or the command surface.

Releases are cut by pushing a `v*` tag, which triggers GoReleaser; the
versioning posture and pipeline-maintenance constraints are in
[`docs/releasing.md`](docs/releasing.md).

## Output and error conventions

- **Verbatim API JSON.** Under `--json`/`--jq`, commands emit the raw upstream
  API response body (clients return `json.RawMessage`); they never rename or
  reshape fields. Human output is a compact per-type summary rendered through
  `output.TabWriter`.
- **Structured errors.** Map failures to `internal/apperr` codes
  (`unauthorized`, `forbidden`, `not_found_or_not_visible`, `rate_limited`,
  `invalid_input`, `feature_disabled`). Under `--json`, an `*apperr.Error` is
  rendered as a machine-readable envelope.
- **Agent paths.** Preserve `--json`, `--jq`, and `--no-prompt`; keep the raw
  `api` command as a first-class escape hatch.

## Test-harness conventions

Tests are **hermetic and deterministic**: no network, no real credentials, no
dependence on the developer's machine state.

> The one deliberate exception is the **live integration suite** under
> `integration/`, which drives the real binaries against a real tenant. It is
> manual-only — gated behind the `integration` build tag *and* an
> `ATL_RUN_INTEGRATION=1` opt-in, and skipped under CI — so it never *runs* as
> part of `make check`. It is, however, *compile-checked*: `make check` runs
> `make compile-integration` (`go vet -tags=integration ./integration/...`),
> so a broken integration test fails CI even though the live suite stays
> opt-in. See [docs/integration-testing.md](docs/integration-testing.md).

### Hard rules

- **No live Atlassian API calls in default tests.** Exercise HTTP commands
  against a local `httptest.Server`. (The tag-gated `integration/` suite is the
  sole, manual-only exception — see above.)
- **No raw tokens, passwords, OAuth tokens, cookies, or private credential
  files** in tests, fixtures, docs, or committed config. A test that needs a
  token sets an environment variable and points the site profile's
  `token_ref` at it (e.g. `env:ATL_API_TOKEN`); the token value never touches
  disk.
- **Isolate config and secrets.** Redirect config writes with
  `t.Setenv("XDG_CONFIG_HOME", t.TempDir())`, and rely on the mocked keyring
  (see below) so the OS keychain is never touched.

### Per-package helpers

- **`internal/cli`** — `execRoot(t, info, args...)` / `execRootIn(...)` build a
  fresh shared root and run it; `jiraInfo()`, `confInfo()`, `bbInfo()` supply
  the per-binary `appinfo.Info`. `TestMain` installs `keyring.MockInit()` so
  credential storage is in-memory for the whole package.
- **`internal/bbcmd`** (command-level) — `execBB(t, args...)` builds the atl-bb
  root with the Bitbucket commands; `loginBBSite(t, srvURL)` writes a site
  profile pointing at the test server (token via `env:ATL_API_TOKEN`, never a
  raw token).
- **`internal/bitbucket`** (and the parallel `internal/jira`, `internal/conf`)
  (client-level) — `newTestClient(srv)` builds a client bound to the test
  server; `serveJSON(t, wantPath, body)` returns an `httptest.Server` that
  asserts the request method is GET and `r.URL.Path` equals `wantPath`, then
  writes `body`.

### Injectable seams

Side-effecting operations are package variables so a test can substitute a
fake without real I/O. Save the original and restore it with `t.Cleanup`:

| Seam | Package | Stubs |
|---|---|---|
| `inferRepoTarget` | `internal/bbcmd` | git-checkout repo inference (`stubInfer`/`stubInferDisabled`) |
| `execLookPath`, `executeExternal` | `internal/cli` | extension discovery and execution |
| `runner` | `internal/git` | the `git` subprocess |
| `runner` | `internal/browser` | the browser-launch subprocess |

When you add a feature that shells out, makes a network call, or reads the
environment, expose it through a seam like these so it stays testable offline.

## Security

Never commit API tokens, PATs, OAuth tokens, passwords, cookies, private keys,
or raw credential JSON. This applies to source, tests, fixtures, docs, and
committed config alike.
