# CSP0.4: Complete HTTP baseline

Status: done for the baseline and numeric-profile acceptance subcases. Production A50 remains UNVERIFIED. Changes remain local and uncommitted.

## Measurement boundary

The harness uses production application composition, real Badger and SQLite files, authentication, rate limits, a key budget, catalog routing, and provider HTTP adapters. It enables request logs, usage capture, and Prometheus metrics. The provider service runs on loopback with fixture credentials.

Each direct/proxied pair carries the same logical request. The fixture verifies its model, message, output bound, path, and credential. Stream checks require all three content events and the terminal event. Authentication refusal must occur before any provider request.

The harness retains client completion, complete handler duration, first response byte, first content event, connection reuse, and per-event forwarding times. It uses one monotonic clock and nanosecond durations. It removes only the measured deliberate provider waits before computing each paired difference.

These differences still include client and loopback work. They do not measure gateway CPU alone. Allocation benchmarks also include the fixture and client. CPU and allocation profiles identify the gateway call paths separately.

## Results

The [summary](csp0.4/baseline-summary.json) records three runs, each with 100 pairs for ordinary responses and 100 pairs for streams. These runs contain 600 warm pairs and 1,200 successful measured HTTP requests. Twelve initial HTTP requests remain separate.

Across the six run variants, paired median added latency ranges from 55.76 to 58.52 ms. The observed p99 ranges from 66.22 to 115.36 ms. These samples do not establish production percentiles or p99.9.

The [allocation benchmark](csp0.4/profile-command.json) records about 113 MB and 1.3 million allocations per measured request. About 89% of sampled allocated bytes pass through `DeepCopyProviderModels`. The [allocation profile](csp0.4/allocation-top.json) and [CPU profile](csp0.4/cpu-top.json) identify repeated catalog copies as the main cost. CSP3.1 and CSP10.1 own those optimizations.

The [source manifest](csp0.4/source-manifest.json) binds the harness to its local source. The baseline uses Starmap v0.16.5, 614 catalog routes, Go 1.27.0, macOS 15.7.2, and Apple M2 Max. The report contains the accepted catalog generation and checksum.

## Checks

- [Race checks](csp0.4/fixture-race.json): both new measurement tests passed.
- [Application, server, and execution checks](csp0.4/server-execution-tests.json): all five packages with tests passed. Two packages have no tests.
- [Go vet](csp0.4/vet.json): passed for the application and controller packages.
- [Component benchmarks](csp0.4/component-benchmarks.json): retain the router and credential baselines separately.
- [Historical baseline](csp0.4/baseline-scope.json): records the old timer omissions and earlier catalog and stored-credential profiles.

The Modern Go wrapper completed for the module's Go 1.26 contract. Starport declares no ago module tool or policy. The attempted tool lookup reported that no such tool exists. The work adds no linter dependency or policy.

The full repository test workflow already runs the new tests through `go test ./...`. The optional long measurement and profiling commands remain explicit. The work preserves every existing gate and assertion.

## Numeric profile and verifier

The [numeric profile](csp0.4/numeric-profile.json) sets warm-request p99 targets of 2 ms locally and 4 ms for the primary fleet.
Its allocation targets are 256 KiB and 1,500 allocations per request, plus 4 KiB and 32 allocations per stream event.
It defines workload sizes, offered rates, concurrency, backend RTT, native platform scope, validity, memory, and deadlines.
Three dedicated-runner measurements must each meet sample, duration, and confidence requirements before release qualification.

The [engineering assessment](csp0.4/numeric-profile-review.json) records the timing, workload, resource, correctness, and qualification review.
It binds the profile and retained baseline evidence by SHA-256.
This is the implementing agent's assessment. It does not claim an independent review or owner approval.
The Starport reference is `docs/performance-targets-v1.json`, linked from `docs/PERFORMANCE.md`.

The [CSP0.4 verifier result](csp0.4/task-verification.json) passed both required subcases.
It reran authentication refusal with the race detector and captured 200 fresh warm pairs through the complete local HTTP path.
It also checked the current target profile against the retained review.
All 50 primary cases remain UNVERIFIED because these two subcases do not complete A50.

The [regression checks](csp0.4/verifier-and-writing-checks.json) passed 30 verifier tests and shell syntax validation.
The repository verification script now runs that regression suite.
Tests reject missing stream samples, invalid timing boundaries, stale review hashes, weakened correctness, and incomplete qualification rules.

## Remaining qualification work

 Production A50 remains UNVERIFIED. The local baseline excludes shared backends, active maintenance, TLS, DNS, large inputs, retries, saturation, and slow clients. CSP22 must measure these conditions, retained memory, and cancellation bounds. CSP3.1 and CSP10.1 own the measured catalog-copy cost.

No production Go behavior changed. The worktree holds new measurement tests, a reusable test helper signature, corrected timer comments, and performance documentation. No commit or publication occurred.

The whole-repository writing check still reports three diagnostics in two preserved historical storage-review captures. Current performance prose passes its targeted check. Historical evidence remains unchanged.
