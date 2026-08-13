# CPO2 generation and codec ownership proof

Date: 2026-08-13 UTC  
Work commit: [`51b9ebfd`](https://github.com/agentstation/starmap/commit/51b9ebfd)

## Fail-before

The CPO0 and CPO1 structural runs proved these conditions:

- `Generation`, `Copy`, and `Validate` belonged to `pkg/catalogstore`.
- The storage package owned both payload decoders.
- The storage encoder was a delegate to `catalogs.EncodeCatalogPayload`.
- CPO-V03 and CPO-V04 failed.
- Artifact and remote imported storage to exchange the generation value.

## Ownership result

`pkg/catalogs` now owns the immutable `Generation` value, its defensive copy, and manifest-to-payload validation. It also owns all three canonical codec functions:

- `EncodeCatalogPayload`
- `DecodeCatalogPayload`
- `DecodeSourceObservationPayload`

The storage codec delegate no longer exists. The direct type change reaches every Starmap signature, implementation, test, and nested consumer module. Artifact and remote now use `catalogs.Generation` and catalog codecs without a storage import.

The schema-resilience tests moved to the codec owner. They continue to prove bounded decoding, strict envelopes, schema rejection, partial diagnostic quarantine, and the separate source-observation decode. New generation tests prove nested copy isolation and payload binding.

The change does not alter schema numbers, media types, payload bytes, manifest bytes, error classes, or module dependencies. The `go.mod` SHA-256 digest remains `563a7e3779efe55001ab43d0a42c53154a14b470bbeecb59464972c48c1d493c`.

## Structural verifier

`bash scripts/verify-catalog-package-ownership.sh` reported all 13 conditions. The result was `Summary: 6 passed, 7 failed`.

- CPO-V03 passed.
- CPO-V04 passed.
- CPO-V05, V06, V10, and V13 remained green.
- The seven remaining failures belong to the package moves and final documentation work in CPO3 through CPO6.

## Verification

| Command | Result |
|---|---|
| `go test -race -count=1 ./pkg/catalogs . ./acquisition ./internal/bootstrap/...` | PASS. Catalogs `62.998s`, root `283.432s`, acquisition `116.224s`, bootstrap `60.602s`, budget `43.206s`, and manifest `2.479s`. |
| `make test-consumer-deps` | PASS for all six modules and their dependency-policy checks. |
| `bash scripts/verify-catalog-package-ownership.sh` | Expected partial failure. 6 passed and 7 failed. |
| Focused `go vet` for the exact affected package set | PASS. |
| `make docs-check` | PASS after regeneration of the affected API documents. |
| `git diff --check` | PASS. |
| SHA-256 check for `go.mod` | PASS. |

All Go commands used normal scheduling. No command set `GOFLAGS=-p`.
