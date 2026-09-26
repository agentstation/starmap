# Provider metadata review repair

The review of `95320e13b` reported two P2 findings. Sol reported none. Opus reported both.

## Accepted: pin prevents ownership and unpin

The served catalog can omit a provider whose acquisition inputs remain retained.
The capability check used that served catalog for provider metadata.
The regression failed at lease renewal before repair.

The runtime now retains reconciled provider metadata before serving pins and
removal policy. The private metadata catalog omits model payloads.
It belongs to acquisition state and adds no inference-request lookups.
The test verifies pin stability, ownership renewal, metadata size, and restored
provider models after unpinning.

An intermediate assertion failed because provider observations cannot add
unauthored models. The corrected fixture first publishes the later upstream
model definition, then retains its provider observation. The assertion remains.

## Rejected: nil context after failed grant capture

The review did not include the complete `runtime/refresh.go` function.
`execute` calls `work` only when `workErr == nil`.
After failed capture, it records the report and calls `r.runs.finish`.
Neither operation uses `runCtx`. Run cleanup uses the active run's cancel
function. No nil-context use exists on this path. The production code needs no change.
