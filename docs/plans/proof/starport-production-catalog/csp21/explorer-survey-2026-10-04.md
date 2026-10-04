# CSP21 explorer survey (2026-10-04)

Verbatim report from the `csp21-explorer` agent (Opus 5.5, read-only) at Starmap `da7e88539` and Starport main `e17ea25a`. The lead design brief is in `design-brief-2026-10-04.md`.

CSP21 survey: R01 and R02 have no registry entries, so `--task CSP21` gives 2 UNVERIFIED results and the gate FAILS. Also, the "11-frame recording" is the old console tour GIF, not the current README demo.

PLAN = docs/plans/starport-production-catalog-plan.html
WT = /private/tmp/starmap-csp20-20261004 (branch csp20-registry, da7e88539)
SP = /Users/jack/src/github.com/agentstation/starport (e17ea25a)
MAP = docs/plans/proof/starport-production-catalog/acceptance-map.json

## (1) CSP21 acceptance and verification

- Paths: SP · docs/assets, scripts/record-readme-demo.sh (new), README media and transcript. Depends on CSP17 and CSP20. "Select the recording release first." (PLAN:847)
- Steps (PLAN:849): create recording and verification scripts with a candidate-artifact manifest. Rehearse installation, catalog discovery, setup, and the streamed response sequence. Define invalidation rules and prepare final capture after publication in CSP24. Verify the CSP21 discovery audit criteria.
- Acceptance (PLAN:851): "R01 and R02 candidate rehearsal checks pass. Duration remains an editorial target. Final A36 through A39 evidence requires the shipped artifact in CSP24."
- Fail-before (PLAN:853): compare the existing 11-frame recording against the required scene checklist.
- Verification commands (PLAN:855):
  bash scripts/record-readme-demo.sh --rehearse --output .tmp/readme-demo   # SP
  bash scripts/verify-readme-demo.sh --manifest docs/assets/starport-demo.json   # SP
  bash scripts/verify-doc-links.sh   # SP
  bash scripts/verify-catalog-product.sh --starport-root "$CSP_STARPORT_WORKTREE" --task CSP21   # SM
- Discovery audit criteria (discovery-readiness-2026-09-07/contracts.md:42-44): "Record discovery without credentials, an appropriate setup action, and caller-ready streaming from the same declared release pair."
- The plan does not define the candidate-artifact manifest schema or the invalidation rules. It only names them. Related text:
  - R02 says: "Product or installer changes invalidate this rehearsal."
  - CSP24 acceptance (PLAN:888) says: "Changed installed behavior invalidates stale media and affected performance evidence."
- R01 and R02 (WT MAP:1820-1828, also in the requirements MAP:1794-1800):
  - R01: "Recording tooling produces an identified candidate rehearsal with disclosed fixtures, cuts, artifact hashes, and an uncut source. It cannot qualify final release cases."
  - R02: "Candidate media passes dimension, size, link, poster, transcript, and human readability checks. Product or installer changes invalidate this rehearsal."
  - task_checks CSP21 = [R01, R02] (WT MAP:904). task_notes: "Candidate rehearsal and recording tooling only. Final capture after release belongs to CSP24."
  - R01 and R02 are outside the 50-case count (PLAN:302).
- A36 to A39 (ENGINEERING_SPEC.md:3762-3765):
  - A36: installation, then catalog discovery, then provider setup, then a successful streamed answer, in that order.
  - A37: the capture identifies the release, catalog, provider path, and timing cuts. No secret or fabricated result appears.
  - A38: GIF < 10 MiB, readable at 900 px, human pacing review.
  - A39: README links, static poster, alt text, transcript, reduced motion.
  - Subcases (MAP:330-351): 2 + 5 + 4 + 3 = 14. All are owned by CSP24 (MAP:36-39), and all are in requires_published_assets.
  - DR08 also links CSP21 to A36 (MAP:2424-2435).

## (2) Current recording inventory

