# Grouped budget reads

The previous five-meter request used eighteen record reads, one clock read, three native writes, and six SQL approval queries.
The new implementation groups meter and history reads within each conditional-write attempt.
It uses five record-read commands, one clock read, three native writes, and six SQL approval queries.

| Phase | Previous calls | Current calls |
| --- | ---: | ---: |
| Before provider dispatch | 19 | 10 |
| Settlement | 9 | 5 |
| Complete budget operation | 28 | 15 |

These counts describe one successful uncontended request with warm authorization and five applicable meters.
They exclude cold authorization, connection setup, optional reporting, unrelated KV work, conflicts, and recovery.
They do not establish elapsed latency, capacity, or allocations.

## Storage contract

`TimeBoundStore.ReadBatchWithLifetime` returns values and lifetime evidence from one native snapshot.
Results retain input order and distinguish missing records from empty values.
A batch accepts one through sixteen unique nonempty keys.
The caller bounds each value before copy or transfer. The maximum possible batch payload is one MiB.

Any validation, size, context, or backend error returns no partial result.
Badger uses one read transaction. Valkey uses one EVAL with its approved incarnation check.
A replacement Valkey process cannot accept the original process approval.

## Reservation contract

A reservation snapshot belongs to one conditional-write attempt.
Admission groups each meter with its continuous-history record.
Settlement groups the original budget-window records.
A conflict discards the snapshot. The retry reads all selected records again.

Records outside the group use fresh reads, including an attempt check after a concurrent settlement.
Window rollover still verifies the previous history window before creating the next window.

No snapshot survives into another request. No cached balance authorizes a provider call.
All native mutation checks and all independent SQL approval reads remain in place.
Missing history, expiring records, changed incarnation, uncertain usage, and inconsistent balances retain their refusal behavior.
No payload version or configuration changes.

## Verification method

The initial production test failed because eighteen record reads exceeded the new five-read limit.
The first grouped implementation passed the storage tests and reduced the measured count.
An existing recovery fault fixture intercepted only individual reads, so its peer-settlement assertion failed.
The fixture now inserts the same peer settlement before either individual or grouped reads. Its original state assertions remain.

New tests cover order, missing and empty values, expiry, size bounds, invalid requests, cancellation, returned-value ownership, and concurrent atomic updates.
A deterministic reservation test changes consumption after a snapshot and requires a fresh read after the conditional-write conflict.
Another test refuses expiring history without creating a reservation or increasing any meter.

Manual review checked request-local ownership, conflict retries, byte bounds, incarnation enforcement, SQL approval, and test cleanup.
The native replacement test uses two independent Valkey processes.
Performance and fleet recovery qualification remain required before CSP12.2 can close.
