# Legacy origin recovery contract

Legacy origin recovery remains an open CSP3 architecture repair. The current embedded catalog contains aggregate provenance without separate child receipts. The [inventory](embedded-legacy-inventory.json) binds this finding to the embedded file digest. A merged record can contain several sources even when its aggregate entry names one leading source.

## Owning boundary

The runtime owns the selected baseline and retained original observations. The reconciler owns field selection and provenance. Recovery must use those independent inputs before the runtime publishes a replacement. The accumulated effective catalog cannot attest to its own origin.

`layerSet.selectedBaseline` already separates the admitted upstream or embedded baseline from local observations. `reconcileManualBatch` currently receives a working catalog that can include earlier manual batches. Its synthetic baseline input has no new observation receipt. Treating that working catalog as independent public evidence would grant false authority to private local facts.

`publishInputs` builds and validates a candidate before it stages retained inputs. A build error therefore preserves the current generation and original observation history. `initializeEffective` currently returns an error when its rebuild fails. Safe legacy recovery during startup still needs implementation and tests.

## Required behavior

Recover independent child evidence from retained original observations when those observations prove the value and scope. Preserve their original identities and checksums. A leading aggregate receipt can support refusal, but it cannot identify every contribution.

A separately admitted baseline can prove that a fact belongs to that catalog generation. Its evidence must identify the admitted generation and its authority. It must not claim a new provider observation or a new local edit. Define and validate that evidence contract before adding a snapshot receipt to the manifest.

Preserve genuine local edits as local contributions. Do not infer public authority from a value match against the accumulated working catalog. Preserve explicit null, false, empty records, membership, and exact numbers through recovery.

When independent evidence cannot establish a replacement, reject that candidate with a typed recovery error. Keep the accepted generation available while its policy remains valid. Report the affected layer and the required evidence rebuild or explicit fresh sync. Construction must not start automatic acquisition.

A required permission withdrawal still blocks new inference until the runtime enforces the current policy. A startup fallback must verify current authority and configuration. It cannot restore a previous binding policy only because rebuild failed.

The executor proposed this recovery behavior to the owner. No answer appears in the current record. Execution uses the stated recommendation while preserving the confirmed permission-withdrawal rule. No legacy startup or migration behavior changes in this patch.

## Required evidence

The following scenarios belong to existing CSP3 acceptance. Their behavior checks remain unimplemented.

| Scenario | Required result |
| --- | --- |
| Original observations remain | Recovery preserves per-field source identity and scope. |
| Only the admitted baseline proves a fact | Evidence identifies that baseline generation without an invented acquisition receipt. |
| No independent evidence remains | Candidate refusal preserves accepted bytes and retained observations. |
| A private sentinel exists only in the working catalog | Recovery cannot assign public or embedded authority to that sentinel. |
| A semantic edit changes one child | The edit remains local without granting local authority to unchanged siblings. |
| An accepted child proves record presence | The parent derives presence from that child without reviving a refused claim. |
| The same valid policy resumes after restart | Recovery preserves accepted availability without automatic acquisition. |
| A required permission withdrawal changes policy | Stale fallback cannot permit new inference. Diagnostics remain available. |
| Payload and YAML recovery repeat | Values and original observation history retain their meanings. |

Storage layout changes, new acquisition services, and backend changes are not goals of this repair. Any new public evidence contract requires an explicit design and compatibility assessment before implementation.
