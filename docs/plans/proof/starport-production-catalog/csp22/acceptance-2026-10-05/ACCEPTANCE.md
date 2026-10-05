# CSP22 candidate gate (2026-10-05)

## Result

- Command: `bash scripts/verify-catalog-product.sh --gate candidate --starport-root <Starport root> --json`.
- The run selected 312 subcases: 308 PASS, 4 UNVERIFIED, 0 FAIL.
- The fail-before run had 265 PASS, 45 UNVERIFIED, and 2 FAIL.
- The four UNVERIFIED subcases are `A50.percentiles_and_load`, `A50.stream_timing_memory`, `A50.allocations_cpu_gc`, and `A50.real_recipe_matrix`.
- The owner assigned those four subcases to CSP22.1 on 2026-10-05. They need a dedicated runner.
- The verifier reports gate status FAIL for each run that needs release qualification. CSP24 owns that qualification.
- The case summary shows 42 passed. Cases A06, A29, and A35 through A39 need published assets, and CSP24 owns them.

## Candidate pair

- Starmap: `f3dc61015efa3eb99588b13bd62bc8c7bdf53977`. Pull request #231 adds proof files to main `2fba452ab`.
- Starport: `17776031a5b5e1ef5b7928c12184a20918c331de`. Pull request #419 adds proof files to main `3234997a`.
- Each proof commit changes only evidence paths.

## Evidence

- `candidate-gate.json` holds the status of each case and subcase. It omits the raw test output.
- The full log has 509,365,218 bytes and the SHA-256 digest `84f0a316b5c82f4682627ea457b60820fa2f1116dc2826d3a13e6c8fd60b3fc7`. The log stays on the lead host.
- `environment.json` holds the Go settings of the run.
- The run used the real PostgreSQL, Valkey, MySQL, and object store fixtures and the recipe image `starport-storage-recipe:csp22-c5246216`.
- Pull request #423 changed only tests and documents after `c5246216`, so the image holds the current product source.
- The hosted publication check returned PASS with five attested subjects.
- The receipt checksum is `sha256:3fdb7007d352fb2f07320e48a9847224890157a00e13cbb0b42ef3f86990887f`.

## Limits

- A GitHub Actions outage cancelled the CI runs of both proof pull requests.
- The lead reruns the cancelled jobs after the service recovers.
- A later scheduled publication can change Starmap main. The captures then bind to `2fba452ab` only.
