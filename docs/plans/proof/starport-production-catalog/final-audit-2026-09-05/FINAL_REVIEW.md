# Final catalog plan review: 2026-09-05

The plan still needs the seven corrections from the preceding audit.
This fresh pass found no additional material gap and closed no finding.
Three findings retain P1 priority. Four retain P2 priority.

The architecture remains consistent with the confirmed product decisions.
The plan remains `proposed`, with 38 tasks marked `todo`.
This review adds evidence only. It changes no canonical requirement, specification, plan, acceptance case, product code, or task status.

## Review inputs

All 46 inputs from the [preceding audit](../coherence-audit-2026-09-05/AUDIT.md) remain unchanged.
The [input manifest](input-manifest.json) also preserves that audit's eight proof files, for 54 frozen inputs.
Both repository revisions remain unchanged:

| Repository | Revision |
| --- | --- |
| Starmap | `4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225` |
| Starport | `042eb97851496ecdd1ef20bfa44fe3d84b86b2d8` |

The review covers the PRD, specification, plan, acceptance map, storage review, latency review, repository findings, and README demonstration brief.
It also checks prior review dispositions, status indexes, and the source evidence for paid operations and budget accounting.

## Open findings

The preceding audit retains the full evidence, consequence, correction, and acceptance test for each finding.
The table below records the fresh assessment without duplicating those findings.

