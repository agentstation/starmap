# Native catalog qualification

The catalog verifier reads recorded GitHub Actions evidence for Linux, macOS, and Windows.
Each platform requires native AMD64 and ARM64 execution with Go 1.25.12.
The workflow must pass before its results can qualify a subcase.

After the reviewed task branch passes its workflow, capture that run:

```sh
python3 scripts/native_catalog.py --run RUN_ID --output docs/plans/proof/starport-production-catalog/csp2/native-qualification
```

The destination must be absent. Preserve an earlier capture in its historical proof directory before replacing it.
The command uses `gh` to read workflow metadata and download all six native artifacts.
It retains raw test events, toolchain reports, job identities, source revision, and file digests.
Linux also requires the administrator-owned configuration read check.
The unprivileged suite can skip that specific check only when the separate privileged invocation runs and passes it.

Run the CSP2 verifier after capture:

```sh
bash scripts/verify-catalog-product.sh --starport-root "$CSP_STARPORT_WORKTREE" --task CSP2 --json
```

Each native subcase names its required tests in `scripts/catalog-product-checks.json`.
Both architectures must run and pass those tests and complete their packages.
Failed or skipped tests, missing records, changed files, or a different host invalidate the evidence.
The verifier also compares the current checkout with the tested commit, including untracked inputs.
The comparison excludes planning evidence and design documents.

Source, test, workflow, and verifier changes require another reviewed CI run.
A proof update can retain existing qualification when the tested inputs remain unchanged.
These results qualify Starmap components. Released-pair qualification retains its separate gate.
