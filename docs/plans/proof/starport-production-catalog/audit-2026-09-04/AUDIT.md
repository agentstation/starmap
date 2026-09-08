# Production catalog plan audit

The plan needs revision before activation. Its ownership model is consistent,
but six high-priority findings leave production behavior or delivery incomplete.
Four further findings affect verification, administration, publication, and the demo.

Date: 2026-09-04. Scope: the complete PRD, engineering specification, plan,
acceptance map, repository findings, and README demonstration brief.
Source review covered the affected contracts and selected callers and tests.
This report is not a release qualification or an audit of every repository file.

The [canonical plan](../../../starport-production-catalog-plan.html) remains
`proposed`. Every implementation task remains `todo`.
The audit changed no product code, workflow, database, or deployment.

## Findings

P1 means a contract or delivery gap that must close before implementation relies
on it. P2 means a required correction before the affected task can pass.
Design findings describe missing guarantees. They do not establish an exploited
production vulnerability.

### AUD01 · P1 · Permission withdrawal needs a fleet enforcement contract

The reviewed specification let an incompatible replica preserve its prior
runtime. D1 makes internal Starmap authoritative over permitted membership.
Those rules conflict when a new generation withdraws permission that an older
replica still grants.

The user confirmed D13 during this audit: block new inference when the replica
cannot enforce an internal withdrawal. Keep diagnostics available.
The PRD and specification now record that decision. The implementation contract
still needs a verifiable withdrawal signal and a rule for incomplete propagation.

Specify how each replica learns the required permission revision, including
unsupported schemas and missed events. Separate ordinary metadata updates from
permission changes. Define what happens to existing streams, new retry attempts,
queued batch lines, and cached replies. A disconnected node cannot know an
unreceived withdrawal, so state the propagation bound and retention limit.

Acceptance must update an internal catalog to remove an offering while one
replica cannot activate the replacement. That replica must admit no new
inference under the withdrawn permission. Exercise restart and missed events.
Retain the existing cache generation-isolation tests.

Owner: CSP4, CSP10, and CSP11. Cases: A09, A10, A18, A21, and A28.

