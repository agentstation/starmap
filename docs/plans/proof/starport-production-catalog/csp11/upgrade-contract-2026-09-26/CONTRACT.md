# Fleet upgrade transition contract

Status: proposed transition design. The owner decision about automatic baseline adoption remains pending.
Owner: CSP11. Current producer: `75e84878d8520a9c5f456643f929bc776d52fa3e`.
Current consumer: `ac679bf6464e448b63576deed54dbdbd0f7696e6`.

## Confirmed failure

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

## Shared requirements for either adoption policy

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

The recovery record contains a compatibility digest, not a complete previous baseline.
Target reconstruction must therefore have an explicit validation contract. It must not claim to replay an unavailable old baseline.
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

## Pending product decision

The recommendation requires one explicit deployment-wide transition for either a baseline change or an acquisition-policy change.
Ordinary source refresh remains automatic. Operators can include the transition in their deployment procedure.

The alternative automatically adopts a newer baseline and requires an explicit acquisition-policy transition.
That alternative also needs a trusted baseline ordering rule and protection against automatic downgrade.
Neither choice permits replicas to alternate incompatible publication policies.
