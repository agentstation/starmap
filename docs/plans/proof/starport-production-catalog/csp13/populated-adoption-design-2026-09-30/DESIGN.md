# Populated in-place adoption design

This design implements `csp13/populated-adoption-contract-2026-09-30/CONTRACT.md`.
It records the owner boundaries for the implementation. It has no qualification evidence.
Integrated Starport base: `codex/recovery-catalog-activation` at `c6b102327482e1a7337ca76720520cc52e28519b`.

## Finding that sets the scope

Non-test Starport source has no command that reopens a closed populated deployment.
`backup close` closes the SQL witness. Only `backup activate` opens a later epoch, and it restores into empty targets.
A native KV restart or promotion changes the Valkey identity, so the stored authority refuses.
Populated adoption supplies the missing in-place path from a closed populated deployment to a new open epoch.

## Terms

| Term | Meaning |
| --- | --- |
| B | The original verified backup. A `RestoreSource`. |
| H | The independent history package. Its manifest binds `B.ImportIdentity(operation)`. |
| C | The capture of the fenced live deployment after the incident. A `RestoreSource`. |
| Prefix | Each history step before the two final authorization steps. |
| Prior approval | The last open witness record, which the operator retains independently. |

`ProjectIndependentHistory(B, H)` and `CompareCaptured(C)` prove that C equals B plus the prefix.
B and C can be the same capture. Then the prefix has no steps and H supplies only the final steps.

## First delivery: zero prefix

The first delivery supports only the case where C is the base capture.
Its inputs are C, the prior approval, and an H that binds `C.ImportIdentity(operation)`.
H holds the operator attestation and the two final authorization steps. A history with a prefix step refuses.
This delivery needs no projection, no comparison scratch space, and no present-prefix journal.

Consequence: software cannot detect an acknowledged write that Valkey lost in a restart.
The operator attests that the interval is complete. The Valkey persistence setting bounds the loss.
Independent detection needs typed activation-effect history. The plan ledger owns that follow-up.
The owner decides under D42 if CSP13 needs it before production readiness.

## Flow

Populated adoption replaces two phases of the coordinated import flow.

| Import phase | Populated adoption phase |
| --- | --- |
| Restore B into empty targets | Claim the populated targets after an exact check against C |
| Replay the prefix | None. The first delivery refuses a history with a prefix step. |

Each later phase stays shared and unchanged.
These phases are catalog preparation, the final KV and SQL authorization steps, `FinalizeImportedHistory`, the decision seal, ordered release, and approval.
The shared phases treat each native claim as an opaque value.

The adoption identity has the shape of `PreparedImportIdentity`.
Its claims bind the C snapshots: `KVOriginal = C.KV`, `SQLOriginal = C.SQL`, `BlobOriginal = C.Blobs`.
Its component operation digest also binds the C digest, the H digest, and the adoption mode.
Thus a populated claim and an empty-target import claim of the same capture are different values.

## Scope

- Destinations: Valkey fleet storage with PostgreSQL or MySQL, and object storage.
- Badger and local filesystem destinations refuse populated adoption.
- The ordinary empty-target `Claim`, `ImportRelationalOnce`, and blob `claimImport` contracts do not change.
- The projection and `CompareCaptured` do not change.

## Known limit

`CompareCaptured` requires exact KV equality apart from the revision singleton.
A completed activation changes the authority record, catalog identity records, and native receipts.
The typed history format has no step for those effects.
Therefore a comparison across an earlier activation refuses.

Two cases pass: B and C from the same closed interval, and B equal to C.
An unplanned restart during an open epoch uses B equal to C with the operator attestation in H.
That case has the same trust as the supported empty-target recovery of C. It copies no domain data.

Independent validation across an earlier activation needs typed activation-effect history. That work is out of scope here.
A closed deployment accepts no domain write, so the first case gives no more coverage than the second.

## Native owners

Each owner supplies one new capability. Each capability keeps the target restricted, binds one operation, and permits an exact retry.
A refusal leaves no change. A retry reuses the original claim, census, and receipt. It never captures again.

### KV: `internal/storage`, Valkey

New capability `ClaimPopulated`, separate from `Claim`.

