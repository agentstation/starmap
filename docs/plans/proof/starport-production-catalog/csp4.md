# CSP4 authority and retained startup

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