Evidence: specification sections 7.5, 8.2, and 9.1, plus the
[runtime lease](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/registry/generation.go#L259)
and [batch submission state](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/server/controllers/batches.go#L33).

### AUD02 · P1 · The outage requirement conflicts with current budget behavior

The PRD requires enforcement without bypassing budgets during a shared-store
outage. Current Starport deliberately allows requests after a budget-read
failure. Batch admission follows the same rule. Existing code chooses this policy.

Two existing tests passed during this audit:
`TestBudgetStorageErrorFailsOpen` and `TestTeamBudgetReadErrorFailsOpen`.
They prove the current behavior contradicts the new target.
CSP12 through CSP15 do not name the online admission change or its policy migration.

Record the product decision explicitly. The recommended production policy blocks requests
when Starport cannot verify their required budgets.
Apply the rule to account, key, and team budgets, including batch work.
Keep diagnostics available. State response codes, retry behavior, and any
separately supported degraded mode.

Acceptance must inject usage and team-budget read failures during ordinary
requests and queued batch execution. The result must follow the selected policy.
Storage compatibility tests alone do not prove that admission enforces it.

Owner: CSP12 or a separate Starport admission task. Cases: A15 and A31 need
explicit outage subcases. The user question remains pending at report creation.

Evidence: [budget middleware](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/server/budget.go#L154),
[batch governor](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/server/batch_governor.go#L43),
and [raw test results](starport-budget-tests.jsonl).

### AUD03 · P1 · Recovery cannot infer changes missing from its backup

A33 requires recovery without resurrecting revoked access or spent budget.
The proposed restore procedure starts from matching KV, SQL, and file backups.
It does not identify an authoritative source for changes after that recovery point.
A stale snapshot cannot reconstruct those changes by checking its own references.

Select a recovery contract before implementing CSP13. It can use independently
durable change records. Without those records, keep affected access and budgets
blocked until an operator reconciles them. State what happens when that evidence is unavailable.
Do not promise preservation beyond the tested acknowledged-write loss bound.

Acceptance must take a backup, revoke a key, consume budget, and lose the primary
stores. Restore the old backup without the later changes. The restored service
must remain restricted until it satisfies the chosen reconciliation contract.
Measure the recovery point and time for this exact sequence.

Owner: CSP13, with storage policy from CSP12. Cases: A16, A31, and A33.

Evidence: specification sections 8.3, 8.5, and 8.6, and
[CSP13](../../../starport-production-catalog-plan.html#task-CSP13).

### AUD04 · P1 · Connected acquisition still lacks an explicit all-source task

P01 requires catalog construction from every enabled, eligible source.
The code has a general source contract and a separate connected acquirer that
observes providers. models.dev refresh in the scheduled generator is another
composition. These paths are not interchangeable.

The plan names source settings and provider-layer reconciliation, but no task
explicitly connects non-provider acquisition sources to the server and embedded
Starport runtime. A correct source-status report alone would not prove that
those sources can change catalog facts.

Add an owned integration task or expand CSP3 explicitly. Distinguish the selected
distribution base from acquisition inputs. Specify supported inputs for the CLI,
scheduled publisher, Starmap server, and embedded Starport composition.
Keep source dependencies and installation under explicit operator policy.

Acceptance must change provider and non-provider fixture data and observe the
expected derived catalog in each supported composition. Also test disabled
sources, missing dependencies, partial failure, and manual refresh.

Owner: Starmap acquisition and runtime, then Starport composition.
Cases: A08 and A20 need ingestion subcases in addition to source reports.

Evidence: [general source contract](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/pkg/sources/source.go#L30),
[connected acquirer](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/acquisition/acquirer.go#L151),
and [scheduled generator](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/scripts/generate-embedded-catalog.sh#L66).

### AUD05 · P1 · Catalog trust does not define permission to send inference keys

The specification binds catalog transport credentials to their selected source.
It does not define an equivalent authorization boundary for inference destinations
that catalog updates can change. A trusted catalog supplies endpoint templates,
and Starport binds selected inference material to those endpoints.

Define whether accepting a publisher also authorizes every credential destination
it supplies. Prefer an explicit deployment policy for provider destinations,
including approved private and local providers. Specify behavior when a catalog
changes the host, scheme, port, or credential-placement contract.

Acceptance must present an otherwise valid catalog update that changes a
credential destination. Verify the selected policy before any outbound attempt.
Include normal and streaming endpoints, operator overrides, account credentials,
and local-provider exceptions. Existing redirect refusal must remain intact.

Owner: Starmap catalog validation and Starport runtime admission and binding.
Cases: A07, A11, and A25 need destination-policy subcases.
This is a missing design boundary, not a demonstrated credential leak.

Evidence: [provider activation](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/providers/activation.go#L109),
[endpoint binding](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/registry/generation.go#L382),
and [redirect refusal](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/providers/connectors/http_client.go#L34).

### AUD06 · P1 · Publisher acceptance needs explicit partial-failure rules

The specification distinguishes required, optional, missing-key, partial, and
failed sources. It does not state which combinations permit publication.
It also leaves the first run without retained evidence and unexpectedly large
removals without a defined admission result.

Define required-source behavior and the permitted retained-evidence age.
Distinguish valid complete deletion from an incomplete empty reply.
Set review or rejection rules for exceptional catalog changes without blocking
ordinary valid updates. Bind the result to the authority-policy version.

An unchanged semantic catalog reuses an immutable artifact. Specify where fresh
per-source receipts live, how they bind to that artifact, and how consumers
verify them. An updated channel confirmation cannot certify every provider as fresh.

Acceptance must cover a missing required source, no eligible credentials, a
partial outage with retained evidence, a complete deletion, and unchanged facts
with new observations. Each outcome needs an explicit publish or reject verdict.

Owner: CSP3 and CSP6. Cases: A05, A08, and A20.

Evidence: specification sections 5 and 6, plus the
[publisher workflow](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/.github/workflows/catalog-generation.yaml#L42).

### AUD07 · P2 · Task commands do not yet prove every assigned acceptance case

The acceptance map has all 39 IDs, but ID coverage is not behavior coverage.
CSP20 claims successful installation and inference on every advertised platform.
Its named commands currently inspect README text and links. They do not install
an artifact or send an inference request.

Some current database contracts register PostgreSQL and MySQL tests only when
their environment settings exist. Zero skipped tests can therefore coexist
with no external-database test execution. The audit deliberately left those
settings unset and makes no database qualification claim.

Require each task to run its assigned campaign cases before marking it done.
Define named subcases and required environments beneath each A-ID.
Missing platforms, backends, or test matches must keep that case UNVERIFIED.
Keep the 39 summary cases if useful, but publish the complete subcase roster.
Also give P07 a positive no-provider-key download and activation test.
A07 currently describes candidate rejection, which cannot prove that success path.

Owner: CSP0 and each task owner. Correct CSP20's commands and phase exit criteria.

Evidence: [README verifier](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/scripts/verify-readme-quickstart.sh),
[database test registration](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/sqlstore/sqlstore_contract_test.go#L30),
and the [acceptance map](../acceptance-map.json).

### AUD08 · P2 · Starmap administration lacks a persistence and bootstrap contract

The target requires distinct subscriber and administrator permissions, durable
audit records, and configuration operations. T1 intentionally has no SQL
requirement. Its directory table does not assign storage for administrative
audit records or operation receipts.

Specify the first administrator bootstrap, the authorization source, audit
storage, retention, permissions, and recovery. Keep subscriber credentials
separate. Define what happens if audit persistence fails during a mutation.
Do not add Starport account storage as an implicit Starmap dependency.

Acceptance must bootstrap a fresh server, deny subscriber mutations, record an
authorized change, and recover its audit and operation state after interruption.

Owner: CSP14. Cases: A12, A26, and A32 need standalone-server coverage.

Evidence: specification sections 4.2, 7.6, 7.7, and T1, plus
[current server routes](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/internal/server/router.go#L115).

### AUD09 · P2 · Automatic promotion needs a bot identity and CI trigger contract

CSP6 depends on a checked bot PR. The existing publisher uses `GITHUB_TOKEN`
and does not declare pull-request write permission. The plan does not select
the identity and trigger path for unattended promotion checks.

Current GitHub documentation says token-created PR checks require approval for
the listed PR events. A GitHub App or personal token can permit automatic
execution. Other token-generated events have different trigger restrictions.
[GitHub workflow trigger rules](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow)

Select the supported bot identity, permissions, event path, and branch-rule
requirements. Verify them in a controlled repository before claiming unattended
promotion. Keep artifact verification separate from permission to merge.
Do not infer actual repository branch settings from workflow YAML.

Owner: CSP6 and the repository release owner. Case: A05.

Evidence: [current workflow permissions](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/.github/workflows/catalog-generation.yaml#L9).

### AUD10 · P2 · The demo needs an explicit release-artifact handoff

CSP21 records the installed release before CSP22 qualifies the candidate and
CSP23 publishes it. The brief requires final UI and truthful installation.
The plan does not state which installable artifact exists at the recording step
or how later changes invalidate the recording.

Name an immutable candidate artifact for rehearsal and recording. Define which
changes require another capture, and verify the published README against the
shipped installation path. If final recording follows release, place that work
explicitly in CSP24 and adjust the earlier case-completion claim.

Acceptance must bind the recording manifest to the installed artifact and the
qualified command sequence. A later change to setup, UI, or request behavior
must prevent reuse of stale demonstration evidence.

Owner: CSP20 through CSP24. Cases: A35 through A39.

Evidence: [README brief](../readme-demo-brief.md) and
[recording task](../../../starport-production-catalog-plan.html#task-CSP21).

## Contracts that remain consistent

- Starmap owns catalog facts, acquisition, reconciliation, and distribution.
- Starport owns inference, accounts, credentials, and request policy.
- Passive library reads remain separate from persistent application startup.
- Complete source replacement avoids restoring removed records from the baseline.
- Internal authority has no implicit public fallback.
- Product roots do not inherit across applications.
- Single-process and fleet storage have distinct ownership and durability rules.
- Valkey does not replace SQL or shared file storage.
- Full embedded and public documentation shares one versioned content source.
- Company size does not select a storage backend.
- The README distinguishes catalog access from credentialed inference.

The native path matrix and typography targets remain proposed product contracts.
The recorded UI review correctly limits its claims to sampled rendered states.
Five screenshot hashes and seven measurement sets remain intact.
Extend the existing distribution runtime through its owned contracts.

## Verification and limits

Both remote main references still matched the inspected revisions:
Starmap `4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225` and
Starport `042eb97851496ecdd1ef20bfa44fe3d84b86b2d8`.

| Check | Result |
|---|---|
| Starmap race tests across five selected packages | 126 top-level tests and 58 subtests passed |
| Starport race tests across six selected packages | 358 top-level tests and 208 subtests passed |
| Starport budget-policy race tests | Two top-level tests passed and confirmed fail-open behavior |
| Starmap ago v0.2.0 | Zero findings, stale ignores, or incomplete errors |
| Go compatibility review | Starmap declares Go 1.25.0. Starport declares Go 1.26. |
| Pinned source references | 62 references resolved, including bounded line anchors |
| Existing UX evidence | Five screenshot hashes and seven measurement sets verified |
| Planned command paths | Existing paths resolved. Four distinct new interfaces remain explicitly planned. |

The test runs reported no failed or skipped tests. They did not exercise real
Valkey, Redis, PostgreSQL, or MySQL services. The SQL test run explicitly removed
`TEST_POSTGRES_URL` and `TEST_MYSQL_DSN` from its environment.

No live provider, native Linux or Windows, full browser accessibility, SDK,
capacity, disaster-recovery, or publication qualification ran. Existing fixtures
and local servers supplied test dependencies. The audit recorded no new demo.
Starport declares no ago module tool, so its ago discovery returned unavailable.
The audit did not install that tool or change the module.

Raw commands, results, and counts are beside this report.

- [Test summary](test-summary.json).
- [Starmap command](starmap-tests-command.json).
- [Starport command](starport-tests-command.json).
- [Budget command](starport-budget-tests-command.json).
- [ago output](starmap-ago.json).
- [Source evidence checks](evidence-checks.json).
- [Planned command paths](command-paths.json).

## Required revision order

First settle and record the enforcement and recovery contracts in AUD01 through
AUD03. The user confirmed D13. The budget question remains pending.
Then define source coverage, publication admission, and credential destinations.
Update the specification and task ownership before expanding the verifier.

Correct the task-level proof requirements and publication prerequisites next.
Complete the Starmap administration and recording handoffs before their tasks
start. Keep existing D6, D7, D11, D12, workload, and recovery choices visible.
The audit does not treat earlier proposals as confirmed decisions.
