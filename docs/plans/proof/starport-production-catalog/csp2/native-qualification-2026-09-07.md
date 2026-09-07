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

Next: complete the repair review, publish the candidates, and rerun native CI.
Use the focused descriptor evidence to repair Windows behavior at its owning
package. Keep CSP2 in progress until native qualification passes.
