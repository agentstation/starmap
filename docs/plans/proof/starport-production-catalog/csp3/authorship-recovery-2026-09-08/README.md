# Authorship recovery

Each accepted author membership retains its own source receipt. The selector uses one resolved input for membership and author details. Rejected stale inputs cannot supply details for accepted carried membership. The aggregate `Authors` entry identifies a leading accepted contribution. A rejected membership clears its current positive claim without changing the original observation history.

Projection policy can refuse a carried membership after an author-detail edit. An explicit new local membership remains eligible. Omitted claims preserve baseline membership. Returned descriptions, aliases, and logo bytes remain independent of source data. Authorship changes preserve the provider-serving identity and canonical model link.

The runtime tests cover retention and replacement through filesystem publication, restart, and source refresh. They check exact receipts, generation degradation, retained author membership, and unchanged original observation history. The YAML test retains separate provider and models.dev receipts after workspace reload.

The final tests record eleven failure events and one pass against the prior production selection. The passing case preserves a missing local claim. The unapplied prototype records two failure events when it selects stale author details. A later regression test records four failure events for stale positive receipts after rejection.

The corrected selector passes all twelve focused minimum-Go events. Ago reports zero findings, stale ignores, and errors. Package lint reports zero issues.

An intermediate full race suite passes 967 events before the receipt-clearing repair. Its source snapshots remain in `before-cleared-receipts`. The final runtime, acquisition, and reconciler race suite passes 967 events with no failures or skips. The [verification report](verification.json) retains intermediate and final counts. Event counts include parent tests and package results. These counts do not represent acceptance subcases.

## Remaining failures

The legacy probe constructs an aggregate-only evidence shape by removing per-author entries from a generated catalog. It preserves the aggregate receipt and original observation identity. Its projection policy rejects that original authorship. The membership still survives, producing two failure events. This probe does not claim a complete upgrade test against an older binary.

The description probe covers unknown, empty, and nonempty descriptions through runtime recovery. Unknown descriptions disappear and lose their receipts. The capture records four failure events and four passes. Empty and nonempty descriptions pass the retention and replacement scenarios.

The scoped composite-policy probe checks five facts in both current and aggregate-only evidence shapes. Current authorship passes. Metadata, mode, and extension values clear, but their current receipts remain. Capability values and receipts remain. All five legacy cases fail, for thirteen failure events and one pass with parent and package events.

Resolve rejected composite receipts and the legacy projection boundary before publication. Rebuild original contributing evidence where retained observations permit it. Do not invent independent receipts from an aggregate that cannot identify each contributor. Complete description, architecture-flag, and optional-record presence, then scoped membership and deletion review. CSP3 remains incomplete.
