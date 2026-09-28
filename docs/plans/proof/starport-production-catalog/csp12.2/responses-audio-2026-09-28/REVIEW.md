# Responses and audio review

CSP12.2 remains in progress. This change repairs two defects found through production HTTP tests.
It does not complete A47 or the full paid-operation matrix.

## Findings and repair

The embedded Whisper records declared text input and streaming, without a speech-recognition tag.
Starport therefore published a chat route and rejected both audio operations before budget admission.
OpenAI documents audio transcription and translation for Whisper. Its API FAQ states that Whisper does not support streaming.

Sources: [model reference](https://developers.openai.com/api/docs/models/whisper-1), [API FAQ](https://help.openai.com/en/articles/7031512-audio-api-faq).
The review accessed both sources on September 28, 2026.

The authored and provider records now declare audio input, text output, `speech_to_text`, and no streaming.
The generated endpoint projection publishes transcription and translation. The bootstrap regression forbids a Whisper chat offering.
The payload comparison contains six changed paths, all within the two Whisper records.
Existing source observations remain intact. A new custom-update observation identifies the corrected payload.

The OpenAI connector replaced each multipart content type with `application/json` during authentication.
The connector now supplies the JSON default only when the request encoder supplies no content type.
Authentication still runs before dispatch. The wire tests parse actual uploads and verify the file bytes and authentication header.
The complete connector package checks the other request protocols.

## Budget evidence

Responses uses the chat reservation once per provider attempt.
The production tests cover measured and missing usage, complete and truncated streams, insufficient capacity, and confirmed absent budgets.
Both token and spend meters bind the same attempt. Missing evidence retains capacity and prevents another dispatch.

The three supported audio HTTP routes reject required spend or token budgets before provider dispatch.
Their error states that no verified billing bound exists. Compressed byte count does not become a duration or token estimate.
Confirmed absent budgets permit one provider dispatch with a valid multipart request.
The tests inspect real Badger reservation records and use SQLite identity storage.

## Batch evidence

The production controller submits two stored input lines for each supported batch endpoint: chat, Responses, and embeddings.
Each line binds its durable request identity to the caller and both budget meters.
Measured usage settles each dispatched attempt at the pinned ordinary rate. These tests assume no provider-native batch discount.

Missing usage permits one dispatch and retains its uncertain reservation. The other line records HTTP 402 without provider contact.
Insufficient capacity records two HTTP 402 results with no reservation or provider dispatch.

These tests cover uninterrupted execution. They do not qualify retry attempts, restart continuation, or shared-storage recovery.

## Evidence limits

The production fixture uses a loopback provider and fake credentials. It sends no paid inference request.
This run qualifies macOS ARM64 with Go 1.27.1 and the local producer through `GOWORK`.
It does not qualify a released dependency pair, shared-storage failover, native Linux or Windows, latency, or capacity.
The seven remaining CSP12.2 subcases remain unverified. The prior sixteen passing gate results remain historical evidence.

The first consumer run failed all nine audio cases because Whisper had no audio offering.
After catalog repair, three absent-budget cases failed because the connector lost the multipart boundary.
The connector regression then reproduced the defect for transcription, translation, and image edits.
The proof retains these failures and the initial test compilation error.

The first generation command rejected changed bytes under the old committed manifest.
The correction tool validates the prior generation, creates a new content-bound manifest, and retains its source observations.
The normal bootstrap generator then updates the payload, manifest, and endpoint projection.

One layout invocation omitted `GOWORK` and used the older published module. The explicit-workspace rerun passes.
Starport has no `make technical-writing-check` target. Direct lint covers all four changed consumer files.

The producer full prose check passes all 1,910 files. The plan full check reports 156 existing diagnostics in historical evidence.
The five changed plan documents pass direct lint. Historical findings remain intact.

No PR or merge occurs in this increment. Required pre-PR review follows complete pair qualification.
Docker remains stopped. The next matrix checks cover video call purposes and production batch attempts.
