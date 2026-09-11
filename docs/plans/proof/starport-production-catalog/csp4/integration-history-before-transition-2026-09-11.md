# CSP4 authority and retained startup

## Current runtime integration

The plan [delivery checklist](../../starport-production-catalog-plan.html#csp4-deliveries) separates merged work from the remaining CSP4 deliveries.
Starmap PRs #146, #148, #149, and #150 merged. Twenty-eight campaign PRs merged across nine completed tasks.
CSP4 remains active because native qualification, production clock integration, explicit transitions, and delivery merges remain open.

### Consumer checks and published dependency

Starport `80f8064e` adds four authority acceptance cases. Its [proof](consumer-acceptance-2026-09-11/verification.json) records 45 passing race events across five repetitions.
The cases cover excluded embedded membership, cold refusal, retained Badger startup, and unsupported-schema permission handling.
An unchanged permission allows the retained catalog. A new permission requirement blocks it before replacement activation.
A different authority or policy also refuses retained inference.

The tests use real Badger candidate, accepted, and lease stores with a deterministic source and qualified clock.
They do not qualify native clocks or live provider routing.
The first schema fixture confused a new publication with a permission withdrawal. The corrected table covers both cases and preserves that initial failure.

Commit `1da7a26` makes the architecture version expression match the existing release verifier.
All twelve architecture checks pass, including the full Go suite. Module replacement rejection remains unchanged.
Starport resolves published Starmap `v0.16.6-0.20260911063726-852a548c4959` with `GOWORK=off`.
The final stable released-pair qualification remains required.

### Task verifier

The [registry proof](consumer-registry-2026-09-11/verification.json) records eight passing CSP4 subcases with no skipped named tests.
Four consumer mappings use ten named checks across catalog, application, router, cache, and HTTP boundaries.
The initial run skipped the absent signed fixture. Preparation verified its published size and checksum.
The prepared run passed four public subcases and reported four missing consumer mappings. The registered run passes all eight.

This task gate records component checks. Full A09, A10, and A21 qualification remains with CSP10.
Production native clock composition, remaining Starport delivery checks, required review, publication, and merge remain open.

### Remaining native and origin delivery

PR #151 contains reviewed `75ac9d07`. The [production proof](windows-rpc-2026-09-11/verification.json) records four Windows cross-builds, static checks, and zero findings from Sol and Opus.
Both Windows access preflights pass in native workflow [34609335965](https://github.com/agentstation/starmap/actions/runs/34609335965). Full native and repository gates remain in progress.
All required checks must pass before merge.

The [diagnostic proof](windows-security-2026-09-11/verification.json) identifies RPC identification-only access as the blocker on both Windows architectures.
Local RPC impersonation succeeds while the pipe retains identification-only access. Enabling the time privilege alone does not resolve the failure.
The production correction retains packet privacy, peer validation, and reply bounds. It refuses delegation and anonymous authentication.
It enables no privilege and changes no time setting.

Clock configuration `e97d7fc3` passes all 41 repository verifier stages. Its [settings proof](clock-settings-2026-09-11/verification.json) retains those checks.
Native parent integration, final review, publication, and merge remain required.

Origin `bb28a0bd` protects replica restart ownership after the periodic adoption work at `9ffcb0f8`.
The [restart proof](origin-restart-2026-09-11/verification.json) records 21 origin and two lease race events per supported toolchain.
A replica with missing retained inputs serves accepted state without an initial lease request or shared publication.
A replica with matching inputs can take ownership without replacing the accepted generation. Read-only startup needs no provider bindings.

The tests use a memory store and a stub lease. CSP11 owns full shared input recovery, equivalent acquisition capability, and atomic lease/head fencing.
Explicit authority transitions, native parent integration, full delivery checks, review, publication, and merge remain required.
The [earlier checkpoint](integration-history-before-rpc-2026-09-11.md) preserves the previous current record.