| Finding | Priority | Recheck result | Required correction |
| --- | --- | --- | --- |
| [CA01](../coherence-audit-2026-09-05/AUDIT.md#ca01-candidate-qualification-omits-checks-that-need-no-publication) | P1 | Six local documentation and installation checks still fall outside the explicit candidate roster. | Require those checks against the final candidate before publication. Keep incomplete parent cases UNVERIFIED. |
| [CA02](../coherence-audit-2026-09-05/AUDIT.md#ca02-failover-restrictions-lack-an-enforceable-history-barrier) | P1 | The target requires restricted admission after lost history but does not select how fresh replicas detect that condition. | Define an enforceable acknowledgment and recovery protocol before shared admission implementation. |
| [CA03](../coherence-audit-2026-09-05/AUDIT.md#ca03-reservation-coverage-does-not-enumerate-all-paid-paths) | P1 | A47 still lacks an explicit paid-operation matrix. Recognition, reranking, and semantic embeddings remain relevant source paths. | Trace each paid operation to reservation before dispatch and exactly-once reconciliation. |
| [CA04](../coherence-audit-2026-09-05/AUDIT.md#ca04-budget-time-and-valuation-rules-remain-unspecified) | P2 | Reservation identity still lacks a selected window and late-usage accounting contract. | Define window identity, valuation, rounding, policy edits, and reconciliation retention. |
| [CA05](../coherence-audit-2026-09-05/AUDIT.md#ca05-recovery-can-start-before-the-reservation-contract-exists) | P2 | CSP13 still has no direct or transitive dependency on CSP12.2. | Add the dependency and include reservation state in migration and recovery evidence. |
| [CA06](../coherence-audit-2026-09-05/AUDIT.md#ca06-starmap-configuration-reports-lack-explicit-positive-coverage) | P2 | Starmap report parity remains a specification requirement without an explicit positive CLI and administrative API test. | Assign implementation and positive report checks to CSP14 using CSP1 descriptors. |
| [CA07](../coherence-audit-2026-09-05/AUDIT.md#ca07-the-latency-acceptance-rows-do-not-render-as-a-table) | P2 | The GFM parser again produces 43 table rows and one paragraph containing A44 through A50. | Repair the separator and validate all 50 rendered acceptance rows. |

CA02 does not require a specific database redesign or a new remote read on every request.
The implementation owner must select a protocol that establishes safe admission after failover, restoration, and fresh replica startup.
A conservative recovery procedure can qualify if every affected gateway enforces its restriction before admitting requests.

CA03 concerns explicit coverage of the existing broad reservation requirement.
The specification already says that every chargeable attempt needs a defensible reservation.
The missing operation matrix permits tests to overlook paid preprocessing and internal calls.
It does not establish that the proposed reservation algorithm intentionally excludes them.

CA06 remains separate from configuration descriptor parity.
Matching descriptors do not prove that an administrator can get a working Starmap report through the promised interface.
The correction does not require a Starmap web console or upstream administration through Starport.

## Cross-document checks

These checks assess the target design. They provide no implementation acceptance credit.
The findings above still qualify the relevant areas.

| Area | Assessment |
| --- | --- |
| Product ownership | Starmap owns facts, acquisition, reconciliation, and distribution. Starport owns inference, account policy, credentials, and its storage adapters. |
| Embedded baseline | Persistent applications export the baseline. Passive library reads remain separate under the explicit D6 proposal. |
| Publication | Successful publication binds artifact, branch promotion, and channel. Existing binaries and module pins keep their original embedded bytes. |
| Internal authority | First boot requires an approved internal catalog. Retained metadata cannot establish permission after its validity expires. |
| Pins and update controls | Pins cannot bypass known withdrawal. Manual and offline controls do not extend permission validity. |
| Authority transitions | A new authority cannot inherit acceptance from an incompatible old authority or pin. Superseded workers lose permission to act. |
| Local and shared configuration | Bootstrap stays node-local. An initialized shared revision wins over stale local deployment values. Store failure does not change authority. |
| Configuration operations | Desired and applied revisions remain distinct. Local saves, shared SQL writes, and external controller changes have separate operation contracts. |
| File ownership | Config, data, state, cache, runtime identity, baseline exports, and optional artifacts have named paths and recovery roles. |
| Deployment recipes | T1 through T7 select stores by process ownership and persistence. Company size alone does not select an engine. |
| Durable stores and caches | Badger or Valkey holds KV records. SQLite or PostgreSQL holds relational records. Disposable caches have separate capacity and expiry rules. |
| Credentials | Acquisition and inference remain separate roles. Starmap inference fallback requires opt-in. Destination grants constrain credential use. |
| Warm request data | Stable records use bounded, valid process memory. Unknown policy and mutable budget capacity retain required admission checks. |
| Runtime consistency | Requests retain a runtime generation. New attempts, retries, queued work, and cache delivery still require current permission. |
| Performance evidence | Numeric profiles precede dependent optimization. Complete HTTP and streaming measurements must identify workloads, artifacts, allocations, and timing boundaries. |
| Documentation and UX | One versioned tree supplies embedded and public docs. Static recovery instructions remain separate from authenticated deployment data. |
| README and media | Early work describes the current release. Final installation, recording, and performance evidence must identify the shipped artifacts. |
| Execution and release | Stable task IDs, named subcases, candidate qualification, publication checks, and cleanup remain present. CA01 and CA05 still need correction. |

The pin and offline rules have an operational consequence that the recipes must retain.
An internal production replica cannot serve indefinitely from expired permission evidence, even if its catalog remains inspectable.
Explicit import or renewed authority evidence must satisfy the selected permission contract before admission resumes.
This follows existing sections 6.2 and 8.2. It adds no new fallback policy.

## Verification

The [structural check](structural-check.json) reruns the existing document verifier without replacing its historical output.
The [rechecks](rechecks.json) expand dependency ranges and reproduce the candidate omissions.
They also verify seven source files against their prior hashes and the recorded Starport revision.
The [Markdown check](markdown-check.json) reproduces CA07 with the GFM parser.

| Check | Result |
| --- | --- |
| Task ledger and articles | 38 unique tasks, all `todo`, in matching order |
| Requirement mapping | 38 PRD requirements map to 50 primary acceptance cases |
| Named acceptance roster | 309 unique required subcases |
| Candidate roster | 43 complete primary cases and 282 subcases, with the CA01 omission retained |
| Final roster | 50 primary cases and 309 subcases |
| Dependency graph | Declared graph is acyclic. The CA05 prerequisite remains absent. |
| Historical evidence | All 57 files checked by the existing verifier remain unchanged. |
| Latency source evidence | All 20 source hashes checked by the existing verifier still match. |
| Additional source evidence | Seven files match their prior hashes and recorded revision. |
| Canonical links | 157 local references and 93 pinned source references pass the existing verifier. |
| Rendered acceptance rows | 43 of 50 rows render inside a table. CA07 remains open. |

The [verification record](verification.json) records final hashes, report links, whitespace checks, and writing results.
The new report must pass its targeted writing check.
The full repository check still has three preexisting diagnostics in historical command-output files.
This review preserves those files and the lint policy.

This pass ran no product test, provider request, backend failover, native installer, or live accessibility session.
Production latency, recovery objectives, installed behavior, and support claims remain UNVERIFIED for the proposed implementation.
The historical benchmark toolchain mismatch also remains unresolved evidence, as recorded in the latency revision.

## Next revision

Close CA01 through CA07 in the canonical documents before unattended plan execution.
Preserve the confirmed decisions, task IDs, and historical evidence.
Recalculate the case rosters after adding coverage, then rerun structural and semantic checks.

Resolve D6, D7, D11, D15, and the declared release-profile choices at their existing gates.
The review does not reopen confirmed authority, credential, storage, or budget decisions.
Closure requires document changes and fresh evidence.
