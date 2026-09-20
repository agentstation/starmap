# CSP12.1 dedicated shared response cache

Starport `6fc0a2cd54ed66ce232002b1ffa518dfa4712e78` adds an explicit optional cache service without the durable KV handle.
The [qualification record](shared-service.json) identifies the tested source and remaining acceptance work.

## Configuration and behavior

`STARPORT_CACHE_BACKEND` selects local memory by default or the Valkey adapter explicitly.
The shared mode requires `STARPORT_CACHE_URL` and `STARPORT_CACHE_NAMESPACE`.
It accepts URI credentials and database selection. TLS uses system roots and hostname verification.
Remote plaintext requires `STARPORT_CACHE_ALLOW_INSECURE=true`. Loopback permits plaintext.

Query options, fragments, invalid namespaces, and invalid database numbers fail validation without secret values.

Keys use `starport:cache:v1:<namespace>:`. Configuration refuses the declared durable KV server, including common loopback aliases.
Different databases cannot isolate eviction. DNS aliases and actual service isolation still need deployment qualification.
Scratch development rejects shared-cache settings before connecting.
Model and extraction caches remain local.

Connection work runs in one background owner. Attempts enforce dial and connection timeouts.
Initial failure permits startup and retries once per second. Optional reads fail through the existing miss path.
Existing permission and budget admission remain mandatory.

The dedicated client disables automatic read retry and client-side caching, and bounds connection buffers.
The admin response exposes configured and available flags without URI values.
Shutdown cancels and joins connection work before closing the client.

## Evidence

The configuration package passes 321 race-test events.
The cache and connection packages pass 57 events with three explicit legacy-service skips.
The final targeted shared-service command passes four events without skips.
It proves namespace isolation, credentials, database selection, expiry, oversized-value refusal, reconnect, and application wiring.
The silent-endpoint check exercises connection shutdown while the server withholds its handshake response.

Eight architecture events and three application checks pass.
Lint reports zero issues. Vet reports no diagnostics. Prose passes for seven files.
All six dependency checks pass. Fourteen action references match their release tags.

CI and the integration Make target now execute the real shared-cache cases.
Hosted CI has not run for this candidate.

One retained run failed during fixture cleanup after a broad disconnect also killed a control connection.
The final fixture disconnects only its own selected cache connection by ID.
The corrected complete cache run and final targeted service run pass.
The disposable service is no longer active.

## Remaining scope

Custom-CA configuration and real TLS service tests remain required.
Namespace ownership, model and extraction fills, concurrent capacity, complete latency, and discovery acceptance remain open.
The existing durable Valkey adapter still requires its separate CSP12 URI and effective-option repair.
No full task completion, review, publication, native CI, or merge follows from this local evidence.
