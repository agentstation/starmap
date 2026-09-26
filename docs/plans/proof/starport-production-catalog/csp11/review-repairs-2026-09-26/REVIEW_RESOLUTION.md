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

Fresh fleet initialization now has an explicit command and passes 29 named race tests with real Valkey and PostgreSQL.
The latest local producer pair also passes those checks. Review and merge remain pending.

## Remaining merge requirements

| Owner | Required repair | Required evidence |
| --- | --- | --- |
| Starmap fleet policy | Retain the fleet baseline across binary upgrades under D41. Preserve refusal of incompatible acquisition policy. | Different binary baselines replay retained inputs. CSP16 owns coordinated policy changes. CSP16.1 owns explicit baseline promotion. |
| Starport fleet initialization | Supply an explicit operation for fresh deployment approval before enabling shared startup. | A fresh Valkey/PostgreSQL deployment starts through the documented operation. Existing, replaced, or uncertain state refuses fresh initialization. Concurrent attempts cannot approve different backends. |
| Starport fleet retention | Bound staged chunks, committed publications, generation indexes, and retry receipts. Protect current, accepted, retained rollback, and pinned generations. Apply canonical retention configuration. | Repeated publication and rejected staging remain bounded. Interrupted collection recovers. Concurrent publication, acceptance, reads, and collection preserve selected data. Disabled automatic cleanup and explicit collection retain their declared behavior. |

Fresh initialization and canonical retention now have local implementations and passing native checks.
The [retention proof](../retention-policy-2026-09-26/verification.json) records 45 fleet and startup race results with no failures or skips.
Complete branch review and final-head qualification remain open for both repairs.

D41 resolves the upgrade decision.
The [transition contract](../upgrade-contract-2026-09-26/CONTRACT.md) defines the shared fencing, retry, and input-validation requirements.
CSP11 retains the fleet baseline across binary upgrades. CSP16 and CSP16.1 own coordinated policy apply and explicit baseline promotion.

Ordinary source refresh remains automatic.
Retained catalog availability remains subject to the existing permission and expiry contracts.
A transition cannot invent permission or override a known withdrawal.

CSP11 owns retained baseline recovery, fresh initialization, and bounded retention before fleet integration.
The transition contract assigns configuration operations to CSP16 and CSP16.1 without removing their required evidence.
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


## September 26 retention checkpoint

Two real-storage regressions confirm leaked rejected chunks and unbounded committed receipts.
The local repair adds a bounded inventory, unique chunk ownership, a native maintenance lease, and a durable deletion record.
It protects accepted rollback content and explicit generation readers. Ordinary diagnostic reads remain read-only.
The adapter also refuses corrupt cleanup records that select protected content.

The storage protocol passes its focused race checks. It is not ready for publication.
Its current publication-triggered cleanup bypasses Starmap's canonical retention scheduler and disable switch.
This discovered integration gap is a mandatory CSP11 repair, not an optional follow-up.

Starmap must continue to own retention settings and scheduling.
Connect the fleet adapter through an explicit collection capability, and remove unconditional committed-generation cleanup from publication and acceptance.
Keep abandoned-upload recovery independent of that switch. Hard storage bounds must refuse uploads when operators disable collection and protected capacity is full.

Report public catalog generations separately from publication receipts and private recovery bytes.
Do not reinterpret the existing generation-count and manifest/payload-byte fields as publication counts or total snapshot bytes.
Verify disabled scheduling, explicit collection, configured limits, safe refusal, and operator-visible capacity state before the final branch review.

## Canonical retention integration

Producer `75e84878d` exposes coordinated collection and copies publication accounting into memory status.
Consumer `ac679bf64` implements explicit collection with canonical limits. Publication and acceptance no longer collect committed data.
Dry runs preserve pending cleanup. Required generations, current and accepted heads, rollback history, and reader claims remain protected.

Public generations count once. Separate publication metrics report receipts, encoded bytes, private recovery bytes, and reader claims.
Oversized scans and cleanup reads refuse before deletion. Interrupted deletion resumes from its durable record.

These local changes repair the policy integration gap. This does not complete CSP11.
The tests used a temporary Go workspace to connect the unpublished producer API.
The consumer module pin, complete branch review, native CI, and ordered merges remain required.