1. Check with no mutation: the barrier is absent or equals this claim.
2. Check the live census against the C records by key, value, and absolute expiry. The two current control roots are not domain records.
3. Run one atomic server script bound to the approved incarnation. It checks that the barrier is absent. It checks the exact preimages of `storage:activation-current:v1` and `storage:reconciliation-current:v1`. It writes an operation-bound closure receipt under a new persistent history key. It sets the barrier to the claim. It deletes only the two current roots.
4. Check the census again under the barrier through `InspectImport` at position zero.

An exact retry finds the barrier and compares the closure receipt.
A record with an expiry in C can be absent only when its captured expiry is not later than server time.
An absent record with a later expiry, or without an expiry, refuses.
Historical reconciliation and activation receipts stay in place.

### SQL: `internal/sqlstore`, PostgreSQL and MySQL

New capability `ClaimPopulatedRelational` for a populated relational claim. `relationalPopulatedPrefix = "relational-populated-v1:"`.

Delivered in Starport commit `a9ee090f` on `codex/recovery-populated-sql-20260930`.

1. Hold the migration owner and one transaction with the portable tables locked. PostgreSQL takes `ACCESS EXCLUSIVE` table locks. MySQL uses `SERIALIZABLE` row and gap locks from the census reads.
2. Export the locked live rows through the portable copy into a private SQLite candidate under the caller's scratch directory. Read the census from the candidate. Require equality with the C census in all four classes and counts.
3. Install `relationalImportMarker` with `relationalImportClaim(C.SQL, identity)`. Do not call `checkRelationalEmpty`. Copy no rows.
4. Delete only `relationalActivationCurrent` and `relationalReplayCurrent`. The control class digest covers both values, so the census equality is their exact preimage check.
5. Write the closure receipt `{"version":1,"claim_sha256","census_sha256"}` under `relational-populated-v1:<claim sha256>`.
6. Run the `restrict` callback in the same transaction. The recovery owner uses it for the witness transition.

An exact retry compares the marker and the receipt. It does not compute the census again and does not run `restrict` again.
SQLite returns `ErrPopulatedBackend`. A census difference returns `ErrPopulatedCensus`.
Startup inspection refuses a closure receipt without a marker or an activation. The census counts the receipt as a control row.

The candidate census uses SQLite binary order, so server collation cannot change the digests.
A native test proves equal digests on both servers with a server sort order that differs from binary order.

Known fact for the recovery owner: the marker bytes equal the ordinary import claim.
The adoption identity must differ from the empty-target import identity of the same capture.

### Blob: `internal/blob`, object storage

New capability for a populated blob claim. Object storage has no atomic multiple-object write.

Delivered in Starport commit `545895a3` on `codex/recovery-populated-blob-20260930`.

1. Verify every retained object against the C image with `GET If-Match` on the listed ETag. Compare the object count and retired count with C.
2. Check the exact preimage of `.starport/import`, `.starport/activation-current`, `.starport/replay-current`, and `.starport/adoption-closure`. `ObservePopulatedControls` supplies the preimage with bytes and ETags.
3. Write the closure receipt `{"version":1,"claim_sha256","controls_sha256"}` with `If-Match` on the observed ETag, or `If-None-Match: *` when absent.
4. Put the import claim over the observed import bytes with `If-Match`, or with `If-None-Match: *`.
5. Retire the replay cursor, then the activation-current root. Each delete carries `If-Match` on the observed ETag after a byte and ETag check, and an absence check follows.
6. Confirm the closure and claim, the absence of both current roots and of this claim's history, and an unchanged listing.

A retry continues from an equal closure receipt. Each step is idempotent for the same operation and refuses a different one.
The claim retires the replay cursor because `readReplayCursor` refuses a cursor bound to the prior claim. This matches the SQL design.
Activation history for the claim consumes the closure. A pending closure restricts the barrier, ordinary open, activation inspection, and repeated activation.

Documented limit: the object storage fixture ignores `If-Match` on `DELETE`. The SDK sends it, and AWS documents a 412 on mismatch. No test ran against AWS.
On a service that ignores it, safety depends on the mandatory writer fence and on the claim barrier. The barrier permits no other writer of these keys.

The residual race is two concurrent retries of the same claim with an activation between them. It fails closed: the store stays restricted and needs manual repair.

## Recovery owner: `internal/recovery`

