# Production catalog plan audit: 2026-09-05

The plan passes structural checks but needs seven corrections before unattended execution.
Three findings have P1 priority. Four have P2 priority.
The accepted product boundaries remain consistent. The main gaps concern release gates, spending, recovery, and task coverage.

This audit changes no PRD, specification, plan, acceptance roster, product code, or task status.
The plan remains `proposed`, with all 38 tasks `todo`.
Recommendations below require a later document revision. They are not completed implementation work.

## Scope and evidence

The review covers the complete PRD, engineering specification, plan, acceptance map, and README demonstration brief.
It checks the findings, prior review dispositions, status indexes, and relevant source contracts against those documents.
Source inspection uses these unchanged revisions:

| Repository | Revision |
| --- | --- |
| Starmap | `4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225` |
| Starport | `042eb97851496ecdd1ef20bfa44fe3d84b86b2d8` |

The [input manifest](input-manifest.json) records document hashes.
The [structural result](structural-check.json) reruns the existing verifier without replacing its historical report.
The [semantic checks](semantic-checks.json) expand dependency ranges and compare candidate check rosters.
The [Markdown check](markdown-check.json) records parsed acceptance rows.
The [verification result](verification.json) records final checks and limitations.

The audit verifies relevant external contracts against official documentation.
GitHub's documented workflow behavior still supports the proposed separate bot identity.
Actual repository protection and hosting setup remain unverified. See [GitHub workflow triggers](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow).

## Findings

| ID | Priority | Correction | Proposed owner |
| --- | --- | --- | --- |
| CA01 | P1 | Keep locally executable release checks in the candidate gate. | CSP0, CSP22 |
| CA02 | P1 | Define an admission barrier for lost or uncertain shared-store history. | CSP11, CSP12.2, CSP13, CSP15 |
| CA03 | P1 | Enumerate every paid operation covered by reservations. | CSP12.2, CSP15, CSP22 |
| CA04 | P2 | Define budget windows, valuation, and reconciliation identities. | CSP12.2, with product review before implementation |
| CA05 | P2 | Make recovery depend on reservation implementation. | CSP13 |
| CA06 | P2 | Assign Starmap's configuration report to a task and positive tests. | CSP14, with CSP1 descriptors |
| CA07 | P2 | Repair the acceptance table and verify its rendered structure. | Documentation revision and document verifier |

### CA01: Candidate qualification omits checks that need no publication

