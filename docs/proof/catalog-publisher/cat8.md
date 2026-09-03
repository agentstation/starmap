# CAT8 proof: the Starport catalog runtime

CAT8 owns the Starport migration to the catalog-source contract. The work lives
in the Starport repository, not in Starmap. This file records where the work
is, what the owner review found, and what the gates reported.

CAT5 owns the runtime core. CAT7 owns the cascade. CAT8.1 owns the console
surfaces. CAT9.1 owns the Starport operator documentation.

## Location

The work starts from Starport main `117ad8f5` on the branch
`worktree-agent-a8eeed01486fd2525`. The head is `9081ca4`. The full proof is
`docs/proof/catalog-publisher/cat8.md` at that Starport commit.

| Commit | Subject |
| --- | --- |
| `f7a9c56` | build: point Starmap at the catalog-publisher plan worktree |
| `33a130f` | feat(catalog): adopt the canonical Starmap settings and one connected runtime |
| `cebecb6` | feat(execution): make route timing route-specific |
| `51c3542` | feat(catalog): serve asynchronous refresh and the operator catalog status |
| `cb1718c` | test(catalog): restore the acquisition and streaming architecture guards |
| `2d77774` | docs(proof): record the CAT8 Starport catalog runtime |
| `5d7d23a` | fix(catalog): one refresh bound, a process-local state directory, and the accepted head |
| `9081ca4` | docs(proof): record the CAT8 owner-review repairs |

The first commit adds a local `replace` directive that points at the plan
worktree. CAT10 replaces the directive with the pseudo-version pin that
CAT-D17 names.

## Owner review

The review at `2d77774` returned five defects. The agent repaired every one in
`5d7d23a`.

1. The refresh path substituted a two-minute cap when the timeout was zero. The
   contract says zero adds no cap. The one cap now comes from
   `STARPORT_CATALOG_REFRESH_TIMEOUT`.
2. Starport passed no listen address and used the workspace path as the state
   directory. Two processes on one host then shared one lease identity.
3. The effective provenance named the candidate generation with the accepted
   timestamp.
4. A canceled operation could read a pruned record.
5. The update channel comment described a drop that the code did not make.

## Design amendment

The review adds `STARPORT_CATALOG_STATE_DIR`. It names the process-local
runtime state directory. The empty default resolves to the user state
directory under `starport/catalog`. The setting can never equal the workspace
path. Starport always passes the listen address to the runtime. CAT9.1
documents the setting.

## Gates at `9081ca4`

Every command ran with `GOTOOLCHAIN=go1.26.6` in the Starport worktree.

| Command | Result |
| --- | --- |
| `make lint` | 0 issues |
| `make test` | pass |
| `make test-race` | pass |
| `bash scripts/verify-starmap-ownership.sh` | 12 passed, 0 failed |
| `bash scripts/verify-v1-architecture.sh` | 11 passed, 1 failed |
| `bash scripts/verify-dependency-direction.sh` | 6 passed, 0 failed |
| `bash scripts/verify-package-layout.sh` | passed |
| `bash scripts/verify-catalog-driven-providers.sh` | 19 passed, 0 failed |

`V01` fails because of the local `replace` directive. It passes again when
CAT10 removes the directive.

The owner ran the four CAT8 packages with the race detector on the accepted
head. Every package passed.

## Verifier

`scripts/verify-catalog-distribution.sh` with
`CATALOG_DISTRIBUTION_STARPORT_ROOT` set to the Starport worktree reports 56
passed, 12 failed, and 0 unverified of 68. Every CAT8 condition passes. The
twelve failures belong to CAT8.1, CAT9.1, and CAT9.2.
