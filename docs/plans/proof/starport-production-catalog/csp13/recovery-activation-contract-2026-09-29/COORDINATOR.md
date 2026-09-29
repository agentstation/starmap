# Complete recovery activation coordinator design

I inspected Starport consumer 2284da89 and native activation changes at 49d791c7.
The catalog topology compiler and Starmap retention work remain separate producer changes.
This document specifies required work. It does not claim implementation or completed CSP13 acceptance.

## Ownership and capabilities


The application package owns the complete operator procedure.
The recovery package owns history, native phases, and the immutable recovery decision.
Catalog and application packages already import recovery. Recovery must not import either package.

Use a private application activation capability that combines these checked owner capabilities:

- ClosedFinalHistory for complete accepted history and the final imported graph.
- VerifiedOperatorInputs for current settings, selected trust, and administrator inputs.
- VerifiedCanonicalFiles for canonical role contents and actual publication continuity.
- The catalog owner's materialization and selected topology capabilities.
- The permission owner's current validation result for the selected catalog.

Reports and supplied digests cannot replace these checks.
The native storage APIs remain component operations. They do not establish complete recovery.

## Ordering before activation

Replay accepted typed history through its non-final owner changes.
Stop before the two required authorization revision rotations.
Apply the catalog compiler's bounded immutable stages under closed SQL and native KV guards.

Retain every historical capsule through the producer owner.
Materialize the selected producer inputs and publish the exact catalog selection.
Complete the final KV and SQL revision rotations.
Capture the final graph at the resulting exact component positions.
Verify the current owner inputs, canonical files, catalog selection, and permission.
Retain the complete activation decision before releasing any component.

The current history runner derives cursors only from declared history steps.
Catalog changes cannot advance the native cursor outside that journal.
Add a private ReplayPrefix capability that stops before final rotations.
Add a recovery-owned CatalogPreparationLane that journals each bounded compiler transaction and its native receipt.
Its guarded target implements the catalog Read, Completed, and Apply methods.

The application supplies the actual private compiled catalog capability.

The CLI accepts no mutation arrays or catalog storage keys.

Final history completion consumes the checked lane and starts rotations at its resulting cursor.

## Durable decision and restart

The immutable decision binds the operation, component identities, backup, prepared import, accepted history, replay journals, and final graph.
It also binds final native cursors, target configuration, canonical files, catalog selection, native approval, and original validation times.
External operator attestations remain distinct from native facts.
Unknown scope, incomplete history, and remain_restricted disposition cannot activate.

ClosedFinalHistory.Check remains a closed-state check.
It calls CompletedHistory.check and native inspection methods that require the original barriers.
It cannot reconstruct a capability after a component release.

Add a private RetainedActivationHistory verifier for restart.
It validates immutable package bytes, prepared and applied journals, fixed graph snapshots, and closed-final preparation without native replay.
It does not call Accept, Prepare, or Replay. It never replaces original graph capture time.
Native checks then depend on the actual activation phase.
Missing or corrupt decisions, images, claims, or receipts refuse further activation.

After each native release, retain the matching phase receipt.
If the process stops between native commit and phase publication, read the exact native receipt and publish that same phase record.
A journal entry alone cannot prove a native release.
Add passive owner activation inspection for status and restart.
An exact completed retry must not repeat preparation, authority issuance, selection, or domain replay.

## Release sequence

Release blobs at the exact final blob cursor and decision.
Release KV at the exact final KV cursor and decision.

Keep SQL approval closed throughout both releases.
Install fleet native authority under the final SQL activation transaction.
Open the exact closed boundary and remove the relational import barrier in that same transaction.
Use ActivateRelationalImportAt with the final SQL replay position.
Start fresh gateways only after complete native receipt verification.

Before release, all import barriers remain closed.
After blob release, KV and SQL still refuse startup.
After KV release, SQL still refuses startup.
A prepared native fleet authority grants no admission while SQL remains closed.
A lost reply never proves rollback of an independent native commit.

## Local and fleet authority

Fleet completion preserves ApproveImportedAuthority's native incarnation binding and authority CAS contract.
Add a strict position-bound form for complete activation.

