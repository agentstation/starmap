# CSP12.1 refill lifetime repair

Starport `d651147e2b665a6d13603439720c889861552f2d` repairs the six recorded refill failures.
The [qualification record](lifetime.json) records 254 passing race-test events and one manual Valkey restart skip.
Both cache and storage packages pass with real Badger and a disposable Valkey service.
Lint, vet, and technical-writing checks pass. No service or test session remains active.

## Contract

Badger reads the value and expiry from one transaction snapshot. Valkey reads the value and PTTL in one Lua operation.
Both adapters enforce a byte bound before returning the payload.
Refills anchor a local deadline before the backing read. Transfer delay consumes the lifetime.
Backends without finite lifetime evidence skip local refill.

Every retained entry carries its source deadline. Cache hits check that deadline independently of delayed insertion or eviction.
Get, GetMulti, Warm, and Exists use the same validity path.
Successful backing writes invalidate local entries. A later read proves the stored version's actual expiry, including backend rounding.

## Evidence and limits

The original six Badger assertions now run in the cache package.
Additional tests cover unknown lifetime, delayed reads, retained expired entries, and Valkey expiry.
Backend tests cover payload bounds, cancellation, absence, non-expiring records, and concurrent value/TTL replacement.

The delayed-read test uses a controlled delay. The retained-entry test simulates delayed eviction with an expired source deadline.
These tests do not qualify native suspend or multi-host timing.
Batch refill now uses per-entry lifetime reads. Remote batch performance remains unqualified.

Application composition still uses local memory by default. The legacy layered constructors remain outside that composition.
CSP12.1 still owns dedicated service configuration, bounded fills, optional read budgets, stream limits, capacity, and discovery checks.
The task remains incomplete and unmerged.
