# Remote Catalog Protocol

The Starmap-to-Starmap online protocol uses a versioned API base URL such as
`https://catalog.example.com/api/v1`. It is distinct from the signed release,
hosted CDN, and OCI artifact distribution channels.

The verified catalog and reactive notification flow has four routes:

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

The retained manifest and payload requests are generation-addressed so a
concurrent server publication cannot mix bytes from different generations.
The configured API origin is the online publisher identity. Non-loopback
publishers must use HTTPS; every HTTPS response must carry a completed,
standard-library-verified certificate chain and remain on that exact origin.
Loopback HTTP is retained only for local embedding and tests. The client bounds
both bodies, requires exact response and descriptor media types, strictly
parses and validates the manifest, and rejects an incompatible catalog-schema
range or oversized descriptor before downloading the payload. It then
requires a bounded canonical path-segment generation ID and verifies payload
size and SHA-256 before decode or durable commit. An HTTP failure, unverified
publisher, malformed/unknown manifest member, wrong media type, incompatible
schema, unsafe identity, truncated/oversize body, corrupt checksum, or semantic
decode error leaves the current catalog and durable store untouched.

Remote updates preserve the received generation and sync-run identities rather
than minting a second local identity. Commit remains compare-and-swap and the
immutable catalog pointer changes only after the exact received generation is
durable. A newer identity for the same payload commits and advances the atomic
identity. It emits one generation event without replacing the catalog pointer
or publishing model-change hooks. A request-cloning transport applies an
optional API key to both requests. It does not change the caller requests.

Starmap no longer supports the old unversioned `GET /catalog` ad-hoc envelope.
Protocol tooling
may explicitly construct `catalogremote.Client`, fetch a current or addressed
generation, and pass it to `starmap.Client.Activate`. Normal reactive consumers
use the opt-in `github.com/agentstation/starmap/remote` package:

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
60-second default liveness deadline. Both are configurable; configuration must
leave room for at least two expected heartbeat intervals. Every comment or
publication resets the liveness deadline, but comments never trigger a catalog
fetch or advance publication identity. Silence closes the response body, joins
the per-stream reader, and enters reconnect/catch-up. The caller context owns
initial fetch, streaming, retries, activation, and termination. `Close` cancels
that context and joins the owned lifecycle within a configurable five-second
default, returning a typed timeout instead of waiting forever.

Stream parsing rejects a line larger than 64 KiB or a cumulative event frame
larger than 256 KiB. A supplied `Last-Event-ID` must be a positive unsigned
integer and is rejected before network I/O otherwise. These fixed bounds keep
an untrusted publisher from growing subscriber memory through one fragmented
event.

Importing the root `starmap` package never enables remote I/O or silently
changes update behavior.
