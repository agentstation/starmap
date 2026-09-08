# Catalog access policy conversation review

The Starmap worktree follows the workspace boundary and passive-diagnostic recommendations.
Workspace permission preservation now passes macOS and Linux component checks.
Starmap now shares role access declarations and POSIX classification across adapters and diagnostics.
Native Windows qualification and Starport adoption remain incomplete.

This review covers the full conversation named `Catalog Access Policy Mismatch`, ID `6a9dcfdd-0664-83e9-be98-ce03fe56a101`.
The conversation is design input. Repository source and recorded checks determine implementation status.

## Recommendation assessment

| Recommendation | Current evidence | Result and owner |
| --- | --- | --- |
| Keep editable YAML separate from accepted state | `pkg/productpaths/policy/access.go` declares workspace roles deployment-controlled and the catalog store owner-only. Workspace readers do not call private-file guards. | Implemented in the Starmap worktree. Native and released-pair qualification remain open. |
| Preserve operator permissions during replacement | The original probe reproduced mode loss. Private staging now restores access and verifies its digest before candidate publication. | Corrected for tested macOS and Linux cases. Native Windows, other-platform foreign ownership, and full recovery qualification remain open. |
| Permit developer-defined providers through YAML | `internal/catalog/pipeline/catalog_driven_provider_test.go` exercises a YAML-only provider through a real local HTTP adapter. It checks reviewed model links and unknown-offering quarantine. | Supported component behavior. It does not prove every hand-authored schema or the full released CLI workflow. |
| Keep diagnostics passive | `pkg/productpaths/inspection.go` scans metadata. Application tests verify unchanged bytes and permissions without catalog, runtime, or credential initialization. | Implemented. The runtime owner comparison separately reads a bounded identity record. |
| Report semantic policy and native metadata | POSIX observations contain policy, mode, UID, GID, status, and reason. The new Windows adapter adds SIDs, DACL state, and private-policy status. | POSIX checks pass locally. Native Windows behavior remains UNVERIFIED. |
| Use one expected policy definition | Application reports and runtime adapters use `pkg/productpaths/policy`. Adapters refuse declarations outside their supported access class. | Implemented in the Starmap worktree. Native and Starport qualification remain open. |
| Share platform policy predicates | Windows inspection and enforcement use `internal/runtimeacl/windows`. POSIX inspection and enforcement now share `policy.PrivatePOSIXReason`. | Implemented with retained diagnostic uncertainty. Native Windows execution remains open. |
| Inventory all roles | The Starmap manifest reports implemented, disabled, planned, and external roles with access and retention descriptors. | Present in Starmap. Complete cross-product adoption belongs to CSP8 and storage-specific verification belongs to CSP12. |
| Support service-managed configuration | Starmap currently requires private configuration owned by the effective process account. | A service-owned input exception still needs the pending owner decision and native procedures. No exception is implicit. |
| Qualify Kubernetes storage | The recipe selects explicit roots, uses a single-writer rollout, and avoids automatic recursive group-permission changes. | Prepared recipe. Cluster, storage-driver, ownership, and recovery qualification remain open. |
| Apply the same contract in Starport | Starport pins Starmap `v0.16.5` and retains its own settings translation. The new file manifest and inspection API have no Starport consumer. | Incomplete. CSP8 remains todo. CSP12 owns engine-specific access and recovery checks. |

## Corrections to the conversation

The earlier diagnostic did not return `OK` for permissive catalog-store modes. Its deployment-controlled classification left access `unverified`.
The defect was a missing known conflict. `config paths` reports selected paths, while `config paths --inspect` adds filesystem observations.
Neither command promises a complete runtime-readiness verdict.

Read access and write access differ. A group-readable file does not, by that fact alone, let the group modify accepted state.
The private store policy restricts disclosure as well as direct mutation. Public embedded exports and editable workspaces retain separate policies.

Owner-only describes the operating-system account boundary. It does not prevent that account from editing its own files or override privileged host administration.
The supported authoring path edits the YAML workspace and submits it through catalog validation. Direct edits to accepted generations remain unsupported.

