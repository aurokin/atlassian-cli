# Bitbucket CLI Migration Roadmap

## Status

Long-term roadmap, not an early implementation constraint. The existing legacy `bb` CLI is already useful and should not be destabilized to satisfy architecture aesthetics. There will likely be an explicit import-and-rewrite period where Bitbucket is brought into the new `atl-*` standards rather than copied over unchanged.

## Goal

Eventually bring the Bitbucket CLI into the same Atlassian CLI ecosystem if, and only if, Jira and Confluence prove that a shared foundation improves reliability and development speed without blurring product-specific behavior.

The target state could be either:

1. **Full monorepo with rewrite:** one repository builds `atl-bb`, `atl-jira`, and `atl-conf` binaries. Legacy `bb` is imported as source/reference material, then refactored onto the new foundation and standards.
2. **Shared foundation module:** legacy `bb` remains in its current repo but imports or vendors a shared Atlassian foundation, with `atl-bb` added when the rewrite is ready.
3. **No migration yet:** `atl-bb` is postponed and repeated code stays duplicated where product differences make sharing expensive.

Do not choose the final shape until after Jira and Confluence MVPs are real.

## Why delay the migration

Auro prefers developing separate CLIs first, then doing a larger refactor and reenvisioning once patterns are obvious. That should guide this roadmap.

Reasons to delay:

- Legacy `bb` has an existing command surface, docs generator, tests, and installed skill.
- Jira and Confluence auth/routing are more complex than Bitbucket Cloud and may change the shared foundation shape.
- Premature sharing can force fake abstractions over product-specific semantics.
- A migration should improve Bitbucket CLI users and establish `atl-bb`, not merely relocate code.
- The new Jira and Confluence CLIs should set the bar: better package structure, faster command startup where practical, clearer errors, stronger tests, and a cleaner agent-facing JSON contract.

## Import-and-rewrite posture

Expect a transition period where Bitbucket is imported, audited, and rewritten toward the new standards. Treat legacy `bb` as the working baseline and behavior oracle, not as the architecture to preserve wholesale.

During this period:

- keep legacy `bb` available until `atl-bb` reaches parity or has an explicit compatibility exception
- use the new Jira/Confluence foundation as the preferred base for Bitbucket, not the other way around
- preserve proven Bitbucket behaviors that users and agents rely on
- replace weak internals when the new foundation is clearly better
- add compatibility fixtures before changing output, config, auth, URL resolution, or recovery guidance
- prefer measured performance improvements: fewer redundant API calls, bounded pagination, faster startup, streaming where useful, and no avoidable shell-outs
- document intentional behavior changes in `bb-compatibility-plan.md`

The rewrite should be allowed to improve command organization, package boundaries, docs generation, tests, and recovery UX. Compatibility means preserving user value and stable machine contracts, not freezing every internal design choice.

## Migration gates

Do not migrate legacy `bb` or introduce `atl-bb` until these gates pass:

1. `atl-jira` and `atl-conf` both have working foundation commands: `auth`, `config`, `api`, `resolve`, `browse`, `version`.
2. Cloud auth supports classic tokens, scoped tokens, and Data Center PATs with clear recovery guidance.
3. Shared output, config, error, HTTP, and pagination packages have real usage in at least two product CLIs.
4. Access-aware UX has fixture coverage for low-access, missing-scope, non-admin, ambiguous 404, and product/license-missing cases.
5. Generated docs/man/completions/metadata pipeline works for both new CLIs.
6. `atl-bb` migration has a compatibility plan for legacy `bb` for config paths, binary name, docs, repo-local skill, releases, and existing user workflows.
7. The new foundation has explicit quality/performance standards that Bitbucket can adopt without regressing existing workflows.

## Candidate shared code to extract before migration

Good candidates:

- output renderer: tables, JSON selected fields, `--jq`, human `Next:` guidance
- config mechanics: path resolution, safe permissions, aliases, settings, default host/site
- auth framework: token storage, signing interfaces, auth status payloads
- HTTP client: API error parsing, retry/rate-limit helpers, request context, trace hooks
- recovery catalog: structured `401`, `403`, `404`, scope, permission, rate-limit, and platform-limit errors
- raw `api` command scaffolding
- URL `resolve` / `browse` framework with product-specific parsers
- docs generation: CLI reference, examples, flag matrix, command metadata, man pages, completions
- test harness: fixture HTTP server, golden output, JSON field validation, live-test gating

Poor candidates until proven otherwise:

