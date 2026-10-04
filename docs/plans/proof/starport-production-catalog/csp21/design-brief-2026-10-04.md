# CSP21 design brief (2026-10-04)

Task: prepare the final release demonstration. Scope: recording and verification tooling, a candidate-artifact manifest, one disclosed rehearsal, and the invalidation rules. The final capture against shipped assets belongs to CSP24. The explorer survey is in `explorer-survey-2026-10-04.md`.

## Current state

- The README links `docs/assets/first-use-v1.2.0/first-use.gif`, a real v1.2.0 capture with a real OpenAI answer.
- Registry check E03 binds that directory and the README by digest. Its adapter accepts only a real release capture with the fixed names `first-use.gif` and `first-use-uncut.gif`.
- The capture pipeline lives inside the asset directory. The `capture.py` script downloads the published v1.2.0 archive with `gh`. It reads a provider key at a hidden prompt. It streams one request through `starport dev --no-open` on port 19325. The `render.py` script draws frames with PIL and Menlo. Its cut points are the fixed event indexes 5 and 8.
- The "11-frame recording" in the plan is `docs/assets/2026-08-29_starport-console.gif`, a console tour. It shows no installation, catalog, setup, or streamed answer scene.
- Starmap registers no check for R01, R02, or A36 to A39. A `--task CSP21` run reports two UNVERIFIED results. The gate fails.
- The CI job `candidate-install` downloads the `starport-release-snapshot` artifact of its own run and installs the archive on five native runners. The format 4 capture in `docs/proof/catalog-native/` binds that evidence to run 37222309299, pull request 414, and head `aeae7664`. The capture is not yet committed.

## Decisions

### D1. Candidate artifact

- The rehearsal installs the release snapshot archive of one pull request CI run. The script downloads it with `gh run download <run> --name starport-release-snapshot`.
- The record binds the run ID, the pull request, the head commit, and the snapshot version. It also binds the archive name, the archive SHA-256, and the binary SHA-256.
- A `--candidate-archive <path>` option accepts a local archive for development and records `source: local`. R01 refuses a local source.
- CSP24 adds `--release <tag>` with the published checksum file as the source.

### D2. Fixture upstream

- The rehearsal runs with no provider credential.
- The record script starts a local OpenAI-compatible stream server from `scripts/readme-demo/fixture_upstream.py`.
- The server answers every chat completion with the streamed text `Rehearsal fixture: Hello from Starport!` and the model name `gpt-4o-mini-rehearsal-fixture`.
- The provider credential is a generated throwaway token. The script discloses it as a fixture.
- The implementer verifies the route that `starport dev` honors for the provider base URL. Candidates are `providers.openai.base_url` in the scratch configuration and an endpoint binding. The record names the exact mechanism.
- If no route exists without a product change, stop and report before any change to `cmd/` or `internal/`.

### D3. Scene checklist

The capture emits named scene markers in this order: `install`, `catalog`, `setup`, `answer`, `next`.

- `install`: verify the archive digest and run `starport --version`.
- `catalog`: run `starport models show openai/gpt-4o-mini --json` with no provider key in the environment.
- `setup`: show the provider credential role and the gateway key role as separate hidden values, then start `starport dev --no-open`.
- `answer`: stream one request to `/api/v1/chat/completions` and end at the `[DONE]` event. The rehearsal shows the fixture notice on screen.
- `next`: show the client base URLs and the persistent setup path.

### D4. Tooling location

- Move the pipeline into `scripts/readme-demo/` as `capture.py`, `render.py`, `verify.py`, and `fixture_upstream.py`.
- `scripts/record-readme-demo.sh` and `scripts/verify-readme-demo.sh` are the entry points.
- Cuts reference scene boundaries by name, not event indexes.
- The renderer keeps the E03 rule that no cut intersects the inference interval.
- The font path is an option with the macOS Menlo default. Linux CI runs the verifier on committed records only, never the renderer.
- The `docs/assets/first-use-v1.2.0/` directory stays unchanged, because E03 binds it.

### D5. Manifest

`docs/assets/starport-demo.json` is the static contract. It holds:

- `schema_version`.
- The scene checklist from D3.
- The README binding rules for the final media:
  - the GIF link, the poster link, and the alt text
  - the transcript link, the uncut link, and the reduced-motion statement
- The limits:
  - the GIF stays under 10 MiB
  - the width is at least 1280
  - the effective font at 900 px is at least 14
  - the duration target of 45 seconds is an editorial target
