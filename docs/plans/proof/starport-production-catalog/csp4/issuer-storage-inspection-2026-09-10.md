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

## Evidence paths

The inspected files are `authority_head.go`, `pkg/catalogs/storage/store.go`, `memory.go`, `filesystem.go`, and `object.go`.
The storage filenames share the `pkg/catalogs/storage/` parent.
This inspection provides design evidence only. It does not qualify a storage adapter or a production deployment.
