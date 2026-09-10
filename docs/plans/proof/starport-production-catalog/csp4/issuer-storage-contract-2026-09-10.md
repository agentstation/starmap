# CSP4 current authority storage contract

Status: selected implementation contract. No implementation qualification credit.
Source inspection: [issuer storage](issuer-storage-inspection-2026-09-10.md).

## Ownership

The catalog store owns atomic generation selection and its independent permission metadata.
The receipt issuer owns freshness timing, authority identity, and clock qualification.
The connected runtime owns retained subscriber permission. Starport reads the resulting admission state from memory.
No request-path storage read follows from this change.

## Current observations

Keep the existing `Store` interface compatible.
Add an optional `CurrentAuthorityHead(context.Context)` role that returns a `catalogs.CatalogAuthorityHead` and an error.
Its result must identify a publication that was current between invocation and completion, including another writer's publication.
The method reads no catalog payload and does not require catalog manifest compatibility.
The method creates no files, repairs no state, and starts no background activity.

An ordinary selected generation reports no authority. It never returns an earlier authority head.
Missing or incompatible permission metadata returns an error. Neither result permits a fresh receipt.

The root client's existing method remains a cached snapshot. Only the store role promises a current observation.
The issuer must use the store role and start receipt validity before its read.

## Immutable metadata

Each new authority generation stores `authority.json` beside its manifest and payload.
The bounded version-1 record contains only its format version and the complete authority head.
Its head identifies the containing generation. It has no receipt timestamps.
Permission schema versions remain independent of this storage format and catalog schemas.
Unknown positive permission versions remain observable so an older consumer can refuse their semantics.

The record must exist before the accepted pointer selects that authority generation.
The filesystem and object current-pointer formats remain unchanged.
A single pointer therefore selects both catalog data and the immutable permission record.
An ordinary generation has no authority record. Its absence prevents authority receipt issuance.
Readers enforce a 16 KiB limit and reject duplicate, unknown, malformed, or mismatched fields.

## Existing stores and repeated commits

A read never silently adds metadata to an existing generation.
An explicit identical authority commit may add a missing record after verifying the complete existing generation.
That repair must finish before the commit reports success, including when the generation is already current.
An existing matching record is idempotent. A conflicting record refuses the commit and remains unchanged.

Readers can refuse while a repair is incomplete. They cannot infer a fresh head from cached state.

Filesystem repair uses the existing commit lock and private-file publication contract.
Object repair uses an immutable conditional create. A conflict requires exact record equality.
No repair changes the generation ID, manifest, payload, or selected permission revision.

## Backend capabilities

Memory reads select the head under the same lock as generation publication.
Local filesystem reads select the current pointer and then its immutable record.
The filesystem contract requires the documented local volume semantics. This change does not qualify shared network filesystems.

Object receipt issuance requires an explicit backend method that guarantees a current object read.
The existing arbitrary `ObjectBackend.Get` remains insufficient for receipt issuance.
A backend without that capability retains ordinary catalog storage and refuses current authority observations.
The reference memory object backend can implement the guarantee under its existing lock.
The S3 adapter must require endpoint qualification before asserting the same guarantee.
A caller-selected SDK, endpoint, or transport alone provides no qualification.

## Required evidence

| Case | Observable result |
| --- | --- |
| Another writer publishes a withdrawal | A new store observation returns that publication or a later publication. |
| Head read stalls across publication | Receipt timing retains the interval that started before the read. |
| Payload is unavailable or uses a newer schema | The independent head remains readable when its own record is valid. |
| An older writer selects an ordinary generation | A new observation refuses authority receipt issuance. |
| Metadata write fails | The new authority generation cannot become current. |
| Existing authority generation lacks metadata | Read refuses until an explicit validated commit completes repair. |
| Identical current commit | Complete metadata is present before success. |
| Conflicting metadata | Commit refuses without replacing the record or current pointer. |
| Backend returns stale ordinary reads | Receipt issuance requires its separate current-read capability. |
| Concurrent writers, reopen, and cancellation | One selected generation owns the observed head and typed failures preserve prior state. |

The storage evidence does not qualify origin issuance, clocks, shared followers, or Starport consumers by itself.
All eight CSP4 acceptance checks and the required implementation merges remain necessary.
