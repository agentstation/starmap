# CSP16.2 decisions

Date: 2026-10-02. Each decision is an engineering interpretation inside the accepted task scope.
None needs new owner authority. The owner can reverse any of them before the final acceptance run.

| ID | Decision | Reason and consequence |
| --- | --- | --- |
| 16.2-1 | The recovery record format does not change. The operation receipt lives in Starport. | The decoder rejects unknown members and other versions (`fleet_recovery.go:150-161`). A new field would make every released binary refuse replay after promotion, which breaks software rollback (condition 13). |
| 16.2-2 | Promotion is a manual run under the publication lease. | `ReplaceRemovalTargets` is the exemplar. The lease, the head compare-and-set, and the increasing revision come from the existing commit path. |
| 16.2-3 | Promotion refuses while a source authority is configured. | D41 names promotion for embedded-only fleets. A configured source needs a coordinated apply, not a baseline promotion. |
| 16.2-4 | A removal target absent from the promoted baseline stays recorded and inert. | Retained removals are explicit operator rules (condition 16). Dropping them silently would change policy without an operation. The receipt lists them. |
| 16.2-5 | The five subcases register under A14 and join `task_checks` for CSP11, CSP16.2, CSP22, and CSP24. | A14 owns fleet replay across binary changes. The roster validator requires every A14 subcase in CSP11, CSP22, and CSP24. CSP11 is the primary owner of A14, CSP22 holds every candidate-case subcase, and CSP24 holds all subcases. |
| 16.2-6 | Status compares packaged and retained identities in memory. | Operators see whether a promotion is available without a storage call (condition 17). |
| 16.2-8 | Starmap verification runs the eight named runtime tests with the race detector, then `make verify`. | The complete runtime package exceeds 10 minutes under the race detector. The repository bounds its race suite at 30 minutes inside `make verify`. |
| 16.2-7 | Two PRs: one Starmap PR with the runtime operation and the subcase registration, then one Starport PR. | Starmap #212 set the pattern. Starport pins the Starmap module, so the Starmap PR must merge and tag before the Starport PR can depend on it. |

### Amendment to 16.2-7 (2026-10-02)

Starmap #214 merged as `595e3c7ba` on 2026-10-02. Starport `go.mod` already pins a Starmap pseudo-version (`v0.16.6-0.20261002020739-80de6d830bf2`, Starport #405), so the Starport promotion PR pins the pseudo-version of `595e3c7ba`. The pin needs no tag or release. A release keeps its separate owner authorization.

## Pending

### Promotion executor (asked on 2026-10-02)

A gateway leader renews the publication lease every 30 seconds until it closes. Starmap `PromoteEmbeddedBaseline` runs only under that lease.
The current Starport implementation opens the fleet runtime inside the CLI process. It refuses while a gateway holds the lease.
In a running fleet, the operator must stop the gateways before a promotion. A fleet with two or more gateways has a new leader soon after the old leader stops.

| Option | Work | Consequence |
| --- | --- | --- |
| (a) In-process CLI with a documented upgrade-window limit | Implemented. Documentation only. | A promotion needs a full fleet stop. A rolling upgrade cannot promote. |
| (b) Leader-executed admin HTTP route, CLI as client | New public route, controller, admin audit entry, CLI HTTP client, tests. | A follower refuses with the leader identity. The gateway audit trail records the actor. The route is a new public API. |
| (c) KV request record that the leader executes | New storage record, a gateway poll loop, CLI wait and poll, tests. | No new public API. More failure states: no leader, stale request, wait timeout. |

Lead recommendation: (b). It follows the existing admin catalog refresh pattern, and the receipt JSON already defines the response schema. Option (a) lands first only when the owner accepts the fleet-stop limit for this release.
