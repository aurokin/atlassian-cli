# 0007 — Generated command reference: ship as a release asset, don't commit it

**Status:** Accepted

## Context

`cmd/gen-docs` produces a per-command Markdown reference (the cobra doc tree)
for all three binaries. The question is where that generated output should live
so a consumer can browse it. The realistic options:

1. **Generate on demand, never commit.** One source of truth (the command
   tree), zero PR churn — but nobody can browse per-command help on GitHub
   without cloning and running `make docs`.
2. **Commit the tree, gated by a CI drift check** (regenerate + `git diff
   --exit-code`). Browsable and linkable on GitHub, but every command change
   spreads across dozens of generated files, and the tree rots instantly
   without the drift gate.
3. **Generate at release time and publish** (docs site or release asset). Clean
   repo, versioned reference per release — needs a publishing step.

This repo already has a strong hand-maintained reference in
[`command-contract.md`](../command-contract.md) (capability matrix, rationale,
known limitations), which is better for a human/agent reader than a raw cobra
dump. So committing the generated tree would mean maintaining two references and
adding generated-file noise to nearly every command PR.

## Decision

Keep the generated tree **generated on demand and gitignored** (`/docs/cli/`),
and **attach a versioned bundle to each release** instead of committing it.

A GoReleaser `before` hook runs `gen-docs` into a throwaway, gitignored
`dist-docs/` and tars it to `atlassian-cli_docs.tar.gz`, which `release.extra_files`
uploads alongside the binary archives (see [releasing.md](../releasing.md)). The
hand-maintained `command-contract.md` remains the canonical in-repo reference.

This is option 3, scoped down: a release asset rather than a full docs site,
because the project is pre-1.0 and private and doesn't yet warrant a published
site.

## Consequences

- A consumer gets a browsable, version-matched command reference without a Go
  toolchain (download the release asset) — documented in
  [consuming.md](../consuming.md).
- `main` stays clean: no generated-file churn in PRs, and no drift gate to
  maintain. `make docs-check` still smoke-tests that the walker *runs*; it does
  not diff committed output (there is none to diff).
- If a real docs site / GitHub Pages is stood up later, it can serve the same
  generated tree (and the brand assets' `og:image`/favicon) — this decision
  doesn't preclude that; it's the lighter-weight step until then.
- Don't "tidy up" by committing the generated tree without also adding a drift
  gate — that reintroduces exactly the rot this decision avoids.
