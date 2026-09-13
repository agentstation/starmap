# CSP5 native recovery correction

Commit `9e278107cdf00e6fc843e5a8fdd835f27b860e6a` corrects the workspace reads that remained after the first Windows repair.
The [verification record](native-recovery-2026-09-13/verification.json) binds ten files to the commands and results.
The [parent native failures](native-correction-2026-09-13.md) remain the failure evidence.

Workspace record reads and tree scans now inspect empty files without reading locked bytes.
The record reader checks size, mode, and modification time before and after the read.
Both paths retain identity, access, and digest checks. The new regression also verifies writer exclusion and changed contents.

The mutation fixture verifies either a completed rename or native refusal with unchanged identity and metadata.
A refused rename must leave the destination absent. Unexpected errors still fail the test.
Completed replacements retain the existing conflict and preservation assertions.
Native refusal permits valid publication or clean cancellation because no replacement occurred.

All six native jobs now run focused filesystem checks before the full suite.
Those checks cover empty locks, migration, preparation cleanup, private publication, and writer replacement.
This step does not replace the complete native qualification.

## Local checks

The complete private-file, workspace, and workflow suites pass 531 race test events across three packages.
The focused selection passes 37 events. The minimum Go toolchain passes 29 focused events across two packages.
The CLI migration regression passes one event. These checks report no failing or skipped test events.

Eight package compilation checks pass across Windows amd64 and arm64. They execute no Windows tests.
Policy and lint pass with zero findings. Generated documentation remains unchanged. Prose passes 1,686 files with zero diagnostics.

The CSP5 component gate passes all twelve selected subcases across 21 commands and 42 race test events.
Eight subcases retain Starport consumer requirements.
The record preserves two initial build failures from an incorrect cleanup callback in the new test.
The corrected callback passes. A later change affects one comment sentence only. Its exact before and after bytes remain recorded.

## Remaining work

Required review `52394` ended with status 1 after the Claude engine failed during section four.
It produced no final verdict. The complete captured output remains in the verification record.
Retry `97629` reviews the unchanged correction with the same profile and threshold.

PR #154 still publishes the parent correction. Fresh native CI and merge remain required.
Shared catalog cleanup and its coordinator decision remain required for CSP5 completion.

The coordinator question remains pending. No answer or scope reduction follows from that request.
