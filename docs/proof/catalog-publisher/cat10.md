# CAT10 proof: reviews, pull requests, and merges

CAT10 carried the campaign through the repository gates, the structured
reviews, and the two pull requests. CAT-D23 places the Starmap release before
the Starport pull request. The Starmap work merged as pull request #116. The
Starport work pins the released Starmap module. Its pull request waits for a
manual merge.

## Starmap

**The branch.** `codex/catalog-publisher-six-hour` closed at `5d0739a7`. The
pre-PR review ran on Opus 5 at high effort in five chunks and returned clean at
`e374ce5a`. The one later code commit `88873f7a` passed a commit-scoped Opus
review, because the branch bundle overflowed the reviewer prompt on the rerun.

**The pull request.** Pull request #116 opened with auto-merge armed. The
hosted Verification Gate ran four times.

| Run | Head | Result | Cause |
| --- | --- | --- | --- |
| 33819884242 | `83f44657` | failure | the server-storage closure measured 341 on Linux against a budget of 340 |
| 33822905151 | `7dceda24` | failure | `internal/ciworkflow` pinned the former budget literal |
| 33825728341 | `e374ce5a` | failure | the `acquisition` coalescing window test waited five seconds under race on the loaded runner |
| 33828957060 | `5d0739a7` | success | none |

Each failure produced one repair commit on the branch, and
`docs/proof/catalog-publisher/cat10.1.md` records the reason for each. The
Linux closure adds three platform packages that Darwin does not compile. The
budget moved to 350, and the contract test moved with it. The hang guards in the
acquisition test now allow thirty seconds, and the package passed twice locally
under race with two processors.

**The merge.** The squash merge landed on main as `0206f315` on 2026-09-04 at
02:43 UTC. The commit subject is `Publish the attested catalog channel and add
the connected runtime package (#116)`.

**The release.** The annotated tag `v0.16.0` points at `0206f315`. The version
is a minor bump. The change adds three packages and the release channel. The
root library keeps its contract. `make release-check` and `make embedded-catalog-budget-check`
passed locally before the tag. The tag push started release run 33830647939.

**The release failure.** The test job of release run 33830647939 failed in
`make verify`. The workflow ran `make technical-writing-check` without the
pinned technical-writing skill on the runner, so the check reported that the
skill was not found. The pull request workflow checks out the pinned
`agentstation/skills` revision before the same check, and the release workflow
did not. The release, recover, and Homebrew jobs did not run. The version
`v0.16.0` exists in the module proxy and the checksum database, and it has no
GitHub release. A module version is immutable there, so the repair needs a new
tag.

**The repair.** Branch `codex/release-verify-skill` from `0206f315` adds the
skill checkout to the release test job and points `TECHNICAL_WRITING` at it.
`internal/ciworkflow/release_workflow_test.go` pins the new step. The checkout
lives in the test job only, and the goreleaser job keeps its own clean tree.
The pre-PR review ran on Opus 5 at high effort in one pass and returned clean
at `11583894`. Pull request #117 opened from the branch with auto-merge armed.

**The second release.** The Verification Gate passed, and the squash merge landed on main as
`03a30b6e`. `make release-check` and `make embedded-catalog-budget-check`
passed locally at that head. The annotated tag `v0.16.1` points at `03a30b6e`.
The tag push started release run 33837853672.

**The second release failure.** The test job of run 33837853672 passed
`make verify` and then failed `make release-check`. That target requires a
clean working directory, and the skill checkout under `.ci` sat in the tree as
untracked files. The pull request workflow never runs `make release-check`, so
the first repair could not expose the second defect. The version `v0.16.1` is
in the module proxy with no GitHub release, like `v0.16.0`.

**The second repair.** Branch `codex/release-check-ignore-ci` from `03a30b6e`
adds `/.ci/` to `.gitignore`, and
`internal/ciworkflow/workspace_ignore_test.go` pins the rule and the checkout
path in both workflows. With the pinned skill revision checked out under
`.ci`, `git status --porcelain` reports nothing and `make release-check`
passes. The pre-PR review ran on Opus 5 at high effort in one pass and returned
clean at `20e80526`. Pull request #118 opened with auto-merge armed, and the
Verification Gate passed. The squash merge landed on main as `789dc26a`, and it
equals the reviewed commit.

**The third release.** Before the tag, the complete release test job ran
locally at `789dc26a` with the pinned skill checked out under `.ci`:
`make embedded-catalog-budget-check`, `make verify`, and `make release-check`
passed in order. The annotated tag `v0.16.2` points at `789dc26a`. The tag
push started release run 33841625845. The test, release, and Homebrew
verification jobs passed. GitHub release `v0.16.2` published on 2026-09-04 at
06:15 UTC as an immutable release with fourteen assets. CAT11 records the
public artifact checks.

## Starport

**The pin.** The combined branch `worktree-agent-ad6e217c4e16a7520` carried a
local `replace` directive to the plan worktree through four tasks. The Starport
review rated that directive a P0 defect. Condition V01 of
`scripts/verify-v1-architecture.sh` rejects both a `replace` and a
pseudo-version. Commit `4affab0` removes the directive, pins
`github.com/agentstation/starmap v0.16.0`, and restores the two `go.sum` lines
for that release. The module cache at
`github.com/agentstation/starmap@v0.16.0` holds `pkg/catalogs`, which the
hosted Release Contract job reads.

