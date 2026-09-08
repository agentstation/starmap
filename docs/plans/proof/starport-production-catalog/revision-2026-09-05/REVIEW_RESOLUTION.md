# Review resolution and document revision

Updated: 2026-09-05. The PRD, specification, findings, and plan now incorporate
the verified Fable feedback and the user's configuration and budget clarifications.
The plan remains proposed. Product implementation has not started.

This revision adds early first-use work, credential migration, model identity
transitions, shared configuration authority, and explicit publication and recovery contracts.
It preserves baseline payload files, manual source refresh, and four-hour promotion.

## User decisions and proposed details

| Item | Recorded result |
| --- | --- |
| D12 | The user clarified local file authority for local deployments and shared authority for initialized shared deployments. |
| D14 | The user selected refusal when required budget permission is unknown. Confirmed absence permits ordinary admission. |
| D15 | Shared PostgreSQL for deployment settings is an engineering proposal. Bootstrap stays node-local. Valkey owns catalog state and coordination. |
| D6 and D7 | Passive-library and reconciliation details remain proposed. The requested baseline file outcome remains mandatory. |
| D11 | Upstream Starmap administration remains a separate proposed product boundary. |

The specification distinguishes budget policy, usage, exhaustion, and absence.
Normal operation should determine the answer from current policy and usage records.
A failed team lookup cannot prove absence. Unknown permission yields a retryable refusal.

The configuration design separates local bootstrap, node settings, and deployment settings.
Once initialized, a shared revision wins over stale local deployment values.
An outage cannot create a file fallback or a second shared head.
UI saves follow the selected authority. External controllers retain exclusive management rights.

## Fable findings

The [verbatim review](../../../../reviews/FABLE_CATALOG_PRODUCT_REVIEW_2026-09-04.md)
and [verified assessment](../fable-review-2026-09-04/ASSESSMENT.md) remain unchanged.
Their statements about pending choices describe the review date.
This revision records the later user clarifications separately.

| Finding | Disposition in current documents | Owner and acceptance |
| --- | --- | --- |
| FBL-01: first-use sequencing | Added early documentation, README, and recording tasks using current behavior. Added J1 through J3 phase outcomes. | CSP0.1 through CSP0.3. E01 through E03. |
| FBL-02: credential precedence | Added persisted resolver-policy migration, explicit conflict resolution, and separate fresh-install behavior. | CSP7 and CSP9. A11 and A12. |
| FBL-03: model identity | Added P29 and specification section 2.1. Aliases cannot bypass permission or silently substitute a different model. | CSP3 and CSP10. A08, A17, and A18. |
| FBL-04: GIF duration | Kept duration and frame timing as human-reviewed targets. Preserved readability, size, and truth requirements. | CSP21 and CSP24. A36 through A39. |
| FBL-05: docs hosting | Added a proposed Pages host, release owner, version paths, deployment artifact, and rollback. Public URL checks follow publication. | CSP18 and CSP23. A29. |
| FBL-06: record-only baseline | Did not adopt. It would remove the requested inspectable payload. Clarified D6 and the application bootstrap contract. | CSP2 and CSP8. A01 and A02. |
| FBL-07: remove manual refresh | Did not adopt. Manual source refresh and effective-catalog pinning have different effects. Simplified the proposed UI instead. | CSP5, CSP8, and CSP17. A22 and A23. |
| FBL-08: export-only configuration | Superseded by the user's storage-authority clarification. Added local saves, shared revision saves, and external controller diffs. | CSP16, CSP16.1, and CSP17. A26 through A28 and A40. |
| FBL-09: daily promotion | Did not adopt. D3 retains four-hour promotion. Clarified that a schedule does not bound maximum catalog age. | CSP6. A05 and A06. |
| FBL-10: native checks | Moved native paths and interrupted migration into their owning task exits. Final qualification repeats them. | CSP2, CSP8, and CSP22. A03 and A04. |
| FBL-11: local enrollment | Kept as a later product proposal. It does not block this release or create a second catalog authoring authority. | Product owner for later scope. Existing local-provider paths remain in the first-use documentation. |

