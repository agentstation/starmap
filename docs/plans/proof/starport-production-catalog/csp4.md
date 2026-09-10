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
