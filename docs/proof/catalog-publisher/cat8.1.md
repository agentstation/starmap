# CAT8.1 proof: the Starport console catalog surface

CAT8.1 owns the console chip and panel that replace the two partial catalog
surfaces. The work lives in the Starport repository, not in Starmap. This file
records where the work is, what the owner review found, and what the gates
reported.

CAT8 owns the Starport runtime. CAT9.1 owns the Starport operator text.

## Location

The work starts from the CAT8 head `9081ca4` on the Starport branch
`worktree-agent-ad6e217c4e16a7520`. The full proof is
`docs/proof/catalog-publisher/cat8.1.md` at that Starport branch.

| Commit | Subject |
| --- | --- |
| `b468a00` | feat(console): give the catalog one chip and one panel in the shell |
| `5df5ea6` | docs(proof): record the CAT8.1 console catalog surface |
| `8da2c6c` | test(console): point the polish gate at the renamed changes section |

The four CAT9.1 commits sit on the same branch above `8da2c6c`. The branch
head after them is `d75dbe2`.

## What changed

The shell mounts one catalog chip on every route. The chip opens one panel
that answers seven questions in order. The freshness bar above the model list
and the catalog card on Overview are gone. The changes list moved from the
models directory into the shell as the Changes section of the panel.

The model detail gives lifecycle, availability, credential, circuit, and
routing a cell each. The console holds no age rule of its own. The gateway
grades the age, and the console reads the grade.

## Owner review

The review at `5df5ea6` found one defect. The console-polish gate named the
old path `components/models/ChangesPanel.tsx` in `CPL-V27`, and the rename
left the gate with a missing file. The owner repaired the path in `8da2c6c`.
The gate keeps its count of 48 and its meaning.

The owner accepted the eleven console decisions that the agent recorded. Each
one states an API gap of the CAT8 routes instead of an invented value.

## Gates at `8da2c6c`

Every command ran in the Starport worktree.

| Command | Result |
| --- | --- |
| `pnpm check` | build, typecheck, and 438 tests in 70 files pass |
| `make lint` | 0 issues |
| `bash scripts/verify-console-polish.sh` | 48 passed, 0 failed |
| `bash scripts/verify-console-session-grants.sh` | 16 passed, 0 failed |
| `bash scripts/verify-console-modernization.sh` | 21 passed, 0 failed |
| `bash scripts/verify-auth-onboarding.sh` | 26 passed, 0 failed |
| `bash scripts/verify-doc-links.sh` | pass |

`make lint` needs a private `GOLANGCI_LINT_CACHE` while sibling worktrees run
the same linter. A shared cache reports the paths of another worktree.

## Verifier

The run uses `scripts/verify-catalog-distribution.sh` at plan head `7129b237`.
`CATALOG_DISTRIBUTION_STARPORT_ROOT` points at the Starport worktree at
`8da2c6c`. The result is 65 passed, 3 failed, and 0 unverified of 68. The three failures
belong to CAT9.1 and pass at `d75dbe2`.

The seven CAT8.1 conditions pass:

- CAT-V50, CAT-V52, and CAT-V53
- CAT-V54 and CAT-V55
- CAT-V63 and CAT-V68