- The "11-frame" recording is docs/assets/2026-08-29_starport-console.gif: 2,208,444 B, 1440x777, 11 frames, 17.1 s. Source: REPOSITORY_FINDINGS.md:504, and my GIF parse agrees. It was added in f1cd4a54 (#307). Memory says it was made with the Chrome gif_creator through `starport dev --no-open` and a launch link. The README still links it as the "earlier console tour" (README.md:17).
- The current README demo is docs/assets/first-use-v1.2.0/ (README.md:12-16). It has 9 files:
  - first-use.gif: 516,182 B, 1280x800, 15 frames, 38.16 s
  - first-use-uncut.gif: 512,086 B, 15 frames, 64.45 s
  - poster.png: 1280x800, 71,362 B
  - events.json: 41 events, release v1.2.0
  - render.json: 2 edits at events 5 and 8. The 40.28 s gap becomes 8 s. Effective font at 900 px is 18.28.
  - capture.py, render.py, replay.py, TRANSCRIPT.md
  - Commits: tools in 7db1cf85 (#369), media in 55629266 (#370), both 2026-09-08.
- How it is produced: a custom pipeline, not vhs or asciinema.
  - capture.py runs `gh release download v1.2.0` for darwin_arm64 (capture.py:18-19,63). It runs `starport models show openai/gpt-4o-mini`. It prompts for the OpenAI key with getpass (capture.py:89). It runs `starport dev --no-open` on port 19325 and streams /api/v1/chat/completions.
  - render.py uses PIL and /System/Library/Fonts/Menlo.ttc. The edits are hard-coded at event indexes 5 and 8 (render.py:60).
- How it is verified: registry E03 (kind reviewed_demo, WT scripts/catalog-product-checks.json:502). E03 binds 10 inputs by sha256, including README.md, and requires 14 observations. The adapter (WT catalog_product_verify.py:448-502) requires:
  - capture verdict PASS for the review release
  - response 200 and a final [DONE] event with content
  - catalog_environment_has_provider_key is False
  - no persistent selectors or leftover home files, and clean shutdown
  - width >= 1280 and font >= 14 at 900 px
  - no edit that crosses the inference interval
  - GIF hashes and dimensions that match render.json
  - first-use.gif < 10 MiB
  - The file names first-use.gif and first-use-uncut.gif are hard-coded.
- E02 (reviewed_first_use, :463) binds 11 inputs.
- cmd/starport/readme_test.go does not bind the media or a digest. It tests section order and claims only (lines 150-545).
- Neither the Makefile nor scripts/ has a recording or media target.
- These do not exist yet: scripts/record-readme-demo.sh, scripts/verify-readme-demo.sh, docs/assets/starport-demo.json.
- Freshness: I hashed the csp20-registry proofs against SP e17ea25a.
  - E03: README.md is stale. The 9 asset inputs match.
  - E02: 9 of 11 inputs are stale.
  - I did not check whether CSP20 renews these elsewhere.

## (3) Registry gaps

- 305 checks are registered. R01, R02, A35-A39 subcases (except the 3 A35 candidate subcases), and CSP21 have no entries.
- `--task CSP21` selects roster task_checks["CSP21"] = [R01, R02] (verifier:68-74).
- Each one returns "No behavior check is registered." (:277-278), so the gate is FAIL and the exit code is 1 (:868).
- CSP21 is not in the qualification-required tuple (:908). After both checks pass, the gate can PASS.
- validate_registry already allows rehearsal IDs (:41, :84-87).
- An unknown kind returns "This evidence adapter has not been implemented" (:366-367). R01 and R02 need a new adapter, or reviewed_demo must be generalized. It currently assumes a real release capture and fixed file names.

## (4) Local tools

- Installed: vhs (/opt/homebrew/bin/vhs), ffmpeg.
- Missing: asciinema, agg, ttyd.
- Also present: gh, jq, PIL 11.1.0, Menlo.ttc.
- I believe vhs needs ttyd at runtime, so vhs probably cannot run here. I did not run it to confirm.

## (5) Risks for a rehearsal with no credentials and a fake upstream

- capture.py cannot run as is. It downloads a published release over the network with gh, prompts for a key interactively, and has v1.2.0, darwin_arm64, and the model ID hard-coded. The rehearsal needs a candidate archive source plus its hash in the manifest.
- I did not check how Starport routes the openai provider to a fake upstream. Find the base-URL override before you design the script.
- Fake inference output must be disclosed as a fixture, as R01 requires. It cannot satisfy these final checks:
  - A36.real_streamed_result
  - A37.no_secret_or_fabrication
  - A38.inference_real_speed
  - E03 real_streamed_answer
  The reviewed_demo adapter cannot tell fake output from real output (it checks only status 200, [DONE], and content). The guard must therefore be manifest provenance, or a new adapter must refuse fixture captures for A36-A39.
- render.py edit indexes 5 and 8 depend on the event count. A different scene or event sequence can cut the wrong gap or trigger the inference-interval refusal.
- Rendering depends on the macOS Menlo font path and the fixed port 19325. It does not work on Linux CI.
- The discovery criterion requires one "declared release pair" (Starmap + Starport). The rehearsal runs before CSP24 publication, so the manifest must name candidate identities and say how they are invalidated.
- New media or a README change will invalidate the E03 digests again, and E02 when its inputs change. Plan the renewals.

Not checked: vhs behavior at runtime, Starport provider base-URL configuration, the CSP20 starport worktree, and the full TRANSCRIPT text.
