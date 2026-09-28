# Budget measurement and endpoint binding

CSP12.2 remains in progress. Consumer `dcad9e2b68f801f463b604390532fd2b6aa46a26` removes a provider-model copy from each endpoint binding.
Producer `89e818f854a79660c8a3c3bb69945679dddc39bb` remains unchanged.
The [verification record](verification.json) binds the source, results, limits, and remaining work.

## Verified behavior

The separated fleet regression command passes 80 results. The adoption command passes 22 results.
Both commands finish within their existing seven-minute limits. These runs precede the registry change.

The profile attributes about 499 MiB to runtime endpoint binding across the sampled run.
The binding method reads the complete provider record, including its model inventory, for each request.
The registry now retains a private inference contract before it publishes each generation.
Request binding still uses the leased generation, selected credential bindings, and permitted operator override.

The allocation regression fails before the change: leased binding uses 6,317 allocations, while direct binding uses two.
After the change, both paths use two allocations in the pure-Go check and three with race detection.
Generation replacement retains the old lease contract. A new lease uses the replacement contract.
Account credentials cannot inherit the operator override. Missing providers remain unavailable.

The registry suite passes 17 race results. Credential and destination tests pass 58 race results.
Four pure-Go results pass. The final real-storage budget test passes with race detection.
No final focused check fails or skips a test.

## Measurement scope

The [measurement summary](measurement-summary.json) retains every timing percentile and operation count.
The first diagnostic measures 1,000 warm requests. The final diagnostic measures 200 warm requests.
Both use one replica, five budget meters, real Valkey and PostgreSQL, and a loopback provider.
The catalog contains 612 routes. Application maintenance loops remain off, while catalog background work remains active.

Whole-process allocation averages fall from 807,169 to 287,031 bytes per request.
Allocation counts fall from 8,982 to 2,664 per request.
These totals include the test client, fixture provider, and instrumentation. They do not qualify gateway allocation limits.

Every measured request retains ten budget storage calls before dispatch and five during settlement.
The final run verifies 201 settled attempts, including warmup, with no remaining reservations.
Its encoded budget records occupy 665,186 bytes across 217 keys.
This count excludes SQL state, backend prefixes, indexes, allocator overhead, and replication. Fleet capacity remains unverified.

The final timing run encountered substantial unrelated host load. Its provider-arrival p99 is 137.34 ms, compared with 17.49 ms initially.
These runs have different sample counts and host conditions. They establish no latency improvement or production performance claim.
CSP22 still requires the complete dedicated-runner profile.

## Preserved failures

The first long rerun includes a background approval query in the request SQL counter.
The tracer now separates queries by request context. It retains the exact two-approval-check assertion for every budget write.

The next run reaches the 60-second authorization refresh boundary and records two extra budget reads.
That run fails the unchanged warm-request assertion after 977 requests, including warmup.
The shorter final run stays within its measured warm interval. No permission deadline or assertion changed.

Whole-file prose lint reports two unchanged comments in `registry.go`. The parent revision reports the same diagnostics.
Changed comments pass. The JSON prose parser does not support the performance target file.
Manual review verifies its single-word correction from six archive targets to five against D39 and the release matrix.

## Reproduction and remaining work

Use Go 1.27.1 and the recorded producer and consumer worktrees through `/tmp/csp122-billing.work`.
The [commands](commands.json) name each test selector and profile setting. Supply loopback test storage addresses through the named environment variables.
The compressed logs preserve successful and failed runs. The raw CPU and allocation profiles include startup and final validation.

The owner must select the settled-charge correction horizon and interrupted-batch continuation policy.
The [batch contract](../batch-recovery-contract-2026-09-27/CONTRACT.md) prohibits automatic replay of uncertain attempts under either policy.
Complete both decisions, their implementations, whole-task checks, latency and capacity qualification, review, native CI, dependency qualification, and paired merges.
No publication or paid inference occurred. All three task containers remain inactive. Docker has no running containers.
