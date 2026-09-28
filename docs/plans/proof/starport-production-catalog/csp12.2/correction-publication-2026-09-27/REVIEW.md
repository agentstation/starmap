# Correction publication transaction review

Consumer `00dfff81cea9b286fc9e58fbc03db75a7c79b68c` adds `reservation.CorrectWith`.
The operation commits owner records, the correction receipt, the attempt, and every original budget window together.
A stale owner record prevents charge changes and dispute removal.

The receipt binds a canonical digest of the publication. The digest includes keys, absence conditions, expected bytes, and replacement bytes.
Exact retries preserve the original receipt even after acknowledgement loss. Reordered mutations retain the same digest.
Changed publication under an existing decision identifier fails. A standalone correction cannot reuse a receipt that required owner publication.

The owner can supply up to eight persistent records. Keys have a 4,096-byte bound, and each value has a 64-KiB bound.
The operation refuses duplicate keys, budget namespace writes, deletion, expiry, and malformed input.
Correction receipt schema 2 replaces schema 1. CSP13 owns migration.

## Evidence

The baseline calls the existing ledger correction before a separate job write.
It clears a dispute before the stale job write fails. The test defines a failure to prevent during API integration.
The new transaction refuses stale publication before any write.

Race tests cover both orders of late-provider publication and an injected change during the correction commit.
The late evidence retains its restriction in each case. Concurrent exact retries do not duplicate charge changes.
Badger process tests terminate a child before or after the complete transaction. Reopen and retry preserve the owner record, audit, receipt, and counters.

The shared limits suite passes 271 results with one PostgreSQL skip. The pure-Go suite passes 21 results without skips.
The final targeted race suite adds old-schema and malformed-digest refusals. The [verification record](verification.json) gives exact counts and source boundaries.
All counts include parent tests and subtests.

## Required job integration

The administrator correction flow remains unfinished. Its owner must retain authenticated intent and immutable history before calling `CorrectWith`.
The job and history records must use the same storage authority as the budget ledger.
The first administrator decision and independent provider evidence remain immutable.

A pending correction must retain the evidence state the operator inspected. New evidence must not silently expand that decision's scope.
Recovery must distinguish an applied decision from a pending decision before constructing another publication.
A fresh operator decision can supersede a stale intent only while preserving its history.

Optional usage reporting needs separate acknowledgement and ordered adjustment history. Its failure must not reverse required settlement or prevent a later budget decision.
The reporter must retain the original accounting entry and use explicit adjustments. Expired reporting history must not recreate counters.

The job service must invoke these contracts before any administrator correction endpoint can claim completion.
No provider call, new public route, release, or merge occurs in this component.
