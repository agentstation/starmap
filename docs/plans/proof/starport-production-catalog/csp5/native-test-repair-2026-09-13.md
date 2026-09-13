# CSP5 native test correction

Commit `71a1af619` corrects two Windows test assumptions and adds both cases to the early native filesystem check.
The [verification record](native-test-repair-2026-09-13/verification.json) binds all nine checks to the three changed files.

Both Windows architectures failed the parent source `9e278107c` in the same two cases.
Each run records 2,758 passing test events, three failing events, and five skips. One failure event represents the parent test.
The [ARM failure proof](native-recovery-result-2026-09-13.md) and the new x64 capture preserve those failures.

The sidecar test writes after byte zero without truncating the file. The Windows writer lock covers byte zero.
The test still checks exact operator bytes and the restored catalog after rollback.
The handle test reads one byte through the captured file and requires `os.ErrClosed`.
It still checks closed directory handles and preserved operator content.
Neither case skips its assertions on Windows.

The complete workspace suite passes 385 race test events without failures or skips.
The two corrected cases pass four events on each supported Go toolchain.

All 28 workflow race events pass. Both Windows test binaries compile. Native execution remains UNVERIFIED for this correction.
Policy and lint report no findings. The prose check passes 1,689 files without diagnostics.

Required cross-lab review passed all six sections with zero findings at the configured P0 threshold.
Sol at xhigh and Opus at high reviewed the committed source. The review finished at 04:14 UTC on September 13.
The verification record retains its terminal result and transcripts.

[PR #154](https://github.com/agentstation/starmap/pull/154) now publishes `71a1af619`.
[Workflow 34737726813](https://github.com/agentstation/starmap/actions/runs/34737726813) checks this source on native runners.
The [publication capture](native-test-repair-2026-09-13/publication.json) records the exact head and pending checks.
Native qualification, merge, and shared cleanup remain required. CSP5 remains incomplete.

The PR spans 277 files. This large change increased review time and repeated CI costs.
The next delivery will keep shared-storage cleanup in a separate PR. CSP5 completion still requires the full task contract.
