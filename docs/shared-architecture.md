# Shared Architecture

## Near-term posture

Develop Jira and Confluence as separate CLIs first. Share only the obvious foundation. Do not over-abstract product semantics early.

## Candidate shared packages

```text
internal/atlassian/     # auth, site resolution, HTTP, pagination interfaces
internal/output/        # table, JSON, selected fields, jq
internal/config/        # config files, aliases, settings, site records
internal/errors/        # recovery catalog and structured errors
internal/resolve/       # URL parser framework with product-specific parsers
```

## Product-specific packages

```text
internal/jira/          # Jira client/models
internal/confluence/    # Confluence client/models
internal/atljiracmd/    # atl-jira Cobra tree
internal/atlconfcmd/     # atl-conf Cobra tree
```

## Realized shared foundations (Phase 9)

The list above was the early aspiration. After Phases 1–8 the proven,
genuinely-duplicated foundations were extracted per
[shared-foundation-scorecard.md](archive/shared-foundation-scorecard.md):

```text
internal/restutil/      # WithQuery, MaxFollowPages, generic Decode/DecodeError
internal/output/        # JSON/jq/field rendering + TabWriter for human tables
internal/httpclient/    # request signing, URL resolution, error classification
internal/apperr/        # structured error model
internal/config/        # config file + credentials path
internal/secrets/       # keychain / 0600-file token store
internal/auth/          # token styles and request signing credentials
internal/cli/           # shared root, auth/api/resolve/browse, SiteClient/Render seam
internal/resolve/       # URL/key resolution
```

`internal/restutil` holds only the helpers that were byte-for-byte
identical across the typed clients; `Decode`/`DecodeError` take a product
label so each client keeps a thin wrapper and the error text still names
the product. `output.TabWriter` centralizes the one tabwriter
configuration both command trees share.

**Deliberately not shared.** The pagination followers
(`internal/jira` token+offset `followAll`/`synthesize` vs `internal/conf`
`_links.next` cursor `followList`/`nextPageURL`), the `setLimit`
parameter name (`maxResults` vs `limit`), the `get`/`send` request
helpers, the per-product `Client` constructors, and the command-tree
wiring all look similar but are coupled to product-specific API shapes.
Unifying them would hide real differences — exactly the over-abstraction
the near-term posture warns against. They are revisited only if a third
product (`atl-bb`) makes a shared shape concrete.

## Shared commands

```text
<bin> auth login|logout|status
<bin> config get|set|unset|list|path
<bin> api <path-or-url>
<bin> resolve <url>
<bin> browse ...
<bin> alias set|list|delete
<bin> extension list|exec
<bin> completion bash|fish|powershell|zsh
<bin> version
```

`alias` and `extension` are shared across all three binaries (`atl-jira`,
`atl-conf`, `atl-bb`). Each binary expands aliases against the shared config's
`aliases` map and discovers extensions under its own `<binary>-` name prefix.

## Raw API behavior

- Absolute URL: allow when it matches configured site/gateway.
- Relative path: resolve against effective product API base.
- `--api-version` can select Jira v2/v3 or Confluence v1/v2 where needed.
- Preserve native pagination semantics instead of hiding them behind a fake universal shape.

## Long-term Bitbucket roadmap

After Jira and Confluence stabilize, consider importing legacy `bb` into the same Atlassian monorepo as `atl-bb` or extracting shared code. Do not constrain early design around this migration, but do expect a rewrite period where Bitbucket is brought up to the new standards set by `atl-jira` and `atl-conf`.

The deeper roadmap lives in [bitbucket-migration-roadmap.md](archive/bitbucket-migration-roadmap.md). The short version:

- inventory `bb` first
- compare real shared seams after Jira/Confluence MVPs exist
- write a `bb-rewrite-plan.md` before importing or replacing internals
- design compatibility before moving code
- extract only proven shared libraries
- introduce `atl-bb` while preserving legacy `bb` behavior, config, JSON fields, generated docs, and repo-local skill behavior where those are stable contracts
- use the new foundation as the base for Bitbucket modernization: structure, error model, docs generation, tests, and performance
- choose full monorepo import/rewrite, shared module, or no migration based on evidence
