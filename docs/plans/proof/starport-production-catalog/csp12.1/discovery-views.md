# Discovery cache views

Starport commit: `b2bcd80`, from `7aae8da`.
Starmap workspace dependency: `152148130`.
Toolchain: Go 1.27.1. Workspace: `/tmp/csp121-host-integration.work`.

The real cache returned stale models, providers, and endpoints after adapter removal.
Independent runtimes also collided when their catalog generation and availability
counter matched. Six scenario failures reproduce these defects in
[before evidence](discovery-revision-before.jsonl.gz).

Each immutable runtime snapshot now owns a random discovery cache identity.
Snapshot construction generates the identity. Inference response keys
retain their existing catalog identity.

A separate regression changes availability during one discovery read. All three
projections failed before the repair. The
[retention evidence](discovery-retention-before.jsonl.gz) records those failures.
Discovery now retains one snapshot in its context for both the cache key and
projection. Live permission checks remain at lookup and delivery.

## Verification

`GOWORK=/tmp/csp121-host-integration.work go test -json -race ./internal/catalog ./internal/proxy ./internal/registry`
passes 480 test events, with no failures or skips.
The [final log](discovery-final.jsonl.gz) records the named tests.
Tests cover removal, restoration, independent runtimes, warm serialized reads,
mid-read changes, and existing permission and disclosure checks.

`GOWORK=/tmp/csp121-host-integration.work make lint` passes with zero issues.
`git diff --check` passes.

This is local component evidence. Native qualification, aggregate heap,
concurrency, stream reconstruction, unavailable-cache latency, full acceptance
registration, review, and merge remain required. CSP12.1 remains incomplete.
