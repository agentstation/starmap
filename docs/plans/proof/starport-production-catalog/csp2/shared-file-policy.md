# Shared file-role access policy

Starmap now declares file-role access classes in `pkg/productpaths/policy`.
Application diagnostics and runtime adapters consume that shared definition.
The package also owns the POSIX mode and owner predicate. Windows retains its shared native DACL policy.
Starport adoption and native Windows qualification remain open.

## Runtime contract

The registry distinguishes owner-only state, deployment-controlled inputs, public-read exports, and external systems.
Unknown roles have no default access class. The application still owns selectors, availability, retention, and recovery descriptions.

Runtime adapters call `policy.Require` before file access to verify their supported access class against the registry.
A changed declaration with an incompatible adapter causes a configuration error before that adapter accesses files.
It cannot silently relax private-file enforcement or apply private-store checks to an editable workspace.

Bindings cover primary configuration, explicit dotenv, catalog storage, runtime identity, migrations, retained evidence, GitHub discovery, workspace operations, preparation, baseline exports, and source paths.
Source paths include file payloads, HTTP cache, and Git checkout selection.
Planned writers and externally managed destinations retain declarations without claims of implemented local enforcement.

POSIX enforcement and inspection now share mode and owner classification.
Enforcement still refuses unavailable owner evidence. Inspection preserves uncertainty when native ownership or effective access is unknown.
An empty metadata-conflict reason does not prove ACL safety or usable access.
The change preserves existing runtime error identities and diagnostic reason strings.

`TestFileRolePolicyMatchesRuntimeEnforcement` compares actual runtime access with passive inspection on macOS and Linux.
It covers primary configuration, explicit dotenv, catalog storage, runtime locks, and editable workspaces.
The test verifies unchanged bytes and modes after refusal. Group-accessible workspaces remain readable through the coordinated workspace API.
Separate tests reject unknown or incompatible roles before private input access.

## Process-lock test correction

The earlier macOS failure came from the child test's lock lifetime.
The child dropped its last reachable lock reference before it waited for the parent.
Go could then finalize the underlying file and release its operating-system lock while the child process remained alive.

The [forced-GC regression](runtime-lock-gc-red.json) reproduced the same failure in all five runs.
The child now retains the lock through `runtime.KeepAlive` after its input wait.
Forced collection remains in the test. Failed parent paths also kill and reap the child.
The [focused correction](runtime-lock-gc-focused.json) passed ten runs with forced garbage collection.

The production runtime already retains its directory lock until close. This correction changes the test harness, not that runtime contract.

## Verification

Counts include named tests and subtests, but exclude package completion events.

| Evidence | Result | Limits |
| --- | --- | --- |
| [Missing shared contract](shared-file-policy-red.json) | Compile failure before implementation | Records the absent public policy API. |
| [Focused policy checks](shared-file-policy-focused.json) | 41 passed, no failures or skips | Go 1.25.12 with the macOS race detector. |
| [Full macOS run](shared-file-policy-macos.json) | 925 passed, no failures or skips | Eleven packages with the race detector. Includes the corrected process-lock test. |
| [Native Linux run](shared-file-policy-native-linux-arm64/summary.json) | 872 passed, three failed, no skips | Ten core packages passed 803 events. Models.dev passed 69 events but failed three tooling tests. |
| [Static checks](shared-file-policy-checks.json) | All commands passed | Ago, host and Windows lint, layout, dependency direction, and 18 Windows builds. |
| [Source manifest](shared-file-policy-input-manifest.json) | 298 selected inputs matched during the macOS run | Later Windows-only changes have a separate manifest below. |

The minimal Linux image has no Bash, Git, or curl.
The failed tests are `TestCatalogGenerationToolingRejectsHTTPErrorBeforePromotion`, `TestCatalogGenerationToolingUsesCurrentCLIAndRealValidation`, and `TestGitClientPinsCommitAndFrozenLockfile`.
Those checks need a native tooling environment. Their failures remain recorded without a passing verdict.
The runner removed all eleven temporary containers and volumes.

## Windows inheritance follow-through

The Windows adapter now requests unprotected DACL inheritance only when the destination is currently protected.
It preserves an already-enabled inheritance state while the candidate remains inside its private preparation directory.
Microsoft defines inheritance behavior in [SECURITY_INFORMATION](https://learn.microsoft.com/en-us/windows/win32/secauthz/security-information).

The native fixture supplies inheritable parent grants and an unprotected workspace.
The test checks inherited entries and access digests after replacement.
`TestWorkspaceReplacementPreservesInheritedAccessPolicy` therefore covers a different parent policy from the private preparation directory.
The earlier protected-DACL and integrity-label test remains unchanged.

The [Windows follow-up checks](shared-file-policy-windows-inheritance-checks.json) passed ago, Windows lint, and both workspace cross-builds.
The [Windows input record](shared-file-policy-windows-inheritance-inputs.json) binds those two changed files.
These Windows-only changes followed the full macOS and Linux runs. They have no native execution credit.
The prepared native workflow now includes the shared policy package. This work did not publish or dispatch workflows.

## Remaining work

CSP2 retains native Windows execution, native tooling qualification, foreign-owner restoration, service procedures, and the pending constructor decision.
The administrator-owned configuration exception still needs the pending owner decision.
CSP5 owns abandoned stages and errors after publication. CSP8 owns Starport adoption, and CSP12 owns engine-specific access and recovery.
No primary acceptance case gains credit from this record. CSP2 remains in progress.

The [task gate](shared-file-policy-task.json) reports two passing subcases and twenty UNVERIFIED subcases.
The [document gate](shared-file-policy-documents.json) preserves the 38-task, 50-case, and 324-subcase contracts.
The [writing check](shared-file-policy-writing.json) reports only three existing diagnostics in preserved historical output files.

## Subsequent native qualification

The [native ownership record](native-ownership-and-tooling.md) supersedes the missing-tooling and Linux foreign-owner gaps above.
The tooling rerun passes 72 events, and the ownership fixture passes 15 events without skips.
Other platform ownership checks, native Windows execution, service configuration, and legacy journal procedures remain open.
