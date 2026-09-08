# Starport cache and request latency review

Reviewed: 2026-09-05. Starport has useful memory caches, but its current request path has substantial avoidable work.
Repeated catalog copies and key derivation for stored provider credentials account for the largest measured costs.
The current overhead metric excludes several gateway stages.

This review records measurements and recommendations for the product requirements and implementation plan.
It does not change product code or activate the proposed plan.
The existing PRD, engineering specification, storage review, and plan retain their accepted text.
Their storage contracts do not yet define allocation budgets or latency acceptance criteria.

The inspected Starport revision is `042eb97851496ecdd1ef20bfa44fe3d84b86b2d8`.
Its Starmap dependency is `v0.16.5`.
The local Starmap revision is `4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225`.
The same expensive offering lookup exists in that Starmap checkout.

**Measurements.** These local benchmarks ran on macOS arm64, Apple M2 Max, with Go 1.26.5.
Each table range covers three benchmark runs.
The times are mean time per operation from each run, not request percentiles.
Allocated bytes count transient allocation, not retained memory or process RSS.

| Isolated operation | Time per operation | Allocated bytes per operation | Allocations per operation |
| --- | --- | --- | --- |
| Existing selection benchmark, registry without catalog | 0.67–0.73 µs | 904 B | 12 |
| Exact selection, 10 synthetic routes | 45–46 µs | 81.9 kB | 736 |
| Exact selection, 100 synthetic routes | 2.81–2.82 ms | 5.72 MB | 52,126 |
| Exact selection, 1,000 synthetic routes | 252–270 ms | 569 MB | About 5.02 million |
| Exact selection, embedded OpenAI catalog with 111 active chat routes | 9.40–10.07 ms | 15.27 MB | 172,240 |
| Stored provider credential decryption | 13.62–14.30 ms | 67.12 MB, about 64 MiB | 39–42 |

The synthetic catalog has one provider and distinct model definitions.
The embedded case enables only the OpenAI chat adapter.
Both cases select one exact model through Starport's actual catalog planner.
Neither case calls a provider, reads a database, or measures HTTP handling.
The credential benchmark uses synthetic encrypted material and measures the existing decryption method.

These results measure separate operations. They do not establish complete request overhead.

The existing overhead test passed with `p50=0ms` and `p99=0ms` over 200 requests.
Its integer header truncates fractions of a millisecond.
Its fixture has no catalog, authentication middleware, budget checks, rate limits, or response cache.
That result does not establish zero overhead.

The existing server benchmark also passed all six runs, including three concurrent runs.
It uses mock storage and a mock connector with a 10 ms delay.
Its sequential result was 10.89–11.04 ms with 196 allocations per operation.
Its concurrent `ns/op` measures throughput across workers, not individual request latency.
Neither result qualifies the production database or catalog path.

Raw results, profiles, benchmark sources, command descriptions, and source hashes are in the [evidence manifest](evidence/latency-review-2026-09-05/manifest.json).
The benchmarks used Go overlays. The product worktree contains no added benchmark source.

**F1. Catalog reads repeat large copies.** Starport retains an immutable catalog and publishes its current snapshot through an atomic pointer.
Those are useful foundations for a fast request path.
However, [candidate construction][sp-candidates] calls `Routes()` and rebuilds all planning candidates for each request.
It reads a definition and offering for every route, even for an exact model request.
It also allocates endpoint maps and other candidate fields on each pass.

Starmap's [Offering method][sm-offering] resolves the provider before it reads the offering index.
The [provider resolver][sm-resolve] returns a deep copy that includes the complete provider model map.
Starport therefore copies that map once for each route from the provider.
For the synthetic single-provider case, this creates quadratic work as model count grows.
About 95% of sampled allocated bytes came through `DeepCopyProviderModels` in the 100-route profile.
The [allocation profile](evidence/latency-review-2026-09-05/allocations-cumulative.json) confirms the call chain.

Recommended changes:

- Resolve canonical provider IDs and aliases through a precomputed index inside the immutable Starmap catalog.
- Preserve caller ownership when returning an offering. Avoid copying unrelated provider models during that lookup.
- Build Starport's static planning candidates and model indexes when a runtime generation becomes active.
- For exact model requests, inspect only matching offerings and explicitly requested fallbacks.
- Apply current health, credential readiness, and request policy to those candidates.
- Keep broad search for `auto` and broad fallback behavior explicit.
- Retain generation leases for request consistency and stream completion.

