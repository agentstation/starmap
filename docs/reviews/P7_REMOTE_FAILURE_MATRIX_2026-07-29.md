# P7 Remote Transport Failure Matrix

Date: 2026-07-29  
Task: P7.10  
Scope: public `remote`, `pkg/catalogremote`, server SSE, and the external
remote-subscriber composition

## Contract

The subscriber performs one verified initial fetch, uses SSE as its normal
notification path, performs mandatory current-state catch-up after every
reconnect, and polls only after an explicit fallback policy reaches its stream
failure threshold. A publication event is a hint; immutable manifest/payload
verification and atomic activation remain the correctness path.

Authentication failures are terminal for the active lifecycle. HTTP 401 and
403 do not enter fallback, do not retry indefinitely, and do not continue
conditional polling. Transport failures, stream EOF, liveness expiry, and
server 5xx responses remain recoverable.

## Executable matrix

| Required behavior | Executable evidence | What the test proves |
| --- | --- | --- |
| Heartbeats flush | `internal/server/sse.TestStreamFlushesHeartbeatAndPublicationOnOneWriter` | The connection receives flushed comment frames and publications from one serialized writer; comments have no event identity |
| Heartbeats are ignored as events | `pkg/catalogremote.TestEventStreamParsesCommentsAndStablePublication`; `remote.TestSubscriberHeartbeatsPreserveStreamLiveness` | Comments reset liveness without carrying a publication, changing event ID, activating a catalog, reconnecting, or polling |
| Interval/timeout validation | `internal/server/sse.TestNewBroadcasterDefaultsAndValidation`; `server.TestDefaultConfigIncludesSSELivenessBudgets`; `remote.TestConfigDefaultsAndLivenessMargin` | Server heartbeat/write budgets and subscriber heartbeat/liveness/shutdown/fallback budgets have defaults and reject invalid margins |
| Missing heartbeat | `remote.TestSubscriberMissingHeartbeatReconnectsAndCatchesUp` | Silence expires liveness, cancels the stream, reconnects, and catches up current state |
| Half-open connection | `remote.TestSubscriberMissingHeartbeatReconnectsAndCatchesUp` | A server handler that remains open but emits no further bytes cannot leave the subscriber falsely healthy |
| Publication write failure | `internal/server/sse.TestWriteFailureUsesDeadlineAndCleansUpConnection` | A failed/dead writer has a bounded deadline, disconnects, and is removed |
| Heartbeat write failure | `internal/server/sse.TestHeartbeatWriteFailureCleansUpConnection` | Heartbeat failure terminates and cleans the connection rather than reporting liveness |
| Slow consumer | `internal/server/sse.TestPublicationBackpressureTerminatesConnection` | A full per-connection publication slot terminates the stream and records backpressure instead of silently dropping a hint |
| Disconnect | `remote.TestSubscriberReconnectCatchesUpWithoutEventAndDeduplicatesReplay` | EOF enters bounded reconnect with the prior `Last-Event-ID` |
| Duplicate event | `remote.TestSubscriberReconnectCatchesUpWithoutEventAndDeduplicatesReplay` | An already-active generation is acknowledged without refetch or republish |
| Duplicate payload | `remote.TestSubscriberDeduplicatesNewIdentityWithSamePayload` | A newer identity for identical canonical bytes advances deduplication without replacing the immutable catalog |
| Out-of-order event | `remote.TestSubscriberOutOfOrderEventsCannotRegressCatalog` | A retained older generation fetched after a newer activation cannot regress the catalog; the stream event ID still advances |
| Skipped event | `remote.TestSubscriberReconnectCatchesUpWithoutEventAndDeduplicatesReplay` | A current generation published without a replayed event is recovered by mandatory reconnect catch-up |
| Unauthorized initial stream | `remote.TestSubscriberRejectsUnauthorizedStreamWithoutRetryOrPolling` | HTTP 403 is typed and terminal before a lifecycle starts; no retry or fallback poll occurs |
| Unauthorized reconnect | `remote.TestSubscriberStopsAfterUnauthorizedReconnectAndRejectsRestart` | HTTP 401 after a prior healthy connection stops the one-shot lifecycle and prevents hidden restart |
| Unauthorized fallback poll | `remote.TestSubscriberStopsWhenFallbackPollBecomesUnauthorized` | HTTP 401 from the conditional current manifest terminates fallback before another stream attempt |
| Corrupt generation | `pkg/catalogremote.TestRemoteCatalogFetchValidatesManifestSnapshotChecksumAndCompatibility`; `remote.TestSubscriberRejectsStaleAndInvalidGenerationsBeforeActivation` | Corrupt bytes, descriptor mismatch, or invalid generation cannot replace the active immutable catalog |
| Incompatible generation | `pkg/catalogremote.TestRemoteCatalogFetchValidatesManifestSnapshotChecksumAndCompatibility`; `remote.TestSubscriberRejectsStaleAndInvalidGenerationsBeforeActivation` | Unsupported schema and incompatible consumer range fail before activation |
| Subscribe while running | `remote.TestSubscriberStopsAfterUnauthorizedReconnectAndRejectsRestart` | A second `Start` returns typed conflict |
| Subscribe after stop | `remote.TestSubscriberStopsAfterUnauthorizedReconnectAndRejectsRestart`; `internal/server/sse.TestServeHTTPRejectsNewConnectionAfterClose` | Subscriber lifecycles are one-shot and a stopped broadcaster rejects new streams |
| Blocked initialization | `remote.TestCloseCancelsAndJoinsInitialFetch` | `Close` cancels and joins a blocked initial manifest request |
| Blocked owned lifecycle | `remote.TestCloseReportsBoundedJoinTimeout` | A lifecycle that cannot join returns a typed timeout within the configured bound |
| Reconnect/backoff | `remote.TestExponentialJitterStaysWithinBoundedSchedule`; `remote.TestSubscriberReconnectCatchesUpWithoutEventAndDeduplicatesReplay` | Retry delay is bounded with jitter and retry attempts progress without overflow |
| Mandatory catch-up | `remote.TestSubscriberReconnectCatchesUpWithoutEventAndDeduplicatesReplay`; `remote.TestSubscriberMissingHeartbeatReconnectsAndCatchesUp` | Every successful reconnect fetches verified current state before resuming event consumption |
| Non-concurrent polling | `remote.TestSubscriberPollingFallbackIsExplicitBoundedAndConditional` | Polling is serialized inside the reconnect loop, conditional, interval-bounded, and stops before the recovered stream is treated as healthy |
| Conditional no-change | `pkg/catalogremote.TestFetchCurrentIfChangedUsesConditionalManifest`; `internal/server/handlers.TestHandleCatalogManifestSupportsConditionalRequests` | Matching ETag yields 304 and no payload fetch |
| Caller disconnect cleanup | `internal/server/sse.TestServeHTTPReturnsOnCallerCancellation` | Request cancellation promptly removes the server connection |
| Server shutdown cleanup | `internal/server/sse.TestCloseTerminatesConnectionsAndRejectsNewOnes` | Close is idempotent, terminates registered streams, and refuses new ones |
| Subscriber shutdown/join | `remote.TestCloseCancelsAndJoinsInitialFetch`; `remote.TestSubscriberMissingHeartbeatReconnectsAndCatchesUp`; external `remote-subscriber` module | Caller cancellation and `Close` join initialization and running transport loops within explicit bounds |
| Concurrent readers | `remote.TestSubscriberReadersObserveCompleteGenerationsDuringActivation` | Readers observe only a complete old or complete new immutable generation under `-race` |

## External composition

`testdata/consumers/remote-subscriber` is a real `GOWORK=off` module. Its
production file imports only the public `remote` package, constructs an idle
subscriber, starts it with caller context, reads the concrete immutable
catalog, and joins `Close`. Its test hosts a real public Starmap server to prove
the complete manifest/payload/SSE path without making server or root packages
part of the production consumer dependency closure.

The measured production closure is 231 packages. CI enforces a ceiling of 240
and rejects acquisition, catalog pipeline, concrete provider clients, internal
or public server implementations, cloud/GenAI/gRPC/OpenTelemetry,
WebSocket/Cobra, and SQLite families.

## P7.10 completion gate

P7.10 is complete only when:

1. every mapped test above passes in its owning package;
2. the focused real-transport matrix passes repeatedly under `-race`;
3. all four external consumer modules pass together;
4. remote, protocol, SSE, internal server, and public server race suites pass;
5. ordinary repository tests, vet, pinned lint, generated documentation,
   documentation checks, and diff checks pass;
6. focused coverage is measured and no maintained floor is weakened; and
7. authored-model YAML, provider-model YAML, and generated `endpoints.yaml`
   remain byte-identical to protected main.
