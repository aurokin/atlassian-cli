# Bitbucket Rewrite Plan

## Status

Planning placeholder for the future `atl-bb` import-and-rewrite period. This is not an instruction to migrate Bitbucket now.

## Intent

Use legacy `bb` as the behavior oracle and working baseline, then bring Bitbucket up to the standards established by `atl-jira` and `atl-conf`.

The rewrite should improve:

- package boundaries and command composition
- shared auth/config/output/error foundations
- access-aware recovery UX
- generated docs, completions, man pages, and command metadata
- fixture coverage and golden JSON tests
- startup and common read-path performance where practical

## Compatibility posture

Compatibility means preserving user value and stable machine contracts, not preserving every internal implementation detail.

Preserve or deliberately migrate:

- command behavior used by current `bb` workflows
- config paths or automatic config migration
- JSON fields used by agents/scripts
- `--json`, `--jq`, and `--no-prompt` behavior
- URL `resolve` output for known Bitbucket URL fixtures
- recovery guidance that users already rely on
- repo-local `bb-cli` skill install path or a clear replacement

Allow improvement when covered by tests and documented:

- package layout
- command internals
- HTTP client behavior
- pagination helpers
- output rendering internals
- docs generation internals
- error classification and `Next:` guidance

## Required inputs before source import

- `bb-inventory.md`
- `shared-foundation-scorecard.md`
- `bb-compatibility-plan.md`
- current `bb` command metadata and generated docs snapshot
- golden fixtures for core output and JSON payloads

## Rewrite guardrails

- Keep legacy `bb` available until `atl-bb` reaches parity or an exception is explicitly accepted.
- Do not combine a mechanical source import with broad behavior changes.
- Prefer small rewrite PRs with focused compatibility tests.
- Every behavior-changing PR must say whether the change is compatible, intentionally breaking, or only internal.
- Performance work should be measured against representative core flows, not assumed.

## Candidate modernization targets

1. Shared `atl-*` output renderer.
2. Shared config and safe credential reference model.
3. Shared HTTP client with structured errors, retry/rate-limit handling, and request tracing.
4. Shared raw `api` command scaffold.
5. Shared URL `resolve`/`browse` framework with Bitbucket-specific parsers.
6. Shared docs/completion/man/metadata generation.
7. Shared fixture server and golden JSON assertion helpers.

## Performance targets to evaluate

- CLI startup time for `version`, `help`, and config-only commands.
- API call count for common read flows: repo view, PR view, pipeline status, issue view.
- Pagination behavior for repository, PR, and pipeline lists.
- Memory use for large list outputs.
- Avoiding unnecessary subprocesses or repeated config reads.

## Non-goals

- Do not fake Jira/Confluence parity in Bitbucket commands.
- Do not break stable JSON fields just to match a new internal model.
- Do not make live Bitbucket tests mandatory in normal CI unless fixtures and credentials are explicitly provisioned.
- Do not remove legacy `bb` before the alias/wrapper/deprecation decision is documented.
