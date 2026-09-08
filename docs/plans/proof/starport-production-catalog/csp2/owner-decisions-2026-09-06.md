# CSP2 owner decisions: 2026-09-06

The owner answered all three pending questions in this task on 2026-09-06.
These decisions replace the earlier pending interpretations. They do not establish implementation or native qualification.

## Constructor storage boundary

Construction may read storage that the caller explicitly supplies, including remote storage.
That explicit read may use network connections and transport workers. It must honor the supplied context where the API accepts one.
Construction must not start automatic provider acquisition, source acquisition, or background refresh.
Default construction remains offline. Construction must not create files or repair the authoring workspace.

D6 now states this distinction. Existing S3 probe evidence proves the explicit read, not the complete absence of automatic acquisition.
A02 network and worker checks remain UNVERIFIED until their behavioral coverage proves the accepted boundary.
Keep their existing acceptance identifiers and record the qualification scope explicitly.

## Service-managed primary configuration

The owner approved an exception for explicitly selected, administrator-owned primary configuration that the service can read.
A POSIX example is `root:starmap` ownership with mode `0640`.
The implementation must check trusted ownership, service read access, and absence of untrusted write access.
Windows requires equivalent native ownership and ACL checks.

The exception applies only to primary configuration. Catalog state and dotenv files retain private-access requirements.
Do not infer this exception from permissive permissions or a failed private-file check.
CSP2 owns the Starmap implementation and native evidence. CSP8 owns Starport integration.
CSP2 must implement the exception.

## Commit and publication authority

The owner authorized commits, required review, task-branch pushes, draft pull requests, and native GitHub Actions execution.
This authority covers the prepared Starmap and Starport goal changes after their required checks and review.
It excludes merges and releases. Existing paid-inference permission remains separate.

The active plan now records this authority. Required pre-PR autoreview still applies before publication.
The prepared Starmap workflow includes native Windows, Linux, and macOS jobs for AMD64 and ARM64.
Approval permits those jobs to run. It does not count compilation as native execution or mark a failed check as passing.

## Native dependency budgets

The owner approved measured platform limits after the publication gate reported the existing 32-package limit.
The [comparison](publication-dependency-comparison.json) identifies six additional macOS packages relative to the recorded base commit.
They implement file publication, private-file policy, and native ACL access. The comparison found no removed packages or new acquisition dependency.

The [platform graphs](publication-dependency-platforms.json) cover AMD64 and ARM64 without claiming native execution or request latency.
The read-only limits are 34 packages on Linux, 35 on Windows, and 37 on macOS.
The pinned-artifact consumer also imports the existing artifact reader. Its corresponding limits are 35, 36, and 38 packages.
The verifier retains all forbidden-dependency rules and the existing server and remote limits.

## Historical command-output lint: pending approval

The owner question proposes excluding exactly two historical command-output files from prose lint.
`storage-revision-2026-09-05/document_structure.txt` contains JSON output.
`storage-revision-2026-09-05/starmap_writing.txt` contains the earlier writing-check transcript.
Both paths are relative to the plan proof root.

The writing gate reports three diagnostics in these files. The document verifier separately checks their original SHA-256 digests.
The proposal retains those byte-preservation checks and all maintained prose checks. The configuration still contains no such exclusion.
