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
internal/jiracmd/       # jira Cobra tree
internal/confluencecmd/ # confluence Cobra tree
```

## Shared commands

```text
<bin> auth login|logout|status
<bin> config get|set|unset|list|path
<bin> api <path-or-url>
<bin> resolve <url>
<bin> browse ...
<bin> alias get|set|delete|list
<bin> completion bash|fish|powershell|zsh
<bin> version
```

## Raw API behavior

- Absolute URL: allow when it matches configured site/gateway.
- Relative path: resolve against effective product API base.
- `--api-version` can select Jira v2/v3 or Confluence v1/v2 where needed.
- Preserve native pagination semantics instead of hiding them behind a fake universal shape.

## Long-term Bitbucket roadmap

After Jira and Confluence stabilize, consider moving `bb` into the same Atlassian monorepo or extracting shared code. Do not constrain early design around this migration.

The deeper roadmap lives in [bitbucket-migration-roadmap.md](bitbucket-migration-roadmap.md). The short version:

- inventory `bb` first
- compare real shared seams after Jira/Confluence MVPs exist
- design compatibility before moving code
- extract only proven shared libraries
- preserve `bb` binary behavior, config, JSON fields, generated docs, and repo-local skill behavior
- choose full monorepo, shared module, or no migration based on evidence