A stored catalog supplies restart and recovery state.
The active process snapshot supplies inference reads.
A metadata TTL cache does not replace this generation contract.

**F2. Stored credentials repeat Argon2 on requests.** Operator material already has a memory-only [CachedMaterial path][sp-material].
Background reconciliation refreshes that material before request use.
Expiry and revocation remain part of its lifecycle.

The account and shared credential paths read an exact KV record and call [decryptMaterial][sp-stored-material].
The [encryption service][sp-encryption] derives the key again through Argon2 for each decryption.
Its configured memory cost is 64 MiB.
The router invokes this path when its credential strategy selects stored account or shared material.
It does not apply to every operator credential request.

Recommended changes:

- Extend the managed material lifecycle to stored account and shared credentials.
- Cache usable material under credential owner, record identity, revision, provider contract, and encryption-key version.
- Bound resident secrets by count, bytes, and validity time.
- Check the current account grant before a shared credential serves a request.
- Invalidate material on revocation, rotation, grant changes, and provider contract changes.
- Prevent an older refresh from restoring a revoked value.
- Coalesce concurrent refreshes for the same credential.
- Keep key derivation outside steady-state inference. Preserve the existing encryption strength.

An envelope-encryption change would need its own migration and security review.
The measured request-path problem does not require weakening Argon2 parameters.

**F3. Stable authorization data still reaches storage.** [Authentication][sp-auth] reads the gateway key by hash and then loads its account.
The [key repository][sp-apikey] uses two sequential KV reads for a successful hash lookup.
The account adds another KV read when its ID is present.
The records require JSON decoding on each request.
There is no repository memory cache around those reads in the inspected application composition.

Badger handles these reads through embedded database transactions and value copies.
With Valkey, the adapter uses `Do(GET)` for each read.
It does not use `DoCache` on this path.
A remote KV database still has network and serialization costs even when its values reside in RAM.

The [team budget callback][sp-team-callback] reads the SQL team repository when a request carries a team ID.
The [team repository][sp-team] executes a SQL query and decodes its record.
Teamless requests skip this query.
The SQL backend can be SQLite, PostgreSQL, or MySQL.

Recommended changes:

- Keep a bounded working set of validated key, account, team, and effective policy records in process memory.
- Preserve explicit record revisions and distinguish absent records from failed reads.
- Use a durable revision stream or equivalent recovery mechanism for invalidation.
- Define the maximum revocation delay and valid local authorization interval.
- If Starport cannot establish required authorization within that interval, refuse new inference.
- Coalesce cold loads and bound their deadlines.
- Keep confirmed absence separate from an unknown budget or missing authority update.

Notifications alone are insufficient after reconnects or missed events.
The accepted internal authority and withdrawal behavior still apply to every replica.

**F4. Mutable limits need a separate fast admission design.** [Rate limiting][sp-rate] currently reads and updates each applicable counter through compare-and-swap.
Contention can cause repeated attempts.
[Budget checks][sp-budget] read applicable usage totals before inference.
Each [Totals call][sp-totals] reads requests, tokens, and spend through three sequential KV operations.
Spend and token rules can repeat totals for the same scope and interval.

These reads are conditional on the applicable rules.
They are not permission to treat mutable balances as indefinitely valid cached values.
Current usage persistence is [asynchronous and bounded][sp-usage], with drops when its backlog fills.
That behavior reduces response delay, but a lossy usage feed cannot establish a strict spending guarantee.

Recommended changes:

- Resolve stable rule definitions from validated local policy.
- Use atomic admission and reservations for strict account, key, and team budgets.
- Batch applicable meters where the storage contract permits one atomic decision.
- Use bounded local quota leases only when the authority reserves their capacity first.
- Account for concurrent replicas, retries, streaming costs, expiry, crashes, and reconciliation.
- Keep analytics export asynchronous with bounded queues.
- Preserve required accounting evidence separately from disposable telemetry.

Shared atomic admission can remain on the request path.
Its latency budget must include the actual deployment's network round trips.
Ordinary TTL caching cannot preserve a global limit by itself.