Redis and MySQL qualification is conditional on a support claim.
The primary release must qualify Valkey and PostgreSQL.
Keep hash-slot guards without claiming untested Cluster failover.
Documentation search remains required. Command-palette integration remains optional.

## Earlier audit findings

The [original audit](../audit-2026-09-04/AUDIT.md) remains historical evidence.
The table below records design and task coverage, not completed implementation.

| Finding | Contract added or clarified | Owner and acceptance |
| --- | --- | --- |
| AUD01: withdrawal across replicas | Permission envelope, enforced revision, bounded validity, cache admission, retry and batch checks, and stream policy. | CSP4, CSP10, and CSP11. A09, A10, A19, A21, and A28. |
| AUD02: budget outage | D14, distinct policy and usage failures, retryable refusal, and preserved known-absence behavior. | CSP12 and CSP15. A15 and A31. |
| AUD03: stale recovery | Independent durable evidence or restricted recovery until reconciliation. Unknown loss scope restricts the deployment. | CSP13. A16 and A33. |
| AUD04: source coverage | One integration contract across CLI, publisher, server, and embedded Starport, with real provider and non-provider fixture changes. | CSP3 and CSP8. A08 and A20. |
| AUD05: credential destinations | Destination grants tied to provider, role, origin, operation, and placement. Approved local exceptions remain explicit. | CSP9 and CSP10. A11 and A25. |
| AUD06: publisher admission | Required and optional source verdicts, retention limits, deletion review, and immutable run receipts bound to artifacts. | CSP3 and CSP6. A05, A08, and A20. |
| AUD07: incomplete task proof | Named required subcases, native checks, actual installers, positive public download, and explicit candidate-versus-release gates. | CSP0 and every implementation task. A01 through A40. |
| AUD08: Starmap admin state | Local first-admin bootstrap, private filesystem audit and receipts, failed-audit refusal, and crash recovery without SQL. | CSP14. A32. |
| AUD09: bot identity | Proposed scoped GitHub App, actual required-check identities, and a controlled promotion test. | CSP6. A05. |
| AUD10: recording handoff | Current-release early capture, immutable candidate rehearsal, final capture after publication, and invalidation rules. | CSP0.3, CSP21, and CSP24. E03 and A35 through A39. |

## Delivery and verification contract

The current plan has 30 tasks. Existing task IDs and their relative order remain stable.
Three early tasks follow CSP0. CSP16.1 separates configuration operations from shared-state ownership.
All task statuses remain todo.

The acceptance map contains 40 primary cases and 194 required named subcases.
E01 through E03 prove early first use independently of final qualification.
R01 and R02 cover candidate recording rehearsal without advance release credit.
Each task invokes its named subcase roster. Missing tests or service environments remain UNVERIFIED.

The candidate gate requires 33 complete primary cases.
A06, A29, and A35 through A39 depend on publication and remain UNVERIFIED until then.
CSP24 must produce the full 40-case result before the complete production claim.
No publication-dependent result receives advance credit.

## Remaining choices and implementation limits

Before dependent implementation, review D6, D7, D11, and D15.
Select the permission-validity interval, clock-skew assumptions, and model deprecation window.
The proposed deprecation default is 30 days with explicit urgent-withdrawal exceptions.
Fix each supported API route's identity-error mapping before CSP10 implementation.

The release profile must also name required sources, retained-evidence ages,
removal thresholds, workload, backend versions, and recovery objectives.
The release owner must verify the proposed Pages setup, bot permissions, and branch rules.
These are explicit task inputs, not implied production guarantees.

This request changed documents and planning evidence only.
It did not activate the plan, change product code, run provider calls, or publish anything.
Earlier test and screenshot evidence remains intact.
The revision checks validate document structure, references, mappings, and writing.
They do not count as product acceptance evidence.

## Evidence

- [Input hashes](input-manifest.json)
- [Current verification results](verification.json)
- [Acceptance map](../acceptance-map.json)
- [Updated PRD](../../../../design/catalog-lifecycle/PRD.md)
- [Updated engineering specification](../../../../design/catalog-lifecycle/ENGINEERING_SPEC.md)
- [Updated repository findings](../../../../design/catalog-lifecycle/REPOSITORY_FINDINGS.md)
- [Canonical plan](../../../starport-production-catalog-plan.html)