Local completion uses the canonical Badger target identity.
RecoveryTargetSHA256 already binds its path and native directory identity.
Do not manufacture an IncarnationProvider or a Valkey incarnation for Badger.
A recovered-local startup gate checks native activation controls, the approved decision, the current local target, and final SQL completion.
Fresh standalone startup with no recovery controls keeps its existing behavior.
Ordinary local requests do not read a SQL Witness.

A completed activation receipt can coexist with a later withdrawal.
Status must report historical completion separately from current admission.
An exact activation retry must retain that withdrawal and refuse to reopen the old approval.

## Catalog and current permission

Require passive completed materialization and selection checks after restart.
Verify actual producer receipts, selected roots, staged immutable contents, and source reconstruction capsules.
Do not invoke a materialization retry that changes state after closed history completion.

Semantic replay grants no permission.
Check current authority, policy, revision, retained permission checkpoint, and expiry through the owning permission implementation.
Embedded and public standalone permission does not require custom qualified UTC.
Internal authoritative absolute expiry retains its qualified-clock contract.
Shared storage alone does not require qualified UTC.

Use a local deadline only where the owner's receipt contract supports it.
Never substitute a cache TTL for authoritative expiry.

The permission owner keeps source queries, refresh operations, leases, and permission issuance disabled during these checks.
Check permission before every first release and final SQL approval.
Expired permission, unknown scope, and known withdrawal remain restricted.
Historical capsules receive structural validation. Current accepted and candidate inputs must satisfy target semantics.

## Canonical files and current inputs

PublishBackupFiles currently invokes preparation before canonical publication.
That path is invalid after the first component release.
Extract passive canonical-role checks over selected contents, native directory identity, and owner publication continuity.

Every captured role needs an explicit disposition.
Supported dispositions include current target selection, restored owner state, inactive history, and inactive staging.
Required selected roles with unresolved owner recovery refuse activation.
Source configuration, dotenv, TLS, and administrator tokens never replace current target choices implicitly.
Provider secret-source binding stays passive.
Absent provider API keys do not block the metadata baseline because authentication was not acquired.

Recheck current settings, token, trust, secret-source bindings, canonical files, and targets at each pending phase.
Changes after decision publication conflict and retain remaining barriers.
Do not rewrite the old decision or released native receipts.
Changed target choices need a separately accepted recovery procedure against isolated targets.
External writer fencing spans the entire procedure between bounded guards.

## Startup preflight defect

Current composition runs local setup, KV opening, and inference-policy initialization before SQL import refusal.
Badger opening starts maintenance before storage.Open checks its native barrier.
SQL Migrate can also mutate before the later CheckImportBarrier call.
Tokens, catalog construction, history initialization, and workers occur after SQL refusal.

Add an early read-only recovery preflight before setup, maintenance, or policy initialization.
It must not create SQL databases, migrate, mint tokens, or contact catalog sources.
Native owner checks must detect closed, malformed, and partial recovery controls.
Fresh standalone first boot remains supported.
The complete recovery-startup gate also prevents worker dispatch during partial activation.

## Operator command and evidence

The activation request reuses the verified backup, operation, history, journal, target binding, and current fencing attestation.
Derive topology, canonical locations, and backend identity from configured native owners.
Status and inspection commands cannot release component barriers.
Report phase, next command, remaining barriers, native receipt uncertainty, and current admission.
Keep secrets and private evidence out of ordinary diagnostics.

Qualification must use fresh processes at each interruption boundary.
Test before and after every component commit and phase publication, including final native authority and SQL approval.
Cover planned and disaster recovery in both local-to-fleet and fleet-to-local directions.
Retain post-backup revocation, spending, grants, uncertain execution, and original accounting windows.
Verify nonzero final SQL position, both final revision rotations, and denied background dispatch.

Test changed targets, cursors, settings, tokens, trust, secret sources, selections, expiry, and later withdrawal.
Test missing and corrupt retained evidence without recapture or permission reconstruction.

Measure recovery point from the actual independently covered activity boundary.
Measure recovery time from command start through verified fresh readiness.
Record interruption and retry time separately.
Use Go 1.27.1 with race, pure-Go, native platform, real-storage, and recovery coverage.
