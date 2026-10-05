# CSP21 acceptance run (2026-10-04)

Starmap registry head `f52b214c0` against Starport `f0f63a45`, the PR #418 head on `csp21-readme-demo` over main `4393ee55`.
The Starport tree holds the rehearsal tooling and the committed rehearsal record `docs/proof/readme-demo/rehearsal-2026-10-04/`.
The wrapper derived the fixture endpoints, and the task exited 0 at `2026-10-05T02:42:41Z` after about one minute.
R01 and R02 read the rehearsal record and run the Starport verifier. They start no Go process and use no fixture.

## Result

CSP21 subcases: PASS 2 of 2.
The summary line reports `0 passed, 50 failed`, because a `--task` run reports the 50-case gate. The selection status is PASS.

| Subcase | Status | Scope |
| --- | --- | --- |
| `R01` | PASS | Candidate rehearsal identity and retained file hashes. |
| `R02` | PASS | Candidate rehearsal media and invalidation checks. |

## Evidence

- `report.json` holds the verifier report. The lead replaced the pyenv hashlib warnings in the R02 stderr field with a note.
- R01 bound record SHA-256 `68d76cc4be981d17acbb2df86e0563e009cf0d6ea4b74b1d9a9de9ce7fc3829e`.
- The record names candidate head `b64d3682` and CI run `37255309275`.
- R02 ran `bash scripts/verify-readme-demo.sh --manifest docs/assets/starport-demo.json --json` with exit 0.
- Neither subcase can qualify A36 through A39. That evidence needs the shipped artifact in CSP24.
