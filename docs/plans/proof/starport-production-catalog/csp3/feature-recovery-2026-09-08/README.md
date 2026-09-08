# Capability recovery defect

The generation at commit `80a684fa` drops a receipt that still supplies a capability. A partial provider reply supplies `tools=true`. A later complete reply supplies `streaming=true` and omits tools. Both known values survive, but the generation removes the partial receipt and reports `degraded=false`.

The [probe record](verification.json) preserves the exact test and failure output. It produces two failure events: the test and its package. The test runs through runtime publication with original provider observations. It does not qualify transport or Starport behavior.

Capability reconciliation records one receipt for the merged feature group. Receipt selection therefore lacks evidence for an older contribution to that group. CSP3 must preserve the contributing evidence before it can retire a receipt. The fix must cover capability omission, explicit false, unknown values, and later complete replacement.

Source publication remains open. The earlier pricing recovery checks remain valid within their tested scope.

Commit `dc54fe0f` repairs this case. The [implementation evidence](../feature-recovery-implementation-2026-09-08/verification.json) covers persistence, restart, and later source refresh.
