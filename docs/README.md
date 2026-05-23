# Documentation

Documentation index for `atlassian-cli` — three separate CLIs (`atl-jira`,
`atl-conf`, `atl-bb`) sharing a common foundation where useful but keeping
distinct product vocabularies. The implemented behavior lives in
[command-contract.md](command-contract.md); the docs below explain how the
system is built and how to use it.

## Living docs

- [command-contract.md](command-contract.md) — the implemented command surface, config schema, and known limitations. The canonical reference.
- [shared-architecture.md](shared-architecture.md) — the shared packages, raw `api` escape hatch, output rendering, config, and pagination.
- [auth-design.md](auth-design.md) — the auth model: cloud-classic / cloud-scoped tokens, Data Center PAT, and OAuth 3LO.
- [auth-runbook.md](auth-runbook.md) — practical end-to-end authentication guide: pick a token style, supply and store the token, log in per product, verify, and troubleshoot.
- [access-error-model.md](access-error-model.md) — permission-aware UX and the structured error envelope.
- [integration-testing.md](integration-testing.md) — the live, opt-in integration suite that drives the real binaries against a real tenant.

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
