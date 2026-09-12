# CSP5 verification cache

Commit `1a7ae757bb68dcebfa94231a58440a67a62210d1` disables Go test-result caching in repository verification and the CI minimum-toolchain test.
The [verification record](verification-cache-2026-09-12/verification.json) preserves the observation, source, probe, workflow checks, and superseded run.
No Go source or test assertion changes from `5160414e4`.

## Observed overhead

The runtime test process produced a filesystem access log of 2,270,280,301 bytes.
After that process exited, the Go driver remained active with 5,548,640 KiB of resident memory.
The proof transcribes these task-tool observations. It does not present them as a new process sample or an inference benchmark.

The installed Go source adds a test log when result caching remains enabled.
Its argument handling disables that cache for `-count=1`.
The focused probe passes one race test and reports `caching disabled for test argument: -test.count=1`.
These results establish the cache behavior. They do not prove that this overhead caused the earlier race timeout.

## Correction and checks

Ordinary, race, and coverage verification now use `-count=1`.
The CI minimum-toolchain test uses the same flag. Existing timeouts, package concurrency, coverage thresholds, and assertions remain unchanged.

The first workflow check reports 26 passing and two failing test events.
Both existing command-prefix checks rejected the changed argument order.
Moving the flag to the command end preserves those checks. The final workflow suite passes all 28 race test events.
No test assertion changes. Repository prose passes with zero diagnostics.

The YAML regression probe passes with cache logging disabled.
Shell syntax and whitespace checks pass. Complete verification remains required.

## Qualification state

The superseded run completed twelve phases, including 85 passing ordinary packages and 22 packages without tests.
No ordinary package result used the test cache. A deliberate stop ended the subsequent race phase with exit status 143 for this correction.
Its terminal logs, stop reason, and completed phase results remain archived. This run does not count as complete repository verification.

Full verification runs in session `2580` on commit `1a7ae757b`.
The worktree is `starmap-catalog-update-controls-qualification`, on branch `codex/catalog-update-controls-qualification`.
The task check on unchanged Go source `5160414e4` passes all twelve selected subcases, including 42 race events across 21 commands.
Eight subcases retain Starport consumer requirements.

Required review, native CI, and merge remain pending.
Shared/object collection and the coordinator decision remain part of the full CSP5 contract.
A full-suite speed comparison requires terminal evidence.
