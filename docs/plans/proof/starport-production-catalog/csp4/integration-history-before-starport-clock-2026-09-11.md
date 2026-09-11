## Current runtime integration

The plan [delivery checklist](../../../starport-production-catalog-plan.html#csp4-deliveries) separates merged work from the remaining CSP4 deliveries.
Starmap PRs #146, #148, #149, and #150 merged. Twenty-eight campaign PRs merged across nine completed tasks.
CSP4 remains active because production clock integration, complete delivery verification, and merges remain open.

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

All six native runtime jobs pass at `75ac9d07` in PR #151.
The [production proof](windows-rpc-2026-09-11/verification.json) retains all six artifacts and exact test counts.
Both Windows suites pass 1,925 events, and both Windows access preflights pass 23 events.
Each platform also passes two Git acquisition events and four publication events.
The proof identifies the separate Linux administrator check and opt-in Windows diagnostic skips.
The repository Verification Gate still runs in workflow [34609335965](https://github.com/agentstation/starmap/actions/runs/34609335965).

Origin `5066856d` scopes alias history to the selected authority and policy.
The [transition proof](authority-transition-2026-09-11/verification.json) records 109 passing race events per supported toolchain, static checks, and 1,514-file prose verification.
The cases cover authority and policy changes with retained or fresh runtime directories, failed publication, recovery, replacement activation, and retained restart.
Prior approval cannot authorize the changed context. Same-authority updates retain the alias-history checks.

The initial transition test reproduces an alias-history conflict across authorities.
Fresh runtime state also exposed startup alias validation against the embedded baseline. The correction retains diagnostics without granting permission.
The first implementation required restart recovery after its deliberately failed store write. The final test follows that existing journal contract.

Combined source `d1bb47e1` merges reviewed native parent `75ac9d07` without conflicts.
It contains clock lifecycle, origin settings, follower restart, and subscriber transitions in one delivery.
Full `make verify` runs in session `8874` against this unchanged source.
The current log is `.tmp/csp4-origin-integrated/verifier.log` in the origin-settings worktree.
Task checks, required review, publication, native CI, and merge remain required.

CSP11 owns full shared input recovery, equivalent acquisition capability, and atomic lease/head fencing.
Starport still needs production clock composition and its final delivery checks.
The [earlier checkpoint](integration-history-before-transition-2026-09-11.md) preserves the previous current record.

