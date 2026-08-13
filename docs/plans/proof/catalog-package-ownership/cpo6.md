# CPO6 Starmap integration and pull request proof

Date: 2026-08-13 UTC  
Candidate commit: [`22a18951`](https://github.com/agentstation/starmap/commit/22a1895108d704bd0bca49052124b45781883a5f)  
Merge commit: [`2daa219d`](https://github.com/agentstation/starmap/commit/2daa219d369a98d0c6826146125e1b1dc752b75c)

## Fail-before

The CPO5 structural run reported `Summary: 12 passed, 1 failed`. CPO-V11 failed because `docs/MIGRATING_TO_V0.5.md` did not exist. Current code already used the approved package tree, but operators and external consumers did not have a complete breaking-change map.

The first pre-PR credential scan also found a synthetic credential-bearing URI in a deleted baseline test line. TruffleHog classified the URI as unknown because its host was not a valid verification target. The implementation branch did not add a credential.

Test-only pull request [#78](https://github.com/agentstation/starmap/pull/78) replaced the raw URI with standard `net/url` construction. The test still proves that `NewClient` rejects credential-bearing publisher URLs. Its package test, race test, lint, and diff checks passed. Its three hosted checks passed, and it merged to `main` at `6c4f6193`. The unpublished implementation branch rebased cleanly onto that commit. The next credential scan was clean.

## Integration result

The integration work added `docs/MIGRATING_TO_V0.5.md` with every removed import path and selector mapping. It updated the changelog, architecture authority, package documentation, generated API documentation, examples, scripts, and all six external consumer modules.

The final review also removed package-name stutter at the projection owner. `pkg/catalogs/projection` now exports `Status`, `StatusApplied`, `StatusPendingRepair`, `IssueWorkspaceFailed`, and `Result`. The higher `pkg/sync` API retains its concept-specific `ProjectionStatus` and `ProjectionResult` names.

The repository does not contain the old package trees. No wrapper, forwarding alias, deprecated symbol, or runtime compatibility path remains. The work did not change `go.mod` or `go.sum`.

The final structural run reported `Summary: 13 passed, 0 failed`. CPO-V01 through CPO-V13 passed.

## Verification

| Command or gate | Result |
|---|---|
| `make verify` | PASS. Ordinary tests, all six external consumers, pure-Go checks, file-size checks, package-layout tests, CPO-V01 through CPO-V13, full race tests, vet, performance, lint, coverage, documentation, build, catalog validation, and CLI smoke passed. |
| Full race suite inside `make verify` | PASS with normal scheduling. Root `269.195s`, acquisition `119.140s`, bootstrap manifest `92.045s`, internal server `101.529s`, modelsdev `71.606s`, catalogs `66.337s`, remote protocol `1.304s`, and reactive remote `17.789s`. |
| Catalog performance gate | PASS in three runs at `8.764`, `8.923`, and `9.067 ns/op`, with `0 B/op` and `0 allocs/op`. |
| Pinned `golangci-lint` 2.12.2 | PASS with zero issues. |
| Coverage gates | PASS. `pkg/catalogs` reported `73.4%`; every other named package met its approved threshold. |
| `make release-check` | PASS with a clean tree, tests, vet, CLI generation, and valid GoReleaser configuration. |
| `make release-snapshot` | PASS. Six platform archives, six SBOMs, checksums, and the Homebrew cask were generated. Optional macOS signing stayed disabled because credentials were absent. |
| `make catalog-generation-check` | PASS. |
| `make embedded-catalog-budget-check` | PASS for 14 providers and 590 embedded models. The payload measured 8,110,865 uncompressed bytes and 298,892 compressed bytes. |
| `make verify-action-pins` | PASS for all eight pinned action references. |
| Strict technical-writing lint | PASS for the plan, proof set, and migration guide. |
| Pre-PR autoreview | PASS. The credential scan was clean. Two review chunks reported zero findings and stored a clean attestation. Direct verified TLS was used because the local interception proxy did not provide a trusted certificate to the reviewer. TLS verification was not disabled. |
| GitHub pull request [#79](https://github.com/agentstation/starmap/pull/79) | PASS and merged. Verification Gate, Security and Reliability, and Action Pin Provenance passed at candidate `22a18951`. |

The hosted Verification Gate ran from `2026-08-13T03:30:38Z` through `2026-08-13T03:52:47Z`. Security and Reliability finished successfully at `2026-08-13T03:32:33Z`. Action Pin Provenance finished successfully at `2026-08-13T03:30:45Z`. Pull request #79 merged to `main` at `2026-08-13T03:53:05Z`.

CPO-V14 passes for the Starmap implementation merge. CPO7 now owns the exact Starmap v0.5.0 release and distribution verification.
