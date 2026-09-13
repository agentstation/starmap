# CSP5 native recovery result

CSP5 remains incomplete. [PR #154](https://github.com/agentstation/starmap/pull/154) remains open at `9e278107c`.
The [status capture](native-recovery-result-2026-09-13/status.json) binds the native results to that source.

[Windows ARM](https://github.com/agentstation/starmap/actions/runs/34733931016/job/103661788291) failed its full native test step.
It records 2,758 passing test events, three failing events, and five skips across eighteen packages.
Seventeen packages pass. The workspace package fails in two cases. The third failure event is their parent test.
The focused filesystem preflight passes 29 events across two packages.

- `TestLegacyRollbackPreservesOperatorSidecars/writer-lock` attempts a write into a Windows-locked region.
- `TestRelocationPreparationClosesHandlesAfterCleanupConflict` expects `os.ErrClosed` from `File.Stat` and receives a native invalid-handle error.

The second case matches the pinned Go implementation: Windows `File.Stat` calls `statHandle` directly.
Workspace retirement defers handle release even when cleanup reports a conflict.
These observations indicate test portability defects. They do not qualify a correction or prove every subsequent assertion.
The repair must preserve the operator-file, rollback, and handle-release checks without skipping either case.

Both Linux jobs and both macOS jobs pass. Windows x64 and the verification gate remain active at this capture.
Native qualification and merge remain required. Shared catalog cleanup and its pending coordinator decision also remain required.
No completion credit applies.

The local PR branch contains 49 commits across 277 changed files, with 26,232 insertions and 1,232 deletions against `f9951ee6`.
This batch size increases review and native retry costs. Complete the qualified local delivery before separate shared-cleanup delivery.
