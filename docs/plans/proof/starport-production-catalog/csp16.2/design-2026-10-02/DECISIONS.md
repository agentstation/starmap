# CSP16.2 decisions

Date: 2026-10-02. Each decision is an engineering interpretation inside the accepted task scope.
None needs new owner authority. The owner can reverse any of them before the final acceptance run.

| ID | Decision | Reason and consequence |
| --- | --- | --- |
| 16.2-1 | The recovery record format does not change. The operation receipt lives in Starport. | The decoder rejects unknown members and other versions (`fleet_recovery.go:150-161`). A new field would make every released binary refuse replay after promotion, which breaks software rollback (condition 13). |
| 16.2-2 | Promotion is a manual run under the publication lease. | `ReplaceRemovalTargets` is the exemplar. The lease, the head compare-and-set, and the increasing revision come from the existing commit path. |
| 16.2-3 | Promotion refuses while a source authority is configured. | D41 names promotion for embedded-only fleets. A configured source needs a coordinated apply, not a baseline promotion. |
| 16.2-4 | A removal target absent from the promoted baseline stays recorded and inert. | Retained removals are explicit operator rules (condition 16). Dropping them silently would change policy without an operation. The receipt lists them. |
| 16.2-5 | The five subcases register under A14 and join `task_checks.CSP22`. | A14 owns fleet replay across binary changes. The verifier requires every candidate-case subcase in CSP22. |
| 16.2-6 | Status compares packaged and retained identities in memory. | Operators see whether a promotion is available without a storage call (condition 17). |
| 16.2-7 | Two PRs: one Starmap PR with the runtime operation and the subcase registration, then one Starport PR. | Starmap #212 set the pattern. Starport pins the Starmap module, so the Starmap PR must merge and tag before the Starport PR can depend on it. |

## Pending

None. The owner decided the fleet-only scope and the task split on 2026-10-02 (see `csp16.1/design-2026-10-02/DECISIONS.md`).
