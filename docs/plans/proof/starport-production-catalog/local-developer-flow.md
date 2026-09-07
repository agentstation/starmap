# Local developer delivery milestone

The owner changed the delivery sequence on 2026-09-07. Complete a usable product flow before another standalone component checkpoint.
The full production goal, 38 tasks, 50 primary cases, 324 required subcases, and required review gates remain unchanged.

## Delivery sequence

1. Reuse the local baseline and path behavior from CSP2. Start Starport with the embedded catalog and public updates disabled.
2. Complete CSP3 manual runtime publication through the actual Starmap CLI and HTTP endpoint. Preserve the selected baseline during fresh acquisition.
3. Apply CSP8 local dependency and runtime adoption in Starport. Preserve CSP9 credential separation and CSP10 generation activation.
4. Refresh catalog data through Starmap. Reset selected acquisition data, preserve unrelated inputs, and restart with the same accepted catalog.
5. Send an inference request through Starport. Record the actual commands and visible results for CSP0.2 and CSP0.3.

These contributions form one coordinated local milestone under CSP3. Other task rows retain their complete criteria and dependency gates.
Local integration evidence cannot qualify the released pair or close enterprise, storage, performance, documentation, or release requirements.

## Verification contract

Use isolated product directories and the actual command and HTTP entry points.
An offline launch must expose embedded models without provider acquisition or public catalog reads.
Use a deterministic local source to prove refresh, selected reset, failure preservation, and restart before the authorized real-provider request.
The reset must reveal the baseline or replacement value and retain unrelated sources, providers, bindings, and reviewed operator input.
A preview must leave the active catalog and retained state unchanged.

Focused regression tests continue during implementation. Run broad checks and update evidence once the integrated milestone is coherent.
Preserve failed evidence and required repository checks. Product evidence takes precedence over component test counts in progress reports.

## Current work

Commit `b227b9a3` contains metadata selection. Commit `eb2cbbd4` contains source resets and CLI/HTTP runtime adoption.
The source reset tests passed 49 focused provider and source reset events with the race detector.

Captures use the `/tmp/starmap-source-reset-` prefix: `red.jsonl`, `focused.jsonl`, `contracts.jsonl`, and `contracts-fixed.jsonl`.
The first contract failed before metadata reset support. A later fixture used a nil limits pointer. The corrected fixture passes.
The integrated milestone preserves these captures in its verification manifest.

The working pipeline preserves the selected baseline for Fresh. CLI and HTTP acquisition now use retained runtime publication.
The focused acquisition flow passes refresh, reset preview, reset activation, and restart.
HTTP accepts `fresh=true`. CLI uses `--force`. A root-only acquisition composition rejects Fresh because it lacks retained layers.

The actual working pair passed the local product flow on macOS arm64.
Starport started with embedded data, public pulls disabled, and acquisition disabled.
A local provider fixture changed the GPT-4o mini context limit from 128,000 to 9,999 through the Starmap CLI.
Starport refreshed from the Starmap server and accepted its generation. HTTP reset restored 128,000, and Starport accepted that replacement.
Restart retained the same generation and payload checksum.

The owner-supplied OpenAI credential completed a streamed request through Starport with the Starmap server offline.
The response returned HTTP 200, nine chunks, a completion marker, and the requested text.
The request allowed at most 32 output tokens. Its 2.422-second duration includes provider processing and is not gateway overhead evidence.

The repository dotenv credential failed with exhausted provider credits. Both repository dotenv files held the same credential.
No credential value enters these captures.

The [verification manifest](local-developer-flow/verification.json) binds product captures and failed attempts by digest.
The first fixture lacked canonical author metadata. A later run polled operation status too often and received HTTP 429 responses.
Another fixture omitted the endpoint field mapping, so its raw context value did not enter the catalog. The final fixture includes that mapping and asserts both values.

The affected Starmap packages passed 834 normal test events. Twelve focused race events passed across acquisition, pipeline, HTTP handlers, and application composition.
Starport catalog checks passed 131 race events and failed two older state-directory fixtures.
The fixture repair uses private directories created by the runtime and closes sequential identity probes before the next open.
The revised contract keeps identity stable across port changes and gives separate state directories separate identities.
All three focused race checks passed, including concurrent directory-use refusal.

Starport still pins Starmap `v0.16.5`. The working pair uses `/tmp/starport-local-flow.work` to select the candidate module.
Its acquisition fixture now carries the required original receipt. Native qualification, published module adoption, and full production acceptance remain open.

The Starmap verifier passed 79 normal package suites, 79 race package suites, and the Linux container check before an unused-helper lint failure.
The lint repair removed the unused wrapper. All pipeline race tests, lint, required coverage thresholds, generated documentation, and four CLI checks pass.
These commands complete the original verifier checks after that repair. The manifest retains the failed invocation and each follow-up capture.
Required pre-PR review remains pending.

An exploratory shared-workspace ago run reports three naked returns in unchanged Starport code.
Starport declares no ago module tool. This run used the Starmap workspace tool and does not add a Starport publication gate.
Starport retains its repository-owned verification requirements.

## Review-tool decision

The shared helper remains unchanged at SHA-256 `ee15ab4246c842576e7ea87473d481d64e8a6b7c7fd3a8d2a716606c8cfa372c`.
The prepared patch changes `MAX_REVIEW_PROMPT_BYTES` from 512,000 to 300,000 in `/Users/jack/.agents/skills/autoreview/scripts/autoreview`.
It keeps the complete review and its existing reviewers, isolation, and scanning. The agent renewed the owner approval question with the sequencing change.
This milestone authorizes no additional external writes, merges, releases, or global changes.
