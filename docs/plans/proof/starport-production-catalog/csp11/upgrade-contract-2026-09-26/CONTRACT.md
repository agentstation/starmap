# Fleet upgrade transition contract

Status: approved third option under D41 on September 26, 2026. Implementation and qualification remain open.
Owners: CSP11 for retained baseline recovery, CSP16 for coordinated policy apply, and CSP16.1 for explicit baseline promotion.
Inspected producer: `75e84878d8520a9c5f456643f929bc776d52fa3e`.
Inspected consumer: `ac679bf6464e448b63576deed54dbdbd0f7696e6`.

## Confirmed failure at the inspected revisions

`runtime/fleet_replay.go` binds replay to the embedded baseline and acquisition policy.
A mismatch returns a conflict before input replacement.

The initializer retains that error. The acquisition check refuses ownership while the error remains.
The refresh operation returns the same error when the shared head does not change.

A compatible publisher can advance the head. If every replica changes policy, no compatible publisher remains.
This produces an upgrade deadlock. The source audit confirms the control flow, without claiming a new native qualification result.
The existing `TestFleetRuntimeIncompatibleReplayRemainsFollower` protects the safe refusal and retained catalog behavior.

The current lease and head contracts do not select a deployment-wide acquisition policy.
Removing the replay check would let replicas with different policies alternate publication ownership.
A transition must repair the deadlock and preserve the refusal of ordinary incompatible publication.

## Coordinated transition requirements

| Boundary | Required behavior |
| --- | --- |
| Starmap runtime | Compute the target policy identity, validate retained inputs, and build the candidate against the target baseline and policy. |
| Starport fleet store | Atomically select the approved policy and fence publication under a previous policy. |
| Independent recovery authority | Preserve the approved backend incarnation and recovery epoch. A policy change cannot approve a replacement backend. |
| Operator surface | Show the predecessor, target policy identity, retained input effects, and operation result without exposing credentials. |
| Replica status | Distinguish serving permission, replay readiness, policy mismatch, and acquisition readiness. A policy mismatch alone must not claim loss of inference permission. |

A transition must bind the deployment, recovery identity, exact predecessor, target policy identity, and stable operation ID.
Its approval must not authorize a different target or silently advance a changed predecessor.
Native commit must select the new policy and its publication together. The same transaction must fence the original publisher.
Every later acquisition, renewal, and publication must enforce the selected policy.
A local compatibility check alone cannot enforce that rule across replicas.

An exact retry after a lost response must return the original result.
Another target with the same operation ID must refuse. Concurrent transitions must produce one winner.
Old publishers must refuse after the transition, including a process that resumes after losing its lease.
Rollback requires another authorized transition and an increasing revision. It cannot reuse an old grant.

## Retained data and permissions

Keep the accepted publication available until replacement validation succeeds, subject to its existing permission and expiry rules.
A transition cannot extend an authority receipt, restore a known withdrawal, or bypass route validation.
Ordinary public acquisition must not become an internal authority fallback.

Validate retained source identity, provider/account bindings, manual history, removals, and any pin against the target policy.
Do not silently discard incompatible retained inputs. Report the conflicting scope to the operator within a bounded response.
A source or authority change must use the existing source-transition and permission rules.
Starmap owns this validation. The storage adapter must preserve recovery bytes as opaque data.

The inspected recovery record lacks the previous baseline. The replacement format must retain its complete validated manifest and payload.
Replay must use that retained baseline. A corrupt or unavailable baseline must refuse acquisition.
The target candidate and its retained input selection must remain stable across a retried transition.

## Required acceptance evidence

1. A fleet can change policy after every old replica stops, without clearing shared storage.
2. During a rolling change, old replicas cannot publish after target activation.
3. Delayed commits and renewals from the previous policy fail in native storage.
4. Lost responses, process loss, and exact retries preserve one transition result.
5. Competing targets, changed predecessors, and reused operation IDs refuse safely.
6. Target reconstruction preserves permitted retained inputs and reports incompatible scopes.
7. Current, accepted, pinned, and rollback data remain protected during transition and collection.
8. Authority expiry and known withdrawals remain effective throughout the transition.
9. A rollback increments the policy revision and cannot restore stale ownership.
10. Status distinguishes a serving follower from a replica eligible to hold refresh ownership.

Qualification must use real Valkey and PostgreSQL, with separate publisher processes for stale-owner and recovery cases.
An in-process adapter can test runtime composition, but cannot prove native fencing.

## Approved upgrade policy

Binary upgrades retain the active baseline and reconstruction inputs independently of the packaged baseline.
A compatible software rollback uses the same retained state. Catalog rollback is a separate explicit operation.
A fresh deployment can initialize from a permitted baseline only after independent fresh-deployment approval.
Missing or uncertain state in an established deployment requires recovery.

Configured GitHub and Starmap catalog updates remain automatic under existing controls, pins, and authority constraints.
An acquisition-policy change requires one coordinated configuration apply.
An embedded-only fleet promotes a packaged baseline through an explicit operation.
Credential rotation within the same identity and declared scope does not require a policy transition.

Internal authority remains binding. A binary upgrade cannot enable public fallback or expand permission.
Request admission continues to use the accepted catalog in memory.

## Additional D41 acceptance evidence

11. Different binary baselines replay the same retained fleet inputs, including after every old replica stops.
12. A configured upstream update advances normally across mixed binary baselines.
13. Embedded-only deployments retain their baseline until explicit promotion, including during software rollback.
14. Fresh initialization remains explicit. Missing, corrupt, or unsupported retained baseline data refuses acquisition.
15. Same-scope credential rotation preserves policy identity. Different scope selection requires coordinated apply.
16. Promotion preserves pins, source authority, valid permission deadlines, and explicit removal rules.
17. Transition and upgrade operations add no request-path storage calls.

## Execution ownership after D41

D41 separates ordinary binary upgrades from changes to deployment configuration.
CSP11 repairs the upgrade deadlock by retaining the baseline and refusing incompatible acquisition policy.
It does not expose a policy-apply operation.
CSP16 owns coordinated policy apply through the shared SQL configuration revision and audit contract.
CSP16.1 owns explicit baseline promotion through the operator operation contract.
All seventeen conditions remain required before the production plan completes.

| Conditions | Required owner and evidence |
| --- | --- |
| 1–10 | CSP16 and CSP16.1 qualify transition selection, native fencing, exact retries, retained inputs, authority, rollback, and status. |
| 11–12 | CSP11 proves baseline replay across binary changes and continuing configured source updates. |
| 13 | CSP11 proves baseline retention. CSP16.1 proves explicit promotion and software rollback behavior. |
| 14 | CSP11 proves refusal of invalid retained data and lost catalog heads. CSP13 qualifies complete storage recovery. |
| 15 | CSP11 preserves the credential-free compatibility identity. CSP16 qualifies coordinated scope changes and credential rotation. |
| 16 | CSP11 preserves existing pin and authority checks. CSP16.1 qualifies those checks during explicit promotion. |
| 17 | Each owner proves that its operations add no request-path storage access. |

SQL owns the approved configuration revision and its audit record.
Valkey owns the applied catalog policy and publication fence.
One operator apply must coordinate those owners and recover a partial result.
It must report saved and applied revisions separately. It cannot assume a transaction spans both databases.
Do not enable shared acquisition-policy writes before the native transition requirements pass.

This routing supersedes the earlier requirement to create a separate policy transition inside CSP11.
It changes task ownership. It removes no product acceptance condition.
