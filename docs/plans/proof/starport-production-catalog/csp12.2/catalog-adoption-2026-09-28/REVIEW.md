# Controlled catalog adoption

CSP12.2 now has a recovery operation that preserves the original publication evidence.
The production backend replacement test passes with PostgreSQL, two Valkey processes, race detection, and Go 1.27.1.
This result closes the recorded startup failure. It does not complete CSP12.2.

## Contract and implementation

Starmap owns `FleetAdoption`, `ValidateFleetRecovery`, and `ValidateFleetReplay`.
The adoption record binds an unchanged publication to a newer deployment recovery identity.
It preserves the catalog generation, private inputs, original acquisition grant, source times, and authority head.
It grants no inference permission or refresh lease.

Starport owns `AdoptFleet` in `internal/catalog`.
The caller supplies the original approval, closed recovery epoch, selected head, replacement backend, operation ID, and evidence reference.
External writer fencing and cross-store reconciliation remain operator prerequisites.
The operation does not infer these actions from copied records.

Recovery validates the selected catalog, accepted catalog, retained inputs, generation history, and pin.
It also checks replay compatibility with the configured source and acquisition policy.
Validation starts no catalog acquisition or filesystem publication.

A native maintenance lease protects validation and selection.
Recovery completes pending payload cleanup and requires explicit consent to abandon reader claims.
One native conditional write changes every retained descriptor, the inventory, selected head, acceptance, and recovery receipt.
The immutable payload chunks stay unchanged.
The same write removes the copied refresh lease and preserves its durable epoch counter.

The operation then installs budget authority before opening SQL approval.
A lost response leaves the gate closed.
An exact retry checks the durable operation receipt and validates all retained data before completing approval.
An already approved retry preserves later ordinary publications.

## Bounds

Existing inventory limits cap retained entries at 96 and payload bytes at 2 GiB.
The inventory encoding remains limited to 16 MiB.
Recovery validates one publication at a time and has a five-minute deadline.
The maintenance lease renews during validation.
Request fields have explicit size limits.
Recovery adds bounded operation and receipt records without copying catalog payload chunks.

CSP13 owns recovery audit retention and the operator archive procedure.

## Verified behavior

The replacement gateway starts after explicit catalog adoption.
It returns HTTP 402 while the original dispatch remains uncertain.
All five meters retain their reservations.
Explicit no-charge reconciliation runs once and permits one new provider dispatch.
An approval for budget storage alone still fails to adopt the old catalog identity.

The Starmap tests verify offline validation, corrupt inputs, incompatible source policy, retained pins, and cancellation.
The authority test verifies that an adopted catalog grants no permission by itself.
A separately obtained receipt permits admission. A known withdrawal blocks it again.
Rereading the adopted catalog does not restore withdrawn permission.

The first native recovery fixture failed because its state directory lacked private permissions.
The corrected fixture lets the runtime create its private child directory.
The original failure remains in the evidence. No access check changed.

## Remaining qualification

The native boundary suite passes all 19 results without failures or skips.
The existing native fleet regression suite passes 37 results.
Repeat recovery into another epoch, simultaneous competing operations, backend changes during selection, and expired authority receipts still need direct adoption evidence.
The complete task gate, cost bounds, retention decisions, interrupted batches, latency, capacity, native CI, review, and paired merges remain required.
No release, paid inference, or remote publication occurred.
