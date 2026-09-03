# Remote Catalog Protocol

The Starmap-to-Starmap online protocol uses a versioned API base URL such as
`https://catalog.example.com/api/v1`. It is distinct from the signed release,
hosted CDN, and OCI artifact distribution channels.

The verified catalog and reactive notification flow has five routes:

1. `GET /catalog/manifest` returns the current strict
   `GenerationManifest` as
   `application/vnd.agentstation.starmap.catalog-manifest+json`.
2. `GET /catalog/generations/{generation_id}/manifest` returns the retained
   immutable manifest for one publication event's generation ID.
3. `GET /catalog/generations/{generation_id}/payload` returns the exact
   immutable canonical payload as
   `application/vnd.agentstation.starmap.catalog+json`.
4. `GET /updates/stream` returns `text/event-stream`. Its sole data event is
   `catalog.published`, containing only `generation_id` and the matching
   positive SSE `id`/`sequence`. Flushed `connected` and `heartbeat` comments
   carry no event ID or catalog data.
5. `GET /catalog/source-chain` returns the served source chain as
   `application/vnd.agentstation.starmap.source-chain+json`. The document
   carries the schema version, the safe instance identity, and the source
   identity. It also carries both health codes, the generation ID,
   `channel_updated_at`, `observed_at`, and one entry for each disclosed hop.
   The document names no URL, no credential, and no operator message. The
   reply header is `no-store`.

A downstream reads the source chain to grade the propagated origin freshness
and to reject a cascade that names itself. The document discloses at most 16
hops, so one node cannot grow the chain without bound.

A runtime rejects four chains. It rejects a chain that repeats an identity. It
rejects a chain that names the reader. It rejects a chain that names a
configured alias of the reader. It rejects a chain longer than the hop budget.
The `STARMAP_CATALOG_SOURCE_MAX_HOPS` setting holds that budget and defaults
to 8.

The retained manifest and payload requests use generation addresses. A
concurrent server publication therefore cannot mix bytes from different
generations. The configured API origin is the online publisher identity.

Non-loopback publishers must use HTTPS. Every HTTPS response must carry a
completed, standard-library-verified certificate chain. It must remain on that
exact origin. Starmap retains loopback HTTP only for local embedding and tests.

The client bounds both bodies and requires exact response and descriptor media
types. It strictly parses and validates the manifest before payload download.
It also rejects an incompatible catalog-schema range or oversized descriptor.
The generation ID must be a bounded canonical path segment. The client verifies
payload size and SHA-256 before decode or durable commit.

Any verification or transport failure leaves the current catalog and durable
store untouched. This includes malformed manifest data, unsafe identity,
truncated data, a corrupt checksum, and semantic decode errors.

Remote updates preserve the received generation and sync-run identities rather
than minting a second local identity. Commit remains compare-and-swap and the
immutable catalog pointer changes only after the exact received generation is
durable. A newer identity for the same payload commits and advances the atomic
identity. It emits one generation event without replacing the catalog pointer
or publishing model-change hooks. A request-cloning transport applies an
optional API key to both requests. It does not change the caller requests.

Starmap no longer supports the old unversioned `GET /catalog` ad-hoc envelope.
Protocol tooling can import `github.com/agentstation/starmap/pkg/catalogs/remote`
as `protocol`. It can construct `protocol.Client`, fetch a current or addressed
generation, and pass it to `starmap.Client.Activate`.

`starmap.Open` is the normal consumer path. It selects this protocol with
`STARMAP_CATALOG_SOURCE=starmap` and the endpoint in
`STARMAP_CATALOG_SOURCE_URL`. The connected runtime owns the startup read, the
stream, the conditional polling fallback, the retained layers, and the served
source chain. [ARCHITECTURE.md](ARCHITECTURE.md#connected-catalog-runtime)
defines every setting and default.

A consumer that owns its own catalog store instead uses the
`github.com/agentstation/starmap/remote` package:

```go
store, err := storage.NewFilesystem(statePath)
if err != nil {
	return err
}
subscriber, err := remote.NewContext(ctx, remote.Config{
	BaseURL: baseURL, CatalogStore: store,
})
if err != nil {
	return err
}
if err := subscriber.Start(ctx); err != nil {
	return err
}
defer subscriber.Close()

state := subscriber.State()
```

Each subscriber requires a `CatalogStore` that the caller owns. `NewContext`
loads and verifies its current generation under the caller context. An optional
`PinnedBootstrap` commits only when the store is empty. A durable current
generation wins over the pin. Corrupt or unavailable durable state causes
construction to fail.

`New` is the background-context convenience wrapper. Both constructors start
no goroutine or network request. `State` returns one atomic catalog, generation
ID, payload checksum, generation timestamp, and process-local sequence.

`Start` normally verifies and activates the remote current generation. It then
establishes SSE and refetches current state to close the fetch-to-subscribe gap.
A nonterminal initial fetch, stream-open, or catch-up failure keeps the verified
local state and starts streaming recovery. HTTP 401 and 403 remain terminal.

Each reconnect sends the last accepted event ID. Replay only improves
efficiency. After each connection, the subscriber fetches and verifies current
state again. Duplicate generation IDs do not republish. A digest-equal new
identity publishes its generation without copying the catalog or emitting model
changes. An older retained event cannot regress the active generation.

The subscriber expects the server's 20-second default heartbeat and uses a
60-second default liveness deadline. Both are configurable. Configuration must
leave room for at least two expected heartbeat intervals. Every comment or
publication resets the liveness deadline, but comments never trigger a catalog
fetch or advance publication identity. Silence closes the response body and
joins the per-stream reader. The subscriber then reconnects and fetches the
current manifest.

The caller context owns
initial fetch, streaming, retries, activation, and termination. `Close` cancels
that context and joins the owned lifecycle. A configurable five-second default
bounds that wait. A timeout returns a typed error.

Stream parsing rejects a line larger than 64 KiB or a cumulative event frame
larger than 256 KiB. A supplied `Last-Event-ID` must be a positive unsigned
integer. The client rejects another value before network I/O. These fixed bounds keep
an untrusted publisher from growing subscriber memory through one fragmented
event.

Importing the root `starmap` package never enables remote I/O or silently
changes update behavior. `starmap.New` stays offline. `starmap.Open` is the
explicit connected constructor. It reads the configured catalog source at
startup under the configured startup policy.
