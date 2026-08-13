# CPO1 evidence and projection contract proof

Date: 2026-08-13 UTC  
Work commit: [`a3df5076`](https://github.com/agentstation/starmap/commit/a3df5076)

## Fail-before

The CPO0 baseline proved these conditions:

- `pkg/catalogmeta` existed as one mixed public package.
- `pkg/catalogs/evidence` and `pkg/catalogs/projection` did not exist.
- CPO-V05 and CPO-V06 failed because the approved leaves were absent.
- Current Go source and generated API documents imported or named `catalogmeta`.
- `pkg/sources` exposed a `ResourceType` alias with legacy compatibility text.

## Ownership result

`pkg/catalogs/evidence` now owns source identity, observation facts, resource identity, review candidates, and stable review ordering. It imports only `slices` and `strings` from the standard library.

`pkg/catalogs/projection` now owns the post-commit workspace status and result. It has no imports and defines no type alias.

The change also did this work:

- Deleted `pkg/catalogmeta` without a wrapper or forwarding alias.
- Updated every current Starmap Go caller to an approved owner.
- Kept source-owned aliases for source IDs, revisions, and observation facts.
- Removed the `sources.ResourceType` alias and its resource constants.
- Updated reconciliation callers to use `evidence.ResourceType` directly.
- Kept sync-owned projection spellings as aliases to the neutral projection contract.
- Regenerated the affected public API documents.
- Updated the architecture tree with the two new catalog leaves.

Direct contract tests preserve source ID order and copy isolation. They also preserve resource string values, review-candidate ordering, JSON and YAML field tags, projection status values, and independent result fields. Existing catalog, source, sync, provenance, acquisition, and root tests preserve the complete wire behavior.

## Structural verifier

`bash scripts/verify-catalog-package-ownership.sh` reported all 13 conditions. The result was `Summary: 4 passed, 9 failed`.

- CPO-V05 passed.
- CPO-V06 passed.
- CPO-V10 remained green for all six nested consumer modules.
- CPO-V13 remained green with the baseline `go.mod` checksum.
- CPO-V02 now fails first on `pkg/catalogstore`. It no longer reports `pkg/catalogmeta`.
- The remaining failures belong to CPO2 through CPO6.

CPO-V02 is an aggregate condition for four removed package trees. The original CPO1 acceptance text incorrectly required the aggregate to pass before later tasks removed the other three trees. The plan now requires removal of the `catalogmeta` failure at CPO1. CPO-V02 becomes fully green after CPO5.

## Verification

| Command | Result |
|---|---|
| `go test -race -count=1 ./pkg/catalogs/evidence ./pkg/catalogs/projection ./pkg/sources ./pkg/sync ./pkg/provenance ./internal/catalog/authority ./internal/catalog/reconciler ./acquisition .` | PASS. The long packages were confirmed explicitly: acquisition `127.166s` and root `276.479s`. |
| `bash scripts/verify-catalog-package-ownership.sh` | Expected partial failure. 4 passed and 9 failed. |
| `go test ./...` | PASS across the main module. |
| `go vet` for the exact affected package set | PASS. |
| `git diff --check` | PASS. |
| SHA-256 check for `go.mod` | PASS. The digest remains `563a7e3779efe55001ab43d0a42c53154a14b470bbeecb59464972c48c1d493c`. |

All Go commands used normal scheduling. No command set `GOFLAGS=-p`.
