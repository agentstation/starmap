# CSP6 publication admission

Status: PARTIAL. Local commit `d15eb1291` adds the admission component.
The publication workflow does not use it yet.
The [verification record](admission-2026-09-13/verification.json) binds source files and complete check output.

Each profile declares its version, required scopes, retained-evidence age limits, optional-failure permission, and disabled-scope removal policy.
Provider evidence must match the complete declared acquisition binding.
Missing credentials cannot make a required scope optional.

The component accepts complete fresh evidence or complete retained evidence within its age limit.
Partial replies cannot replace complete evidence.
Retained receipts preserve their observation time and identity.
An explicit embedded source can satisfy its own scope without claiming fresh acquisition.
A rejected run supplies no catalog inputs or removal operations.

Both Go 1.26.6 and Go 1.25.12 pass 48 race test events, with no failures or skips.
Policy and lint pass without findings.
The prose check passes 1,669 files without diagnostics.
The record preserves the initial missing-component failure, fixture compilation correction, and prose failure.

Manual review covered exact scope matching, source completeness, expiry, input ownership, and refusal across multiple scopes.
Manual review found no remaining component defect.

The acquisition pipeline reconciles observed replies before it returns a prepared candidate.
Workflow integration must reconcile the admitted inputs before publication.

Immutable run receipts, trusted retained-input loading, workflow integration, and checked bot promotion remain required.
No CSP6 PR or external publication occurred.
Full A05 acceptance remains UNVERIFIED.
