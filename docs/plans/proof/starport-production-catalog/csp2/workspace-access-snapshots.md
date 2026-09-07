# Workspace access snapshots

CSP2 now binds access metadata to workspace replacement and recovery snapshots.
Permission preservation during staging remains incomplete. The recorded `0770` to `0755` and `0600` to `0644` defect remains open.

## Contract

Each tree entry records a content digest, full permission mode, and native access digest.
POSIX metadata includes UID, GID, ordinary permissions, and special mode bits.
Linux records access and default POSIX ACL attributes. macOS records the native extended-security attribute.
Windows records owner, group, DACL, and descriptor control metadata. Native Windows execution remains UNVERIFIED.

These observations do not prove effective access, ancestor safety, or an atomic filesystem snapshot.
Windows SACL and integrity-label coverage remain outside this implemented snapshot.

Atomic replacement now compares the full original tree before publication. Journaled replacement and recovery use the same access-bound inventory.
Backup cleanup also checks the final empty root against its recorded access metadata.
The inventory bounds now apply to existing workspaces on both replacement paths: 10,000 entries and 256 MiB of file content.
ACL reads permit at most 64 KiB per native attribute or descriptor representation.

New replacement journals use version 2 and require access digests.
Recovery refuses version 1 and missing access digests without moving the workspace or changing the journal.
The refusal preserves the candidate and backup for explicit recovery. It does not provide a legacy-journal migration procedure.

## Evidence

The first regression run reproduced missed ACL changes and missed mode changes on the atomic path.
A corrected sticky-bit fixture also failed before the snapshot change.
The initial fixture changed ordinary permissions too, so its earlier pass did not prove special-mode detection.
Both records remain available as `workspace-access-snapshot-red.json` and `workspace-special-mode-red.json`.

The first macOS adapter used `acl_get_fd_np` and a separate errno read.
The full race suite reported native-read failures on absent ACLs. The recorded run contains 99 passed and 9 failed test events.
The replacement adapter uses `fgetattrlist`, which returns explicit extended-security attribute data.
The macOS SDK headers define the buffer layout. The parser checks offsets, lengths, magic, and entry counts.

The settled macOS workspace suite passed 115 test events with the race detector and no failures or skips.
Native Linux ARM64 passed 107 test events with no failures or skips. That run disabled CGO and race instrumentation.
Its isolated container had no network access. The runner removed its container and temporary volume.

Windows AMD64 and ARM64 test binaries compiled. Native Windows tests remain UNVERIFIED.

The restriction linter, host linter, Windows linter, and package-layout check passed.
The dependency-direction check passed all eight conditions.
The related caller checks passed 58 test events with the macOS race detector.
They cover workspace, repair, and projection tests in the root, runtime, acquisition, pipeline, and application packages.

Detailed commands and outputs are in `workspace-access-settled-checks.json`, `workspace-access-settled-macos.json`, and `workspace-access-native-linux-arm64/`.
Rejected checks remain in `workspace-access-checks.json` and `workspace-access-macos.json`.

## Remaining work

Preserve source access metadata when constructing replacement files and directories.
Keep copied private operator files private throughout staging, including inherited ACL behavior.
Test read-only files, changed ownership, ACL preservation, metadata-write failures, and interrupted replacement.
Handle Windows integrity labels and other security attributes before claiming native access preservation.

Define an explicit recovery procedure for earlier journals without access bindings.
Do not grant the full `TestWorkspaceReplacementPreservesAccessPolicy` acceptance criterion from these snapshot checks.
CSP2 remains in progress. No primary acceptance case or released-pair qualification changes status.
