# Architecture Decision Records

Standing decisions for `atlassian-cli` — the *why* behind choices that shape
the codebase and the product contract. Living behavior lives in
[command-contract.md](../command-contract.md); these records capture the
reasoning so a future contributor (human or agent) doesn't relitigate a settled
call or quietly undo it.

Each ADR is short and follows the same shape: **Status**, **Context**,
**Decision**, **Consequences**. An ADR is immutable once accepted; to change a
decision, add a new ADR that supersedes it rather than editing history.

| # | Decision | Status |
|---|---|---|
| [0001](0001-per-category-exit-codes.md) | Per-category process exit codes | Accepted |
| [0002](0002-shared-foundation.md) | Shared foundation, and what is deliberately not shared | Accepted |
| [0003](0003-destructive-verbs-require-yes.md) | Destructive verbs require `--yes` | Accepted |
| [0004](0004-mixed-version-confluence-client.md) | Confluence is a mixed-version (v2 + v1) client | Accepted |
| [0005](0005-keychain-first-token-storage.md) | Keychain-first token storage; tokens never in config | Accepted |
| [0006](0006-verbatim-json-no-fake-parity.md) | Verbatim upstream JSON; no fake parity | Accepted |
| [0007](0007-generated-docs-as-release-asset.md) | Generated command reference: release asset, not committed | Accepted |

Several of these were referred to during the build as "D1/D2/D3" — D1 is
[0001](0001-per-category-exit-codes.md), D2 is [0002](0002-shared-foundation.md),
D3 is [0003](0003-destructive-verbs-require-yes.md).
