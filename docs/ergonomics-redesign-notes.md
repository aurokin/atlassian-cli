# CLI ergonomics: later redesign

Recorded 2026-09-15. This is a backlog note, not an approved interface design.

The owner recalls the CLI feeling clunky for agents in an earlier workplace:
common settings repeatedly needed attention, and configuration was error prone.
The exact historical failures have not been reproduced. Preserve this concern
for a later redesign rather than treating API correctness as sufficient usability.

Investigate:

- How an agent discovers the correct product, site, project, space, workspace,
  and repository without repeatedly supplying settings or guessing defaults.
- Whether auth setup can expose token type, required scopes, cloud ID, expiry,
  and a useful live verification step with fewer manual decisions.
- Whether errors identify the failed setting and give a concrete recovery
  action in both human and structured output.
- Whether command discovery, pagination, output shapes, and confirmations are
  consistent enough for agents to compose ordinary workflows reliably.

Observed during credential restoration: Bitbucket scoped API tokens require
the CLI's `cloud-classic` setting, while Jira/Confluence scoped tokens require
`cloud-scoped` plus a cloud ID. This distinction is easy to misinterpret.
Also, `atl-bb workspace list` printed help and exited successfully despite no
implemented `list` subcommand. Capture this as a command-discovery/error case.

Use the expanded E2E suite to collect concrete failed attempts and configuration
retries. A later redesign should measure completion of common agent workflows
from a fresh configuration, including recovery from wrong or missing settings.
Keep explicit targeting and non-interactive operation as constraints; do not
silently choose an arbitrary personal site to reduce the number of flags.

Live read-only-token tests also returned HTTP 401 with `Unauthorized; scope does
not match` from Jira/Confluence despite successful identity and content reads.
Consider explicit scope-recovery guidance in errors so agents can distinguish
a valid token missing write scope from an invalid token. Preserve the documented
upstream status and exit-code contract when evaluating that redesign.
