# P7 Production Health Matrix

Date: 2026-07-29  
Task: P7.11  
Scope: atomic catalog state, public `server.Health`, public
`remote.Subscriber.Health`, `/api/v1/stats`, and SSE delivery

## Contract

Catalog freshness and transport liveness are independent signals. Catalog age
is derived only from the timestamp of the immutable active generation.
Heartbeat or publication activity can update stream timestamps, but cannot
change the generation timestamp or make stale data appear fresh.

The publisher reports catalog state, post-commit callback delivery, and SSE
delivery. The subscriber reports its own stream/retry/fallback lifecycle,
catch-up, activation, and remote errors. Publisher-only concepts such as hook
coalescing and connection termination do not appear as subscriber state;
subscriber-only concepts such as reconnect retry and catch-up do not appear as
publisher state.

## Executable matrix

| Required behavior | Executable evidence | What the test proves |
| --- | --- | --- |
| Atomic generation timestamp | `starmap.TestActivateUsesExactImmutableGeneration`; `starmap.TestPostCommitEventRunsAfterDurableCommit` | `CatalogState` atomically pairs catalog pointer, generation ID, generation timestamp, and sequence |
| Subscriber stream state | `remote.TestSubscriberHeartbeatsPreserveStreamLiveness`; `remote.TestSubscriberReconnectCatchesUpWithoutEventAndDeduplicatesReplay`; terminal-auth tests | Idle/starting/streaming/retrying/polling/stopped states follow the owned one-shot lifecycle |
| Last heartbeat is transport-only | `remote.TestSubscriberHeartbeatsPreserveStreamLiveness`; `remote.TestHealthCatalogAgeIsIndependentOfTransportActivity` | A real flushed heartbeat advances only `LastHeartbeatAt`; it neither creates an event nor changes catalog generation time |
| Last publication event | reconnect and out-of-order subscriber tests | A parsed `catalog.published` frame advances event activity independently of activation and duplicate handling |
| Last successful catch-up | heartbeat and reconnect subscriber tests | Initial gap closure and reconnect current-state recovery record success only after verified activation |
| Active generation freshness | subscriber heartbeat/reconnect tests; `server.TestHealthSeparatesCatalogFreshnessFromStreamLifecycle` | Both public health APIs report the exact active generation ID/time and derive age from that time |
| Retry count | `remote.TestSubscriberReconnectCatchesUpWithoutEventAndDeduplicatesReplay` | Every scheduled reconnect is counted; two failed opens plus final recovery produce the exact expected total |
| Secret-free last error | `remote.TestHealthErrorClassificationDoesNotExposeSecrets` | Health retains operation, category, HTTP status, terminal flag, and time while excluding URL userinfo, endpoint, body, and wrapped error text |
| Recoverable vs terminal error | reconnect test plus initial/reconnect/fallback 401/403 tests | HTTP 5xx remains observable and recoverable; 401/403 is observable, terminal, and does not retry or poll |
| Server lifecycle and stream state | `server.TestHealthSeparatesCatalogFreshnessFromStreamLifecycle`; `sse.TestCloseTerminatesConnectionsAndRejectsNewOnes` | Publisher idle/serving/stopped and SSE idle/streaming/stopped are independently visible |
| Successful server heartbeat/event delivery | `sse.TestStreamFlushesHeartbeatAndPublicationOnOneWriter` | Last successful heartbeat/event time, generation, and sequence advance only after write and flush |
| Coalesced publication delivery | `server.TestHealthExposesCoalescedPublicationDelivery`; root hook isolation/order tests | Every superseded pending generation increments the public publisher health counter |
| Terminated publication delivery | SSE backpressure, publication-write, and heartbeat-write failure tests | Backpressure and write failures terminate the connection and increment explicit counters plus a secret-free failure classification |
| HTTP operational health | `handlers.TestStatsExposesCatalogAndPublicationHealthSeparately` | `/api/v1/stats` exposes catalog generation/freshness separately from publisher callback and SSE delivery health |
| External consumer composition | `server-embed` and `remote-subscriber` external modules | Real `GOWORK=off` consumers compile and assert the public health APIs without internal imports |

## Silent-loss invariant

The bounded root callback dispatcher reports every pending generation it
coalesces. The SSE broadcaster either writes and flushes a publication or
terminates a connection that cannot accept or write it, incrementing
`BackpressureTerminated` or `Failed`. A subscriber reconnects and performs
mandatory current-state catch-up. Therefore a notification may be coalesced or
a connection may fail, but neither condition can remain a silently healthy
whole-generation loss.

## P7.11 completion gate

P7.11 is complete only when:

1. every matrix test passes under `-race`;
2. both external health consumers pass together with unchanged dependency
   budgets;
3. ordinary repository tests, vet, pinned lint, generated documentation,
   documentation checks, and diff checks pass;
4. the public README, architecture document, GoDoc, and `/api/v1/stats` describe
   the same freshness/liveness separation;
5. authored-model YAML, provider-model YAML, and generated `endpoints.yaml`
   remain byte-identical to protected main; and
6. the control-plane ledger records the exact commit and gate evidence.
