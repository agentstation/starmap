# CSP3 reset projection checks

The existing runtime passes ten focused race test events for catalog reset projections.
These tests add coverage. They require no production code change.
The [verification record](reset-projection/verification.json) binds the two test files and six captured outputs to their SHA-256 digests.

| Contract | Evidence |
| --- | --- |
| An unchanged acquired-only offering disappears after reset and remains absent after restart. The baseline offering remains. | `TestResetRemovesAcquiredOnlyOfferingFromProjection` |
| Metadata acquisition cannot add a provider outside the selected baseline. Reset and restart preserve that restriction. | `TestResetCannotIntroduceProviderOutsideSelectedBaseline` |
| Reset removes a projected zero from the cleared acquisition scope and restores the known baseline limit. | `TestProviderResetProjectedLimitPresence/unchanged-explicit-zero` |
| An operator's explicit zero remains known zero after reset and restart. | `TestProviderResetProjectedLimitPresence/operator-explicit-zero` |
| Unknown and missing local limits permit the known baseline fallback. | The `operator-unknown` and `operator-missing` subtests |
| An unchanged known acquisition value resets to baseline. A changed operator value survives. | `TestProviderResetProjectedFactsAndOperatorEdits` |

Each new test checks restart and the accepted generation identity.
The unknown and missing cases follow the existing field authority policy. They do not mean deliberate deletion.
The offering check observes quarantine after the reset removes its only accepted canonical model reference.
It does not prove every membership or operator-edit combination.

The first provider fixture incorrectly expected metadata to introduce a provider outside the selected baseline.
That premise failed before reset. The final test verifies the existing membership restriction instead.

A separate test compile attempt used a nonexistent `ModelLimits.Get` method. The corrected test calls `ModelLimits.Value`.
Both unsuccessful attempts remain in the evidence directory. Neither is a production defect reproduction.

The focused normal run passed seven test events. The focused race run passed ten test events, including the existing operator-edit checks.
The pinned ago check passed without findings or incomplete errors.
Production source remains unchanged from the local developer milestone, whose normal and race suites passed across 79 packages.

Scoped tombstones, complete deletion authority, other field-presence combinations, native qualification, and released-pair acceptance remain open.
CSP3 remains in progress. These checks grant no primary acceptance credit.
