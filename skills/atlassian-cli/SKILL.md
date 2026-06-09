---
name: atlassian-cli
description: Atlassian CLI skill for this repository's `atl-jira`, `atl-conf`, and `atl-bb` commands. Use when a task mentions Jira, Confluence, Bitbucket Cloud, Atlassian, `atl-jira`, `atl-conf`, `atl-bb`, Atlassian site URLs, Jira issue keys, Confluence spaces/pages/blogposts, or Bitbucket workspace/repository/pull request/pipeline/issue URLs. Covers installation, authentication, site targeting, structured output, URL resolution, browse flows, product command workflows, pagination, exit codes, destructive-command guardrails, and raw official Atlassian REST API fallback with `api`.
---

# Atlassian CLI

## Overview

Use this skill for Atlassian work through the `atl-*` CLI family:

- `atl-jira` for Jira
- `atl-conf` for Confluence
- `atl-bb` for Bitbucket Cloud

The CLIs keep product vocabularies separate while sharing auth, site targeting,
structured output, URL resolution, browse flows, errors, and the raw `api`
escape hatch.

## Use This Skill When

- The task mentions Jira, Confluence, Bitbucket Cloud, Atlassian, or an `atl-*` binary.
- The task includes an Atlassian, Jira, Confluence, or Bitbucket URL.
- The task includes a Jira issue key, Confluence page/space/blogpost target, or Bitbucket workspace/repository/PR/pipeline/issue target.
- The task needs deterministic Atlassian CLI output for an agent.
- The task needs an official Atlassian REST API fallback through `atl-jira api`, `atl-conf api`, or `atl-bb api`.

## Do Not Use This Skill When

- The task is plain local git work with no Bitbucket Cloud API need.
- The task targets GitHub, GitLab, Trello, Slack, Bitbucket Server/Data Center, or another non-Atlassian Cloud product.
- The task can only be done by inventing fake parity for a behavior Atlassian does not expose through an official API.

## Core Rules

- Pick the product binary first: `atl-jira`, `atl-conf`, or `atl-bb`.
- Prefer explicit `--site <name>` targeting. If omitted, the CLI resolves `ATL_SITE`, then the configured default site.
- Use `--json`, `--json='*'`, or `--jq` for machine parsing. Do not parse human-readable output when structured output is available.
- Treat JSON as the verbatim upstream Atlassian API body. Field names and shapes are Atlassian's, not normalized CLI fields.
- Use `--no-prompt` for headless or non-interactive flows. For `browse`, this prints the URL instead of opening a browser.
- Run `resolve` first when starting from a pasted URL or compact resource identifier.
- Use `browse --no-browser --json` or `browse --no-prompt --json` when the task needs a canonical browser URL.
- Use typed commands first, then fall back to `api` for official endpoints not wrapped yet.
- Some destructive verbs require explicit `--yes`; add it only when command help or the product reference documents that guard.
- Branch on process exit codes and JSON error envelopes, not stderr text.

## Quick Setup

Install or update from a release when available, or from a local clone:

```bash
make install
atl-jira version
atl-conf version
atl-bb version
```

Authenticate one site profile per product:

```bash
printf '%s' "$ATLASSIAN_TOKEN" | atl-jira auth login \
  --site work --url https://your-site.atlassian.net \
  --token-style cloud-classic --username you@example.com --token-stdin

atl-jira auth status --site work --json='*'
atl-jira status --site work --json='*'
```

For scoped tokens, OAuth, Data Center PATs, Bitbucket Cloud auth, CI token
handling, and starter scopes, read `references/auth.md`.

## First Commands To Reach For

```bash
atl-jira resolve PROJ-123 --json='*'
atl-conf resolve https://your-site.atlassian.net/wiki/spaces/DEV/pages/123456/Notes --json='*'
atl-bb resolve https://bitbucket.org/workspace/repo/pull-requests/7 --json='*'

atl-jira issue view PROJ-123 --site work --jq '.fields.status.name'
atl-conf page view 123456 --site work --json='*'
atl-bb pr view 7 --repo workspace/repo --site work --json='*'

atl-jira api /myself --site work --json='*'
atl-conf api /spaces --site work --jq '.results[] | {id, key, name}'
atl-bb api /repositories/workspace/repo/pullrequests --site work --jq '.values[] | {id, title, state}'
```

## Recommended Agent Workflow

1. Identify the product and binary.
2. Resolve pasted URLs or compact identifiers before guessing resource shape.
3. Target the site explicitly with `--site`, unless `ATL_SITE` or a default site is clearly intended.
4. Prefer the narrowest typed command that exists.
5. Add `--json='*'` or `--jq` before parsing output.
6. Use `--limit`, `--all`, and `result_truncated` handling for list/search commands.
7. Use `api` only for official Atlassian endpoints not covered by typed commands.
8. For destructive commands, validate intent first and add `--yes` only when that command supports or requires it.

## References

Read only the reference needed for the task:

- `references/automation.md` - output, pagination, exit codes, URL resolution, browse, and `api`.
- `references/auth.md` - token styles, login examples, CI auth, status checks, and scopes.
- `references/jira.md` - Jira projects, issues, comments, worklogs, attachments, JQL, and status.
- `references/confluence.md` - Confluence spaces, pages, blogposts, comments, labels, attachments, CQL/text search, and mixed v1/v2 behavior.
- `references/bitbucket.md` - Bitbucket Cloud repositories, pull requests, pipelines, issues, source, branches, tags, deployments, and repository targeting.

Use repository docs as the source of truth when command behavior is unclear:
`docs/command-contract.md`, `docs/consuming.md`, `docs/auth-runbook.md`,
`docs/access-error-model.md`, and `docs/token-scopes.md`.