Delivered in Starport commit `596bcccb` on `codex/recovery-populated-recovery-20260930`.
The lead review found three deviations. Item 7 needed no change, because both functions already accept a closed record with a backend identity.
The SQL restrict callback places the adoption-prepared record at the captured epoch plus one, so it is the prior epoch plus two.
The retained activation runner also refuses a mode mismatch. It reopens only an import journal, so the application owner supplies the adoption path.

1. **Adoption identity.** Derive the claims from C and the adoption binding. Check the H manifest against `C.ImportIdentity(operation)`.
2. **Prior approval binding.** The operator supplies the prior approval record. The captured witness in C must equal that record after `Close`. The KV authority record in C must hold the same approval. A lower SQL witness epoch refuses, because it shows a restored SQL database. Different independent epochs refuse.
3. **Closed-adoption epoch.** New `ClosedAdoptionEpochRequest{Closed, PriorApproval, Snapshot, Import, Evidence}`. It requires a closed record with the backend identity of the prior approval, and `PriorApproval.Epoch < Closed.Epoch`. It requires `HighestEpoch >= Closed.Epoch-1`. The next epoch is `max(Closed.Epoch, HighestEpoch+1)`. The closed record keeps the old backend identity, and evidence is `"adoption-epoch:" + digest`. The flow never simulates an imported empty witness.
4. **Witness sequence.** The SQL claim `restrict` callback moves the captured closed record to the adoption-prepared record at the next epoch. The acceptance step moves it to the closed-adoption epoch record. Approval opens that record with the new native identity.
5. **Acceptance.** `acceptedHistoryState` holds one of two epoch transitions: imported or closed-adoption. The acceptance record gains an optional adoption binding. The binding holds the C digest, the adoption identity digest, the prior approval, and the closed record. The import acceptance bytes do not change.
6. **Zero-prefix journal.** `runner.json` records the adoption mode. The runner refuses a prefix step in this mode. Catalog preparation and the final steps start from native position zero. A present-prefix journal is a later delivery.
7. **Approval.** `ApproveImportedAuthorityCheckedAt` and `GuardClosedImport` accept a closed record with a backend identity. Approval runs only after the catalog, revision, native owner, and current permission checks pass.

## Application and command: `internal/app`, `internal/cli`

- `PreparePopulatedRecovery` verifies C and H, observes the native control roots, and retains the observation.
- `ActivatePopulatedRecovery` takes the populated claims, accepts H with the closed-adoption transition, and then runs the shared phases.
- `InspectPopulatedRecovery` checks retained evidence. It repairs nothing and captures nothing.
- The native opener omits `validateExistingImportedTargets`.
- The flow verifies canonical files in place against C. It publishes no file.
- Catalog identity moves through the existing topology compile and preparation lane. `AdoptFleet` alone is insufficient.
- One new `starport backup` subcommand exposes the flow. `docs/RECOVERY.md` gains the procedure.

## Required tests

| Layer | Tests |
| --- | --- |
| Native owners | Exact claim, exact retry, process loss between each durable step and its reply, changed preimage, missing acknowledged record, expired record, different operation, refusal without change, unchanged empty-target claim behavior |
| Recovery owner | Epoch transitions, conflicting independent epochs, restored SQL witness, prefix step refusal, mode mismatch, import acceptance bytes unchanged |
| Application | Populated adoption through the operator command, exact sealed retry, coherent approvals, refusal without change |
| Process | Actual Valkey restart with persistent data, promotion, acknowledged-data loss, restored SQL witness, old primary reachable with closed permission that blocks dispatch |

## External requirements that software does not supply

External process and network fencing of all writers stays mandatory until fresh gateway readiness.
A caller string, a captured control, a projection result, or an empty-target restore is not authority.
Aggregate projection scratch capacity has no qualification. The first delivery uses no projection.
RPO and RTO targets need measurement and owner approval under D42.

## Delivery order

1. Phase A, parallel: KV, SQL, and blob populated claims. Each owner uses its own native fixture.
2. Phase B: recovery owner.
3. Phase C: application, command, documentation, and process tests.
4. Phase D: recovery measurement for D42.

Each phase lands as its own reviewed pull request. The integrated base branch lands first.
The lead reviews each delivery and commits it. A delegated job does not commit.
