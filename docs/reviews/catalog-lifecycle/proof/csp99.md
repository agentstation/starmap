# CSP99 Archive the completed plan

CSP99 moves the completed plan out of the active work path. The archive on Starmap main holds the plan, the task proofs, and a closure record.

## Preconditions

- 44 tasks are `done`, and CSP22.1 is `deferred(owner decision 2026-10-05)`.
- The final implementation PR for CSP24 is Starmap #253, merged as `4d452ebab`.
- On 2026-10-07 the owner set the archive scope to one Starmap PR. It moves the plan HTML, the `csp*.md` task proofs, and their small JSON records. The bulk proof stays on the pushed plan branch.

## Archive

- Destination: `docs/reviews/catalog-lifecycle/` on Starmap main.
- Contents: `starport-production-catalog-plan.html`, `proof/csp*.md`, `proof/csp23/`, `proof/csp24/`, and `CLOSURE.md`.
- Link rule: a target that main holds becomes a relative path. A target that only the plan branch holds becomes a blob URL pinned to the plan branch tip.
- The verifier inputs under `docs/plans/proof/starport-production-catalog/` stay on main, because `scripts/catalog_product_verify.py` reads them there.
- The plan branch `codex/catalog-qualification` remains with the complete proof tree.

## Indexes

- Starmap `docs/plans/README.md` lists no active plan and points at the archive.
- Starmap `docs/README.md`, the engineering specification, and the storage review point at the archived plan.
- Starport `docs/TASKS.md` records the completion, the release pair, and the archive path.

## Pull requests

- Starmap #254: the archive, the closure record, and the index updates.
- Starport #437: the task index record.

## Verification

- `python3 archive.py <plan worktree> <archive worktree> <tip> --check`: every archive link resolves.
- Starmap `git grep -n starport-production-catalog-plan`: no `docs/plans/` pointer remains.
- Starmap `make technical-writing-check`: PASS.
- Starport `bash scripts/verify-doc-links.sh`: PASS.
