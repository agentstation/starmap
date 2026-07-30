# Starmap HTTP API

The public [`server`](../server) package and `starmap serve` expose the same
versioned HTTP API. The default base URL is `http://localhost:8080/api/v1`.
The embedded, reproducibly generated OpenAPI documents are the normative schema
for query parameters and native JSON response bodies:

```text
GET /api/v1/openapi.json
GET /api/v1/openapi.yaml
```

This document owns the composition, security, immutable-generation, and
reactive-delivery semantics that are not usefully duplicated from OpenAPI.

## Start the standalone server

```bash
starmap serve
starmap serve --host 0.0.0.0 --port 8080
starmap serve --auth --cors-origins https://console.example.com
```

The CLI composes its filesystem generation store and explicit acquisition
syncer before constructing the public server. An embedding Go application
constructs its `*starmap.Client`, chooses a filesystem or conditional
S3-compatible store, and passes the client to `server.New`. Server construction
does not open storage, discover credentials, bind a listener, or start a
goroutine.

Current flags:

| Flag | Default | Meaning |
| --- | --- | --- |
| `--host` | `localhost` | Bind address |
| `--port` | `8080` | TCP port |
| `--prefix` | `/api/v1` | Versioned API path prefix |
| `--auth` | `false` | Require the `API_KEY` value on protected routes |
| `--auth-header` | `X-API-Key` | Primary API-key header |
| `--cors` | `false` | Enable CORS |
| `--cors-origins` | empty | Explicit origin allowlist; empty with CORS enabled permits all |
| `--rate-limit` | `100` | Requests per minute per client IP; zero disables |
| `--cache-ttl` | `300` | Derived response-cache TTL in seconds |
| `--read-timeout` | `10s` | HTTP read timeout |
| `--write-timeout` | `10s` | HTTP write timeout |
| `--idle-timeout` | `120s` | HTTP idle timeout |
| `--sse-heartbeat-interval` | `20s` | Flushed SSE comment cadence |
| `--sse-write-timeout` | `10s` | Per-frame SSE write and flush deadline |
| `--metrics` | `true` | Expose `/metrics` |

`HTTP_HOST` and `HTTP_PORT` override the corresponding CLI flags. `API_KEY`
provides the expected authentication value. Provider API credentials are
separate acquisition inputs and are never server API keys.

## Routes

The configured prefix replaces `/api/v1` in every versioned route.

| Method and path | Contract |
| --- | --- |
| `GET /health` | Process liveness; always public |
| `GET /api/v1/health` | Versioned process liveness; always public |
| `GET /api/v1/ready` | Catalog readiness, including embedded-bootstrap policy; always public |
| `GET /api/v1/models` | Paginated/filterable canonical model and offering rows |
| `GET /api/v1/models/{id}` | Canonical model definition lookup |
| `POST /api/v1/models/search` | Structured model search |
| `GET /api/v1/providers` | Provider list |
| `GET /api/v1/providers/{id}` | Provider detail |
| `GET /api/v1/providers/{id}/models` | Exact provider offerings |
| `POST /api/v1/update` | Explicit acquisition; present only when a syncer was composed |
| `GET /api/v1/stats` | Catalog, cache, runtime, callback, and SSE health |
| `GET /api/v1/catalog/manifest` | Strict current generation manifest |
| `GET /api/v1/catalog/generations/{id}/manifest` | Retained immutable generation manifest |
| `GET /api/v1/catalog/generations/{id}/payload` | Retained immutable canonical payload |
| `GET /api/v1/updates/stream` | Heartbeat-enabled catalog publication hints |
| `GET /api/v1/model/{author}/{slug}` | OpenRouter-compatible model response |
| `GET /api/v1/models/{author}/{slug}/endpoints` | OpenRouter-compatible provider endpoints |
| `GET /api/v1/openapi.json` | Generated OpenAPI JSON; always public at the default prefix |
| `GET /api/v1/openapi.yaml` | Generated OpenAPI YAML |
| `GET /metrics` | Process metrics when enabled |

Model/list responses carry `X-Starmap-Generation-ID`, so a caller can associate
derived results with the immutable catalog generation used to produce them.
The OpenRouter routes are server-local projections over the same catalog. They
do not read generated `endpoints.yaml`, persist another representation, or
invent runtime provider telemetry.

## Native response envelope

Native JSON endpoints return:

```json
{
  "data": {},
  "error": null
}
```

Failures set `data` to `null` and return an error with a stable code, safe
message, and optional details. HTTP status remains authoritative. The
OpenRouter-compatible routes intentionally use OpenRouter's response and
numeric error dialect instead of the native envelope.

## Authentication and CORS

When `--auth` is enabled, send either the configured header or an Authorization
value:

```bash
curl -H 'X-API-Key: ...' http://localhost:8080/api/v1/models
curl -H 'Authorization: Bearer ...' \
  http://localhost:8080/api/v1/models
```

The health/readiness probes and default-prefix OpenAPI JSON route remain
public. Authentication comparison is constant-time; logs record only whether a
key was supplied, never the key. OpenRouter routes return the OpenRouter 401
error dialect, while native routes return Starmap's native 401 envelope.

Enable CORS only with a deployment-owned origin policy. An explicit
`--cors-origins` allowlist echoes only matched origins and adds `Vary: Origin`.
Using `--cors` without an allowlist emits `Access-Control-Allow-Origin: *`.

## Immutable generation protocol

The current manifest is mutable discovery state and supports conditional
requests with `ETag`. Addressed manifests and payloads are immutable retained
objects. Payload responses use the catalog payload media type and immutable
cache headers. A consumer must verify manifest schema compatibility, payload
media type, size, digest, and identity before activation.

Use the public [`remote`](../remote) package for the complete verified
initial-fetch, SSE, reconnect/catch-up, durable activation, and shutdown
lifecycle. The exact wire contract is documented in
[REMOTE_CATALOG_PROTOCOL.md](REMOTE_CATALOG_PROTOCOL.md).

## Reactive updates

`GET /api/v1/updates/stream` is the sole Starmap catalog-publication transport.
The only data event is:

```text
id: 42
event: catalog.published
data: {"generation_id":"...","sequence":42}
```

`connected` and `heartbeat` comments carry no event ID or catalog bytes. An
event is a fetch hint, not a catalog payload. Each connection has one
write-deadline-aware writer. Backpressure or write failure terminates the
connection so the client reconnects and performs mandatory current-manifest
catch-up; the server never silently drops a hint while reporting a healthy
stream.

Polling is not the normal path. The remote subscriber can enable a bounded,
observable conditional-polling fallback only after an explicit number of
stream failures.

## Health semantics

Liveness, catalog readiness/freshness, callback delivery, and SSE delivery are
separate surfaces:

- `/health` answers whether the process and HTTP handler are alive.
- `/ready` checks that a usable catalog satisfies configured bootstrap policy.
- `/stats` exposes the active generation time, callback coalescing/failure, SSE
  clients, backpressure termination, and cache/runtime data.

A recent heartbeat proves stream transport activity only. It never changes the
active catalog generation time or makes stale catalog data fresh.
