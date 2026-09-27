# Paid-operation admission matrix

Status: implementation contract for CSP12.2. No row establishes production qualification.
The source audit uses Starport commit `ca2d2057d60c07ad0bd6668023af8ae8647534c1`.
Its checkout is `/Users/jack/src/github.com/agentstation/starport-native-catalog`.

[Engineering specification 8.9.4](ENGINEERING_SPEC.md#894-atomic-limits-and-recoverable-reservations) owns the reservation contract.
Starmap owns billing units and prices. This matrix owns the coverage inventory for Starport admission.
It supplies no provider prices or unsupported billing assumptions.

## Common contract

Every potentially charged provider attempt requires a durable admission decision before dispatch.
The decision binds account, key, team, policy revision, offering, catalog generation, pricing basis, windows, and attempt identity.
Confirmed absence of a required budget permits dispatch without a usage-total lookup.
Missing policy evidence cannot mean an absent budget.

Each applicable meter participates in the same atomic decision.
The bound covers every billable component of the selected request, including input media and hidden or reasoning output where applicable.
Request limits must enforce that bound. Unknown prices, unsupported units, or unbounded work refuse strict-budget dispatch.
Known zero-priced work still needs explicit catalog evidence and any applicable token reservation.

Retries and fallback attempts reserve separately. Earlier charged or uncertain attempts retain their capacity deductions.
Provider cancellation, local cancellation, timeout, and missing usage cannot independently authorize a refund.
Required reconciliation survives process failure. Optional analytics never restores capacity.

## Operation coverage

Every row below requires production execution tests and real-backend reservation evidence.
The paths refer to the pinned Starport checkout. Function names identify the current dispatch functions.
Billing components always come from the selected catalog offering.

| Surface | Current source and dispatch | Required bound and settlement evidence |
| --- | --- | --- |
| Chat: `chat-completions` | `internal/router/router.go`, `RouteWithFallback`, then `connector.Chat` | Bound input and output components. Reconcile every attempt, including failed attempts that can incur charges. |
| Streamed chat | `internal/router/execution_adapter.go`, `RouteStream` | Reserve before opening the provider stream. Preserve uncertain usage after disconnect or missing final usage. |
| Responses | `internal/server/controllers/responses.go` calls chat or streamed chat | Apply the chat contract once per provider attempt. The protocol adapter creates no second generation charge. |
| Embeddings: `embeddings` | `internal/router/embeddings.go`, `RouteEmbeddings` | Bound all input items and billable units. Each retry retains its own reservation identity. |
| Semantic-cache embeddings | `internal/proxy/semantic_cache.go`, `GatewayEmbedder.Embed` | Reserve the child embedding separately, even when a later cache hit avoids chat generation. |
| Recognition: `documents-recognition` | `internal/proxy/parser.go` calls `RouteDocumentRecognition` | Bound actual page or token billing under D37. Retain recognition charges when later chat fails or is refused. |
| Reranking: `rerank` | `internal/router/rerank.go`, `RouteRerank` | Bound all submitted query and document units, including any provider-defined minimum. |
| Moderation: `moderations` | `internal/router/moderations.go`, `RouteModerations` | Use explicit priced or zero-priced catalog evidence. Missing price data cannot become zero. |
| Guardrail moderation | `internal/proxy/guardrail_moderation.go`, `GatewayModerator.Moderate` | Reserve every charged child classification, including calls that cause the outer request to stop. |
| Images: `images-generations`, `images-edits` | `internal/router/media.go`, `RouteImages` | Bound count, input, output, resolution, and quality components that the offering bills. |
| Speech: `audio-speech` | `internal/router/media.go`, `RouteSpeech` | Bound the offering's actual input and output units. A response byte limit alone cannot prove a billing bound. |
| Transcription: `audio-transcriptions`, `audio-translations` | `internal/router/media.go`, `RouteTranscription` | Bound decoded duration and other billed components. Do not infer duration from compressed file size alone. |
| Video submission: `videos-generations` | `internal/router/video_jobs.go`, `RouteVideoSubmit` | Reserve before submission. Retain provider acceptance identity and the pinned valuation until durable settlement. |
| Video poll, cancel, and content | `RouteVideoPoll`, `RouteVideoCancel`, `RouteVideoContent` in the same file | Distinguish these calls from submission. Reserve any independently charged work without reserving the original generation again. |
| Background batch lines | `internal/server/controllers/batches.go`, `batchLineRunner.execute` | Current paths serve chat, responses, and embeddings. Reauthorize each line and reserve each actual provider attempt. |
| Local reads and cache delivery | Document text extraction, retained extraction reads, response delivery, job record reads | No provider reservation when no provider work occurs. Apply existing authorization and cache-delivery policy. Reserve any paid child call separately. |

The video operation name currently covers submission and subsequent provider calls.
Admission must use a typed call purpose and verified billing contract at that boundary.
It must not classify every video call as a new generation or assume all follow-up calls are free.

Batch submission is gateway-managed work in this source revision.
It does not prove support for a provider-native batch discount or provider-native batch settlement contract.
Any future provider-native batch path requires its own matrix entry before strict-budget support.

## Observed gaps

The current HTTP budget gate reads totals before dispatch. It does not reserve capacity.
The [production dispatch probe](../../plans/proof/starport-production-catalog/csp12.2/provider-dispatch-before.json) sends two concurrent requests through the production router and OpenAI connector.
Both reach the loopback provider before either response completes. Both return HTTP 200.
Badger records 1,202 tokens against a 1,000-token key limit.
Each request permits 600 output tokens, so their combined output allowance already exceeds that limit.

The [earlier concurrency probe](../../plans/proof/starport-production-catalog/csp12.2/current-budget-before.json) remains evidence about middleware behavior.
The newer probe uses real local storage and production dispatch. Shared storage, other scopes, other operations, and recovery remain unqualified.
Acceptance must also inspect durable reservations after implementation.

`internal/limits/spend.go` returns an unbounded allowance when context contains no allowance.
Internal calls therefore need explicit policy evidence through the shared admission owner.
A copied remaining balance cannot prove exclusive capacity.

Recognition's `affordable` check uses a preliminary page-price floor.
It does not reserve the selected route's cost or establish a token-billing upper bound.
Reranking's allowance check also cannot replace atomic admission.

`internal/jobs/accounting.go` marks a terminal job accounted before calling `RecordJob`.
It discards the accounting error. A later settlement sees the accounted marker and does not retry that record.
This behavior can lose required accounting after the marker succeeds.
The job-slot meter bounds outstanding job count, not spend or token capacity.

Required budget settlement needs a recoverable state transition separate from optional usage reporting.
A job's terminal state alone cannot establish zero provider cost.
Current terminal accounting and a final `Chargeable` flag cannot prove that every provider attempt settled.

## September 27 dispatch integration

Unpublished Starport `3a292316` installs admission in production routing.
Chat has a conservative token-only projection from declared offering limits.
The shared operation path and embeddings refuse required budgets without a supported projection. Confirmed absent budgets retain provider access.
Spend projections and the remaining operation-specific bounds are incomplete.

The production tests use the OpenAI connector, local HTTP provider, Badger, and SQLite.
They verify concurrent refusal, measured settlement, absent budgets, unknown spend bounds, and stream completion or interruption.
OpenAI-compatible settlement requires the protocol's completion marker. A truncated body retains its full reservation despite partial usage.
Other stream protocols still need their completion contracts and qualification.

The broader application race run has four failures because existing spend-budget fixtures reach the missing monetary projection.
These failures block publication. Preserve their budgets and assertions while implementing complete Starmap billing contracts and consumer projections.
The [dispatch proof](../../plans/proof/starport-production-catalog/csp12.2/provider-integration-2026-09-27/verification.json) retains failures, repairs, source hashes, and check limits.
This work does not establish A47 or complete this matrix.

## September 27 text billing integration

Producer `d94b3cc63` adds schema 11 and an explicit text-chat billing contract for OpenAI `gpt-4o-mini`.
Consumer `e7a93be1` reserves and settles its declared input, cached-input, and output charges.
Missing usage detail retains uncertain capacity. Local budget exhaustion returns HTTP 402.
Other offerings still need their own complete billing declarations.

The [text billing proof](../../plans/proof/starport-production-catalog/csp12.2/text-billing-2026-09-27/verification.json) records 4,318 passing repository results and 133 skips.
The repaired application race tests pass eighteen results. Final focused race tests pass 24 results.
These checks use the local producer through `GOWORK`. The published dependency pin remains unqualified.

The four earlier application failures no longer block this component.
Their performance fixture now reserves against a $1 budget that covers its maximum sample count.
A separate refusal test retains the original $0.001 budget and requires HTTP 402 with zero provider calls.
This replaces the provisional fixture-preservation instruction above. Product budget policy and success assertions remain unchanged.

The current bound uses full declared provider limits. Narrower request bounds and the remaining operation contracts still require qualification.
Asynchronous settlement, real shared-backend evidence, overhead measurement, and A47 remain open.

## September 27 request identity

Consumer `a189b8dc` preserves gateway request IDs through the proxy and router.
Each provider attempt retains a separate reservation ID. Reusing an HTTP request ID cannot reuse a dispatch permit.
Recognition retains the outer request ID. Semantic embeddings and guardrail moderation retain their existing child request IDs.

The [identity proof](../../plans/proof/starport-production-catalog/csp12.2/request-identity-2026-09-27/verification.json) records three production regression results and 428 package race results.
Six pure-Go results, lint, and vet pass. The production test covers fallback and repeated request IDs in ordinary and streamed chat.
Other operations have identity-transfer checks. Their paid dispatch contracts remain unqualified.

Required job settlement must use retained reservation evidence.
The current optional usage writer increments counters after a separate record write. Retrying that writer alone cannot provide idempotent accounting.
Durable job association and asynchronous settlement recovery remain open.

## September 27 usage presence repair

Consumer `0ea27f36` prevents missing token counts from authorizing refunds.
Ordinary and streamed chat retain full capacity when required counts are absent or null.
Complete explicit zeros remain valid evidence. Native conversion rejects missing counts, negative components, and overflow before settlement.

The [usage presence proof](../../plans/proof/starport-production-catalog/csp12.2/usage-presence-2026-09-27/verification.json) retains the failing production regression.
Final focused race checks pass 35 results. The repository suite passes 4,357 results and skips 133.
The producer dependency still resolves through the candidate workspace.

Vertex embedding conversion currently estimates tokens from whitespace. Ollama embedding conversion reports zero without provider usage.
Neither value can authorize a strict-budget refund. Embedding support needs measured usage or retained uncertain capacity.

## September 27 embedding admission

The [embedding proof](../../plans/proof/starport-production-catalog/csp12.2/embedding-billing-2026-09-27/verification.json) records schema 12 and the first embedding projection.
The OpenAI `text-embedding-3-small` offering declares input-token billing and no separate request charge.
Its declared input limit is 8,192 tokens per item.
The gateway reserves that full limit for each text or token-ID item with checked multiplication.
Other offerings still require complete verified monetary declarations.

The real application tests use Badger, SQLite, and a local HTTP provider through the production OpenAI connector.
They cover measured settlement, complete zero usage, missing or inconsistent usage, insufficient capacity, and unknown billing.
Refused calls reach no provider. Unknown usage retains the full reservation.
Native Google, Vertex, and Ollama usage cannot authorize refunds from absent or estimated counts.

A semantic-cache child call receives a separate attempt identity and the parent's budget meters.
It retains the caller's model and credential-routing restrictions.
Eleven production race results pass. The repository suite passes 4,403 results and skips 133.
The exact released dependency and native CI remain unqualified.
Real shared-backend qualification, remaining operation contracts, and settlement recovery remain open.
This evidence does not complete A47 or CSP12.2.

## September 27 embedding usage provenance

Consumer `0231ec7a` repairs Vertex text batches and preserves every returned vector.
It disables truncation, requests dimensions, and uses complete provider token statistics.
Incomplete usage remains unknown through canonical responses, cached records, wire responses, and optional accounting.
Legacy cached vectors remain reusable with unknown usage. Complete measured zeros remain valid.

The [usage proof](../../plans/proof/starport-production-catalog/csp12.2/vertex-usage-2026-09-27/verification.json) retains failed regressions and final checks.
The final package race run passes 900 results. Ten cache-matrix runs pass 110 results.
The repository suite passes 4,440 results and skips 133. Pure-Go checks pass 58 results.
The repository and pure-Go checks precede the final test-only changes. The final race run covers those changes.

Vertex monetary billing remains undeclared. This adapter repair does not qualify strict spend admission for Vertex.
Recognition and the remaining matrix rows still require complete bounds, declarations, and settlement recovery.
A47, the exact dependency pin, review, native CI, and merges remain open.

## September 27 recognition admission

Producer `5790fb04e` adds schema 13 and complete recognition charges for Gemini 2.5 Flash.
Consumer `8b1dc0eb` reserves declared token charges and enforces the selected output cap.
Raw measured usage survives incomplete documents and later chat failures.
Missing usage retains capacity. Token-only settlement preserves unknown monetary cost.

The [recognition proof](../../plans/proof/starport-production-catalog/csp12.2/recognition-billing-2026-09-27/verification.json) records 1,349 producer race results and ten production recognition results.
A separate token-only run passes two results. The repository run passes 4,485 results and skips 133.
These consumer runs precede the final optional-report provenance repair.
Final affected-package race checks pass 738 results and skip 22. Final lint and vet pass.

The cache regression found missing runtime retention when settings disabled chat-response caching.
The repair retains one generation through parsing and chat. Repeated documents reuse extraction without another recognition charge.
Another regression found missing cached-token measurements reported as an exact price.
The final repair retains known totals while reporting unavailable cost and the missing measurement.

Fixed-page valuation and processed-page evidence have component tests. These checks do not qualify a shipped fixed-page connector.
Context tiers and service-specific charge variants still require qualification.
The remaining operation contracts, durable settlement recovery, real shared-backend evidence, A47, exact dependency pin, review, native CI, and merges remain open.

## September 27 fixed-page recognition

Producer `9d81c4293` declares the Mistral OCR protocol and exact `mistral-ocr-4-0` offering.
Consumer `1253668c` sends an explicit page selection and reserves its fixed page charge before dispatch.
The request excludes annotations and provider-native batch options. Required token budgets refuse the page-only contract.

The [page recognition proof](../../plans/proof/starport-production-catalog/csp12.2/page-recognition-2026-09-27/verification.json) records the protocol and production execution checks.
Final checks pass 4,513 repository results with 133 skips, 27 focused race results, and 61 pure-Go results.
Producer race checks pass 1,350 results. All nineteen provider-ownership conditions pass.
These checks use the local producer workspace and loopback provider responses.

Missing page usage retains uncertain capacity. Reported charges survive incomplete or invalid page text.
Optional reports preserve measured page counts separately from local document counts.
The earlier reporting path priced unmeasured pages. A failing regression and the repair remain in the proof.

The provider documentation does not establish a fixed page ceiling for this offering. The catalog leaves that limit unknown.
Production tests verify that explicit page selection enforces the reserved request count.
The remaining operation contracts, recognition variants, durable recovery, shared-backend qualification, A47, dependency pin, review, native CI, and merges remain open.

## September 27 moderation admission

Producer `5cef96251` adds schema 14 and explicit zero request prices for two OpenAI moderation offerings.
Consumer `9e54be6a` reserves direct and guardrail moderation through the shared operation admission path.
Missing declarations and prices remain unknown. Required token budgets refuse the request-based contract before dispatch.

Guardrail calls preserve routing restrictions and budget identity. Request and response classifications receive separate durable reservations.
Failed provider calls retain uncertain reservations. Optional reports preserve unknown tokens beside known free cost.
The guardrail pipeline preserves admission failures, so an unavailable token bound returns HTTP 503.
A forbidden moderation model returns HTTP 400 and reaches no provider.

The [moderation proof](../../plans/proof/starport-production-catalog/csp12.2/moderation-billing-2026-09-27/verification.json) records 1,354 producer race results and 4,531 consumer repository results with 133 skips.
Focused race checks pass 42 results. Pure-Go checks pass 34 results.
The final application race check passes nine results, including the added provider-failure case.
The proof records check timing, earlier failures, and the local workspace dependency.

Media, rerank, recognition variants, asynchronous recovery, shared-backend qualification, A47, dependency pin, review, native CI, and merges remain open.

## Rerank token admission component

Starmap `abd3475b3` adds schema 15 and complete token contracts for two Voyage offerings.
Starport `17564bb4` reserves every submitted query-document pair before dispatch.
A smaller requested result count does not reduce the bound. Invalid ranking results retain measured charges.
Exact decimal decoding distinguishes missing or invalid counts from measured zero.

The [rerank proof](../../plans/proof/starport-production-catalog/csp12.2/rerank-billing-2026-09-27/verification.json) records 1,358 producer race results and 4,575 consumer repository results with 133 skips.
The shared valuation refactor passes 181 focused race results. Final decoder and wire checks pass 41 race results and 41 pure-Go results.
Earlier pure-Go checks pass 145 results. The proof identifies each run's tested source and preserves failures.
All nineteen provider-ownership conditions and twenty-two rerank structural conditions pass.

Cohere search-unit admission remains open. Its billing chunks differ from inference chunks.
A document-count limit alone cannot establish the search-unit charge bound.
The complete contract must establish query repetition, truncation, billing chunks, rounding, applicable prices, and required token evidence.
Optional search-unit cost reports do not qualify this contract.

Media, recognition variants, asynchronous recovery, shared-backend qualification, A47, the dependency pin, review, native CI, and merges remain open.

## Character-priced speech component

Starmap `3635aa986` adds schema 16 and character contracts for TTS-1 and TTS-1-HD.
Starport `7326c120` reserves input characters before provider dispatch and retains charges independently of response conversion.
Completed audio carries a request-derived count. Failed or truncated responses retain uncertain capacity.
Required token budgets refuse this contract. Token consumption remains unknown.

The [speech proof](../../plans/proof/starport-production-catalog/csp12.2/speech-billing-2026-09-27/verification.json) records 1,361 producer catalog race results and 689 reconciler race results.
Consumer race checks pass 63 results and skip four Valkey cases. Pure-Go checks pass 50 results with the same four skips.
The full consumer suite passes 4,598 results and skips 133. Lint, vet, goago, ownership, and modality checks pass.
The proof preserves the routing failure, copy-isolation regression, fixture failure, source hashes, and raw test events.

Public pricing names characters. The code-point interpretation follows JSON string semantics and remains an inference without invoice verification.
No live paid provider call or shared-service qualification occurred in this component.
Other speech billing units, images, transcription, video, recognition variants, and search-unit admission remain required.
Asynchronous recovery, shared-backend qualification, A47, the published dependency, review, native CI, and merges remain open.


## Image billing component

Starmap `fb7f8ae21` adds schema 17 with image-count and pixel-iteration contracts.
Starport `2a2e8a5b` reserves declared image charges before provider dispatch.
The production cases use Badger, SQLite, and a local HTTP provider through the actual gateway and connector.
An empty image entry retains the complete reservation as uncertain. The regression failed before this repair.

The [image proof](../../plans/proof/starport-production-catalog/csp12.2/image-billing-2026-09-27/verification.json) records fifteen production cases and their parent result.
Final consumer race checks pass 45 results. Final catalog race checks pass 1,355 results.
Reconciler checks pass 690 results. Provider checks pass 103 results and skip three opt-in cases.

The full consumer suite passes 4,629 results and skips 133. Pure-Go checks pass 28 results.
The proof identifies source timing, failed attempts, source hashes, and remaining qualification limits.

DeepInfra public acquisition, fixture refresh, and wire drift checks pass. Its fixture contains 187 records and 26 image-unit prices.
The broader live-provider gates fail for missing credentials and stale fixtures outside DeepInfra.
No paid inference or provider invoice verification occurred. Those failures and all skips remain recorded.

Image edits, token-priced images, other speech units, transcription, video, recognition variants, and search-unit admission remain incomplete.
Asynchronous recovery, shared-backend qualification, A47, the published dependency, review, native CI, and merges remain required.

## Asynchronous accounting foundations

Optional usage now commits each record and all aggregates atomically. Exact retries preserve totals and TTLs.
Conflicting contents and replay after the event's retention deadline refuse. Corrupt or overflowing counters leave the complete write unchanged.

Job replacement now compares the caller's observed record. A stale same-state update cannot erase accounting or asset progress.
An ambiguous asset commit retains its bytes until the stored record resolves the result.

These repairs support later settlement recovery. They do not bind a provider submission to a durable reservation.
Pinned asynchronous valuation, idempotent slot release, recovery enumeration, and unreferenced asset collection remain incomplete.
The terminal stamp still precedes optional accounting. Retrying that sequence requires its own repair and qualification.

## Acceptance evidence

Each supported billing contract must provide these observations:

1. Concurrent production requests cannot dispatch attempts that exceed jointly reserved account, key, and team capacity.
2. Unsupported units, unknown prices, and missing policy evidence cause zero provider dispatches under a required budget.
3. Internal paid calls reserve independently and preserve their identity after outer cancellation or refusal.
4. Retries and fallbacks preserve earlier deductions and settle each charged attempt once.
5. Stream termination, ambiguous submission, and missing usage retain uncertain capacity.
6. Restart and storage recovery preserve attempt identity and cannot duplicate settlement, release, or available capacity.
7. Catalog changes and window boundaries do not change existing reservation valuation or meter ownership.
8. Failed required settlement remains recoverable after optional analytics fails or drops its record.
9. Confirmed absent budgets avoid usage-total queries. Warm permission checks remain local.

Qualification must cover the actual Badger and Valkey ownership contracts.
PostgreSQL remains the independent recovery witness under specification 8.6, not the budget ledger.
Redis and other alternatives require their declared compatibility evidence before a support claim.

Async qualification must specify concrete reconciliation and usage-replay horizons for each supported provider contract.
Those values remain unqualified. Unknown horizons cannot justify deleting unresolved reservation records.
Tests must exercise production dispatch and real storage, including failures between durable transitions.
Ambiguous submission cannot trigger an unverified resubmission after restart.
Mocked billing or middleware entry counts alone cannot establish A47 acceptance.


## Durable asynchronous submission component

Consumer `c270de27` records the selected attempt before provider dispatch.
Its record binds the offering, catalog generation, and required reservation ID.
Acceptance persists before success. Uncertain submissions retain their records and slots without automatic retries.
Polling and elapsed polling windows cannot resolve an unknown provider outcome.

The [submission proof](../../plans/proof/starport-production-catalog/csp12.2/async-submission-2026-09-27/verification.json) records 39 focused race results and 25 pure-Go results.
Production HTTP cases cover both protocol prefixes, accepted work, malformed responses, missing handles, provider failures, and required-budget refusal.
The full suite passes 4,825 results and skips 71. Eight packages skip.

These tests do not qualify video billing or required settlement.
Pinned valuation, durable settlement, idempotent slot release, recovery enumeration, and provider reconciliation remain required.


## Durable slot claims component

Consumer `f5788320` binds video and batch slots to durable claim IDs.
Repeated reserve and release operations preserve account counts. Released claims cannot become active through a delayed retry.
Video release retries independently of optional reporting. Batch cancellation retains its slot until admitted lines finish.

The [claim proof](../../plans/proof/starport-production-catalog/csp12.2/slot-claims-2026-09-27/verification.json) records 156 jobs and claim race results and 77 pure-Go results without skips.
The broad race run remains incomplete because the application package exceeded four minutes in existing speech tests.
The proof separates earlier broad results from final storage-format qualification.

Slot claims do not settle spending reservations or supply missing provider usage.
Pinned valuation, required settlement, orphan resolution, complete enumeration, retention, and populated-state migration remain required.


## Recovery enumeration component

Consumer `3d14fb65` adds native cursor continuation and background release for finished batches.
The [recovery proof](../../plans/proof/starport-production-catalog/csp12.2/recovery-scans-2026-09-27/verification.json) records Badger and Valkey regressions, cancellation, corruption, and production composition.
Final focused race checks pass 29 results. Pure-Go checks pass 25 results. Both have no skips.

This work does not establish asynchronous billing or required settlement.
Claims without job records, interrupted running batches, pinned valuation, and required settlement remain open.


## Atomic claim attachment component

Consumer `a8137780` commits each job record with its claim attachment.
The background recovery pass closes old unattached claims. Delayed publishers cannot use capacity that recovery released.
Attached uncertain submissions retain their claims.

The [attachment proof](../../plans/proof/starport-production-catalog/csp12.2/claim-attachment-2026-09-27/verification.json) records 200 final race results and 60 pure-Go results without skips.
A separate production race run confirms attachment before dispatch. This work does not settle required spending or establish provider billing.
Pinned asynchronous valuation, durable settlement, provider reconciliation, interrupted batch recovery, and safe collection remain required.


## Video cancellation and current provider contracts

Consumer `e0b4b237` preserves cancellation outcomes and requires matching provider identifiers.
Queued, running, missing, and mismatched outcomes retain outstanding capacity. A later confirmed terminal outcome can release that claim.
The [cancellation proof](../../plans/proof/starport-production-catalog/csp12.2/cancellation-outcomes-2026-09-27/verification.json) includes native Badger and Valkey checks and production HTTP checks through both API families.
This release concerns outstanding job count. It cannot refund a spending reservation.

The [repository audit](REPOSITORY_FINDINGS.md#september-27-cancellation-and-video-contract-audit) identifies current provider unit and transport differences.
Replace the generic per-video assumption with each provider's actual billing contract before strict-budget video dispatch.

## Media duration units

Schema 18 preserves input and output prices per second without declaring a complete billing contract.
Consumer `06982eb2` reports unknown video cost when duration quantities are absent, including data that also carries a per-video rate.
The [duration proof](../../plans/proof/starport-production-catalog/csp12.2/media-duration-units-2026-09-27/verification.json) records conversion, schema, copy, and consumer accounting checks.

Fixed-page OCR can reserve the decoded page count without an undocumented provider page ceiling.
Unknown capabilities remain explicit. Historical identity review and historical test results remain intact.
These changes do not qualify video dispatch, option-dependent charges, provider usage, or durable settlement.
Retain the submission valuation and reliable usage through durable settlement. Failed or cancelled state alone cannot establish a zero charge.

Consumer `1143dfb0` retains unresolved provider work after local polling exhaustion.
An explicit provider check can recover a confirmed handle without resubmitting generation.
The [polling proof](../../plans/proof/starport-production-catalog/csp12.2/polling-recovery-2026-09-27/verification.json) covers both HTTP families and native storage.
Background reconciliation, uncertain handles, billing settlement, and retention horizons remain required.

## Retained settlement recovery

The reservation worker now retries durable measured usage through its original authority.
It preserves selected prices and windows, missing usage, and independent recovery approval.
The [recovery proof](../../plans/proof/starport-production-catalog/csp12.2/retained-settlement-recovery-2026-09-27/verification.json) records startup, native storage, pagination, concurrency, and restart checks.

This worker consumes evidence already in the reservation ledger. It does not query providers or reconstruct missing usage.
Required asynchronous settlement still needs complete billing contracts, pinned valuation, and durable provider evidence.
Process-loss, failover, capacity, PostgreSQL, and final production-path qualification remain required under CSP12.2 and CSP15.

## Required job settlement boundary

The job service now checks its required reservation before the optional accounting mark.
The ledger binds one attempt to one job before dispatch. The application checks the account, key, offering, generation, and operation.
An unresolved charge retains reserved spending after a terminal job releases its concurrency slot.
Recovery can settle retained evidence under the original valuation and independent approval.

The [job settlement proof](../../plans/proof/starport-production-catalog/csp12.2/job-settlement-boundary-2026-09-27/verification.json) records the regression and component checks.
Attempt payload version 2 prevents older writers from silently discarding job ownership. CSP13 owns populated-state migration.

This boundary does not qualify video dispatch. Optional reports still need pinned measured charges. Delivery now retries after an error or missing acknowledgement.
Terminal notifications now claim an independent best-effort attempt. Required settlement remains mandatory before the reporting acknowledgement.
Provider evidence acquisition, uncertain submissions, interrupted batches, and full task qualification remain open.


## DeepInfra video transport qualification

The public OpenAPI declares asynchronous video endpoints and numeric `seconds`.
That declaration does not prove that every model supports those endpoints.
The live Wan2.2-T2V-A14B request rejects asynchronous jobs and names its native inference route.
Its native input validator requires five seconds, despite the public model page advertising two or five seconds.

Provider-level operation routing must not apply one video protocol to every offering.
Qualification must bind the selected offering to its protocol, duration constraints, measured usage, and complete billing contract.
The [live probe](../../plans/proof/starport-production-catalog/csp12.2/deepinfra-video-probe-2026-09-27/verification.json) records the request outcomes.
Do not infer cancellation, asynchronous state words, or a zero charge from these validation responses.


### Native video contract and remaining execution work

Catalog schema 19 pins output-duration billing and exact-model endpoint selection.
For the qualified Wan input shape, five output seconds at USD 0.075 per second produce USD 0.375 under the catalog contract.
The provider's `cost` field is an estimate. Settlement must use its measured `output_length` and the submitted valuation.
Container duration (5.062012 seconds) cannot replace provider-reported output seconds.

Local checks verify endpoint precedence, schema rejection, copy isolation, defaults, and request bounds.
They also verify zero usage, unknown usage, and measured overage.
Native transport checks cover one dispatch, bounded assets, usage retention, and explicit provider failures.
The [component proof](../../plans/proof/starport-production-catalog/csp12.2/video-contract-2026-09-27/verification.json) records their exact scope.

Consumer `2e3c7733` registers native video execution with bounded gateway workers and durable inline receipts.
Optional reports use retained rates and measured duration. Missing usage remains unknown in every terminal state.
The production HTTP fixture proves early job return, measured settlement, missing-usage retention, and insufficient-budget refusal before dispatch.
The [native job proof](../../plans/proof/starport-production-catalog/csp12.2/native-job-flow-2026-09-27/verification.json) records exact checks and earlier failures.

Consumer `fa42a9c7` adds separately approved external downloads and bounded recovery.
The first accepted digest prevents changed asset bytes from replacing an interrupted result.

The production fixture spends the exact allowance, then retrieves the job through both API families.
All 19 paid routes reject further work. Job reads, content retrieval, cancellation, and reconciliation retain authentication without requiring new capacity.
The [external asset proof](../../plans/proof/starport-production-catalog/csp12.2/external-assets-2026-09-27/verification.json) records checks, failures, and remaining scope.

Uncertain native responses still need an operator recovery contract. No additional paid generation ran.
Complete the remaining matrix, interrupted batches, CSP12.2 qualification, A47, and paired merges.
