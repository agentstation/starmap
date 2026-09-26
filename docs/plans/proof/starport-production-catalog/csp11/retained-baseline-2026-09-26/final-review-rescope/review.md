autoreview panel findings: 2
[P2] Implicit fleet capability check needs credentials for every catalog provider; upstream catalog growth can deadlock ownership
runtime/fleet_capability.go:46
Reviewer: claude model=claude-opus-5-5 thinking=high

In implicit acquisition mode (no explicit bindings), `checkFleetAcquisition` adds every provider in `fleetAcquisitionCatalog` to `request.Providers`. `providerSourceObserver.CheckFleetAcquisition` then calls `resolver.ResolveCatalog` for each one and fails on the first provider without credentials. The check runs before lease acquisition and on every renewal, and a failed renewal check releases the grant (`renewOnce` → `k.release(current)`).

`fleetAcquisitionCatalog` is rebuilt from the effective catalog on every build, including upstream source updates. When the upstream catalog adds a provider the deployment has no credentials for, the owner's next renewal fails and ownership is dropped. Every equivalently configured replica fails the same check, so no replica can take ownership again. Configured upstream updates (which D41 says continue automatically) and all acquisition stop until an operator adds explicit bindings.

The ordinary acquisition path treats an unconfigured provider as `ProviderOutcomeSkippedNotConfigured` (degraded, still publishes), not as a failure. The ownership gate is therefore stricter than the work it protects.

Fix: restrict the implicit requirement to providers the acquirer would actually query (retained provider scopes plus providers with configured acquisition credentials). Treat a missing profile for a never-observed provider as a skip, not a capability failure.

[P1] [P1] Publish the retained catalog when an existing follower takes over
runtime/refresh.go:360
Reviewer: codex model=gpt-6-sol thinking=high

When an already running follower acquires the fleet lease during `RefreshSource`, `captureFleetGrant` records the new grant, but an unchanged source read returns without a publication. The shared head still carries the former owner's grant, so route acceptance cannot validate the catalog under the new owner even though the refresh succeeds. Startup handles this through `needsFleetOwnershipPublication`, but live takeover does not. Rebind the retained catalog and recovery inputs to the new grant when a refresh produces no input change.

overall: patch is incorrect (0.88)
Panel review complete. claude model=claude-opus-5-5 thinking=high: 1 finding(s), codex model=gpt-6-sol thinking=high: 1 finding(s).
