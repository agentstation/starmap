# Valkey and Redis coordination records

This package stores bounded coordination records with atomic conditional writes.
It implements `storage.ObjectBackend` and `storage.CurrentObjectReader` for a caller-supplied `valkey.Client`.
The constructor validates local configuration without network access.

The caller owns client creation, authentication, TLS, retry settings, and shutdown.
Importing the passive Starmap library does not import this adapter or create its client.

## Storage contract

Each record has an opaque version and complete data bytes.
Writes require `IfAbsent` or `IfVersion`. A successful write assigns a new version even when the bytes remain identical.
Conflicting conditions return a typed conflict without changing the record.
An interrupted request can have an ambiguous outcome. Read current state before deciding whether to retry.

Reads execute on the server without client-side caching.
Each atomic script checks the primary role, persistent lifetime, record shape, and size before returning or changing data.
Replica access, expiring records, invalid records, and oversized records cause refusal.
The adapter does not repair these conditions.

`Config.Prefix` selects the caller's namespace. Physical keys append the SHA-256 digest of the logical record key.
`Config.MaxObjectBytes` defaults to 8 MiB and cannot exceed 32 MiB.
These bounds apply to coordination records, not catalog payloads.

## Deployment requirements

Coordination records are durable state. Use persistent primary storage with `maxmemory-policy noeviction` and no expiration on these keys.
The qualification setup uses AOF with `appendfsync always`.
The adapter checks individual record expiration but does not validate the server's persistence or eviction configuration.

The client needs `EVAL`, `INFO replication`, `PTTL`, `TYPE`, `HLEN`, `HGET`, `HEXISTS`, `HSTRLEN`, and `HSET` access to its namespace.
The caller must keep credentials and transport configuration separate from stored records.

This adapter alone does not add object-store collection or generation leases.
The [coordinated object store](../../../../docs/COORDINATED_OBJECT_STORE.md) binds publication, reader protection, and retirement to these records.
Automatic failover and recovery after lost coordination state require separate qualification.

## Local qualification

Set `STARMAP_TEST_VALKEY_ADDRESS` to a dedicated test server, then run:

```sh
go test -race -count=1 -timeout=5m ./pkg/catalogs/storage/valkey
```

Without that variable, integration tests skip network access.
Server-change tests also require `STARMAP_TEST_COORDINATION_SERVER_CHANGES=1`.
These tests change replication settings. Use an isolated server that contains no application data.

The restart test also requires `STARMAP_TEST_COORDINATION_CONTAINER` to name that server's Docker container.
Use a fixed published port so the endpoint remains unchanged after restart.
It restarts the container, waits for readiness, and verifies retained bytes and conditional versions through a new connection.
Run one qualification suite per server at a time.
