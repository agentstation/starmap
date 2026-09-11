# Integration evidence before cancellation and follower delivery

## Current runtime integration

The plan [delivery checklist](../../../starport-production-catalog-plan.html#csp4-deliveries) separates merged work from the remaining CSP4 deliveries.
CSP4 still requires operator configuration, shared-store followers, explicit transitions, and four Starport consumer checks.
Windows qualification is one remaining delivery, not the sole blocker.

PR #150 merged as `852a548c` with fifteen successful checks and exact reviewed-tree equality.
Twenty-eight campaign PRs merged, and CSP4 remains active.

Both Windows jobs at `e4a80fa6` pass 1,913 events and fail only the successful empty-principal check.
Correction `e6bf2fce` supports that unnamed endpoint through a null SSPI target on the verified local pipe.
Its [proof](windows-unnamed-2026-09-11/verification.json) records seventeen passing local checks, review results, and native evidence.
Sol and Opus report zero findings, and the secret scan passes. PR #151 now contains the correction.

Workflow `34593352017` passes 22 Windows preflight events per architecture. The status RPC still returns access denied on both architectures.
The full Windows suites do not run after that mandatory failure. Native qualification remains open.

Clock configuration `e97d7fc3` adds eight canonical host settings and passive native composition.
Its [proof](clock-settings-2026-09-11/verification.json) records 35 focused race events per toolchain with one Windows-only skip.
Corrected settings packages pass 112 events. Four Windows cross-builds, static checks, generated docs, and the 1,490-file prose check pass.

All 41 repository verifier stages pass at `e97d7fc3`. The proof retains the complete log and final metadata.
Integrate the qualified native parent before final review and publication.

The [monitor proof](clock-lifecycle-2026-09-11/verification.json) and [runtime proof](clock-runtime-2026-09-11/verification.json) retain the earlier lifecycle checks.
Production clock bounds, configured permission owners, shared-store followers, authority transitions, and four Starport consumer checks remain open.

Origin selection `8206a2a8` refuses ordinary startup with an authoritative catalog store, including after runtime-directory replacement.
Its [proof](authority-selection-2026-09-11/verification.json) records three failing events before the guard and twelve focused passing events per toolchain.
Static checks, generated docs, and the 1,491-file prose check pass. Integration, full verification, and publication remain required.

### Origin configuration delivery

Commit `b76b9f10` adds one complete origin declaration across YAML, environment, and CLI inputs.
An enabled origin uses the application's canonical catalog store and native clock monitor.
A higher-priority declaration replaces every lower origin field, including bootstrap permission and receipt lifetime.
Disabling issuance preserves the selected store and refuses implicit ordinary startup with retained authority.

The [settings proof](origin-settings-2026-09-11/verification.json) records three fail-before events and the initial 46 passing focused events.
Configuration packages pass 120 events per toolchain. Runtime and CLI checks pass 49 events per toolchain.
Native parent integration, complete verification, review, publication, and merge remain required.
Shared-store followers, explicit authority transitions, and four Starport consumer checks remain open.