- The invalidation rule from D8 with its product path list.
- `current_rehearsal`, which points at one record directory.

### D6. Record

Each capture writes a record directory with `record.json`, `events.json`, `render.json`, `first-use.gif`, `first-use-uncut.gif`, `poster.png`, and `TRANSCRIPT.md`. The `record.json` file carries:

- `kind` (`rehearsal` or `release`) and `qualifies_release_cases`, which is `false` for a rehearsal.
- The candidate identity from D1 and the fixture list from D2.
- The scene markers with event indexes and seconds, and the inference interval.
- The cuts, the output hashes, and the uncut source hash.
- The discovery audit fields from D11.
- A `human_review` block with a reviewer, a date, and a pacing verdict.

The rehearsal record for this task lives in `docs/proof/readme-demo/rehearsal-2026-10-04/`. The README never links a rehearsal record.

### D7. Verifier

`scripts/verify-readme-demo.sh --manifest docs/assets/starport-demo.json [--record <dir>]` exits 0 only when every check passes:

- Each output hash and size matches the record.
- The GIF header matches the record dimensions, and the GIF is under the budget.
- The effective font meets the limit.
- The five scene markers appear in order.
- The catalog scene ran with no provider key.
- The answer stream ends with `[DONE]` and carries content.
- No cut intersects the inference interval.
- The uncut source exists.
- No fixture token value appears in the events or the transcript.
- The poster has the GIF dimensions.
- The transcript names each fixture.
- The `human_review` block names a reviewer, a date, and a pacing verdict.
- For a rehearsal record, the README does not link the record. The verifier reports which README media the README links.

The verifier prints a JSON report with one entry per check. It exits 2 with status `INVALID` when the invalidation rule fires.

### D8. Invalidation

- A rehearsal is invalid when the current tree changes a product path after the candidate head commit.
- The product paths are `cmd/`, `internal/`, `pkg/`, `console/` without `console/dist`, `go.mod`, `go.sum`, `.goreleaser*`, and the installer sources. The implementer takes the exact list from the release snapshot inputs.
- The check runs `git diff --quiet <candidate_head> HEAD -- <paths>` in the Starport checkout.
- A release record is bound to the release tag assets and never invalidates by tree.
- The manifest states both rules.

### D9. Starmap registry

- Add the adapter kind `rehearsal_demo` to `scripts/catalog_product_verify.py`.
- R01 reads the record that the manifest names. It requires `kind: rehearsal` and `qualifies_release_cases: false`. It requires a `source: ci-run` candidate with every identity field. It requires a non-empty fixture list and at least one cut with a named boundary. It requires matching output hashes and the uncut source.
- R02 runs `bash scripts/verify-readme-demo.sh --manifest docs/assets/starport-demo.json` in the Starport root. It requires exit 0 and status PASS.
- Both entries carry `task: CSP21`.
- The future A36 to A39 adapters must refuse a record with `kind: rehearsal`.
- Add verifier unit tests with a synthetic record tree.

### D10. Delivery order

- PR A on Starport lands the committed format 4 capture and the tooling. It also lands the manifest, the rehearsal record, and the fail-before record.
- PR B on Starmap lands the adapter and the two registry entries. It starts from the `csp20-registry` head to avoid a registry conflict and rebases onto main after #219 merges.
- The acceptance run is `--task CSP21` at the merged Starmap head against the merged Starport main.

### D11. Discovery audit

- The record carries the Starport snapshot version from `starport --version`. It carries the catalog generation that the catalog scene reports. It carries the provider path `openai` with the fixture upstream.
- The rehearsal names the candidate pair, not a declared release pair. CSP24 records the declared release pair.

## Fail-before

- `--task CSP21` at Starmap `da7e88539` against Starport main `e17ea25a`: two UNVERIFIED results, gate FAIL.
- The console tour GIF against the D3 checklist: zero of five scenes.

## Limits known before implementation

- The rehearsal answer is a fixture. It cannot satisfy `A36.real_streamed_result`, `A37.no_secret_or_fabrication`, `A38.inference_real_speed`, or the E03 real-answer rule. The record discloses this, and the README never links it.
- The renderer needs macOS and the Menlo font. CI verifies committed records only.
- The `human_review` block records the lead review of the rehearsal pacing. It is a manual observation.
- The rehearsal candidate comes from the pull request run of the current main. A later product change invalidates it by D8, and the final media comes from CSP24.
