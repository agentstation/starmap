# CSP16.1 decisions

Date: 2026-10-02. Each decision is an engineering interpretation inside the accepted task scope.
None needs new owner authority. The owner can reverse any of them before the final acceptance run.

| ID | Decision | Reason and consequence |
| --- | --- | --- |
| 16.1-1 | Origin checks are same-origin checks, not loopback-only checks. | The auth-mode switch is loopback-only because it changes the lock. Configuration saves must work from a remote operator workstation. A console session always carries a browser Origin, so a cross-site page cannot drive a save. |
| 16.1-2 | A field save refuses a change to the acquisition policy identity. | D41 requires one coordinated `config apply` for a policy change. The save API reports the refusal with the apply instruction. CSP17 presents that path. |
| 16.1-3 | The local revision token is the SHA-256 checksum of the file bytes. | `productfiles.CompareAndPublish` compares bytes. One token serves the effective report, the save body, and the receipt. |
| 16.1-4 | Local receipts live in an operation journal next to the configuration file. | Local mode has no SQL store. The journal and the file share one writer lock, so an exact retry can complete from the published checksum. |
| 16.1-5 | Deployment authorization is the admin scope plus a `deployment_id` binding on each write. | No narrower operator scope exists, and the console session holds `*` by design. A foreign deployment identity refuses before any write. Denied-access journeys with a non-admin key belong to CSP17. |
| 16.1-6 | `db.Migrate` leaves the save path. | A field save must not change the schema. A store behind the current schema refuses the save and names the migrate operation. |
| 16.1-7 | The console UI stays in CSP17. | CSP17 owns console settings, forms, and types. CSP16.1 owns the HTTP API, the writers, and the receipts. |
| 16.1-8 | CSP16.1 lands as one Starmap contract PR and two Starport PRs. | The configuration operations and the baseline promotion are different concepts with different owners inside Starport. Two reviewable PRs keep each review bounded. The promotion PR follows its own survey. |
| 16.1-9 | The journal is manifest entry `config-operation-journal`, and a backup captures it. | Owner choice on 2026-10-02 through the question tool. The manifest is Starport-owned, and A24 requires generated files at the configuration leaf. The journal holds no setting values. A restore gives it the `target-configuration` action, and canonical recovery keeps the captured journal inactive, because each receipt binds to the file checksum. |

## Promotion survey result (2026-10-02)

- The retained baseline lives in the Starmap fleet recovery record (`runtime/fleet_recovery.go:47-58`). Replay adopts the record's baseline and treats the packaged baseline as "a candidate for explicit promotion" (`runtime/fleet_replay.go:72-73`).
- No Starmap runtime method changes the baseline. A promotion needs a new exported runtime operation under the lease. It also needs a record field for the operation ID and a read-only packaged-versus-retained status.
- Starport owns the operator command, the operation identity, the fenced request, and acceptance. Starport never calls `PinAcceptance`, `ReadPermission`, `RefreshPermission`, or `ReplaceRemovalTargets`, and has no removal store.
- Local (non-fleet) mode with no recovery checkpoint takes the packaged baseline on every start (`runtime/runtime.go:377-379`). A single-node binary upgrade adopts the new packaged catalog without an explicit operation. CSP11 qualified retention for fleets only.
- Open risks: an older binary that replays a promoted newer baseline has no proven payload-schema refusal path. A removal target absent from the new baseline has no defined behavior. No test counts storage calls on the request path during a transition.

## Pending owner decisions (asked 2026-10-02)

| ID | Question | Options and consequences | Recommendation |
| --- | --- | --- | --- |
| Q16.1-A | Does D41 condition 13 (retain the baseline until explicit promotion) apply to single-node local mode, or only to fleets with shared storage? | Fleet only: single-node upgrades keep the current behavior and get the packaged catalog at once. The plan records a documented limit. Both modes: Starmap gains a local retention store, and every single-node upgrade needs `starport catalog promote-baseline` before new models appear. That adds one Starmap PR and changes the default install experience. | Fleet only, with the documented limit. |
| Q16.1-B | Where does explicit baseline promotion sit in the ledger? | Inside CSP16.1: CSP16.1 stays open until two more PRs (Starmap runtime operation, Starport command) merge, and CSP17 waits. New sibling task CSP16.2 owning D41 conditions 13 and 16: CSP16.1 closes on A26 and A27 plus condition 17, CSP17 starts after it, and promotion runs in parallel with CSP17. | CSP16.2. |

Answers (2026-10-02, question tool): Q16.1-A fleet only, with the documented limit. Q16.1-B new sibling task CSP16.2.
The ledger has the CSP16.2 row and the local-mode limit. The owner reported disappearing questions, so this table is the durable copy.
