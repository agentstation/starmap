# CSP11 fleet commit audit

The current Starport lease check does not fence the shared head write.
Three tests reproduce this defect against real Valkey with the race detector.
An old owner can advance the accepted head after expiry, release, or another process takes the lease.
The implementation task remains open.

## Tested source

The producer merge is `8122081e0494648fb1e356647053dd870a05fb1c`.
The consumer merge is `f5f066ddb3d9ee2c016d828c6a031fb9a3e3ffda`.
The probe uses a Go test overlay and changes no repository source.
The [failure record](lease-failure-2026-09-26/verification.json) retains the exact test, runner, commands, and output.

The test pauses the real storage call after acceptance reads the lease epoch.
It then expires or releases the lease before the head compare-and-swap.
The takeover case starts a separate process that gets epoch 2.
The original process still commits its epoch 1 candidate.
Each case returns no acceptance error and selects the rejected generation.
The disposable service stops successfully after the tests.

The [recovery failure](recovery-failure-2026-09-26/verification.json) uses a real filesystem catalog store and separate runtime directories.
After removal of the former leader directory, the follower opens the same accepted generation.
Its next partial observation update loses the earlier model.
This test isolates retained-input recovery. It does not qualify distributed lease behavior.

## Owning contracts

| Concept | Required behavior | Current evidence |
| --- | --- | --- |
| Fleet publication | Compare the holder, epoch, validity, and expected head in one backend operation. | Starport `internal/catalog/acceptance.go` checks the epoch before `GenerationStore.Commit`. The latter compares only the head. |
| Candidate identity | Retain the ownership evidence from candidate production. | `candidate` and `offer` in `internal/catalog/runtime.go` read the current lease epoch when they construct a candidate. |
| Exact ownership | Reject an unknown, expired, released, or mismatched grant. | `fenceEpoch` rejects only smaller epochs. The existing test explicitly permits a later epoch. |
| Lease lifetime | Let the owning backend enforce expiry at commit. | `LeaseStore` persists `ExpiresAt`, but `CurrentEpoch` returns the epoch without checking expiry or the holder. |
| Recovery inputs | Retain the exact inputs required for the next merge outside the former leader directory. | Starmap `runtime/layers.go` and `publication_journal.go` retain source, provider, manual, and removal inputs in private files. A separate recovery probe confirms loss of an earlier model after a later partial update. |
| Deployment isolation | Bind the lease, heads, inputs, and channels to one deployment. | Starport currently uses fixed catalog and lease key constants. Complete isolation remains UNVERIFIED. |
| Recovery authority | Bind shared operations to the approved KV incarnation and recovery epoch. | The specification assigns this contract to CSP11. The source search found no matching adapter contract. |

## Repair boundary

Starmap must own the runtime publication contract and the retained-input format.
Starport must implement that contract through its configured storage adapter.
The standalone runtime must preserve its existing local publication behavior.

The fleet adapter must commit the ownership conditions, expected head, and retained-input reference atomically.
It can stage immutable bytes before this operation, but unselected bytes confer no permission.
A retry must preserve the original grant and expected state.
Reading a newer epoch cannot authorize an older candidate.

Backend expiry must remain authoritative during the commit.
A local deadline can stop work early, but cannot replace the backend check.
The durable epoch must survive ordinary lease expiry.
Duplicate active process identities must not silently renew each other.

The recovery envelope must include all inputs needed to reproduce the selected generation.
It must preserve source receipts, provider scopes, manual history, resets, and explicit removals.
Follower validation must reject incompatible input formats or acquisition policy before leadership.
Private credentials must not enter a public catalog artifact.

The recovery identity contract must use the independent PostgreSQL witness described in specification section 8.6.1.

CSP12.2, CSP13, and CSP15 retain their admission, recovery-procedure, and failure-qualification work.
Shared storage alone must not impose qualified UTC on every replica.

## Required acceptance

Keep all eleven CSP11 subcases in the acceptance map.
The current baseline passes two and leaves nine unregistered.
The new failure probe does not qualify A13 or A14 as passing.

After the repair, run real concurrent processes through expiry, release, takeover, and stale candidate retries.
Verify that a refused commit changes neither the selected head nor its retained-input reference.
Remove the former leader directory before follower restart and a subsequent partial source update.
Verify deployment isolation, duplicate identities, missed events, and the approved recovery identity.
Preserve source review, native platform checks, and the ordered producer and consumer merges.
