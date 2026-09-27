autoreview panel findings: 1
[P2] Preserve the accepted pin during fleet ownership rebinding
runtime/fleet_runtime.go:195
Reviewer: codex model=gpt-6-sol thinking=high

An unchanged operation after a pinned fleet changes refresh owners reaches this path, but `prepareFleetCommit` always encodes recovery with a nil pin. Permission refreshes remain active while a pin is configured, so a successful takeover can publish a head whose selected generation is pinned but whose recovery record has no acceptance receipt. Subsequent replicas cannot replay that head; if activation rejects the missing pin capability first, the new owner cannot complete the takeover. Pass the accepted pin record through this rebind and use the pin publication context.

overall: patch is incorrect (0.82)
Panel review complete. claude model=claude-opus-5-5 thinking=high: 0 finding(s), codex model=gpt-6-sol thinking=high: 1 finding(s).
