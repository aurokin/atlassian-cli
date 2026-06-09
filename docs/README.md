# Documentation

Documentation index for `atlassian-cli` — three separate CLIs (`atl-jira`,
`atl-conf`, `atl-bb`) sharing a common foundation where useful but keeping
distinct product vocabularies. The implemented behavior lives in
[command-contract.md](command-contract.md); the docs below explain how the
system is built and how to use it.

## Living docs

### Using the CLIs

- [command-contract.md](command-contract.md) — the implemented command surface, config schema, and known limitations. The canonical reference.
- [consuming.md](consuming.md) — using the CLIs from scripts and agents: install from a release, the output and exit-code contracts, pagination, and non-interactive use.
- [auth-design.md](auth-design.md) — the auth model: cloud-classic / cloud-scoped tokens, Data Center PAT, and OAuth 3LO.
- [auth-runbook.md](auth-runbook.md) — practical end-to-end authentication guide: pick a token style, supply and store the token, log in per product, verify, and troubleshoot.
- [token-scopes.md](token-scopes.md) — product-neutral starter scope sets for Jira, Confluence, and Bitbucket Cloud API tokens.
- [access-error-model.md](access-error-model.md) — permission-aware UX, the structured error envelope, and the process exit-code table.

### Building & maintaining

- [shared-architecture.md](shared-architecture.md) — the shared packages, raw `api` escape hatch, output rendering, config, and pagination.
- [engineering-notes.md](engineering-notes.md) — contributor conventions and gotchas: validation-before-auth, the nil-body trap, the reusable-helper inventory, destructive-verb rules, and local gates.
- [releasing.md](releasing.md) — versioning posture and how a release is cut (tag → GoReleaser), plus pipeline-maintenance constraints.
- [integration-testing.md](integration-testing.md) — the live, opt-in integration suite that drives the real binaries against a real tenant.
- [adr/](adr/) — architecture decision records: the *why* behind standing choices (exit codes, the shared foundation, `--yes`, the mixed-version Confluence client, token storage, verbatim JSON, generated-docs delivery).

## History

The completed phase plans, per-product MVP specs, the OAuth 3LO design, and the
Bitbucket (`atl-bb`) import/rewrite arc are kept as frozen historical records
under [archive/](archive/). They are not maintained; consult the living docs
above for current behavior.

## Naming

`atl-jira`, `atl-conf`, and `atl-bb` are the binary names. The `atl-` prefix
avoids collisions with common packages and makes these feel like one CLI family.
Avoid bare `jira`, bare `confluence`, `jj`, `cc`, and `conf` because of
collisions or ambiguity.
