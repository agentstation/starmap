# Remote Catalog Protocol

The Starmap-to-Starmap online protocol uses a versioned API base URL such as
`https://catalog.example.com/api/v1`. It is distinct from the signed release,
hosted CDN, and OCI artifact distribution channels.

The complete read flow has two routes:

1. `GET /catalog/manifest` returns the current strict
   `GenerationManifest` as
   `application/vnd.agentstation.starmap.catalog-manifest+json`.
2. `GET /catalog/generations/{generation_id}/snapshot` returns the exact
   immutable canonical payload as
   `application/vnd.agentstation.starmap.catalog+json`.

The second request is generation-addressed so a concurrent server publication
cannot mix a newer payload with the manifest already selected by the client.
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

The old unversioned `GET /catalog` ad-hoc envelope is removed. Consumers should
configure `WithRemoteServerURL` or `WithRemoteServerOnly` with the versioned API
base URL, not just the origin.