- product resource models
- product command vocabulary
- Bitbucket workspace/repo/PR/pipeline semantics
- Jira workflow/transition semantics
- Confluence body/version/page-tree semantics
- product-specific permission/scope maps

## Proposed phases

### Phase B0: Inventory legacy `bb`

Run a focused inventory of the current Bitbucket CLI:

- command tree and generated docs pipeline
- config schema and migration constraints
- auth assumptions and Bitbucket-only limits
- output renderer and JSON field selector
- raw API command behavior
- URL resolver and browser URL builder
- recovery catalog and error handling
- test helpers and live-smoke conventions

Output: `docs/bb-inventory.md`.

### Phase B1: Shared-foundation comparison

Compare legacy `bb` internals with the new Jira/Confluence foundation.

For each candidate shared package, decide:

- identical enough to share now
- similar but should be adapted later
- product-specific, keep separate
- wrong abstraction, delete or redesign

Output: `docs/shared-foundation-scorecard.md`.

### Phase B1.5: New-standards rewrite plan

Before moving Bitbucket source, write the modernization plan that says what should be preserved, rewritten, or intentionally changed.

Must cover:

- package structure and command composition target
- output/JSON compatibility guarantees
- config/auth migration and fallback behavior
- recovery/error model upgrades
- performance opportunities and non-goals
- docs/man/completion generation strategy
- test coverage needed before replacing legacy internals

Output: `docs/bb-rewrite-plan.md`.

### Phase B2: Compatibility design

Design migration without breaking users.

Must cover:

- introduce canonical binary name `atl-bb`
- clean break on the binary name: **no `bb` alias/wrapper/shim or
  deprecation window** (decision confirmed in B2) — `atl-bb` replaces `bb`
- preserve current config path or provide automatic migration
- preserve command behavior and JSON fields unless explicitly versioned
- preserve repo-local `bb-cli` skill installability or provide a clean replacement that documents `atl-bb`
- preserve generated docs URLs or redirects where applicable
- preserve manual live-test boundaries

Output: `docs/bb-compatibility-plan.md`.

### Phase B3: Extract shared libraries

Extract only proven shared code behind stable internal APIs.

Rules:

- no product command behavior changes in the same PR as extraction unless unavoidable
- golden tests must prove output compatibility
- config migration tests must run before any path changes
- legacy `bb` continues to build and pass its existing checks throughout the rewrite as the source/behavior oracle; it is not shipped alongside `atl-bb` (clean break — no alias/wrapper)

### Phase B4: Move or integrate Bitbucket as `atl-bb`

Choose one:

- import Bitbucket source into the monorepo as a rewrite baseline for `atl-bb`, preserving Git history if practical
- keep legacy `bb` in its repo and add/build `atl-bb` against a shared foundation
- postpone migration if the compatibility cost is too high

If source is imported, expect follow-up rewrite PRs rather than a single mechanical move. Each rewrite PR should keep tests green and clearly state whether behavior is preserved, improved compatibly, or intentionally changed.

### Phase B5: Release and docs transition

After migration/integration:

- publish release notes explaining the `atl-bb` name and any legacy `bb` compatibility behavior
- update install docs
- update skill docs
- update generated command metadata
- run manual smoke tests against existing Bitbucket fixtures

## Compatibility checklist for `atl-bb` and legacy `bb`

Before declaring migration done:

- `atl-bb auth login/status/logout` works with migrated config; legacy `bb auth login/status/logout` either still works or has an explicit transition path
- `atl-bb api` behavior matches intended Bitbucket API behavior; legacy `bb api` compatibility is deliberate
- `atl-bb resolve` outputs compatible JSON for known URL fixtures
- `atl-bb browse --no-browser` outputs compatible URLs
- `--json`, `--jq`, and `--no-prompt` behavior remains compatible
- generated docs are regenerated and reviewed
- repo-local `bb-cli` skill still points to valid install/use instructions
- live tests remain manual-only unless explicitly provisioned
- `make check` passes in the migrated location
- startup/API-call performance is no worse than legacy `bb` for core read paths, or any regression is documented and accepted

## Open questions

1. Should the eventual monorepo be named around Atlassian generally, or around developer CLIs more broadly?
2. Should legacy `bb` preserve its current repository as the public home even if source moves?
3. Should shared code be private/internal forever, or become a versioned Go module?
4. How much Git history preservation matters for a future source move?
5. Should `atl-bb` remain Bitbucket Cloud only, or should the shared foundation make Bitbucket Data Center support easier later?
