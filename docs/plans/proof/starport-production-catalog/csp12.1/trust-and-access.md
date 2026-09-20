# CSP12.1 cache trust and namespace access

Starport `51767baeb929829149c23a4ff260cd9544661a6e` adds explicit trust roots, namespace access checks, and fixed diagnostic codes.
The [qualification record](trust-and-access.json) binds local evidence to the source and lists remaining work.

## Behavior

`STARPORT_CACHE_CA_FILE` selects a bounded PEM bundle for TLS connections.
The canonical path resolver anchors relative paths only with explicit configuration-directory selection.
The file inventory reports `cache-ca` under deployment-controlled access.
The confined reader accepts regular files up to 1 MiB. Startup rejects unreadable or invalid bundles without exposing their contents.
Trust changes require restart.

A connection monitor checks read and write access to its namespace through a short-lived reserved probe.
The server ACL defines access. Starport cannot grant itself access to another namespace.
Tests use two distinct scoped credentials and confirm that denied writes preserve the other owner's cached record.

Fixed diagnostic codes distinguish TLS trust, hostname, certificate validity, authentication, and namespace failures.
Caller cancellation does not mark a healthy cache unavailable. The regression fails before that repair and passes afterward.
Optional cache failure does not change gateway permission or budget admission.

## Evidence

The final cache run passes 70 race-test events and skips `TestValkeyPubSubReconnection`, which requires manual service restart.
Both the new shared-cache adapter and the legacy backend tests reach a real disposable Valkey service.
Nine final targeted service and application events pass without skips.
The preceding configuration, cache, and architecture run passes 396 events with three service-dependent skips.
It preceded the final diagnostic changes. The final real-cache run covers those changes.

TLS terminates in a local Go relay before real Valkey traffic.
Trusted roots permit reads and writes. Unknown roots, expired certificates, and wrong hostnames prevent cache protocol traffic.
These checks do not qualify a native Valkey TLS deployment or other host platforms.
Lint, vet, whitespace, and eight-file prose checks pass. The agent stopped the disposable service.

## Coherence findings and next work

The candidate still exposes an independent cache namespace. The target requires the canonical deployment identity to own that prefix.
CSP12.1 must correct this before production acceptance. Keep the service ACL as the enforcement boundary.

The layered, hybrid, and durable-KV cache adapters have no production callers in the current Starport tree.
The only production cache imports are application composition and admin diagnostics.
Remove these unused internal adapters after preserving their historical evidence.
Verify expiry and isolation on the actual application cache paths. Do not count unreachable adapter tests as full product acceptance.

Model and extraction fills, complete capacity and latency, discovery acceptance, review, native CI, and merge remain open.
