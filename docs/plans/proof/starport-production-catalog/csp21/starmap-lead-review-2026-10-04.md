# CSP21 Starmap slice lead review (2026-10-04)

The Starmap implementer delivered the `rehearsal_demo` adapter and the R01 and R02 registry entries in `/private/tmp/starmap-csp21-20261004` on `csp21-registry`. The lead verified the worktree from its git state and reran every check.

## Delivered change

- `scripts/catalog_product_verify.py` adds `rehearsal_demo` with the `record` and `verifier` modes. The record mode requires an identified CI candidate, disclosed fixtures, and scene-bound cuts. It also requires the five retained outputs, the uncut source, and matching SHA-256 hashes. The verifier mode runs `scripts/verify-readme-demo.sh --manifest <manifest> --json` in the Starport root. It requires PASS with all 15 named checks.
- `scripts/catalog-product-checks.json` registers R01 (`mode: record`) and R02 (`mode: verifier`) for CSP21 against `docs/assets/starport-demo.json`.
- `scripts/test_catalog_product_verify.py` adds 13 tests. They cover the complete record, each refused condition, a missing uncut file, and an absent manifest or repository. They also cover an escaping manifest path, a passing verifier, a compact report, and verifier failures. The rest cover a missing verifier, a verifier timeout, unimplemented kinds, and the registry binding.

The adapter matches the shared contract in `csp21-record-schema.md`. An absent manifest reports UNVERIFIED, so the Starmap PR can land before the Starport rehearsal record exists.

## Checks

- `python -m unittest test_catalog_product_verify test_catalog_component_checks`: 164 tests OK. The base is 151.
- `make technical-writing-check` with the default PATH: PASS, 1975 files, 0 diagnostics.
- Rebase onto Starmap main `b6c30e0a7`: clean, commits `571b756b5` and `d6e2c7d60`, 3 files and 364 insertions. The unit tests pass again after the rebase.
- Autoreview `--gate pre-pr --mode auto` with reviewer `codex gpt-6.1-sol high`: clean, "patch is correct (0.99)".

## Publication

Draft PR agentstation/starmap#220 opened at `d6e2c7d60`. A background poll watches its CI. The merge waits for the settled checks only, because R01 and R02 report UNVERIFIED until the Starport rehearsal record exists.
