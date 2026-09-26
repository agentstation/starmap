autoreview panel findings: 2
[P1] Fleet replay compatibility gate permanently blocks refresh ownership after a fleet-wide upgrade or policy change
runtime/fleet_replay.go:55
Reviewer: claude model=claude-opus-5-5 thinking=high

recoverFleetState refuses replay whenever record.Compatibility differs from the local value. That value includes the embedded baseline generation ID and payload checksum, requireAuthority, the provider bindings, the acquisition-source list, the publisher aliases and the declared source capabilities. A replay failure is stored in fleetReplayError. After that, lease.start is called with acquisition disabled, refreshFleetInputs(ctx, true) returns the error for every non-accepted run, and checkFleetAcquisition returns it before any lease grant. Nothing clears it. Only a newer head replaces fleetReplayError, and only a compatible owner can publish one. Starmap embeds its catalog, so ordinary binary upgrades change the baseline. Once every replica runs the new binary, or the operator changes the acquisition sources or bindings everywhere, no process can replay the accepted head. No process can then take the lease, and explicit and scheduled refreshes fail with a conflict. The catalog stays frozen until the shared store is cleared by hand. This also applies to a single-replica fleet that restarts on a new binary. The non-fleet path rebuilds retained inputs on the new baseline; this path has no adoption or migration route. A fix needs a defined way for a replica with different compatibility to take over: adopt the retained inputs, rebuild on its own baseline and policy, and publish a new revision under its own grant. That is a contract change, but without it this patch turns routine upgrades into a permanent loss of refresh ownership.

[P2] Check acquisition access for providers without retained observations
runtime/fleet_capability.go:39
Reviewer: codex model=gpt-6-sol thinking=high

With implicit provider acquisition (`providerBindings == nil`), this loop adds only providers that already have retained observation layers. On a new fleet, or when a provider has not yet been observed, the capability check omits that provider even though the next provider refresh can acquire it from the catalog. A replica lacking its credentials can therefore take or renew the sole refresh lease and block a capable replica from updating it. Build the required provider list from the providers eligible for the upcoming acquisition, including those with no retained layer.

overall: patch is incorrect (0.84)
Panel review complete. claude model=claude-opus-5-5 thinking=high: 1 finding(s), codex model=gpt-6-sol thinking=high: 1 finding(s).
