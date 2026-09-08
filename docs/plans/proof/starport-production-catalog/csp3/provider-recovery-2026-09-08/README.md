# Provider evidence recovery

A complete provider reply now retires a superseded receipt from the active generation. Durable history keeps the original receipt and payload. Current facts, unresolved review candidates, and uncovered offerings retain their evidence.

Replay previously retained every provider receipt, which kept a recovered generation degraded. Replay also relabeled retained pricing as upstream data and lost its provider receipt. The local projection now preserves that provenance before receipt selection.

The [verification record](verification.json) binds the final source files and commands. Both Go toolchains pass thirteen race events. The broader policy and replay suite passes 83 race events. The old implementation produces four failure events across two scenarios and their parent events.

Five publication scenarios cover full replacement, retained pricing, the same binding, another account binding, and an uncovered offering. Each scenario checks publication, retained history, and runtime reopen. Separate checks cover review candidates, incomplete replacements, equal timestamps, older evidence, and cancellation.

The intermediate fixture failures remain in the captures. Their partial observation omitted its required issue. The corrected fixture supplies that issue without weakening assertions.

CSP3 remains in progress. This change does not establish removal authority, transport restrictions, or released-pair acceptance. CSP5 still owns physical history collection.
