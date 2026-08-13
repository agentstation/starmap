# CPO4 artifact package proof

Date: 2026-08-13 UTC  
Work commit: [`6698ce5f`](https://github.com/agentstation/starmap/commit/6698ce5f)

## Fail-before

The CPO3 structural run proved these conditions:

- The deterministic artifact package still used `pkg/catalogartifact`.
- `pkg/catalogs/artifact` did not exist.
- CPO-V08 failed because the approved package was absent.
- Current code, consumer guards, generated docs, and normative docs used the flat path.

CPO2 had already removed the storage dependency and changed the value to `catalogs.Generation`. The original CPO4 fail-before combined two defects. Only the flat package path remained when CPO4 started. The durable plan now records that sequence.

## Ownership result

`pkg/catalogs/artifact` now owns the deterministic portable generation format. Its package name is `artifact`. It depends on the catalog root for `catalogs.Generation` and does not import storage.

The direct breaking move updated:

- all Starmap imports and selectors.
- the acquisition release-import boundary.
- the catalog release command and bootstrap budget.
- the pinned offline consumer and dependency guard.
- workflow contract assertions.
- current architecture, format, trust, package, and README documentation.
- the generated package documentation and generator path.

The deeper package path required two test-fixture corrections. Workflow tests now reach the repository root through three parent directories. Generation fixtures now use the sibling `catalogs/testdata` tree. These are test location changes, not production behavior changes.

The old package tree is absent. No wrapper, forwarding alias, or compatibility package remains.

## Byte contract

`TestBundleReproducibleFixtureHashes` passed before and after the move with the same pinned values:

- archive: `sha256:b8ba14cc7880adc14afb696194145fce6b3714e1afcf93f16fe5e73117e29cce`.
- detached statement: `sha256:933f717e4e7170ba92ff9e0b0c0acada8b933409612019d152cbd95f6f01c84d`.

The race suite also covered release staging, exact retry, tamper rejection, checksum checks, publisher verification, import reconciliation, and rollback.

## Structural verifier

The final CPO4 run reported `Summary: 8 passed, 5 failed`.

- CPO-V08 passed.
- CPO-V03 through V08, V10, and V13 are green.
- CPO-V01 now fails first on the missing remote package.
- CPO-V02 now fails first on the old remote package.
- The five remaining failures belong to CPO5 and CPO6.

## Verification

| Command | Result |
|---|---|
| `go test -race -count=1 ./pkg/catalogs/artifact ./acquisition ./cmd/starmap-catalog-release ./internal/bootstrap/budget` | PASS. Artifact `1.307s`, acquisition `141.647s`, release command `1.857s`, and budget `55.356s`. |
| `(cd testdata/consumers/pinned-artifact && GOWORK=off go test ./...)` | PASS in `2.977s`. |
| `make test-consumer-deps` | PASS for all six modules and dependency-policy checks. |
| `bash scripts/verify-catalog-package-ownership.sh` | Expected partial failure. 8 passed and 5 failed. |
| `go test ./...` | PASS across the main module. Root `44.903s` and acquisition `22.976s`. |
| Focused `go vet` for the affected package set | PASS. |
| `make docs-check` | PASS. |
| Stale current-authority scan for `catalogartifact` and its import path | PASS. |
| `git diff --check` | PASS. |
| SHA-256 checks for `go.mod` and `go.sum` | PASS. Both files are unchanged. |

All Go commands used normal scheduling. No command set `GOFLAGS=-p`.
