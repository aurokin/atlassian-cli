# 0001 — Per-category process exit codes

**Status:** Accepted (referred to during the build as "D1")

## Context

The CLIs are built for scripts and agents as much as for people. A caller that
gets a non-zero exit needs to know *what kind* of failure occurred — re-auth,
back off, "no such thing," or "you sent bad input" — to react correctly.
Parsing stderr or the `--json` envelope to recover the category is brittle and
forces every consumer to special-case our message wording.

We already classify failures into a stable set of `apperr` codes
(`unauthorized`, `forbidden`, `not_found_or_not_visible`, `rate_limited`,
`invalid_input`, `timeout`, …). The question was whether to also surface that
category through the *process exit code*.

## Decision

Map the common, actionable failure categories to **distinct process exit
codes**, and let everything else collapse to the generic `1`:

| Exit | Category |
|---|---|
| `0` | success |
| `4` | `unauthorized` |
| `5` | `forbidden` |
| `6` | `not_found_or_not_visible` |
| `7` | `rate_limited` |
| `8` | `invalid_input` |
| `9` | `timeout` |
| `1` | everything else (generic/uncategorized) |

Codes start at `4` to stay clear of `0` (success), the conventional `1`
(generic error, which is also where a usage error from the argument parser
lands), and `2` (the shell's conventional usage-error code, left unused here).
Only categories a
caller would plausibly *branch on* get a dedicated code; rarer categories
(`http_error`, `gone`, `feature_disabled`, `untrusted_url`,
`response_decode_failed`, `result_truncated`, `request_failed`) intentionally
share `1` rather than inflating the table with codes nobody branches on.

The exit code and the `--json` error envelope carry the **same** stable code,
so a consumer can use whichever is convenient (exit code for control flow,
envelope for an in-band reason).

## Consequences

- Scripts branch on failure kind with a `case $?` and no output parsing — see
  the example in [consuming.md](../consuming.md#exit-codes--branch-without-parsing-output).
- The exit-code table is now a **public contract**. Reassigning a number is a
  breaking change for consumers; new categories should map to `1` unless they
  are genuinely branch-worthy, in which case they get a new number (never a
  reused one).
- The canonical, always-current table lives in
  [access-error-model.md](../access-error-model.md#process-exit-codes); this ADR
  records why it exists and why the numbering is shaped the way it is.
