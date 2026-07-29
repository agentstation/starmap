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
3. `GET /catalog/generations/{generation_id}/snapshot` returns the exact
   immutable canonical payload as
   `application/vnd.agentstation.starmap.catalog+json`.
4. `GET /updates/stream` returns `text/event-stream`. Its sole data event is
   `catalog.published`, containing only `generation_id` and the matching
   positive SSE `id`/`sequence`. Flushed `connected` and `heartbeat` comments
   carry no event ID or catalog data.

The retained manifest and payload requests are generation-addressed so a
concurrent server publication cannot mix bytes from different generations.
The client bounds both bodies, requires exact media types, strictly parses and
validates the manifest, rejects an incompatible catalog-schema range before
downloading the snapshot, then verifies payload size and SHA-256 before decode
or durable commit. An HTTP failure, malformed/unknown manifest member, wrong
media type, incompatible schema, truncated/oversize body, corrupt checksum, or
semantic decode error leaves the current catalog and durable store untouched.

Remote updates preserve the received generation and sync-run identities rather
than minting a second local identity. Commit remains compare-and-swap and the
immutable catalog pointer changes only after the exact received generation is
durable. An optional API key is applied to both requests by a request-cloning
transport so caller requests are not mutated.

The old unversioned `GET /catalog` ad-hoc envelope is removed. Protocol tooling
may explicitly construct `catalogremote.Client`, fetch a current or addressed
generation, and pass it to `starmap.Client.Activate`. Normal reactive consumers
use the opt-in `github.com/agentstation/starmap/remote` package:

```go
subscriber, err := remote.New(remote.Config{BaseURL: baseURL})
if err != nil {
	return err
}
if err := subscriber.Start(ctx); err != nil {
	return err
}
defer subscriber.Close()

catalog := subscriber.Catalog()
```

`New` validates and starts no goroutine or network request. `Start` verifies and
activates the initial current generation, establishes SSE, then immediately
refetches current state to close the fetch-to-subscribe gap. Each reconnect
sends the last accepted event ID but treats replay only as an optimization:
successful connection establishment is always followed by another verified
current-state catch-up. Duplicate generation IDs and identical payload digests
do not republish the immutable catalog. An older retained event cannot regress
the active generation.

Importing the root `starmap` package never enables remote I/O or silently
changes update behavior.
