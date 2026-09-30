# Populated storage adoption contract

CSP13 must support recovery of a surviving populated deployment after a native
KV restart. Import into unused storage is a separate operation. The existing
import claim must retain its empty-target requirement.

The owning concepts are independent recovery history, native storage claims,
SQL approval, blob claims, catalog approval, and application activation.
`operator_recovery_inputs` owns this extension. The root session reviews it.
The complete populated adoption procedure remains UNVERIFIED.

## Evidence and refusal

The application must bind the original backup and independently retained complete
post-backup history. It must also bind independently known SQL approval and
closed epoch, selected live native identity, operation identity, and private recovery journal.
An operator-supplied digest cannot establish the live storage census.
An external fence reference cannot prove that all writers stopped.
Independent process or network fencing remains mandatory.

Concept owners must derive the expected complete state from the original
backup and independent history. They must compare it with an exact bounded
census of live KV, SQL, blob bytes, and canonical files. Refuse if an acknowledged
record is absent, changed, or outside the checked history. Retain revoked keys
and grants, spent and held charges, uncertain attempts, catalog state, and
linked file and job bytes. Do not replay old preimages over committed work.
Do not refund or resubmit uncertain work to make recovery pass.

## Adoption and activation

Restricted native adoption capabilities must retain existing bytes and prior
activation receipts. They must install operation-bound closed barriers and
refuse changed preimages, native identities, or SQL boundaries. A general
`AdoptFleet` call alone does not establish this contract.

The application may reuse the checked domain replay and activation owners after
these capabilities exist. The new epoch must exceed independently known
history. SQL admission can open only after budget approval and catalog approval
bind the same recovered deployment and native identity.

Crash retries must reuse the original census, receipts, operation identity, and
deadlines. They must not recapture a changed target as new authority, erase
records, reset bootstrap permission, or renew retained permission. Missing
independent history must refuse admission.

## Required evidence

1. Prove pure history projection and current-image verification. Include
   acknowledged loss, changed state, retained uncertainty, and conflicting
   independent epochs.
2. Prove restricted native claims and exact census guards at their owners.
   Include process loss between each durable claim and its reply.
3. Exercise application and operator commands with populated storage. Prove
   exact retry, coherent approvals, and no changes on refusal.
4. Exercise actual native restart, promotion, acknowledged data loss, and
   restored SQL witness in separate process tests. Keep the previous primary
   reachable while proving that its observed closed permission blocks dispatch.
5. Measure recovery point and recovery time on the reference deployment.
   Numeric product objectives remain an owner decision.

An empty-target restore after a restart supplies partial evidence. It cannot
qualify the full restart-and-recovery case or the in-place adoption procedure.
