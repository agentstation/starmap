# CSP21 Starport tooling lead review (2026-10-04)

The Starport implementer delivered the recording and verification tooling in `/private/tmp/starport-csp21-20261004` on `csp21-readme-demo`. The branch holds `78fccd5e` (manifest), `37569649` (tooling), and `ddb26c84` (CI test step) over the proof commit `b315ac43` on Starport main `d22f1e60`. The lead read every new file and reran each check.

## Delivered change

- `docs/assets/starport-demo.json` holds the scene list, the README binding, the limits, the invalidation rules, and `current_rehearsal` (D5).
- `scripts/readme-demo/capture.py` binds a CI run or a local archive, then runs the five scenes in a temporary home (D1, D3).
- The capture generates a throwaway fixture token per run and refuses to write any output that holds a credential value.
- The catalog scene receives only `PATH`, `HOME`, the three `XDG_*` names, and `STARPORT_SERVER_PORT`.
- The capture sets `STARPORT_OPENAI_INFERENCE_BASE_URL=http://127.0.0.1:<port>` without `/v1`, and the fixture serves `/v1/chat/completions`.
- `scripts/readme-demo/render.py` resolves cuts by scene boundary name and refuses a cut inside the inference interval (D4).
- `scripts/readme-demo/verify.py` evaluates the 15 named checks with the standard library only, and exits 2 on INVALID (D7, D8).
- `scripts/record-readme-demo.sh` runs the capture under `sandbox-exec` with loopback-only network access. It also owns `--review`.
- `scripts/verify-readme-demo.sh` is the verifier entry point. `scripts/test-readme-demo.py` holds 25 unit tests.
- `.github/workflows/ci.yml` runs the unit tests in the Release Contract job.
- The branch leaves `docs/assets/first-use-v1.2.0/` unchanged, so check E03 keeps its binding.

## Findings

- The product path list comes from the release snapshot inputs. The exclusion is `internal/console/dist/`, because `console/dist/` does not exist.
- The verifier fetches `refs/pull/<n>/head` only when the candidate head is absent from the checkout. That is a read-only network action.
- The local run self-review is not a human review. The lead reviews the CI candidate record before the commit.
- The lead inspected the poster frame. The text is legible, every line fits the frame, and the fixture notice shows.
- No change to `cmd/`, `internal/`, `pkg/`, or `README.md`. No AI attribution in the commit messages.

## Checks (lead rerun)

- `/usr/bin/python3 scripts/test-readme-demo.py`: 25 tests OK.
- `bash scripts/verify-doc-links.sh`: PASS.
- `scripts/readme-demo/verify.py --record .tmp/readme-demo --json` against the local-source output: PASS, 15 of 15 checks.
- `shellcheck` on both entry scripts: clean.
- The implementer reported the Starmap adapter at `f52b214c0` accepts the record shape for R01 and R02 without an adapter change.
- Autoreview `--gate pre-pr --mode auto` exited 0 without a model call. The bundle needed more than 8 passes.
- The branch carries the 400,123-line format 4 capture in `docs/proof/catalog-native/`. That data inflated the bundle.
- The lead ran `--gate manual --mode branch --base b315ac43`, which covers the three tooling commits in full.
- Sol 6.1 at high reviewed a 95,598-byte bundle in 1 pass. TruffleHog was clean. No P0 finding. Score 0.98.

## Publication

PUBLICATION_RESULT
