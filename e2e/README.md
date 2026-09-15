# CLI process contracts

Run `go test ./e2e -count=1`. The suite builds all three actual binaries once
and invokes them with a ten-second process deadline, temporary HOME/config,
an environment-referenced dummy token, and local HTTP fixtures. It never uses
the normal user configuration or writes a keychain credential. Git inference
cases require Git on PATH. These tests run with ordinary `go test ./...`.

Current assertions cover:

- Binary identity, help, generated Bash completion entry point, unknown flags.
- Real process exit codes and stderr envelopes for HTTP 400/401/403/404/410/429/500
  across all three products; raw API preservation of upstream error bodies.
- Request timeout exit 9, trace separation from JSON stdout, authentication
  header delivery, and absence of the dummy credential from output.
- Rejection of cross-origin raw API requests before credentials can escape.
- Environment-token login persistence and authentication in a later process.
- Selected irreversible-command confirmation and missing create-input errors
  before authentication or network access.
- Both Bitbucket PR listing commands crossing a page boundary with `--all`,
  the endpoint-specific default page size of 50, exact result IDs, and jq output.
- Confluence direct-child types in human and jq output; bodyless label removal
  and comment deletion producing the documented JSON objects.
- Bitbucket repository inference from real temporary Git checkouts using
  bitbucket.org, ssh.bitbucket.org (SCP and port-443 forms), and altssh.bitbucket.org.

`coverage.json` inventories all 152 canonical runnable commands, with explicit
process/live/package evidence or an unverified reason. The inventory test rejects
missing commands and stale test pointers. This is focused behavioral coverage;
inventory completeness is not full workflow coverage. The local suite does not prove
live API compatibility, cloud gateway routing, OAuth login/refresh, native
keychain behavior, attachments, all pagination families, extensions, or all
mutation workflows. Data Center PAT profiles are only a local fixture-routing
mechanism here. Generating Bash completion does not prove shell execution.
Native platform results must be reported separately; these cases do not
establish Windows-specific extension or process-tree termination behavior.

For live tests, use the separately invoked
[matrix runner](../docs/integration-testing.md). Its result-accounting tests run
with `python3 -m unittest discover -s scripts -p '*_test.py'`; no live service is
contacted by those checks.