**F5. Response-cache misses can delay answers.** The current [cache manager][sp-cache-manager] uses Ristretto plus the KV store for a local deployment.
Its distributed path uses the KV response cache without a local response tier.
Local model projections use a separate Ristretto cache.
Those projections serve listing surfaces. They do not remove catalog candidate reconstruction during inference.

The [chat cache middleware][sp-chat-cache] waits synchronously for a successful response to enter the cache.
It uses a separate five-second context for that write.
The response therefore waits on disposable cache storage before the controller encodes it.
The hybrid implementation also calls Ristretto `Wait()` after population and writes.
The timeout does not establish a five-second hard bound for adapters that ignore context cancellation.

The stream wrapper retains a clone of every event until completion.
It then reconstructs the response and writes the cache synchronously on EOF.
The cache item-size check occurs after accumulation and encoding.
It does not bound memory during a long stream.

Recommended changes:

- Keep the proposed local response-cache default separate from durable KV state.
- Apply account isolation, policy validity, absolute expiry, and generation identity to every cache hit.
- Bound optional cache reads by a small, explicit request budget.
- Skip an unavailable optional cache when its budget expires.
- Use a bounded, lifecycle-owned queue for optional cache writes.
- Drop cache fills under pressure while preserving the answer and required accounting.
- Remove cache population barriers from production requests after verifying admission and visibility semantics.
- Stop stream accumulation at a byte or event limit while continuing delivery.
- Avoid storing entire event histories when an incremental bounded accumulator is sufficient.
- Measure cache-disabled requests, hits, misses, oversized entries, expiry, and cache outages separately.

The [semantic cache][sp-semantic] is already opt-in.
On an exact miss, it can call an embedding model before the original request proceeds.
Its cost and latency need separate reporting.
It is not a universal mechanism for low gateway overhead.

**F6. Advisory fleet state still uses synchronous I/O.** Route planning calls availability refresh.
The [shared availability implementation][sp-health] scans peer records and reads their values when its refresh interval permits.
Health transitions can synchronously publish shared state.
The [shared latency tracker][sp-latency] can also read peers or publish a snapshot from a request callback.
Some operations use a background context without a request deadline.
The default five-second refresh interval limits frequency, but the request that starts the refresh still pays its cost.

Move advisory peer refresh and publication into bounded background workers.
Keep local breaker decisions and explicitly required admission checks synchronous.
Define a separate validity rule for authority data, since permission withdrawal cannot use advisory health semantics.

**F7. The overhead metric omits gateway work.** The [chat controller][sp-controller] starts the timer after request decoding and routing-policy extraction.
It reads the header value before final response encoding.
Authentication, rate limits, and budgets execute outside that timer.
The router subtracts complete connector calls, including connector work around network I/O.
The [performance document][sp-performance] currently describes a broader measurement than these boundaries establish.

The current CI test is useful as a narrow regression check.
It does not support a production p99 claim for the complete gateway.
Its 50 ms threshold also cannot detect many changes that matter to a low-overhead product.

Recommended measurement contract:

1. Measure the full HTTP route with authentication, applicable limits, decoding, routing, encoding, and configured observability.
2. Use controlled upstream timing to distinguish gateway work from provider delay.
3. Record stage durations with submillisecond precision.
4. Measure time to first byte and first token separately from complete response time.
5. Measure stream forwarding delay, memory growth, cancellation, and backpressure.
6. Report p50, p95, p99, and p99.9 at stated concurrency and offered load.
7. Report bytes and allocations per request and per stream event.
8. Record CPU use, garbage collection, queue depth, database calls, and connection reuse.
9. Compare equivalent direct and proxied requests without subtracting unrelated percentiles.
10. Set release budgets on fixed runners after this full-path baseline exists.

Connection setup, provider network delay, client backpressure, and optional semantic work need explicit measurement boundaries.
The primary optimization target should include uncached successful requests.
A cache hit rate alone does not establish low gateway overhead.

**Recommended storage use by deployment.** Persistent authority and memory serving state have different lifetimes.
The [storage review](STORAGE_REVIEW.md) remains the detailed storage and path inventory.
This table describes proposed request-path behavior, not completed implementation.

