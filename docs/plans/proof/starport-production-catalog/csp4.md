# CSP4 authority and retained startup

## Current runtime integration

CSP4 remains in progress. All eight assigned acceptance subcases remain UNVERIFIED.
Local commit `a9b369dca409f4d47813da32b092622fb3a4aedd` follows ownership fix `ea4f7f11` and runtime integration `9ddf5fd0`.
The [runtime proof](csp4/runtime-integration-2026-09-10/verification.json) binds 47 changed files to the committed tree.

Each final focused suite passes 29 test events and two package outcomes.
The suites use Go 1.26.6 and Go 1.25.12.
Lint, ago, generated documentation, and prose checks pass. The prose check covers 1,400 files with zero diagnostics.
The complete root race suite passes 105 test events and one package outcome.

The runtime retains the highest authenticated requirement separately from its finite receipt.
It checks configured authority and policy identities before activation.
The private `catalog-runtime/permission.json` checkpoint stays uncertain until shutdown stops every reader and retains the complete state.
A crash needs a fresh verified receipt before admission. A clean restart can use a valid retained receipt with a qualified clock.

The runtime reserves client publication through cumulative guards on `Update`, `Activate`, and `Rollback`.
Direct calls cannot alter the serving catalog or add alias history that prevents trusted-source recovery.
Local provider acquisition stops before provider calls in `require_authority` mode.
The serving client and retained authority catalog align before readiness.

A same-permission renewal preserves the confirmed lease and its original expiry until the replacement receipt is durable.
A changed requirement or unknown permission semantics blocks new attempts immediately.
A catalog manifest cannot restore a rejected permission receipt.
Admission reads the runtime snapshot and cached clock evidence. These component checks do not measure Starport overhead.

The proof retains failures for local acquisition, warm client alignment, direct client publication, rollback, ordinary renewal, and rejected-receipt recovery.
An intermediate broad run passed 1,012 test events and failed three settings checks.
The complete settings repair suite passes 98 test events plus two package outcomes.
The proof preserves an earlier root API-list failure and two command syntax errors that ran no tests.
None of these intermediate runs qualifies the final runtime tree.

The [generation-binding proof](csp4/generation-binding-2026-09-10/verification.json) records a second publication defect and its repair.
The runtime now activates the original authority generation with its complete manifest and payload.
The exact generation survives refresh and retained restart. Both toolchains pass 27 authority test events and one package outcome.

An unchanged generation requires no catalog-store write. This preserves retained diagnostics during a write outage.
The complete runtime, remote, and artifact race suite remains running. Current resume state names its session and output path.

Next, complete publisher receipt issuance, the host clock adapter, shared-store follower activation, authority transitions, and required consumer checks.
Full CSP4 verification, review, native CI, PR, and merge remain pending.
No owner decision remains. No task completion credit applies.

## Historical permission-envelope checkpoint

CSP4 remains in progress. All eight assigned acceptance subcases remain unverified.
Local commit `1f1b01148f86115a053a0dc8ac9fd77c5b8a9b47` adds the permission-envelope contract.
The [primitive proof](csp4/permission-envelope-2026-09-10/verification.json) records 1,212 catalog race events and 54 minimum-toolchain events.
Both suites pass without failed or skipped events. Static checks pass.

The envelope binds authority, policy, publication sequence, payload identity, and required permission revision.
A renewal can extend a receipt without changing the publication head.
Its time check uses the approved five-minute lifetime and thirty-second uncertainty ceiling.
Unknown clock validity refuses the receipt. The isolated time check allocates no memory.
This result does not measure Starport request overhead.

The strict parser rejects duplicate fields, case variants, unknown fields, invalid UTF-8, trailing documents, and oversized envelopes.
Three case-variant regressions fail before the parser fix.
Unknown positive permission versions remain readable so a consumer can record a withdrawal before refusing unsupported semantics.

The [startup baseline](csp4/startup-baseline-2026-09-10/verification.json) retains the two failing authority probes.
The [preparation](csp4-preparation-2026-09-10.md) names the transport and retention boundaries.
Independent authenticated receipt reads, publication, retention, runtime activation, and admission enforcement remain incomplete.
Complete those paths before task qualification or publication.

The [context record](csp4/permission-envelope-2026-09-10/context-maintenance.json) verifies unchanged task contracts, goal, and invariants during history rotation.

## Independent permission transport

Local commit `12c89f6901c7fb07fe7257fca87c138d60458bf6` adds the independent permission request.
The [transport proof](csp4/permission-transport-2026-09-10/verification.json) records 51 remote-client race events and 16 minimum-toolchain events.
All pass without skips. Static checks pass.

The transport verifies the configured publisher before decoding the envelope.
It bounds the body before parsing and preserves cancellation and refusal deadlines.
A renewal reads no catalog payload. Runtime and publisher integration remain incomplete.

## Committed authority binding

Local commit `115f0c7e4a6b7c5f848ca2918bdc7c6db97fa974` adds authority manifest version 3.
The [manifest proof](csp4/authority-manifest-2026-09-10/verification.json) records 1,525 catalog-package race events and 72 minimum-toolchain events.
All pass without skips. Static checks pass.

The manifest stores the authority head with its catalog generation.
Validation binds the generation ID and payload digest. Ordinary manifests retain version 2.
Memory, filesystem, and the conditional object adapter preserve this binding through compare-and-swap.
The object test uses the existing memory backend fixture. It does not qualify an external service.

The filesystem interruption test preserves the prior catalog and permission revision after reopen.
A retry publishes the complete replacement. The permission receipt can renew against that immutable head.
Publisher policy, receipt issuance, retained permission state, and runtime admission remain incomplete.

## Runtime permission state draft

The active worktree contains seven uncommitted files above `d2c738d6`.
The [draft proof](csp4/runtime-state-draft-2026-09-10/verification.json) preserves their exact bytes, test output, and remaining lint findings.
The permission state passes nine race events on each Go toolchain.
The full remote-source suite passes 45 events. Its focused minimum-toolchain test passes.

The late-retention regression fails before the fix. A pending confirmation now cannot restore permission after an invalid authenticated receipt.
The isolated state check allocates no memory. It does not measure gateway latency.

Whole lint reports five unused private declarations.
Connect the state to startup, independent receipt refresh, durable retention, and readiness before clearing those findings.
Do not suppress them or count the draft as task completion.

The [main integration](csp4/main-integration-2026-09-10.json) records actual CSP3 merge `a87262e3`.
The integration preserves the qualified committed tree and every draft file.
