# Capability recovery defect

The current generation drops a receipt that still supplies a capability. A partial provider reply supplies `tools=true`. A later complete reply supplies `streaming=true` and omits tools. Both known values survive, but the generation removes the partial receipt and reports `degraded=false`.

The [probe record](verification.json) preserves the exact test and failure output. It produces two failure events: the test and its package. The test runs through runtime publication with original provider observations. It does not qualify transport or Starport behavior.

Capability reconciliation records one receipt for the merged feature group. Receipt selection therefore lacks evidence for an older contribution to that group. CSP3 must preserve the contributing evidence before it can retire a receipt. The fix must cover capability omission, explicit false, unknown values, and later complete replacement.

Source publication remains open. The earlier pricing recovery checks remain valid within their tested scope.
