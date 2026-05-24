# Releasing

How a release of the `atlassian-cli` binaries (`atl-jira`, `atl-conf`,
`atl-bb`) is cut, what it produces, and how to keep the pipeline healthy.

## Versioning posture

- **Semantic versioning**, tags prefixed with `v` (e.g. `v0.1.0`).
- **Pre-1.0 (`v0.x`).** The command surface and human output may still change
  between minor versions. The machine contract is more stable than the human
  one — see [consuming.md](consuming.md) for what is and isn't promised — but
  consumers should pin a version until `v1.0.0`.
- `v1.0.0` is the point at which the command surface and exit-code taxonomy
  become stable commitments. We are not there yet.

## How to cut a release

A release is triggered entirely by **pushing a `v*` tag** to the repository.
There is no manual upload step.

```bash
# From a clean, up-to-date main that already contains everything you want shipped.
git checkout main && git pull --ff-only

# Annotated tag (annotated, not lightweight — the message becomes part of the record).
git tag -a v0.2.0 -m "Release v0.2.0

<short summary of what's in this release>"

# Pushing the tag is what starts the release.
git push origin v0.2.0
```

Pushing the tag fires [`.github/workflows/release.yml`](../.github/workflows/release.yml),
which runs [GoReleaser](https://goreleaser.com/) (`goreleaser release --clean`)
against [`.goreleaser.yaml`](../.goreleaser.yaml). Watch it with:

```bash
gh run list --workflow=release.yml -L 1
gh run watch <run-id> --exit-status
gh release view v0.2.0          # inspect the published release + assets
```

The job has `permissions: contents: write` and checks out with
`fetch-depth: 0` (GoReleaser derives the changelog from the full history and
the previous tag — a shallow clone would break it).

### Tag hygiene

- Tag a commit that is already merged to `main` and green in CI. The release
  job does **not** re-run the test suite; it builds and publishes whatever the
  tag points at.
- The version baked into the binaries (`atl-* version`) comes from the tag via
  ldflags, so the tag *is* the version — there is no separate version constant
  to bump.
- To redo a botched release, delete the tag locally and remotely
  (`git tag -d vX; git push origin :vX`) and delete the GitHub release, then
  re-tag. Prefer cutting a new patch version over reusing a tag.

## What a release produces

One combined archive per platform, each bundling **all three** binaries, plus a
checksums file:

```
atlassian-cli_<version>_darwin_amd64.tar.gz
atlassian-cli_<version>_darwin_arm64.tar.gz
atlassian-cli_<version>_linux_amd64.tar.gz
atlassian-cli_<version>_linux_arm64.tar.gz
atlassian-cli_<version>_windows_amd64.zip
atlassian-cli_<version>_windows_arm64.zip
checksums.txt
```

Rationale for one archive instead of three: a user downloads a single
`atlassian-cli` archive and gets `atl-jira`, `atl-conf`, and `atl-bb` together —
they are one CLI family. See the install instructions in
[consuming.md](consuming.md).

The three builds are `CGO_ENABLED=0` static binaries. Each is stamped with the
same ldflags the `Makefile` uses for local builds —
`-X main.version / main.commit / main.date` — so a released binary's
`version` command reports the tag, the short commit, and the build date.

## Keeping the pipeline healthy

These are the non-obvious constraints. Changing them carelessly breaks the
release:

- **GoReleaser is pinned to `~> v2`** in both the workflow
  (`goreleaser-action`'s `version:` input) and validated against the current v2
  schema. `.goreleaser.yaml` uses the **modern v2 archive fields**
  (`archives[].ids`, `archives[].formats`, `format_overrides[].formats`). Older
  v2 releases (e.g. v2.5.x) used `builds`/`format` and will **reject** this
  config. **Do not downgrade the pin** to "fix" a schema error — fix forward.
- **Validate config changes locally** without installing GoReleaser:
  ```bash
  go run github.com/goreleaser/goreleaser/v2@latest check
  ```
  (Use a recent v2; the schema validation tracks the version you run.)
- **The action runtime tracks Node.** `goreleaser/goreleaser-action` is pinned
  to **`@v7`**, which runs on the Node 24 actions runtime. `@v6` ran on Node 20,
  which GitHub is force-migrating and then removing in 2026 — the v0.1.0 release
  run flagged this and #95 bumped it. When GitHub deprecates Node 24, bump the
  action major again; the inputs (`version: "~> v2"`, `args: release --clean`)
  have been stable across majors.
- **CI smoke-tests the doc walker, not the release.** `make docs-check` runs in
  the `check` CI job (and is part of the catch-all gate); it generates the
  command reference into a throwaway dir to catch a broken cobra command tree.
  It does **not** exercise GoReleaser — config correctness is covered by
  `goreleaser check`, run locally when you touch `.goreleaser.yaml`.

## Changelog

GoReleaser generates the GitHub Release notes from the commit log since the
previous tag, excluding `docs:`, `test:`, and `chore:` commits (see the
`changelog` block in `.goreleaser.yaml`). There is no hand-maintained
`CHANGELOG.md`; the GitHub Releases page is the changelog. Conventional-commit
prefixes on merge commits keep those notes readable.
