# CPO0 baseline and red verifier proof

Date: 2026-08-13 UTC  
Work commit: [`1136ce63`](https://github.com/agentstation/starmap/commit/1136ce63e9120d868ff8d3ad76a6e7c5edaf7f94)

## Repository baselines

| Repository | Pinned commit | Default branch | Latest release or tag | Open pull requests |
|---|---|---|---|---:|
| Starmap | `42f1b4c6763084cc0ae7b2ced413de4d765cd9ef` | `main` | GitHub release `v0.4.1` | 0 |
| Starport | `4fbde1eaf3a624a57112a54a872ed2e15de139dd` | `main` | GitHub release `v1.0.3` | 0 |
| Modelwiki | `122d27007678eeea32e927678f853db48a3c30b5` | `master` | Git tag `v0.0.3`. No GitHub release. | 0 |

Plan pull request [#77](https://github.com/agentstation/starmap/pull/77) merged to Starmap `main` at the pinned Starmap commit. Its Verification Gate, Security and Reliability, and Action Pin Provenance checks passed. Modelwiki is an external no-action consumer for this campaign. The plan does not change its branch.

The baseline dependency files have these SHA-256 digests:

- `go.mod`: `563a7e3779efe55001ab43d0a42c53154a14b470bbeecb59464972c48c1d493c`
- `go.sum`: `90c7f228a063197428235b547c828c3d4121725cb2331df4176dbb7a23322211`

## Import and graph inventory

The Starmap package graph has 31 unique direct first-party edges into the four packages that this plan removes. The edges cover the root client, acquisition, bootstrap, catalog authority and reconciliation, CLI, server handlers, artifact and remote distribution, provenance, sources, sync, and storage adapters.

The source scan found 125 affected Starmap import statements in 95 Go or module files. It also found 37 files that refer to `catalogstore.Generation`. These source counts include tests and the six nested consumer modules.

Starport has eight affected import statements in five files:

- `internal/catalog/remote_runtime.go`
- `internal/catalog/remote_runtime_test.go`
- `internal/catalog/generation_store.go`
- `internal/app/remote_catalog_test.go`
- `internal/diagnosis/service.go`

Modelwiki has no import of `catalogmeta`, `catalogstore`, `catalogartifact`, or `catalogremote`. Its existing Starmap use is outside this breaking package move, so CPO8 records it as no-action after the exact-module scan.

## Public API and format inventory

The move starts with these public contracts:

| Package | Exported contract groups |
|---|---|
| `pkg/catalogmeta` | `SourceID`, `ResourceType`, observation revision/status/completeness/issues/counts, review candidates and ordering, projection status and result |
| `pkg/catalogstore` | `Generation`, `Store`, memory/filesystem/object stores, object backend contracts, and catalog/source-observation codecs |
| `pkg/catalogstore/s3` | caller-owned S3 `Backend` and `Config` |
| `pkg/catalogartifact` | format constants, bundle build/open/inspect, release staging and verification, descriptors, digests, attestations, and publisher verification |
| `pkg/catalogremote` | stable paths and media types, manifest functions, verified client, publication stream, events, and stream records |

The wire and artifact values are:

- catalog schema version: `5`
- generation manifest version: `2`
- bootstrap manifest version: `2`
- artifact format version: `1`
- catalog payload: `application/vnd.agentstation.starmap.catalog+json`
- artifact archive: `application/vnd.agentstation.starmap.catalog-artifact.v1+tar+gzip`
- remote manifest: `application/vnd.agentstation.starmap.catalog-manifest+json`
- event stream: `text/event-stream`

The focused packages expose these top-level test function counts before the move:

| Package | Test functions |
|---|---:|
| `pkg/catalogs` | 149 |
| `pkg/catalogartifact` | 14 |
| `pkg/catalogmeta` | 0 |
| `pkg/catalogremote` | 14 |
| `pkg/catalogstore` | 19 |
| `pkg/catalogstore/s3` | 6 |

The baseline includes the named regression contracts `TestBundleReproducibleFixtureHashes`, `TestRemoteCatalogFetchValidatesManifestPayloadChecksumAndCompatibility`, `TestEventStreamParsesCommentsAndStablePublication`, and `TestCatalogStoreConformance`.

## Fail-before result

`bash scripts/verify-catalog-package-ownership.sh` exited nonzero as required. It reported all 13 conditions:

| Condition | Baseline result |
|---|---|
| CPO-V01 | FAIL |
| CPO-V02 | FAIL |
| CPO-V03 | FAIL |
| CPO-V04 | FAIL |
| CPO-V05 | FAIL |
| CPO-V06 | FAIL |
| CPO-V07 | FAIL |
| CPO-V08 | FAIL |
| CPO-V09 | FAIL |
| CPO-V10 | PASS |
| CPO-V11 | FAIL |
| CPO-V12 | FAIL |
| CPO-V13 | PASS |

Exact summary: `Summary: 2 passed, 11 failed`.

This is the expected red state. The approved child packages do not exist, and all removed package trees still exist. The old owners still hold generation and codec contracts. Current authority still imports the old paths, and the migration guide does not exist. The six external consumer modules compile. The package graph resolves without a `go.mod` change.

## Verification

| Command | Result |
|---|---|
| `bash scripts/verify-catalog-package-ownership.sh` | Expected failure; 2 passed and 11 failed |
| `bash scripts/test-catalog-package-ownership-verifier.sh` | PASS; incomplete reports, historical allowlist behavior, stale-current-path rejection, and recursion guard verified |
| `go test -count=1 ./pkg/catalogs ./pkg/catalogartifact ./pkg/catalogmeta ./pkg/catalogremote ./pkg/catalogstore ./pkg/catalogstore/s3` | PASS; `catalogmeta` correctly reported no test files |
| `git diff --check` | PASS |

No production Go file changed in CPO0. `scripts/verify.sh` runs the verifier. The verifier reports CPO-V01 through CPO-V13 independently and uses a portable SHA-256 command. It does not invoke a full repository gate.
