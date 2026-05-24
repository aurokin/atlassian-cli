# Engineering Notes

Conventions and hard-won gotchas that live below the level of the
[command contract](command-contract.md) but that a contributor needs to know
to add a command without reintroducing a bug we already fixed. Pair this with
[CONTRIBUTING.md](../CONTRIBUTING.md) (the development loop, PR workflow, and
test-harness conventions) and the [ADRs](adr/) (the standing decisions).

## Invariants

### Validate input before constructing a client

Flag/argument validation, confirmation guards (`--yes`), and "you must change
something" checks **must run before the product client is built** (`bbClient` /
`confClient` / the Jira equivalent). The client constructor resolves the site
profile and credentials; if validation runs after it, a command invoked against
an unconfigured or clean test config returns `site_not_configured` /
`unauthorized` when the *real* problem is a bad flag — which should be
`invalid_input`.

This is also why the hermetic command tests can assert `invalid_input` from a
clean config without any site set up: the validation path never reaches auth.
A regression here is easy to introduce and was caught more than once in review —
when you add a guarded command, put the guard first.

```go
func (…) RunE(cmd, args) error {
    if err := validateFlags(...); err != nil {   // first
        return err                                 // -> invalid_input
    }
    c, err := confClient(info, g)                  // only after validation
    if err != nil {
        return err
    }
    ...
}
```

### Bodyless requests must send untyped `nil`

A nil map (or nil typed pointer) boxed into an `any` is a **non-nil interface
value**. The shared `Send` helper guards with `if payload != nil { marshal }`,
so a boxed-nil payload passes the guard and marshals to the literal JSON
`null` — which some Atlassian endpoints reject. For a request with no body
(most `DELETE`s), pass an **untyped `nil`** so the guard correctly skips
marshaling. Don't pass `var body map[string]any` (nil map) expecting "no body."

## Reach for the shared helpers

Before writing new plumbing, use what's already extracted — these exist
specifically to avoid the duplication the review waves kept flagging:

| Need | Use | Where |
|---|---|---|
| Decode a raw body and render human/JSON/jq from one call site | `cli.RenderDecoded[T]` | `internal/cli` |
| The render dispatch (`--json` vs `--jq` vs human) directly | `cli.Render` / `g.WantsStructured()` | `internal/cli` |
| HTTP verbs against the product base (GET/Send/Upload/paginate) | `restutil.Base` (`Get`, `Send`, `GetAccepting`, `Upload`, `APIBase`, `WithQuery`, `SetLimit`, `Aggregate`, the `follow*` helpers) | `internal/restutil` |
| Build a multipart file body (attachment upload) | `restutil.MultipartFile` | `internal/restutil` |
| Aligned `key value` human output | `output.NewLabelWriter` | `internal/output` |
| Aligned tabular human output | `output.TabWriter` | `internal/output` |
| `--limit` / `--all` flags on a list/search command | `cli.AddPaginationFlags` | `internal/cli` |
| Structured error with a stable code | `apperr.Error` + the code helpers | `internal/apperr` |
| A `{resource, id, deleted}` result for a delete verb | `deleteResult` | `internal/bbcmd` (pattern to mirror) |
| Confluence page/blogpost edit (shared get → mutate → update) | `runContentEdit` / `contentEditOps` | `internal/confcmd` |

If something *looks* shareable but is coupled to a product's API shape (the
pagination followers, the `get`/`send` request helpers, the `setLimit` field
name), it is **deliberately not shared** — see
[ADR 0002](adr/0002-shared-foundation.md). Don't unify those just because the
code rhymes.

## Destructive verbs

Any command that destroys or irreversibly mutates server state requires an
explicit `--yes` confirmation, validated before the client is built. Confluence
page delete additionally distinguishes trash (default) from permanent
`--purge`, and `--purge` also requires `--yes`. The full rule and rationale are
in [ADR 0003](adr/0003-destructive-verbs-require-yes.md); follow it for every
new destructive verb.

## Local gates

Run before every commit. `make check` is the core gate (the first line below);
`make lint` and `make docs-check` are the additional checks CI runs as separate
jobs:

```bash
make check        # fmt-check + compile + compile-integration + vet + test
make lint         # fmt-check + vet + golangci-lint (if installed)
make docs-check   # gen-docs into a throwaway dir — catches a broken command tree
go vet -tags=integration ./integration/...   # type-check the live suite (no run)
```

`make docs-check` is the same smoke test CI runs in the `check` job; run it
after any change to the command tree, flags, or `cmd/gen-docs`. The integration
vet is folded into `make check` via `compile-integration`, but is worth running
directly when you touch `integration/`.

## Testing seams

Side-effecting operations are exposed as package variables so a test can
substitute a fake without real I/O — `inferRepoTarget` (`internal/bbcmd`),
`execLookPath`/`executeExternal` (`internal/cli`), and the `runner` vars in
`internal/git` and `internal/browser`. When you add a feature that shells out,
hits the network, or reads the environment, expose it through a seam like these.
The full per-package helper inventory (`execRoot`, `execBB`, `newTestClient`,
`serveJSON`, the keyring mock) is in
[CONTRIBUTING.md](../CONTRIBUTING.md#test-harness-conventions).