Configuration is not automatically shareable because it is input. Selected configuration and dotenv files can contain credentials and currently require private access.
The proposed administrator-owned, service-readable exception remains a separate decision. It must not silently apply to accepted catalog state.

The workspace remains editable, subject to its filesystem permissions, schema validation, and coordinated Starmap operations.
An external editor does not participate in Starmap's locks. Changed files can therefore cause a conflict that preserves operator work.

## Required follow-through

CSP2 now gives runtime enforcement and diagnostic declarations one semantic file-role policy source.
Native qualification must complete the existing checks and preserve uncertainty when inspection lacks evidence.
This does not require diagnostics to read payloads, take runtime locks, or change filesystem permissions.

CSP8 must consume that same contract in Starport and retain the editable-workspace boundary.
CSP12 must verify Badger, SQLite, blob files, and restore paths under their declared engine policies.
For example, Starport's current Badger restore path creates its directory with mode `0750` and does not yet use the new file-role contract.
That source observation does not prove actual group access or qualify an engine's effective security.

The current review does not authorize a permission change, a service exception, or publication.

The [review checks](access-policy-conversation-checks.json) passed one YAML-only provider test and four workspace tests with the macOS race detector.
They cover acquisition, workspace separation, accepted layouts, operator-file preservation, and dirty-workspace repair refusal.
They do not qualify the full released Starport integration or every native filesystem.

## Workspace permission defect

The [permission probe](workspace-permission-review-red.json) reproduced two mode changes during workspace replacement on macOS.
The workspace changed from `0770` to `0755`. An unrelated operator note changed from `0600` to `0644`.
The probe used temporary files and removed its temporary test source after recording the result.

At the initial review, `stageCatalog` created a replacement root with `0755` and copied existing files through `os.CopyFS` before rendering the new catalog.
That implementation did not preserve the selected root mode or the copied private note mode.
This can remove group write access and add group or other read bits. Actual exposure still depends on ancestor access and native ACLs.

CSP2 must preserve declared workspace access through replacement or refuse before publication when preservation is unavailable.
Checks must include root access, managed YAML, unrelated operator files, ownership, native ACLs, and interrupted replacement.
The initial content-preservation tests did not establish permission preservation. Native Windows and shared-service behavior remain unqualified.


## Subsequent implementation

[Access snapshots](workspace-access-snapshots.md) now bind ownership and native ACL metadata to workspace replacement and recovery.
The subsequent [permission-preservation record](workspace-access-preservation.md) covers private assembly, access restoration, inheritance, and operator notes.
It records 129 workspace race test events on macOS and 120 native Linux workspace events without failures or skips.
These checks correct the reproduced mode-loss defect for those environments. They do not qualify every filesystem or the released product pair.

The latest six-package macOS run passed 695 test events and failed one runtime process-lock test.
Ten focused reruns passed. The initial failure remains unresolved and does not receive a passing status.
That checkpoint matched all 134 recorded source inputs.

CSP2 owns native qualification. CSP8 owns Starport adoption. CSP12 owns engine qualification.
Service configuration exceptions, native Windows inheritance, and recovery procedures remain open. No primary acceptance case changes status.

## Shared-policy completion and lock diagnosis

The [shared-policy record](shared-file-policy.md) supersedes the earlier consolidation gap and unresolved lock-test finding.
Forced garbage collection reproduced the child test failure in all five runs. An explicit lock lifetime corrected it without changing production lock ownership.
The subsequent macOS run passed 925 test events with the race detector and no skips.

Native Linux passed ten core packages, but three additional tooling tests failed because the image lacks Bash, Git, and curl.
Windows inheritance now has a prepared native regression and a targeted adapter correction. Native execution remains UNVERIFIED.
The plan retains these qualification requirements and Starport adoption. No primary acceptance status changes.

## Native ownership follow-through

The [native ownership record](native-ownership-and-tooling.md) verifies Linux refusal and preservation across 12 ownership scenarios.
It also verifies service updates with an authorized supplementary group.
The tooling rerun closes the three missing-executable failures. Other platforms, service configuration, and Starport adoption remain open.
