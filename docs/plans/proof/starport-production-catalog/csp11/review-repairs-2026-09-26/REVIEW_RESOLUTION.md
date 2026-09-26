# CSP11 fleet review repairs

The Apple silicon changes remain local in both task branches.
Fleet review findings prevent publication and merge of the complete branches.
This record preserves the remaining delivery contract.

## Completed local repairs

Starmap checks all implicit catalog provider scopes before lease acquisition, including providers without retained observations.
Explicit bindings restrict the check to their selected scopes.
A failed check prevents ownership. Restored access permits a new grant.
The check reads acquisition credentials without fetching provider inventories.
The provider subset must use explicit bindings.

Starport retains a standalone candidate when lease preparation fails.
A background retry delivers it after storage recovery without another source publication.
A newer pending candidate replaces the older pending candidate.
Unknown ownership does not permit acceptance. Idle standalone forwarding does not read storage.
The retry does not add work to inference requests.

The producer regression failed before repair and now passes with the race detector.
The consumer regression also failed before repair and now passes.
The verification record contains commands, source commits, counts, and raw output.

## Remaining merge requirements

| Owner | Required repair | Required evidence |
| --- | --- | --- |
| Starmap fleet policy | Define a safe transition for a changed embedded baseline or acquisition policy. Preserve ordinary replay compatibility checks. | Mixed-version replicas cannot alternate policy. The selected transition preserves permitted retained inputs. Old owners cannot publish after transition. |
| Starport fleet initialization | Supply an explicit operation for fresh deployment approval before enabling shared startup. | A fresh Valkey/PostgreSQL deployment starts through the documented operation. Existing, replaced, or uncertain state refuses fresh initialization. Concurrent attempts cannot approve different backends. |
| Starport fleet retention | Bound staged chunks, committed publications, generation indexes, and retry receipts. Protect current, accepted, retained rollback, and pinned generations. | Repeated publication and rejected staging remain bounded. Interrupted collection recovers. Concurrent publication, acceptance, reads, and collection preserve selected data. |

The upgrade decision remains pending with the owner.
The recommendation requires one explicit deployment-wide transition for a changed baseline or acquisition policy.
Ordinary source refresh remains automatic.
Retained catalog availability remains subject to the existing permission and expiry contracts.
A transition cannot invent permission or override a known withdrawal.

CSP11 owns these merge requirements because its integration enables the fleet path.
CSP12 retains complete recipe validation and all remaining storage contracts.
CSP13 retains populated-state migration and recovery procedures.
Move the minimal fresh-initialization operation into the CSP11 delivery without deleting the broader CSP12 acceptance requirements.

The retention repair needs a publication-owned collection protocol.
Deleting content-addressed chunks independently can race a staged commit that reuses those chunks.
A time-to-live on committed chunks can delete a selected catalog during an outage.
Neither approach satisfies the contract without additional reference protection.
Specify collection ownership and crash recovery before changing the adapter.

## Rejected review advice

The selected Starmap runtime already schedules follower refresh in `runtime/scheduler.go`.
Its runtime constructor starts that schedule.
Starport does not need a duplicate remote poll.

An epoch-read failure must not produce an epoch-zero candidate.
Acceptance checks the current epoch again, so that fallback neither proves ownership nor repairs delivery reliably.
Retaining the pending candidate preserves the existing acceptance checks.

## Delivery sequence

1. Resolve the pending upgrade contract.
2. Implement the remaining repairs at their owning boundaries.
3. Run the required behavior, storage, and platform checks on the final source pair.
4. Commit the changes and repeat the complete branch review.
5. Publish the existing PR branches and verify their exact required checks.
6. Merge Starmap #185 before Starport #385.
7. Record actual merge commits before task completion.

No native suspend requirement changes in this work.
Historical Intel Mac releases and evidence remain available.
