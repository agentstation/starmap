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
