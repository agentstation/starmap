# Legacy origin recovery prototype

The prototype remains outside production source.
It refuses replacement when aggregate legacy provenance cannot establish a permitted child fact.
Original observations or a verified baseline can establish that fact.
A synthetic copy of the current catalog cannot establish its own authority.

The final prototype requires the baseline value to match the retained value.
It records the selected baseline generation and payload digest in the observation link.
A mismatched baseline value cannot silently replace a legacy fact.
The complete reconciler candidate passes 367 events, including its package result.

The prototype still fails an upgrade scenario.
An older accepted history contains a private fact that the compiled baseline cannot prove.
Startup rebuilds that history and returns `reconciliation.legacy_provenance`.
The next repair must preserve accepted availability only when current authority and configuration still permit it.
A blanket fallback would violate required permission-withdrawal enforcement.

## Captures and stages

The [archive manifest](archives.json) binds 98 captures by SHA-256.
The [results](results.json) separate test and package events for each recorded run.
All listed prototype runs have zero skip events.
These files also retain the catalog byte-limit investigation that the upgrade probe exposed.

| Stage | Captures | Interpretation |
| --- | --- | --- |
| Initial refusal | `before.jsonl`, `candidate.jsonl`, `reconciler-candidate.jsonl` | The old source invents child receipts. Refusal repairs isolated cases. |
| Expanded evidence cases | `expanded-before.jsonl`, `expanded-candidate.jsonl` | Original observations can recover permitted facts. |
| Invalid first runtime fixture | `runtime-before.jsonl`, `runtime-candidate.jsonl` | Both fail because the selected model lacks canonical authorship. No regression credit. |
| Corrected compiled model | `embedded-before.jsonl`, `embedded-candidate.jsonl` | Refusal alone breaks ordinary embedded publication. |
| Admitted baseline | `embedded-admitted.jsonl`, `admitted-reconciler.jsonl`, `runtime-final.jsonl` | A verified baseline repairs ordinary publication and restart. |
| Baseline generation binding | `runtime-bound.jsonl` | Observation links identify the selected baseline generation and digest. |
| Baseline value matching | `matching-before.jsonl`, `matching-final.jsonl` | A mismatched baseline cannot silently substitute another value. |
| Original accepted upgrade | `upgrade-write.jsonl`, `upgrade-open-original.jsonl`, `upgrade-open-candidate.jsonl` | The old writer produces a catalog above its own reader limit. |
| Upgrade with the repaired codec | `budget-upgrade-original.jsonl`, `budget-upgrade-candidate.jsonl` | The catalog reopens with the current reconciler. The legacy prototype still refuses startup. |
| Small fixture creation | `upgrade-small-write.jsonl` | Fixture creation only. No startup qualification. |

The original source is commit 33728b3f.
Compiler overlays substitute the candidate files without changing repository production files.
The `admitted` directory retains the initial, generation-bound, and value-matching variants separately.
The `catalog-budget` directory retains the independent 32 MiB codec candidate.

Use `before-expanded-preserved.json` and `candidate-expanded-preserved.json` for the expanded captures.
They reference `recovery_expanded_original_test.go`, which preserves the tests before later option cases.
The later `recovery_expanded_test.go` contains those additional cases.
Do not attribute an earlier capture to the expanded file after those additions.

For reproduction, extract compressed files into a private temporary directory.
Rewrite the overlay source prefix for the selected checkout.
Rewrite the overlay replacement prefix for the extracted files.
Use Go 1.26.6 and the test selector named by each probe.
Minimum-Go captures use Go 1.25.12.
The runtime fixture probes require a separate `CSP3_LEGACY_FIXTURE` directory for each independent write scenario.

The candidate still needs checked startup recovery, complete source-scope coverage, operator recovery diagnostics, and publication review.
No native or released-pair qualification follows from these overlays.
