# CSP12.2 concurrent budget failure

The current middleware admits two requests against a one-token key budget.
All three race-enabled repetitions failed the exclusive-capacity assertion.
The probe uses real on-disk Badger and the production usage repository.
It holds both admitted handlers before either writes usage.

[The record](concurrent-budget-before.json) identifies the source pair and command.
[The probe](concurrent-budget-probe.go.txt) supplies the overlay test.
[The complete output](concurrent-budget-before.jsonl.gz) preserves all three failures.
The overlay leaves the reviewed implementation tree unchanged.

Two runs report two consumed tokens. One run reports one consumed token.
Neither write returns an error. The accounting discrepancy needs a separate
regression test. Verify its cause before claiming a repair.

Badger expiry currently reads a value and writes it in separate operations.
The probe does not prove that this code caused the missing token.

CSP12.2 must replace the read-then-admit decision with atomic capacity reservation.
Its acceptance scope still includes account, key, and team limits, provider
attempts, uncertain charges, retries, recovery, and shared storage.
This single-process middleware probe does not qualify those requirements.
CSP12.2 remains todo. CSP6 remains the only task in progress.
