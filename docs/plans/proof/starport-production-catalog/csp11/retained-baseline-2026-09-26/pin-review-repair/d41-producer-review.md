autoreview panel findings: 1
[P2] Pinned followers with a policy mismatch fail startup
runtime/acquisition_policy_publication.go:41
Reviewer: codex model=gpt-6-sol thinking=high

When an existing fleet publication has an accepted pin but this replica's acquisition policy differs, replay fails and initializeFleetLayers leaves pinRecord unset. This check then aborts Open, even though the accepted publication was loaded and the replica should remain able to serve as a follower. Validate the retained pin receipt independently of replay readiness, while continuing to refuse refresh ownership.

overall: patch is incorrect (0.85)
Panel review complete. claude model=claude-opus-5-5 thinking=high: 0 finding(s), codex model=gpt-6-sol thinking=high: 1 finding(s).
