# Accepted recovery and correction policies

The owner answered both product questions on September 28, 2026.
These decisions remove the CSP12.2 owner blocker. Implementation and qualification remain open.

## Batch restart

Automatically resume only lines that durable records prove never started.
Retain completed results. Never repeat uncertain attempts automatically.
Use current caller authorization and normal paid-operation admission for each resumed line.
Preserve cancellation, account isolation, input identity, and per-line attempt ownership across competing workers.

## Settled-charge correction

Allow administrators to correct a settled charge for 90 days after its original settlement.
A correction does not restart this horizon. Exact accepted retries remain idempotent.
Keep unresolved reservations until reconciliation. Never release capacity because a retention deadline passes.
Preserve audit evidence for accepted decisions and corrections.

## Remaining evidence

Qualify restart, competing recovery, permission withdrawal, and the correction deadline in their owning packages.
Complete the task matrix, real-storage checks, review, native CI, published dependency qualification, and paired merges.
Historical blocked observations remain unchanged.
