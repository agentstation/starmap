# Workspace access preservation

The Starmap worktree now preserves workspace access in the tested macOS and Linux replacement paths.
This component work addresses the mode-loss defect from the [conversation review](access-policy-conversation-review.md).
CSP2 remains in progress. Native Windows and released-pair qualification remain UNVERIFIED.

## Preparation and publication

`internal/catalog/workspace/stage_access.go` creates a private `.<workspace>.preparing-<id>/` enclosure beside the selected workspace.
The enclosure contains rendered catalog data, retained operator files, and the candidate tree during access restoration.
An empty candidate first inherits the actual workspace parent's defaults, then moves inside that enclosure before receiving any payload.
Only the enclosure root requires owner-only access. Its descendants can carry restored workspace permissions while the private root restricts access.

`stage_source.go` holds the original directory root and copies the recorded source entries with identity, size, and content checks.
The assembler restores existing entry ownership, modes, and native ACL data before publication.
New entries inherit from their restored parent. Existing read-only files remain writable through held handles until data and metadata flushes finish.
The assembler verifies each restored access digest and rechecks the original source tree before publishing the candidate.

`internal/filepublish.DirectoryBetweenRootsNoReplace` moves direct child directories between held roots without replacing an existing destination.
Atomic replacement and journaled replacement retain their later source-conflict checks.
Access restoration errors before publication preserve the selected workspace. This statement does not cover errors after publication.

`operator_files.go` preserves non-YAML files and nested directories inside model directories that the catalog writer recreates.
Model `.yaml` records remain catalog-managed. An operator note inside a model directory now survives replacement with its original access policy.

## Native access

Linux restores UID, GID, permission bits, and POSIX access and default ACL attributes.
macOS restores UID, GID, permission bits, and the native extended ACL.
Both paths verify the resulting access digest. A mismatch refuses the candidate before publication.

Windows code copies ownership, DACLs, integrity labels, resource attributes, and the read-only attribute.
Its snapshot also binds central access policy metadata, but restoration does not set that privileged field.
A mismatch therefore refuses publication. Full audit SACL preservation is outside the implemented snapshot coverage.

Windows inheritance needs a native test with different ACLs on the workspace parent and private preparation directory.
The current adapter requests unprotected DACL inheritance while the candidate is inside the preparation directory.
This is a review concern, not a reproduced Windows failure.
Microsoft documents automatic inheritance propagation for [SetSecurityInfo](https://learn.microsoft.com/en-us/windows/win32/api/aclapi/nf-aclapi-setsecurityinfo).

## Evidence

All counts below include named tests and subtests. Counts exclude package completion events.

| Evidence | Result | Scope |
| --- | --- | --- |
| [Initial regression](workspace-access-preservation-red.json) | Failed before restoration | Both replacement paths changed existing permissions. |
| [Final macOS run](workspace-preservation-qualified-macos.json) | 695 passed, one failed, no skips | Six packages with Go 1.25.12 and the race detector. |
| Workspace portion of that run | 129 passed, no failures or skips | Modes, native ACLs, new-file inheritance, private staging, operator notes, and interrupted replacement. |
| [Final Linux run](workspace-preservation-qualified-native-linux-arm64/summary.json) | 330 passed, no failures or skips | Workspace 120, file publication 25, application 185. Native ARM64 containers, without race instrumentation or external network. |
| [Static checks and Windows builds](workspace-preservation-settled-checks.json) | All commands passed | Ago, host and Windows lint, layout, dependency direction, and six Windows binaries across AMD64 and ARM64. |
| [Input manifest](workspace-preservation-input-manifest.json) | 134 hashes match | Workspace, file publication, application, product paths, and module inputs. |
| [Focused lock reruns](access-policy-review-lock-check.json) | Ten passed | The failed process-lock test alone, with Go 1.25.12 and the race detector. |

The macOS failure was `TestRuntimeDirectoryLockSurvivesProcessInterruption` in the runtime package.
Its parent accepted a lock after the child reported readiness: `live child lock = <nil>, want conflict`.
The focused reruns do not establish its cause or clear the failed broader check.
CSP2 must investigate process-lock lifetime and repeat the required runtime verification after a verified correction.

Earlier intermediate runs and failed lint attempts remain in the proof directory. They do not replace the final-source evidence.

## Remaining scope

Foreign UID and GID restoration needs a native fixture with the intended service identities and privileges.
Windows APIs, inheritance, mandatory labels, and editor-handle behavior remain UNVERIFIED despite successful compilation.
Version 1 workspace journals remain refused without file changes. Operator recovery guidance still needs completion.

CSP5 owns abandoned preparation cleanup and errors after publication, including old-backup cleanup under restrictive directory access.
Cleanup failure must not become an unconditional claim that an error leaves the original current state unchanged.
The prepared journal tests do not prove power-loss durability on every filesystem.

CSP2 still needs one semantic file-role policy source for diagnostics and enforcement.
CSP8 owns Starport adoption. CSP12 owns engine-specific access and restore qualification.
No primary acceptance case or required subcase gains credit from this record alone.

## Subsequent qualification

The [shared-policy record](shared-file-policy.md) records the child lock lifetime diagnosis and a subsequent 925-event passing macOS race run.
It also records the Windows inheritance correction and its prepared native test. Native Windows execution remains UNVERIFIED.