**The gates at `4affab0`.** Every command ran with `GOTOOLCHAIN=go1.26.6`. The
two verification scripts that read a Starmap tree pointed at the downloaded
`v0.16.0` module.

| Check | Result |
| --- | --- |
| `go build ./...` and `go vet ./...` | pass |
| `go test ./...` | 55 packages pass |
| `make lint` | 0 issues |
| `go test -race -short` with `GOMAXPROCS=2` on the ten branch-touched packages | 9 packages pass, 1 without tests |
| the 26 `scripts/verify-*.sh` and `scripts/test-*.sh` gates in the agent instructions | 26 pass |
| `scripts/verify-v1-architecture.sh` | 12 of 12, V01 green |
| `scripts/verify-starmap-ownership.sh` | 12 of 12 |
| `scripts/verify-catalog-driven-providers.sh` | 19 of 19 |
| Starmap `scripts/verify-catalog-distribution.sh` against this tree | 68 passed, 0 failed, 0 unverified |

The three release-build gates and the two smoke scripts belong to the hosted
jobs and did not run locally.

**The review and the pull request.** The pre-PR review ran on Opus 5 at high
effort in two chunks. The bundle held 545,731 bytes. The review returned clean
at `4affab0`. The branch pushed as `codex/catalog-runtime` with eighteen commits
over main `117ad8f5`. Pull request #360 opened from it. Starport has no
auto-merge, so the merge waits for green hosted checks and a manual action.

**The hosted failures at `4affab0`.** Run 33831976514 failed two jobs.

| Job | Cause |
| --- | --- |
| Release Contract | `smoke-first-run.sh` starts `starport dev` with an empty environment, and the development loader failed with "configured paths could not be resolved", because the catalog state directory resolved through the user home directory |
| Test (windows-latest) | `TestResolveStateDirectoryIsProcessLocal` set `HOME` only, and Windows reads `USERPROFILE` |

The first failure exposed a defect in the CAT8 design, not in the smoke script.
Before the repair, a development gateway wrote its catalog state under the
user state root, and the development contract requires no persistent state.

**The repair at `4b81b1f`.** The development loader marks an empty catalog
state directory as session scratch. The mark is an unexported field with no
environment tag, so no environment variable or configuration file can select
it for a serving gateway. The development composition creates the state
directory beside the file scratch under one temporary root and removes both on
close. An operator value is never scratch, and a test proves the composition
keeps it.

**The serving gateway at `4b81b1f`.** A serving gateway with no home directory and no state root refuses
to start, and the error names `STARPORT_CATALOG_STATE_DIR` and
`XDG_STATE_HOME`. The smoke script supplies `XDG_STATE_HOME` inside its smoke
root for the init and serve sections, and its dev section runs with no home
directory. The state directory test sets `HOME`, `USERPROFILE`, and `home`.
The operator guide, the environment example, and the task record describe the
behavior.

**The gates at `4b81b1f`.** The same commands ran again, with the same Starmap
tree.

| Check | Result |
| --- | --- |
| `go build ./...` and `go vet ./...` | pass |
| `go test ./...` | 55 packages pass |
| `go test -race -short` with `GOMAXPROCS=2` on `internal/config` and `internal/app` | pass |
| `GOOS=windows go vet` on `internal/config` and `internal/app` | pass |
| `make lint` | 0 issues |
| the 26 `scripts/verify-*.sh` and `scripts/test-*.sh` gates | 26 pass |
| `scripts/smoke-first-run.sh` | pass, and `~/.local/state/starport` stays absent |
| `shellcheck scripts/smoke-first-run.sh` | pass |
| Starmap `scripts/verify-catalog-distribution.sh` against this tree | 68 passed, 0 failed, 0 unverified |

The pre-PR review ran again on Opus 5 at high effort in two chunks and
returned clean at `4b81b1f`. The branch pushed to `codex/catalog-runtime`, and
run 33836574604 passed every job. The jobs are Lint, the three Test jobs,
Security Scan, Release Contract, OpenRouter SDK Compatibility, Release
Snapshot, Action Pin Provenance, and Build.

**The pin at `f0ffe0d`.** Commit `f0ffe0d` moves the pin to
`github.com/agentstation/starmap v0.16.1` and changes `go.mod` and `go.sum`
only. Build, vet, the 55 test packages, lint, and the five verification
scripts that read the Starmap tree pass against the downloaded `v0.16.1`
module. The verifier reports 68 of 68. The pre-PR gate reused the clean
attestation, because the substantive diff did not change. Run 33838468154 at
`f0ffe0d` passed every hosted job.

**The pin at `60615e0`.** Commit `60615e0` moves the pin to
`github.com/agentstation/starmap v0.16.2`, the first tag with a GitHub
release. The same gates passed against the downloaded `v0.16.2` module, the
verifier reports 68 of 68, and the pre-PR gate reused the clean attestation.
The branch pushed after release run 33841625845 succeeded. Run 33843823866
at `60615e0` passed all ten hosted jobs. Pull request #360 squash-merged as
`4a52d48` on 2026-09-04, and the merge deleted the branch. The Starport main run
33844884647 at `4a52d48` completed with success.

## Untouched by this task

CAT10 changed no Starmap runtime behavior. Its Starmap commits move one budget,
one budget literal, two hang guards, one release workflow step, and the plan
and proof prose. Its Starport commits move one `go.mod` pin and the
development catalog state directory. A serving gateway with a home directory
or a state root behaves as before.
