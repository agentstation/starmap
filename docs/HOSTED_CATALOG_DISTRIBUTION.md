# Hosted Catalog Distribution

The canonical hosted origin is `https://starmap.agentstation.ai`. Its transport
contract is independent of the legacy Starmap server update API:

| Route | Meaning |
| --- | --- |
| `GET /v1/catalogs/latest?schema_version=2&channel=stable` | Small strict JSON pointer selecting the channel's exact schema-v2 generation; omitted channel defaults to `stable` |
| `GET /v1/catalogs/{generation}` | Immutable deterministic catalog archive |
| `GET /v1/catalogs/{generation}/attestation` | Detached in-toto statement for the same archive |

The latest pointer includes its own version, explicit `dev`, `canary`, or
`stable` channel, generation ID, exact catalog schema, and exact
URL/media type/SHA-256/size descriptors
for archive and statement. It does not contain a Starmap or Starport binary
version.

A consumer submits schema version 2 to `latest`. The handler, pointer, client,
downloaded manifest, and payload must all equal the current schema version;
neither side negotiates a range or accepts an older prelaunch payload. No
pointer or selection code has a Starmap version, Starport version, release tag,
or binary-version field.

`catalogdistribution.Client.FetchLatest` bounds every body, strictly parses the
pointer, requires all asset URLs to remain on the configured origin, verifies
media types/sizes/checksums, opens the artifact and detached statement, and
requires the downloaded manifest identity and exact schema to equal the
pointer. No catalog is returned before all checks pass.

`catalogdistribution.Handler` and its repository boundary provide the serving
side. Published generation IDs are immutable; exact publication retry is
idempotent and ID rebinding is a typed conflict. `MemoryRepository` is the
conformance/reference adapter. Object-storage/CDN deployment and hosted runtime
evidence are separate deployment/closeout gates.

Generation archive and attestation responses use content-derived ETags and
`public, max-age=31536000, immutable`. The latest pointer uses a content-derived
ETag and `public, max-age=60, must-revalidate`; matching conditional requests
return 304 with no body. Rollback atomically changes only the latest selection.
All previously published generations remain addressable and immutable.

## Promotion control

Publication and promotion are separate operations. A verified immutable
generation is first selected in `dev`. `canary` accepts only the exact generation
currently selected in `dev`. `stable` accepts only the exact canary generation
after a recent hosted canary probe proves all of the following against the public
HTTP protocol:

- the pointer, archive, and attestation are available and fully verifiable;
- the pointer channel, generation ID, and archive checksum match the candidate;
- generation age is within the configured freshness SLO;
- probe latency and evidence age are within their configured SLOs.

The default budgets are seven days of generation age, five minutes of probe
age, and two seconds of probe latency. Deployments may supply stricter positive
budgets. Failed order or SLO attempts are recorded alongside successful
promotions. Every event has a monotonic sequence, action, channel, before/after
generation, observation time, outcome, and reason.

Rollback changes one channel pointer atomically to a generation previously
served by that same channel. It requires an operator reason, retains newer
immutable generations and other channel pointers, and emits the same telemetry.
This makes rollback immediate without granting an arbitrary or never-observed
generation stable authority.
