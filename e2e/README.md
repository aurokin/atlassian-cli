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

`coverage.json` inventories all 152 canonical runnable commands. Every row now
requires process or live evidence; package tests and an exclusion reason alone
cannot satisfy the gate. Referenced files/test names are checked for staleness.
This still does not prove every flag, authentication mode or live capability.

Additional real-binary workflows cover auth defaults/logout and targeting
precedence; aliases/quoting/cycles; offline resolve and browser suppression;
extension arguments/exits/PATH/symlinks and Windows PATHEXT/case matching;
generated completion callbacks; raw body methods and output shapes; download
faults that preserve destination files; and Bitbucket pipelines, deployments,
project administration and independent reviewer requests against local fixtures.

Run required local shells with
`ATL_E2E_REQUIRED_SHELLS=bash,zsh,fish go test ./e2e -count=1`.
CI requires those shells on Unix and PowerShell on Windows. Missing required
shells fail. Zsh captures callback candidates before interactive shell matching.

`ATL_E2E_NATIVE_CREDENTIALS=1` enables native credential lifecycle tests only on
ephemeral macOS/Windows CI hosts. Do not set it on a personal workstation.
The test uses unique dummy credentials and verifies deletion and preservation
of another test profile. Ordinary local tests isolate file-backed stdin storage
on Unix and never touch the personal keychain. Native results must be recorded
separately; default local skips do not certify that backend.

For live tests, use the separately invoked
[matrix runner](../docs/integration-testing.md). Its result-accounting tests run
with `python3 -m unittest discover -s scripts -p '*_test.py'`; no live service is
contacted by those checks. See [the merge gate](../docs/e2e-merge-gate.md) for
required capabilities, explicit exclusions and final evidence expectations.
