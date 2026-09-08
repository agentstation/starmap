# CSP2 native qualification: first run

CSP2 remains in progress. Native CI found Windows failures, a public-network
read in a test, and a macOS Intel suite timeout. No release qualification or
primary acceptance credit follows from these runs.

## Published review stack

The owner approved reviewed commits, draft pull requests, and native CI.
The shared helper uses the approved 300,000-byte prompt limit.

| Pull request | Purpose | Review |
| --- | --- | --- |
| [125](https://github.com/agentstation/starmap/pull/125) | Historical records and media | Non-code gate; secret scan passed |
| [126](https://github.com/agentstation/starmap/pull/126) | Product contracts and capture helpers | Five portions; zero findings |
| [132](https://github.com/agentstation/starmap/pull/132) | Runtime foundation | Eight portions; zero findings |
| [133](https://github.com/agentstation/starmap/pull/133) | Scoped acquisition and reset integration | Four portions; zero findings |

Code review used Sol at xhigh and Opus at high through the cross-lab profile.
The helper used its default P0 threshold. These results do not establish an
absence of lower-priority defects. Both code branches passed all 39 local
verification stages before publication. Each run reported 79 ordinary and 79
race package passes.

## Native evidence

The [foundation run](https://github.com/agentstation/starmap/actions/runs/34164034368)
qualified its native jobs on both Linux architectures and both macOS
architectures. Both Windows jobs failed. Its verification job also found the
public-network test failure described below.

The [follow-up run](https://github.com/agentstation/starmap/actions/runs/34164035791)
passed its Linux ARM64 and macOS ARM64 native jobs. Linux AMD64 failed on the
public-network test. macOS Intel exceeded the runtime package's five-minute
suite limit. Both Windows jobs failed.

[The verification record](native-qualification-2026-09-07/verification.json)
binds the source commits, run IDs, artifact hashes, and exact event counts.
Windows recorded 785 passing and 409 failing test events on each architecture.
Many failures share the same bootstrap cause. They are not 409 independent bugs.

## Verified causes and local repairs

Git's CRLF conversion changes embedded SVG bytes. Their base64 representation
adds 136 bytes to the canonical payload. A checkout with `core.autocrlf=true`
reproduced the native payload size of 8,270,770 bytes. The manifest requires
8,270,634 bytes.

The new catalog attributes preserve exact checkout bytes. The same probe now
passes the unchanged manifest. The checkout regression verifies 1,278 files.
It fails without the attributes and passes with them. Retained before and after
records keep both outcomes.

`TestProviderReceiptReportsDegradedAcquisition` called `Refresh` with the default
GitHub source. Its acquisition stub did not replace that source. The test now
selects the embedded source. Its acquisition assertions remain unchanged.

The macOS Intel timeout occurred while tests still progressed. Four active tests
ran for at most one second when the package alarm fired. The native suite
limit now permits ten minutes, within the unchanged twenty-minute job limit.
Every test remains enabled.

The rename fixtures now open the selected root through a parent root. This
permits delete sharing on Windows while retaining the selected directory handle.
The UNC fixture now compares equivalent share-root forms. These fixture changes
still require native execution.

## Open Windows work

Workspace access copying fails the existing descriptor equality check. A focused
native regression now reports the before and after descriptors for temporary
files and directories. This diagnostic fixture does not relax the check.

Access-denial fixtures, changed-identity inspection, absent-path classification,
legacy migration, and journal recovery still need investigation. Go's Windows
metadata can resolve identity lazily. Privileged backup access can also affect
denied-read fixtures. These are investigation leads, not verified fixes.

Primary references include the [Go root implementation](https://go.dev/src/os/root_windows.go).
See [Windows access rights](https://learn.microsoft.com/en-us/windows/win32/fileio/file-security-and-access-rights) and [ACL propagation rules](https://learn.microsoft.com/en-us/windows/win32/secauthz/automatic-propagation-of-inheritable-aces).
The local inspection used Go 1.26.6 source. Native CI uses Go 1.25.12.

## Starport verification boundary

Starport commit `de0e002` corrects private performance fixture paths and isolates
mutation verifier modules from an outer Go workspace. Local integration now
passes 32 of 33 required command checks, including the first-run rerun on a free
port. V01 still requires a published Starmap module. Starport remains pinned to
v0.16.5. The latest local changes remain unpublished.

Commit `90c8932e` contains the local repairs and diagnostic fixture. Twelve
focused race events and three Go 1.25.12 test events pass. Both Windows
architectures compile four affected test packages. Windows test runs for these
changes remain UNVERIFIED. Code lint, Ago, and strict prose checks pass.

Next: inspect the native reruns and repair the remaining Windows failures.
Use the focused descriptor evidence to repair Windows behavior at its owning
package. Keep CSP2 in progress until native qualification passes.

## Reviewed repair publication

Both candidates passed the required cross-lab review at the P0 threshold.
The foundation review covered eight portions. The follow-up review covered five.
Each reported zero findings. Both commits now appear in their draft pull requests.

The [foundation rerun](https://github.com/agentstation/starmap/actions/runs/34167265050)
and [follow-up rerun](https://github.com/agentstation/starmap/actions/runs/34167265992)
are in progress. The [publication record](native-qualification-2026-09-07/publication.json)
binds each run to its reviewed commit. Both remain draft candidates.

## Repair rerun results

Both branches passed all four Linux and macOS runtime jobs. Both Windows runtime
jobs failed on each branch. The [rerun record](native-qualification-2026-09-07/repair-rerun-results.json)
retains the run state, commit IDs, artifact hashes, and test counts.

The foundation Windows AMD64 artifact contains 936 passing and 102 failing test
events. The follow-up artifact contains 1,096 passing and 108 failing events.
Neither artifact contains skipped tests. Parent test and subtest failures count
as test events, so these totals do not count independent defects.

The native rerun confirms the catalog checkout repair. The payload now matches
the unchanged manifest. The renamed-root and UNC fixtures pass on Windows AMD64.
The runtime package also passes on that foundation job.

The descriptor test found extra inherited grants after copying access metadata.
It also found that the copy changed an absent SACL to a null SACL. The candidate
repair assigns the captured descriptor with `NtSetSecurityObject`. The strict
before-and-after equality check remains in place.

The call omits absent SACL
components and privileged central policy assignment. Descriptor comparison still
refuses a candidate that loses central policy metadata. Native execution must
verify the copy behavior.

Baseline publication retains open child files during directory rename. The
candidate closes each file after its write and sync. Validation reopens the file
and checks its original identity, metadata, and bytes before publication or cleanup.

Windows inspection resolves an initial identity from an open metadata handle.
This avoids a later path lookup that can resolve the identity of a replacement.
The candidate also distinguishes a missing path from an existing file ancestor.

Denied-read tests use a duplicate thread token with no enabled privileges. They
retain their deny assertions and report the inherited backup and restore privilege
state. Native evidence must establish whether those privileges caused the earlier
fixture failure. The test helper does not change the process token.

Legacy migration remains unresolved. A native test checks whether an external
hard link to the commit lock permits directory relocation while retaining the
same lock. It checks writer exclusion before and after the move. This test does
not change the production migration algorithm or grant qualification credit.

The candidate repairs remain local. CSP2 remains in progress. Required checks,
code review, publication, and another native run remain open.

The Windows API contracts describe [security assignment](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/nf-ntifs-ntsetsecurityobject)
and [rename constraints](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/ns-ntifs-_file_rename_information).
The [security information contract](https://learn.microsoft.com/en-us/windows/win32/secauthz/security-information) defines the required rights for central policy assignment.

## Second repair candidate

Foundation commit `0451b039` passes all 39 repository verification stages.
Ordinary tests and race tests each pass 79 packages. Two focused packages pass
106 race results on Go `1.26.6`. Three packages pass 153 race results on Go
`1.25.12`.

Five affected packages compile for each Windows architecture. Windows-targeted
lint, Ago, strict prose, and workflow shell syntax pass.

[The candidate verification record](windows-repair-2026-09-07/verification.json)
retains the logs and their hashes. Required cross-lab review remains open.
Follow-up commit `0911e767` includes the foundation and preserves the Linux
administrator-owned configuration job. Its service configuration test now uses
the same privilege-controlled callback.

Service test commit `6b8ed1c4` passes all 39 follow-up verification stages. Ordinary
tests and race tests each pass 79 packages. Both Windows app test binaries
compile with Go `1.25.12`. Windows-targeted lint, Ago, prose, and document
checks pass. [Follow-up verification](windows-repair-2026-09-07/followup-verification.json) retains the command outputs. Required review and native execution remain open.

Both predecessor runs finished. Their Linux verification gates passed, but their
Windows failures kept the overall runs red. Each foundation Windows architecture
recorded 936 passing and 102 failing events. Each follow-up Windows architecture
recorded 1,096 passing and 108 failing events. All four artifacts report no skips.
These results do not qualify the second repair candidate.

## Third native execution

Both complete branch reviews passed eight portions with zero P0 findings.
Draft PRs 132 and 133 now contain foundation `0451b039` and follow-up `8b034d5a`.
Native runs [34170862943](https://github.com/agentstation/starmap/actions/runs/34170862943)
and [34171467367](https://github.com/agentstation/starmap/actions/runs/34171467367) test those commits.

Each foundation Windows architecture reports 1,030 passing and ten failing test
events. Each follow-up Windows architecture reports 1,196 passing and ten failing
test events. All four artifacts report zero skips. All Linux and macOS runtime
jobs pass on both branches. Both Linux verification gates remain in progress at
this capture.

Baseline publication, path inspection, denied-read checks, and service-managed
configuration now pass on Windows. The runner reports enabled backup and restore
privileges. The controlled thread-token tests pass without changing process privileges.
The external lock-alias test also passes on both Windows architectures.

The ten failures cover five legacy migration cases, two access-copy cases, one
workspace identity case, one editor-handle case, and one platform-home fixture.
The next candidate must retain the commit lock during migration and preserve exact
access descriptors. Native execution must qualify all repairs.

The local Starport pair passes all 12 ownership checks and five state-directory
race tests against follow-up `8b034d5a`. This does not repeat the earlier complete
Starport command roster. Its published-module check remains unqualified.

[Publication and native results](native-third-2026-09-07/publication-and-native-results.json)
record commits, reviews, artifact hashes, and all failing test names. Gzip files
retain exact native JSONL bytes. Each record includes compressed and uncompressed
hashes. No public pull request merge, release, or primary acceptance credit occurred.

## Third repair candidate

Foundation commit `a441a72f` preserves the legacy commit lock through an external
Windows hard link. It verifies the original path, alias, and locked handle before
migration. Cancellation closes the handle and removes the unchanged alias. The
file inventory and CLI instructions name this temporary file and its recovery rules.

Workspace reads now capture Windows file identity before the callback. Shared-lock
validation also compares the open lock handle with the selected lock identity.
Application tests isolate `HOME`, `USERPROFILE`, `APPDATA`, and `LOCALAPPDATA`.
The editor fixture requests directory-list access, rather than metadata alone.
Its native sharing behavior still needs qualification.

Access copying now supplies the source DACL protection state. Eight focused native
variants compare complete descriptors for files and directories with default,
protected, inherited, or labeled access. The production equality check remains strict.
Native execution must establish whether the protection flags resolve both access-copy failures.
The [Windows security information contract](https://learn.microsoft.com/en-us/windows/win32/secauthz/security-information) defines those flags.

Baseline export records final timestamps after closing its writing handle. Identity,
mode, size, and content checks still guard publication and cleanup. This change
addresses the [Windows file-time contract](https://learn.microsoft.com/en-us/windows/win32/sysinfo/file-times).
The preceding NTFS run passed baseline tests. It did not expose a timestamp failure.
A new regression verifies refusal after an external timestamp change.

[The third candidate record](windows-third-repair-2026-09-07/verification.json)
retains compressed logs with both byte hashes. All 39 foundation verification stages
pass. Ordinary and race tests each pass 79 packages. Local integration passes 392
race events. The final cleanup check passes 16 events, and minimum-Go checks pass 38 events.
Six affected test binaries compile across Windows AMD64 and ARM64. Windows lint,
Ago, and strict prose pass.

Follow-up merge `0e1986b8` contains this candidate. Foundation review remains in
progress. Follow-up integration checks must complete. Qualify the foundation on
native Windows before repeating follow-up review and publication. CSP2 remains in
progress without primary acceptance credit.

## Pattern-parent diagnostic correction

Manual review found that pattern entries applied their managed-file access policy
to the scan parent. A shared workspace parent could therefore report a private-access
conflict without violating any managed file policy.

The regression fails against `a441a72f`. The correction reports the scan parent as
`not-assessed`, with reason `pattern-anchor` and no managed-file policy. Matching
files retain their access checks. Managed tree roots retain their policy.
All 44 minimum-Go package results pass, including both POSIX and Windows diagnostic
cases. Windows-targeted lint and Ago pass.

[The correction record](windows-third-repair-2026-09-07/pattern-anchor-correction.json)
retains the failure and passing results. The orchestrator interrupted foundation
review after three portions. Exit 130 is not a clean review. Complete verification and a new review must pass before the orchestrator publishes
the corrected candidate.

Follow-up merge `0e1986b8` passes all 39 verification stages. Ordinary and race
suites each pass 79 packages. Its local Starport pair passes all 12 ownership
checks and five state-directory race results. [The follow-up record](windows-third-repair-2026-09-07/followup-verification.json)
retains these results before the pattern-parent correction.

Correction commit `6488e02f` passes all 39 foundation verification stages. Ordinary
and race suites each pass 79 packages. Four affected packages compile for each
Windows architecture with Go `1.25.12`. A new complete branch review is in progress.
Native execution remains open.

## Follow-up integration and component bindings

Follow-up merge `c7ff1e55` includes foundation `6488e02f`. The merge retains
service-managed configuration tests and the new pattern-parent regression.
Its full repository verification is in progress.

[Component evidence](windows-third-repair-2026-09-07/component-bindings.json)
records five passing checks among 22 selected CSP2 subcases. Three new bindings
verify that constructors remain passive, an accepted catalog stays unchanged,
and startup refuses a required baseline write failure. The worker observer also detects blocked
and scheduled workers in its positive fixture. All 43 verifier tests pass.

Seventeen selected subcases remain unverified. The acceptance command returns
exit 1 and grants no primary credit. Cold offline startup still needs denied-egress
evidence. Portable component results do not qualify native behavior.

## Fourth foundation native run

Foundation `6488e02f` passed all eight portions of the required cross-lab review
with no findings. Draft PR 132 now contains that commit.
[Native run 34175904597](https://github.com/agentstation/starmap/actions/runs/34175904597)
reports `in_progress`. Native qualification remains unverified.

The follow-up code from `c7ff1e55`, with the registry committed as `f6b702c0`,
passes all 39 repository verification stages. Ordinary and race suites each pass
79 packages. [The publication record](windows-third-repair-2026-09-07/reviewed-foundation-publication.json)
retains both complete logs. Follow-up review and publication await the foundation
native results. No public merge, release, or primary acceptance credit occurred.
