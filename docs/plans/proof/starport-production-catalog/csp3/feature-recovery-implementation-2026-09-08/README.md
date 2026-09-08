# Capability provenance and recovery

Reconciliation now records a separate source entry for each Boolean capability and each documented modality. Generation selection retains every contributing receipt. The aggregate feature entry continues to identify the leading authority.

The runtime regression covers ten recovery scenarios through publication, filesystem persistence, reopen, and source refresh. Cases include missing records, omitted capabilities, explicit false, unknown values, retained false, and complete replacement. Input and output modalities retain their own evidence. Original observation history remains available.

Reasoning policy can remove a subordinate capability after field selection. Its resulting provenance now describes the policy result and clears the obsolete provider claim.

The missing-evidence path checks for legacy metadata before it checks global model-ID uniqueness. An ordinary Go 1.26.6 check measured 19 and 121 allocations before the change for one and 101 models. The corrected lookup uses nine allocations for either catalog. These measurements describe refresh work, not request latency.

The captures preserve intermediate candidates and their failures. Unknown-value projection and modality projection required additional repairs. The final source records and command results identify the qualified revision. The original [failure record](../feature-recovery-2026-09-08/verification.json) remains available.

CSP3 still requires full presence coverage, scoped membership, removal review, application integration, and released-pair acceptance. The owner decisions about global removal authority and the default review threshold remain pending.

The repair is commit `dc54fe0f`. [Final verification](verification.json) records 874 runtime, acquisition, and reconciler race events, plus 588 catalog events. The minimum-Go checks pass fifteen events. All counts include parent tests and package events. Ago and package lint report no findings.
