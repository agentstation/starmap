# Offline server probe

Run `python3 scripts/cold_server.py` to verify baseline persistence through the actual Starmap HTTP server.
The check requires Go and a Linux AMD64 or ARM64 Docker engine.
It builds the current CLI and observer with Go 1.25.12 for the engine's native architecture.

Two fresh containers start the server against one private named volume.
Each container has no external network, no provider credentials, a read-only root, and an unprivileged user.
The default public source remains configured. Offline startup must serve the embedded baseline.
The observer checks readiness, the catalog manifest, and the catalog payload through HTTP.
It validates both manifests with Starmap's parser and compares served payload bytes with the saved baseline and manifest digest.

Cold startup and restart must retain the same generation, payload digest, instance identity, and volume.
Baseline files must have mode 0600. Both server processes must stop normally.
The report includes actual command results, binary hashes, container isolation, server logs, and cleanup results.
The check removes only its own containers, volume, and image. Docker can retain its normal build cache.

Missing tools, incomplete observations, and resource failures remain unverified.
A failed server contract or failed cleanup fails the check.
The unit tests reject altered digests, exposed files, changed identities, credential injection, and external network configuration.

This check supplies Linux component evidence for `A01.starmap_cold_offline`.
It does not qualify Starport, native Windows or macOS, a released pair, or all of A01.
The Pull Request workflow runs it separately and retains `cold-server-linux-amd64`.
