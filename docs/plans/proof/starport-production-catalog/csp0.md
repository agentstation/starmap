# CSP0: Baseline and red verifier

Status: done. Date: 2026-09-05. Work remains local under the active commit policy.

The user activated the whole-plan goal. The plan and both indexes now identify the same execution owner.
The [activation resolution](activation-2026-09-05/RESOLUTION.md) records the seven audit corrections and selected engineering defaults.
The [profile roster](activation-2026-09-05/profiles.json) identifies T1 through T7 and each remaining qualification input.

The initial verifier command failed with exit 127 because the script did not exist.
The new verifier reports 50 primary cases and 324 required subcases.
The candidate roster includes 305 subcases, including eight local prerequisites from publication-dependent parents.

The current baseline is `Summary: 0 passed, 50 failed`, with 50 UNVERIFIED cases.
This result establishes the fail-before baseline. It grants no product acceptance credit.
Owning tasks must register real behavior checks as implementation proceeds.
Real service, artifact, and reviewed-measurement adapters remain unimplemented and cannot qualify a release.

## Verification

| Command or check | Result |
| --- | --- |
| `python3 scripts/test_catalog_product_verify.py` | 14 tests passed, including real Go test execution and missing-match rejection. |
| `bash scripts/verify-catalog-product.sh --all --json` | Exit 1. All 50 cases remain UNVERIFIED. |
| Candidate, task, and A47 selection | Expected nonzero reports with the selected subcase counts. |
| Unknown A99 selection | Exit 2 with an explicit unknown-case error. |
| Activation document verifier | Task order, dependency, references, historical evidence, and roster checks passed. |
| GFM acceptance rendering | 50 three-cell rows. The injected blank-line fixture reproduces 43 rows and fails the expected count. |
| Targeted writing checks | Eight Starmap files passed with zero diagnostics before this closeout record. |
| Repository writing check | Three existing diagnostics remain in historical command-output files. No new diagnostic appeared. |
| Whitespace | Both worktrees passed `git diff --check`. |

The [test result](activation-2026-09-05/verifier-tests.json) retains the regression output.
The [document result](activation-2026-09-05/document-check.json) records structural checks and hashes.
The [render result](activation-2026-09-05/markdown-check.json) records the positive and negative GFM checks.
The [writing result](activation-2026-09-05/repository-writing.json) preserves the existing repository failure.

The checks used Python 3.14.7 and Go 1.27.0 on macOS arm64.
Those runner versions do not replace either product's declared release toolchain.
No product source, release, provider resource, or customer deployment changed during CSP0.

Next task: CSP0.1 improves current documentation access and readability.
