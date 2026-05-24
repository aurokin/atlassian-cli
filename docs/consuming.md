# Consuming the CLIs

A guide for **using** `atl-jira`, `atl-conf`, and `atl-bb` from scripts and
agents — installing a built binary, what the output and exit codes promise, and
how to drive the tools non-interactively. For the exact per-command surface see
[command-contract.md](command-contract.md); for how releases are produced see
[releasing.md](releasing.md).

## Install

### From a release (no Go toolchain needed)

Each release attaches one archive per platform, bundling all three binaries.
Download the archive for your OS/arch from the
[Releases page](https://github.com/aurokin/atlassian-cli/releases), verify it
against `checksums.txt`, then extract the binaries onto your `PATH`:

```bash
# Example: macOS arm64, version v0.1.0.
ver=v0.1.0
base="https://github.com/aurokin/atlassian-cli/releases/download/$ver"
curl -fsSLO "$base/atlassian-cli_${ver#v}_darwin_arm64.tar.gz"
curl -fsSLO "$base/checksums.txt"

# Verify before extracting.
shasum -a 256 -c checksums.txt --ignore-missing

tar -xzf "atlassian-cli_${ver#v}_darwin_arm64.tar.gz"
sudo mv atl-jira atl-conf atl-bb /usr/local/bin/
atl-jira version
```

(The archive name embeds the version without the leading `v`, e.g.
`atlassian-cli_0.1.0_darwin_arm64.tar.gz`. Windows archives are `.zip`.)

### From source

See [the README](../README.md#install--build) — `make install` builds and
installs all three into `$GOBIN` with version metadata stamped in.

## Identifying the version

`atl-* version` prints the binary, product, and version; `--json` emits the
machine form:

```bash
$ atl-jira version --json
{"binary":"atl-jira","product":"jira","version":"v0.1.0","commit":"fa10375","date":"..."}
```

Include this in bug reports — `commit` and `date` pin the exact build.

## Output contract

- **Human output by default**, a compact per-type summary. This is for people
  and is **not** a stable contract — it may change between versions.
- **`--json` / `--jq` emit the verbatim upstream Atlassian API response.** The
  CLIs return the API's JSON body unmodified — they never rename, reshape, or
  invent fields. Two consequences for scripts:
  - The field names and structure you script against are **Atlassian's**, not
    ours. Their stability is the upstream API's stability; consult the relevant
    Atlassian REST API docs for the shape, and prefer scoping with the API
    version (`--api-version`) where a command exposes it.
  - We do not paper over upstream differences. Where Atlassian exposes no real
    API path, the CLI does not fake one (see
    [ADR 0006](adr/0006-verbatim-json-no-fake-parity.md)).
- `--json` selects top-level fields with `--json=field1,field2`; bare `--json`
  is all fields (`--json='*'` is the explicit, glob-safe form). `--jq` runs a
  full [jq](https://jqlang.github.io/jq/) expression over the same JSON and
  prints each result as compact JSON on its own line. The two cannot be
  combined with a `--json` field list. See
  [command-contract.md](command-contract.md#global-flags) for the full flag
  semantics.

## Exit codes — branch without parsing output

Every command maps failures to a stable category with a distinct process exit
code, so a script can branch on the *kind* of failure without scraping stderr.
The full table lives in
[access-error-model.md](access-error-model.md#process-exit-codes); the ones you
will branch on most:

| Exit | Meaning |
|---|---|
| `0` | success |
| `4` | unauthorized (bad/expired token, wrong style or base URL) |
| `5` | forbidden (authenticated, missing permission/scope/license) |
| `6` | not found or not visible to this account |
| `7` | rate limited |
| `8` | invalid input (bad flags/args — no request was made) |
| `9` | timeout (retryable) |
| `1` | any other / uncategorized error |

```bash
if atl-jira issue view "$KEY" --json >/tmp/issue.json; then
  jq -r '.fields.summary' /tmp/issue.json
else
  case $? in
    4) echo "re-authenticate: atl-jira auth login ..." >&2 ;;
    6) echo "no such issue, or not visible to this token" >&2 ;;
    7) echo "backing off (rate limited)" >&2; sleep 30 ;;
    *) echo "unhandled error" >&2 ;;
  esac
fi
```

Under `--json`, a failure is also rendered as a machine-readable error envelope
carrying the same stable `error` code — handy when you want the reason in-band
as well as via the exit code.

## Pagination — `--all` and `result_truncated`

List and search commands return one page by default. Pass `--all` to follow
every page, or `--limit N` to cap. `--all` is bounded by an internal page-follow
cap; if it hits the cap with pages still remaining, the command exits with the
`result_truncated` category (exit `1`) so a script can tell "complete" from
"stopped early." Narrow the query or page explicitly rather than assuming
`--all` is exhaustive on very large result sets. See
[command-contract.md](command-contract.md#pagination----limit-and---all).

## Non-interactive use

- Pass **`--no-prompt`** to force non-interactive behavior. For `browse` it
  means "print the URL, never open a browser." (Most commands never prompt.)
- Set the target site once with `ATL_SITE=<name>` or
  `atl-jira auth default <name>` instead of repeating `--site` (resolution
  order: `--site` → `ATL_SITE` → `default_site`).
- For headless/CI auth, supply the token via `--token-env NAME` so nothing is
  written to disk; see [auth-runbook.md](auth-runbook.md).
- The raw **`api`** command is a first-class escape hatch for any endpoint a
  typed command doesn't cover yet — it emits the verbatim response and honors
  the same `--json`/`--jq`/error model.

## The generated command reference

The full per-command Markdown reference is generated on demand (it is not
committed to the repo). Build it from a clone with `make docs` (wraps
`go run ./cmd/gen-docs`), or read [command-contract.md](command-contract.md),
which is the canonical hand-maintained description of the surface.
