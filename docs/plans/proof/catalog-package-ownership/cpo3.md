# CPO3 storage package proof

Date: 2026-08-13 UTC  
Work commit: [`55de2a75`](https://github.com/agentstation/starmap/commit/55de2a75)

## Fail-before

The CPO2 structural run proved these conditions:

- Durable storage still used `pkg/catalogstore`.
- `pkg/catalogs/storage` did not exist.
- CPO-V07 failed because the approved package and adapters were absent.
- Current code, verification scripts, workflows, consumer fixtures, and docs used the flat path.

## Ownership result

`pkg/catalogs/storage` now owns the three-method `Store` seam and the memory, filesystem, and conditional object adapters. `pkg/catalogs/storage/s3` owns the caller-configured S3-compatible `ObjectBackend` adapter.

Every storage method accepts or returns `catalogs.Generation`. The move preserves compare-and-swap, idempotency, retained generations, defensive copying, corruption rejection, cancellation, reopen, rollback, and fault-preservation behavior.

The direct breaking move also updated:

- all Starmap imports and selectors.
- all six nested external consumer modules.
- CLI, server, bootstrap, workspace, and reactive remote composition.
- pure-Go and consumer dependency guards.
- hosted workflow assertions.
- current architecture, storage, Docker, remote, API, server, and package docs.
- the repository `AGENTS.md` package contract.

The old package tree is absent. No wrapper, forwarding alias, or compatibility package remains.

## Verifier correction

The plan defines reviews and archived evidence as historical. The structural verifier incorrectly scanned `docs/STARMAP_ARCHITECTURE_CONTROL_PLANE.md` as current normative documentation. The verifier now excludes only that exact archived control-plane file, in addition to the existing review, proof, and plan directories.

The regression fixture now proves both documentation allowlists. It also proves that the verifier still rejects a stale path in current source. `scripts/test-catalog-package-ownership-verifier.sh` passed.

## Structural verifier

The final CPO3 run reported `Summary: 7 passed, 6 failed`.

- CPO-V07 passed.
- CPO-V03 through V06, V10, and V13 remained green.
- CPO-V01 now fails first on the missing artifact package.
- CPO-V02 now fails first on the old artifact package. It no longer reports the storage tree.
- The remaining failures belong to CPO4 through CPO6.

CPO-V01 and CPO-V02 are aggregate conditions for all target and removed package trees. The original CPO3 acceptance text required them too early. The plan now tests the CPO3 contribution and requires both aggregates to pass after CPO5.

## Verification

| Command | Result |
|---|---|
| `go test -race -count=1 ./pkg/catalogs/storage/...` | PASS. Storage `1.471s` and S3 `1.354s`. |
| `make test-consumer-deps` | PASS for all six modules and dependency-policy checks. |
| `make test-pure-go` | PASS for the library, pinned artifact, stores including S3, server, remote, and CLI. |
| `bash scripts/verify-catalog-package-ownership.sh` | Expected partial failure. 7 passed and 6 failed. |
| `bash scripts/test-catalog-package-ownership-verifier.sh` | PASS. |
| `go test ./...` | PASS across the main module. |
| Broader changed-seam race matrix | PASS. Root `280.177s`, workspace `4.943s`, internal server `109.133s`, public server `25.921s`, and remote `24.662s`. |
| Ten-repetition storage race matrix | PASS. Storage `4.019s` and S3 `1.858s`. |
| Focused `go vet` for the affected package set | PASS. |
| `make docs-check` | PASS. |
| `git diff --check` and shell syntax checks | PASS. |
| SHA-256 check for `go.mod` | PASS. The baseline digest is unchanged. |

All Go commands used normal scheduling. No command set `GOFLAGS=-p`.
