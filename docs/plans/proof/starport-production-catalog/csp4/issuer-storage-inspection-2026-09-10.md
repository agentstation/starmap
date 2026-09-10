# Authority issuer storage inspection

This inspection concerns the second CSP4 delivery. It changes no implementation or accepted permission bound.
The inspected Starmap source is `44a400a481748c5e7ae1268cecb63b260a91bdab`.

## Observed storage contracts

`pkg/catalogs/storage/store.go` exposes `Current`, `Get`, and `Commit` for complete generations.
The memory store copies the selected generation, including its payload.
The filesystem store reads its current generation ID, manifest, and payload.
The object store reads a current JSON pointer containing the generation ID, then retrieves the manifest and payload.

`Client.CurrentAuthorityHead` reads the committed client snapshot without storage access.
It follows that client's publication. It does not observe another writer's later durable publication.
The first CSP4 delivery therefore does not use this snapshot to mint fresh permission receipts.

## Required second-delivery contract

The issuer needs a trusted current-head observation independent of catalog payload transfer and compatibility.
The required permission head and selected generation must share one publication decision.
A cached snapshot, notification, or failed storage read must not extend the previous observation's validity.
A backend must establish current durable state before claiming a fresh authority observation.

The small head format must remain readable when the catalog payload exceeds the issuer's supported schema.
The design must also account for future manifest versions without silently treating an old cached head as current.
Select the storage representation and migration rules before changing the backends.
The memory, filesystem, and conditional object adapters need the same observable contract.
Embedding applications retain ownership of SQL and key-value adapters.

A receipt's validity must not start after an unbounded head read and thereby extend stale permission.
Qualification must cover a withdrawal during the read, delayed completion, lost events, restart, and clock uncertainty.
The existing five-minute bound and thirty-second uncertainty ceiling remain mandatory.
The issuer must account for both its own clock evidence and the consumer's clock evidence.

## Object read consistency

The current `ObjectBackend` contract specifies atomic conditional writes and exact object-version tokens.
It does not require a read to observe the latest committed pointer.
An exact ETag identifies returned content. It does not establish that the content is current.
Adding a head method that delegates to arbitrary `ObjectBackend.Get` would therefore leave the issuer's freshness requirement unproved.

The second delivery must require an explicit current-read contract for receipt issuance.
The returned head must be current at some point between the read's invocation and completion.
That guarantee includes other writers' committed publications.

Ordinary generation storage can retain its existing interface. An adapter without the current-read guarantee cannot issue new permission receipts.
The S3 adapter delegates endpoint, client, and transport selection to its caller.
Its type alone cannot qualify every compatible service.

Receipt timing must start before the current-head read.
A head change during that read can leave the returned head superseded, even when the read contract holds.
Delayed completion must consume the original validity interval. It must not start another five-minute interval.
An expired interval requires refusal and a fresh observation.

## Publication and migration checks

The filesystem pointer contains a generation ID as text. The object pointer contains a JSON generation ID.
Neither pointer currently carries a permission head. The generation manifest owns the head in the first delivery.
Both stores return early when the selected generation already matches an identical commit.
Any added head representation must account for those idempotent paths and existing authority generations.

The storage design must bind permission metadata to the same immutable generation before pointer promotion.
A separate mutable permission pointer would require another atomic publication contract.
A record beside each immutable generation could preserve the current pointer formats, but needs explicit creation and migration rules.
This inspection does not select a new file format or authorize silent migration during a read.

Qualification must distinguish these cases:

| Case | Required evidence |
| --- | --- |
| Conditional writes with stale reads | Ordinary storage remains usable; the issuer refuses to treat those reads as fresh authority. |
| Withdrawal during a delayed head read | The returned receipt cannot extend the validity interval that began before the read. |
| Existing authority generation without the new head representation | Reads report the missing capability or metadata; no cached head supplies a new receipt. |
| Repeated identical commit | The defined migration or repair contract establishes complete permission metadata before reporting success. |
| Newer catalog or manifest schema | Independent permission metadata remains readable, or receipt issuance fails without renewing retained permission. |
| Older writer selects an ordinary generation | The issuer refuses new authority receipts; it cannot keep renewing the prior authority head. |

## Evidence paths

The inspected files are `authority_head.go`, `pkg/catalogs/storage/store.go`, `memory.go`, `filesystem.go`, and `object.go`.
The storage filenames share the `pkg/catalogs/storage/` parent.
The S3 adapter is `pkg/catalogs/storage/s3/backend.go`.
This inspection provides design evidence only. It does not qualify a storage adapter or a production deployment.