| Deployment | Durable owners | Request-serving memory | Required shared request work |
| --- | --- | --- | --- |
| Persistent local developer or single-server startup | Files, Badger, SQLite, local blobs | Active catalog indexes, validated policy records, usable credentials, local health, bounded response cache | None across hosts. Local admission still preserves limits. |
| Temporary local development | In-memory Badger and SQLite, isolated scratch files | Same serving concepts with explicit restart loss | No shared service requirement |
| Replicated startup or enterprise | Valkey for authoritative KV state, PostgreSQL for SQL state, shared blob storage | Each replica holds its own active indexes and bounded validated records | Atomic global admission or capacity reservations |
| Enterprise with an authoritative Starmap server | Internal catalog authority plus the selected Starport durable stores | Each replica holds the latest accepted permitted generation | No catalog fetch per inference request. Authority validity still gates service. |

Redis and MySQL remain alternatives subject to the accepted compatibility tests.
An internal Starmap server and GitHub updates belong to catalog refresh and control operations.
Turning GitHub pulls off must not add a new inference lookup.
Badger's internal caches do not remove repository transactions, value copies, or record decoding.
In-memory Badger also does not remove those operations.
Ristretto should hold disposable data, not the sole copy of active authority or required accounting state.

**Production acceptance recommendations.** Optimize allocations at measured sites before changing runtime memory settings.
Go's [garbage collector guide](https://go.dev/doc/gc-guide) explains how allocation rate and retained heap affect CPU and latency.
Keeping more data in memory can also increase heap scanning and memory pressure.
The target keeps useful state within limits and minimizes repeated work.

| Area | Proposed acceptance evidence |
| --- | --- |
| Catalog selection | Exact-model work does not grow with unrelated routes. Preserve alias, fallback, authority, and generation behavior. |
| Credentials | Warm requests perform no key derivation or external secret resolution. Verify rotation, revocation, expiry, and concurrent refresh. |
| Policy | Warm valid reads avoid database access. Verify bounded invalidation delay, missed updates, restart, and authority failure. |
| Limits | Prove global correctness under concurrent requests and failures. Record actual atomic operations and round trips. |
| Response cache | Cache failure and backlog do not delay answers beyond the stated cache budget. Preserve isolation and expiry. |
| Streaming | Memory stays within a stated bound as stream length grows. Verify timely forwarding and cancellation. |
| Fleet health | Advisory refresh performs no remote I/O on inference callbacks. Verify stale-hint behavior. |
| Performance claims | Full-path results cover warm and cold states, cache misses, tenant diversity, catalog scale, and each supported deployment. |

Reusable provider HTTP transports already support connection pooling and HTTP/2.
Preserve that behavior while testing saturation and reconnects.
Go's [Transport documentation](https://pkg.go.dev/net/http#Transport) recommends reusing transports.
Arbitrary request bodies and protocol encoding still need memory.
Zero allocations for every complete request is not an established requirement or measured capability.
Zero avoidable catalog copies and no repeated steady-state credential derivation are concrete initial targets.

The review did not test live providers, remote storage services, Linux, Windows, or production percentile latency.
No product optimization is complete from these measurements alone.

[sp-candidates]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/router/planner_adapter.go#L262
[sm-offering]: https://github.com/agentstation/starmap/blob/v0.16.5/pkg/catalogs/readonly.go#L267
[sm-resolve]: https://github.com/agentstation/starmap/blob/v0.16.5/pkg/catalogs/providers.go#L75
[sp-material]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/credentials/resolver.go#L299
[sp-stored-material]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/providers/keyring/provider_keys.go#L148
[sp-encryption]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/credentials/encryption.go#L94
[sp-auth]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/server/middleware.go#L228
[sp-apikey]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/apikey/repository.go#L328
[sp-team-callback]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/server/server.go#L205
[sp-team]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/identity/repository.go#L325
[sp-rate]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/ratelimit/repository.go#L83
[sp-budget]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/server/budget.go#L19
[sp-totals]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/usage/repository.go#L323
[sp-usage]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/proxy/usage_capture.go#L104
[sp-cache-manager]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/cache/manager.go#L73
[sp-chat-cache]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/proxy/cache.go#L69
[sp-semantic]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/proxy/semantic_cache.go#L85
[sp-health]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/availability/shared.go#L162
[sp-latency]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/router/latency_shared.go#L52
[sp-controller]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/server/controllers/chat.go#L41
[sp-performance]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/docs/PERFORMANCE.md