**Evidence.** [CSP22](../../../starport-production-catalog-plan.html#task-CSP22) requires 43 complete cases.
The [acceptance map](../acceptance-map.json) excludes all subcases of the seven publication-dependent cases from that task.
Its candidate roster contains 282 of the 309 required subcases.

Six excluded checks already run locally in earlier tasks:

- `A29.embedded_offline_search`
- `A29.recovery_without_auth`
- `A29.no_dynamic_data_disclosure`
- `A35.each_advertised_native_install`
- `A35.clean_catalog_no_keys`
- `A35.documented_inference`

CSP18 and CSP20 require those checks. CSP23 or CSP24 repeats them after publication.
An earlier pass does not prove that the final candidate still passes after later content or build changes.
The prose requires regression checks, but the explicit candidate roster omits these checks.

**Consequence.** Candidate qualification can pass while final candidate docs require authentication or an advertised archive no longer completes setup.
The release could expose that regression before its required post-publication check.

**Correction.** Keep the seven incomplete primary cases UNVERIFIED until publication.
Require their locally executable subcases against the exact candidate as additional candidate prerequisites.
Bind those checks to the candidate artifact and content manifest.
Invalidate them when either input changes. Include applicable checkout and embedding checks from A06.

**Acceptance.** Break offline search or candidate installation after an earlier task passes.
The candidate gate must fail before CSP23 can publish.
A partial primary case must not count as a complete pass.

### CA02: Failover restrictions lack an enforceable history barrier

**Evidence.** Specification sections [8.6](../../../../design/catalog-lifecycle/ENGINEERING_SPEC.md#86-recipe-format-migration-and-disaster-recovery) and [8.9.4](../../../../design/catalog-lifecycle/ENGINEERING_SPEC.md#894-atomic-limits-and-recoverable-reservations) require restricted admission after missing history.
Section 8.8 permits a declared acknowledged-write loss bound for durable Valkey.
The documents do not select how every replica discovers an uncertain history before trusting restored records.

This is a contract gap, not a claim that atomic Valkey operations cannot work.
Atomicity on one primary does not establish continuity after promotion or restoration.
Valkey documents possible loss of acknowledged writes during failover, including when applications use `WAIT`.
See [Valkey WAIT consistency](https://valkey.io/commands/wait/) and [replication behavior](https://valkey.io/topics/replication/).

**Consequence.** A promoted store can return a valid-looking balance without an acknowledged reservation.
It can also return an older permission record.
A newly started gateway has no prior memory with which to detect that loss.
A successful read or ordinary reconnect cannot establish safe admission.

**Correction.** Select the supported acknowledgment and recovery protocol before implementing strict shared admission.
Define a recovery epoch, history floor, or equivalent barrier that fresh replicas can verify.
Do not keep its only evidence in the store whose history can roll back.
Identify who sets the barrier, what invalidates cached authorization, and who can clear the restriction.
If the selected topology cannot prove continuity, require restricted admission until reconciliation completes.

This correction preserves Valkey and PostgreSQL as the primary recipe.
It adds an explicit contract to the existing failure and recovery requirements.

**Acceptance.** Acknowledge a reservation and permission withdrawal, then promote a replica that lacks those writes.
Start a gateway with no local history. It must refuse affected admission before consulting the older balance or permission as authoritative.
Also test a recovered connection on an existing gateway and an old primary that remains reachable.
These tests need real backend failures and an independently verified recovery result.

### CA03: Reservation coverage does not enumerate all paid paths

**Evidence.** [CSP12.2](../../../starport-production-catalog-plan.html#task-CSP12.2) owns reservations in limits, execution, usage, and storage.
A47 names concurrency, retries, streams, and recovery. It does not name operation-specific coverage.

The current product has paid work outside a simple chat attempt:

| Path | Inspected behavior |
| --- | --- |
| Document recognition | `internal/proxy/parser.go:295` checks a copied allowance against the lowest page price before route selection. |
| Reranking | `internal/proxy/rerank.go:39` checks a copied allowance against a minimum estimate. |
| Semantic cache embedding | `internal/proxy/semantic_cache.go:45` makes a separate embeddings call with its own request identifier. |

The [recognition source](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/proxy/parser.go#L285-L313) explicitly permits overshoot under its current estimate.
The [reranking source](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/proxy/rerank.go#L25-L54) uses a similar minimum estimate.
The [semantic embedding adapter](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/proxy/semantic_cache.go#L44-L66) invokes the gateway internally.

**Consequence.** A chat-only reservation suite could pass while paid preprocessing still uses the earlier allowance contract.
Recognition can incur a charge even if the later chat call never runs.
A semantic cache hit can still require a paid embedding.

**Correction.** Add a chargeable-operation matrix to A47 and its task ownership.
Include recognition, reranking, semantic embeddings, applicable moderation, each supported media operation, and asynchronous job reconciliation.
Trace each path to one reservation owner before its first chargeable dispatch.
Distinguish a free cache hit from an embedding needed to find that hit.
Replace minimum-price allowance checks where strict reservation rules apply.

**Acceptance.** Race compound requests against one remaining budget.
Cancel after recognition but before chat, test an embedding followed by a cache hit, and test multi-unit reranking.
Every chargeable operation must reserve adequate capacity or refuse before dispatch.
Late asynchronous usage must reconcile exactly once.

### CA04: Budget time and valuation rules remain unspecified

**Evidence.** [Section 8.9.4](../../../../design/catalog-lifecycle/ENGINEERING_SPEC.md#894-atomic-limits-and-recoverable-reservations) binds reservations to requests, attempts, policy, offerings, and pricing.
It does not select the budget window for an attempt or its later reconciliation.
Current limits use fixed UTC day, ISO week, and month intervals.
See [limit definitions](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/limits/limits.go#L9-L17).
Current usage aggregation chooses windows from the usage record timestamp.
See [usage aggregation](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/usage/repository.go#L243-L259).

Section 2.1 also states that actual provider charges can differ from catalog estimates.
The strict-budget contract must identify which amount it guarantees.

**Consequence.** Two implementations can disagree about a request admitted before midnight and reconciled afterward.
One can release capacity into the wrong period or price late usage against a newer catalog.
A usage estimate cannot silently become a guarantee about the provider's eventual invoice.

**Correction.** Define admission time, meter identity, window identity, valuation units, rounding, and retained reconciliation records.
Specify whether spend means Starport-accounted usage under pinned prices or a bound on actual provider charges.
Define price validity and refusal when the selected guarantee lacks sufficient evidence.
Keep policy revision separate from the identity of accumulated spending.
An edited limit must not reset consumption merely because its revision changes.

**Acceptance.** Add midnight, ISO-week, month, clock-skew, late-usage, and policy-change cases.
Test retries across a boundary, retained uncertain capacity, duplicate reconciliation, and expired deduplication records.
Verify that later price changes do not change an earlier reservation's accounting basis.

### CA05: Recovery can start before the reservation contract exists

**Evidence.** [CSP13](../../../starport-production-catalog-plan.html#task-CSP13) lists CSP11, CSP12, and CSP12.1 as prerequisites.
It does not depend directly or transitively on CSP12.2.
The goal permits work on eligible independent tasks when another task blocks.
The [expanded dependency check](semantic-checks.json) confirms the missing edge.

**Consequence.** If CSP12.2 blocks, CSP13 can migrate and qualify recovery without the new reservation records or their invariants.
Ledger order alone does not prevent this execution path.

**Correction.** Make CSP13 depend on CSP12.2.
Include reservation identities, window records, uncertain capacity, and recovery barriers in migration and backup evidence.
Preserve A33 and repeat A47 recovery checks after migration changes.

**Acceptance.** A scheduler must refuse to start CSP13 while CSP12.2 remains incomplete.
Restore a backup containing unresolved reservations and verify their accounting before permitting new dispatch.

### CA06: Starmap configuration reports lack explicit positive coverage

**Evidence.** [Section 7.7](../../../../design/catalog-lifecycle/ENGINEERING_SPEC.md#77-proposed-configuration-api) requires Starmap to expose the same semantic configuration report through its administrative surface.
CSP1 owns descriptors. [CSP14](../../../starport-production-catalog-plan.html#task-CSP14) owns Starmap administration.
Its steps and A32 roster cover bootstrap, permissions, rotation, audit failure, receipts, and restore.
They do not explicitly require a successful Starmap schema or effective-configuration report.
A26 and A27 assign configuration operations to Starport.

**Consequence.** The named Starmap tests can pass without delivering the operator report promised by the specification.
A user can have working subscriber authentication but lack an administrative view of effective settings and origins.

**Correction.** Assign the Starmap report to CSP14 using CSP1 descriptors.
Name the CLI and administrative API operations, their authorization, and their shared report contract.
Keep subscriber access separate and keep upstream administration outside the Starport UI.

**Acceptance.** Compare CLI and authenticated API output for source, schedule, paths, explicit values, defaults, and redacted origins.
Test a server without an approved catalog and reject a subscriber credential on the same report route.
Do not grant a pass from descriptor presence or subscriber-refusal tests alone.

### CA07: The latency acceptance rows do not render as a table

**Evidence.** A blank line separates A43 from A44 in the [acceptance matrix](../../../../design/catalog-lifecycle/ENGINEERING_SPEC.md#11-acceptance-matrix).
The GFM parser reports 43 table rows and a paragraph containing A44 through A50.
The [parser result](markdown-check.json) reproduces this on the unchanged source.
GFM ends a table at an empty line. See [the table specification](https://github.github.com/gfm/#tables-extension-).

**Consequence.** The seven latency acceptance cases lose their table structure in rendered documentation.
The existing verifier counts pipe-prefixed source lines and still reports 50 cases.

**Correction.** Remove the unintended separator or create a complete second table with a header and delimiter row.
Verify parsed rows in addition to source identifiers.

**Acceptance.** The renderer must produce 50 acceptance rows, each with ID, PRD reference, and expected result cells.
No acceptance case may appear as a raw pipe-delimited paragraph.

## Contracts that remain consistent

- Starmap owns catalog facts, acquisition, reconciliation, and distribution.
- Starport owns inference policy, credentials, runtime activation, and gateway storage adapters.
- Internal authority has no implicit public fallback. Permission validity remains separate from metadata retention.
- Configuration has one selected authority. Shared-store failure does not switch authority to a local file.
- Local and replicated recipes distinguish KV, SQL, blob bytes, process state, and disposable caches.
- Credential roles, source-bound secrets, and destination grants remain separate.
- Four-hour publication changes future embedded inputs. Existing binaries and module pins retain their original bytes.
- Low latency depends on valid memory state and bounded optional work. It does not permit bypassing strict admission.
- Early README work and final released-artifact recording have distinct evidence requirements.
- Numeric performance limits require a reviewed profile before dependent optimization tasks.

## Existing decisions and evidence limits

D6, D7, D11, and D15 remain explicit proposals in the PRD.
Permission validity, deprecation intervals, exact backend profiles, bot setup, and documentation hosting need their named review gates.
These existing decisions require resolution at their named gates.
The confirmed product decisions remain preserved.

The historical benchmark review and manifest disagree about the Go toolchain version.
The latency revision already records that uncertainty and requires actual build identity in new measurements.
This audit does not reinterpret the historical measurements as production latency evidence.

The review runs document and source checks, not production qualification.
It runs no live provider request, paid inference, backend failover, native installation, or accessibility session.
Those implementation outcomes remain UNVERIFIED.

Close CA01 through CA07 in the canonical documents and acceptance map before activating unattended execution.
Preserve task IDs and historical evidence. Recalculate counts if required subcases change.
