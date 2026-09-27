# Output retirement and delayed publication

Status: required CSP12.2 storage repair. CSP13 retains migration and reclamation ownership.

## Evidence

The [probe](verification.json) pauses a prepared-output writer immediately before blob publication.
Another service expires the file, deletes its metadata, and closes its byte claim.
The writer then resumes. Publication fails at the file layer, but a blob remains without metadata or a storage charge.
The failure reproduces with memory, Badger, and Valkey metadata over filesystem blobs.

Restore `output_retirement_probe_test.go.txt` as `internal/files/output_retirement_probe_test.go` before the repair.
Run the exact command in the proof. Preserve its assertion that retired bytes cannot reappear after quota release.

## Required ownership

The blob owner (`internal/blob`) must enforce publication and retirement ordering at the backing store.
The file owner (`internal/files`) must retain the file and its byte claim until the backend confirms retirement.
The limit owner (`internal/limits`) must release the durable claim only after that confirmation.
A final metadata check after publication cannot provide this guarantee.

## Required contract

1. A retired object identity cannot accept a delayed publication from an earlier writer.
2. Confirmed publication remains immutable. Exact retries inspect the same identity and content.
3. Lost publication or retirement acknowledgments preserve recoverable evidence.
4. A failed or uncertain retirement retains the file record and its charge.
5. Filesystem and shared-object backends must provide the same observable contract.
6. Account isolation, content validation, and retention remain mandatory.
7. Recovery never repeats inference or assumes that a timeout proves a writer stopped.
8. Retirement metadata must survive process loss and backup recovery. Do not discard it while an old writer can resume.
9. Any reclamation protocol must name the proof that old writers cannot publish, its storage cost, and its migration owner.

Use conditional backend publication and durable retirement evidence, or an equivalent protocol that proves these properties.
Do not replace this requirement with a longer grace period or an in-process lock.
Do not claim object-store qualification from filesystem-only tests.

## Remaining sequence

Repair and qualify this backend boundary first. Then reconstruct aggregates from retained line results, publish aggregates under stable identities, and clean checkpoints after confirmed publication.
Complete interrupted-run behavior after the owner answers its pending policy question.
Keep the producer-then-consumer PR sequence and full CSP12.2 acceptance gates.
