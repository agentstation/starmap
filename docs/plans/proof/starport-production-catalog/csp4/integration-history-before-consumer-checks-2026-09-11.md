## Current runtime integration

The plan [delivery checklist](../../../starport-production-catalog-plan.html#csp4-deliveries) separates merged work from the remaining CSP4 deliveries.
PRs #146, #148, #149, and #150 merged. Twenty-eight campaign PRs merged across nine completed tasks.
CSP4 remains active because native clocks, follower integration, explicit transitions, and four mapped Starport checks remain open.

### Native qualification and permission cancellation

PR #151 now contains `a6545493`. Its [proof](permission-cancellation-2026-09-11/verification.json) records the cancellation regression and native privilege diagnostic.
Cancellation before a permission source read preserves the accepted requirement. Cancellation after a new withdrawal retains the existing refusal behavior.
The regression fails before the fix. Forty-five retention race events pass per Go 1.25.12 and 1.26.6.

The Windows probe changes one held privilege in a disposable test process and restores its original attributes.
It leaves production privileges and the mandatory preflight unchanged. Sol reports zero findings.
Opus flags the known red native gate. The merge block remains. Disabling that gate would conceal the unresolved failure.

Workflow [34597460388](https://github.com/agentstation/starmap/actions/runs/34597460388) fails the Windows status call before and after privilege enablement on both architectures.
Enabling that privilege alone is insufficient. Investigate the verified local RPC authentication and impersonation contract. Other native jobs continue.

Its parent workflow 34593352017 passed 22 Windows preflight events per architecture and failed the status RPC with access denied.
The parent also failed the macOS Intel warm-start case. The deterministic cancellation defect can explain that failure, but native confirmation remains required.

Clock configuration `e97d7fc3` adds eight canonical host settings and passive native composition.
All 41 repository verifier stages pass at that source. The [settings proof](clock-settings-2026-09-11/verification.json) retains those checks.
Integrate the qualified native parent before final review and publication.

### Origin configuration and follower startup

Commit `b76b9f10` adds one complete origin declaration across YAML, environment, and CLI inputs.
An enabled origin uses the application's canonical catalog store and native clock monitor.
Disabling issuance preserves the selected store and refuses implicit ordinary startup with retained authority.
The [settings proof](origin-settings-2026-09-11/verification.json) retains its tests.

Commit `4bb4b870` lets a publication-lease follower serve a matching accepted authority catalog without shared writes.
Its receipt issuer reads the latest shared authority head even when another writer advances beyond the local catalog.
The [follower proof](origin-followers-2026-09-11/verification.json) records the failing startup cases and seven final passing race events per toolchain.
Memory and filesystem fixtures pass. Static checks and 1,506-file prose pass.

Periodic catalog adoption, safe takeover, explicit transitions, and real fleet qualification remain open.
Full verification, native parent integration, review, publication, and merge remain required.

### Published Starport dependency

Starport commit `190a8124` selects published Starmap `v0.16.6-0.20260911063726-852a548c4959` with `GOWORK=off`.
The old pin cannot compile the authority consumer. The published replacement passes 161 catalog race events with zero skips.
A transient Git module-cache error failed the first download. The retry succeeded without cache edits.

The focused proxy, router, and controller permission checks pass 24 race events with zero skips.

The architecture gate passes eleven checks, including the full Go suite, and fails V01 because its version expression accepts only release tags.
This conflicts with the CSP4 published-module contract. Resolve that candidate check while retaining stable released-pair qualification.
The [dependency proof](published-starport-dependency-2026-09-11/verification.json) records the exact failure and module resolution.

The four mapped consumer cases remain UNVERIFIED: `A09.excluded_membership`, `A10.cold_owner_refusal`, `A10.warm_retained_authority`, and `A21.mixed_schema_permission_envelope`.
The compatible published dependency removes that integration blocker. Full consumer delivery gates and merge remain required.

Previous integration evidence remains in the [dated history](integration-history-before-cancellation-2026-09-11.md).

