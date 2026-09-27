autoreview panel findings: 2
[P1] Provide an adoption path before requiring recovery approval on existing fleets
internal/catalog/fleet_store.go:44
Reviewer: codex model=gpt-6-sol thinking=high

Every Valkey startup now requires an approved `catalog_recovery` row, but the migration creates no approval for existing deployments. `fleet init` refuses populated stores, so an existing Valkey deployment cannot start after upgrading. Ship a controlled adoption procedure or preserve the prior startup path until one exists.

[P1] Provide recovery before binding approval to a Valkey process
internal/storage/valkey_incarnation.go:65
Reviewer: codex model=gpt-6-sol thinking=high

The approved identity includes Valkey's process run ID, so an ordinary restart or failover changes it and `BindIncarnation` rejects the backend. The bundle explicitly provides no operator recovery command, leaving a freshly initialized production fleet unable to restart. The identity check needs a controlled recovery workflow in this release.

overall: patch is incorrect (0.96)
Panel review complete. claude model=claude-opus-5-5 thinking=high: 0 finding(s), codex model=gpt-6-sol thinking=high: 2 finding(s).
