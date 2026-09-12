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

Repository session `2580` ended with exit status 2 at 19:34:17 UTC on September 12.
Twelve phases passed before the race suite. All 85 ordinary package results passed without cached results, alongside 22 packages without tests.
The ordinary runtime package passed in 447.139 seconds.
These package results do not expose exact individual test or skip counts.

The race suite exhausted local disk capacity. Its output contains 29 passing package results, 50 failing package results, and 122 failed test lines.
Those test lines include parent and nested subcase failures. They are not a count of independent tests.

The logs contain 113 disk-capacity errors in stdout and 49 in stderr.
The first disk-write errors precede the first CLI assertion failure. Later packages could not compile because temporary writes failed.
The resource record preserves the original output and counts. Remaining race packages and later repository gates remain unverified.

The Go build cache occupied 64,529,824 KiB. `go clean -cache` completed with exit status 0 and restored about 63 GiB of available disk capacity.
This cleanup removed regenerable build and test cache entries. It preserved source, module downloads, worktrees, and proof logs.
The cleanup record retains the command, exit status, and disk observations.

Full qualification restarted in session `7815` on unchanged commit `1a7ae757b`.
The worktree is `starmap-catalog-update-controls-qualification`, on branch `codex/catalog-update-controls-qualification`.
The retry uses the same complete verification command, test selection, and time limits.

Before the race suite, the retry passed twelve phases, including all 85 ordinary package results without test-cache reuse.
It also reports 22 packages without tests. Its ordinary runtime package passes in 516.644 seconds.

At 20:51:43 UTC, the retry reports 43 passing race packages and no failure lines.
The CLI application package passes in 350.617 seconds. The earlier disk-capacity failure affected this package.
The proof archives this intermediate output snapshot.

Full repository verification passed with exit status 0 at 21:31:39 UTC on September 12.
All 41 phases pass on unchanged source `1a7ae757b`. Both ordinary and race suites report 85 passing packages and 22 packages without tests.
The full runtime race package passes in 1,551.575 seconds within the unchanged 30-minute limit.

Vet, the performance gate, container smoke, lint, policy checks, and all fifteen critical coverage gates pass.
Generated documentation, whitespace, and CLI smoke checks pass. The prose check passes 1,672 files with zero diagnostics.
The verification record archives the complete output, commands, coverage values, and terminal result. Earlier failed and interrupted runs remain preserved.

The task check on unchanged Go source `5160414e4` passes all twelve selected subcases, including 42 race events across 21 commands.
Eight subcases retain Starport consumer requirements.

A review preflight passes for both configured reviewers on source `1a7ae757b`.
This dry run checks review readiness. It provides no review verdict or attestation.

The actual required review passed with exit status 0 at 21:57:28 UTC after complete verification.
Sol at xhigh effort and Opus at high effort reviewed all six sections.
The combined report contains no findings at the configured P0 threshold. Secret scanning passes.
The verification record archives the report, rendered result, commands, and complete review output.

[Starmap PR #154](https://github.com/agentstation/starmap/pull/154) publishes the reviewed source as a draft.
The [PR workflow](https://github.com/agentstation/starmap/actions/runs/34721468698) started all six native platform jobs.
Its initial snapshot records those jobs as running. Native qualification and merge remain pending.
Shared/object collection and the coordinator decision remain part of the full CSP5 contract.
A full-suite speed comparison requires terminal evidence.
