# CSP6 source collection boundary

Status: PARTIAL. Commit `b6e7891a9` separates source collection from reconciliation in the existing pipeline.
The [verification record](collection-2026-09-13/verification.json) binds four source files and all recorded checks.

`Pipeline.Collect` returns original observations, source failures, separate provider attempts, source activity, and the collection interval.
It does not build or publish a candidate.
Source selection, provider binding, credentials, dependencies, and cleanup use the shared pipeline.
Cancellation during observation or cleanup returns no collected evidence.

Existing `Prepare` retains its health policy, strict-source checks, fresh-update behavior, and reconciliation.
Its policy and reconciliation body matches the preceding commit after the workspace reference changes.
Source cleanup now completes before that body runs.

The full pipeline and acquisition suites pass 223 race test events across two packages.
The focused collection tests pass seven events on each supported Go toolchain.
All three runs have zero failures or skips.
Policy and lint pass with zero findings.
Generated documentation remains unchanged, and prose passes 1,688 files without diagnostics.

The proof retains the original missing-method failure, fixture corrections, and the cancellation failure during cleanup.
The final cancellation check passes after the collector checks the context again when cleanup returns.
Manual review covered original receipts, provider/account separation, cleanup, cancellation, and unchanged acquisition policy.

The producer must select declared scopes and reconcile only admitted inputs from this collection result.
Admission, receipt assembly, trusted retained-input loading, checked bot promotion, and channel binding remain required.
No CSP6 PR, push, or merge occurred.
Full A05 acceptance remains UNVERIFIED.

The [publisher setup inspection](publisher-config-2026-09-13.json) records two required branch checks and no repository-level GitHub App settings.
The required checks are `Security & Reliability` and `Verification Gate`, both from app `15368`.
The branch requires current base integration.
This inspection does not establish organization or external app configuration.
