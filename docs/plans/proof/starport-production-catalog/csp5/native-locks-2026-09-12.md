# CSP5 Windows ownership-lock failure

Both Windows jobs failed in [PR workflow 34721468698](https://github.com/agentstation/starmap/actions/runs/34721468698) on source `1a7ae757b`.
The [native verification record](native-locks-2026-09-12/verification.json) preserves complete artifacts, job logs, source bindings, and live check states.
The remaining full verification job continues. PR #154 cannot merge yet.

| Native target | Passing packages | Passing test events | Failing test events | Failed packages |
| --- | --- | --- | --- | --- |
| Windows ARM | 10 | 1,863 | 842 | 8 |
| Windows amd64 | 10 | 1,862 | 843 | 8 |

These event counts include parent tests. They do not count independent defects.
The ARM log contains 716 locked-region error lines. The amd64 log contains 722.
Both runs also reach the thirty-minute storage test limit.
`TestFilesystemRetentionSerializesOrdinaryReads` remains active in the captured timeout.

Many failures occur while a record writer reads its locked `.owner.lock` through another handle.
Baseline recovery and several workspace tests encounter the same Windows read refusal.
[Microsoft documents this restriction](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex), including access through another handle in the same process.

The correction must validate empty ownership files without reading a locked byte range.
It must retain identity, access, size, change detection, and writer exclusion checks.
The remaining assertions and storage timeout require separate confirmation after that correction.
No test exclusion, timeout increase, or native acceptance claim follows from this diagnosis.

## Terminal parent workflow

The [terminal snapshot](native-locks-2026-09-12/terminal-workflow.json) records the completed run on the original parent source.
The full verification gate passes. Both Windows jobs fail.
The corrected commit still requires its own native and required CI checks.
