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

| 16.2-9 | The fleet promotion tests forge one fleet head whose retained baseline is the packaged generation under another generation ID. | No exported Starmap option replaces the embedded catalog, and a public test hook would add API only for a test. The forge recomputes the compatibility checksum and checks itself against the real record, so a Starmap composition change fails loudly. |
| 16.2-10 | The lease holder executes a promotion request that the CLI records in shared storage. The CLI never takes the lease and never opens the gateway state directory. | Owner direction on 2026-10-02: decide from exemplars. The Starmap operation runs only under the publication lease. A gateway leader renews the lease every 30 seconds until it closes, so an in-process CLI refuses in every running fleet and needs a fleet stop. The Starport exemplar for an operator fleet change is `config apply` (CSP16): the CLI writes the record, and the leader reads it at lease renewal. An admin HTTP route would follow the refresh route, but a load balancer hides the leader from the operator and the CLI has no HTTP client. The request record adds no public API. |
| 16.2-11 | The receipt key is `catalog:promotion:{<deployment digest>}:v1:<operation_id>`. | The key shares the hash tag of the lease key, so one native transaction checks the live grant and creates the receipt. The contract named `catalog:promotion:{operation_id}:v1` before implementation. |

## Pending

None. The owner directed the executor decision on 2026-10-02, and decision 16.2-10 records it.
