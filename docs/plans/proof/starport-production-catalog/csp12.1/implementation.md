# CSP12.1 local cache implementation

Local implementation commit is `ba68306420a94c39b5f0dcb5ca76a92eff75bc1a`.
The [qualification record](local-isolation.json) records 910 passing race-test events and seven explicit skips across six packages.
Vet, lint, the document-parser guard, and the pure-Go build pass.

The preparation branch is `codex/starport-cache-storage`, based on reviewed Starport `31feeca`.
The worktree is `/Users/jack/src/github.com/agentstation/starport-cache-storage`.
The paired workspace is `/tmp/csp121-host-integration.work`, with Starmap `152148130`.

## Implemented boundary

The cache manager now accepts `cache.Cache` instead of `storage.KVStore`.
Application composition supplies no remote cache by default, regardless of the durable backend.
The response cache uses local memory. Injected cache-only stores require the explicit `distributed` strategy.
The manager closes both cache stores and reports close errors.

Extraction caching uses a separate local store with application-owned cleanup.
It retains its independent lifecycle when the operator disables response caching.
The response, model, and extraction Ristretto capacities are 256 MiB, 16 MiB, and 16 MiB.
These values describe cache cost limits, not measured total heap or request overhead.

Tests preserve explicit shared-cache delivery through a cache-only adapter.
New tests cover local isolation, invalid shared configuration, real-Badger composition, and extraction without a durable store.
The original test fixtures needed a seeded gateway API key and an explicit distributed-cache adapter.
Those repairs preserve the authorization and serialized-discovery assertions.

## Remaining scope

CSP12.1 remains incomplete. Shared-service operator configuration and lifetime-preserving reads still require implementation.
The legacy layered and hybrid cache constructors remain outside application composition.
Their six refill failures retain their original evidence. They need replacement or repair, not an exemption.

Bounded asynchronous fills, cancellable optional reads, stream bounds, and the discovery audit remain required.
No pre-PR review, hosted qualification, merge, or release qualification applies to this preparation branch yet.
