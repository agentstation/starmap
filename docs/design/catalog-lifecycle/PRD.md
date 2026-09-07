# Starmap and Starport catalog lifecycle PRD

Starmap must supply a verified catalog from embedded data, published artifacts,
and configured sources. Starport must use that catalog for inference routing.
Both products must work with a local baseline when external catalog services
are unavailable, subject to the deployment's authority policy.

This draft defines product behavior for developers, operators, and engineering
teams. It records requirements, not an implementation commitment.
The [engineering specification](ENGINEERING_SPEC.md) defines the proposed
contracts. The [repository findings](REPOSITORY_FINDINGS.md) separate current
behavior from gaps at the inspected revisions.

Updated: 2026-09-05. Status: target requirements for the active implementation plan.
The revision incorporates the verified Fable review and the earlier plan audit.
The [review resolution](../../plans/proof/starport-production-catalog/revision-2026-09-05/REVIEW_RESOLUTION.md)
records each finding, its disposition, and remaining implementation evidence.
The [storage revision](../../plans/proof/starport-production-catalog/storage-revision-2026-09-05/REVIEW_RESOLUTION.md)
incorporates all fourteen findings from the [storage review](STORAGE_REVIEW.md).
Its contracts remain proposed until implementation and qualification finish.

The user accepted the [latency review](LATENCY_REVIEW.md) recommendations for this target design.
The [latency revision](../../plans/proof/starport-production-catalog/latency-revision-2026-09-05/REVIEW_RESOLUTION.md) assigns their implementation and acceptance evidence.

## Decisions

| ID | Decision | Status |
| --- | --- | --- |
| D1 | An internal Starmap server defines the complete catalog that its Starport subscribers may use. | User confirmed |
| D2 | An outage preserves the last accepted internal catalog. First boot without an accepted internal catalog blocks inference. | User confirmed |
| D3 | Each successful four-hour publication updates the catalog channel and the default branch's embedded input. New releases embed that input. | User confirmed |
| D4 | Existing binaries and pinned Go modules retain their original embedded catalog. Runtime updates require no binary update. | Consequence of D3 |
| D5 | Starport uses Starmap-prefixed inference credentials only through an explicit fallback setting. | User confirmed |
| D6 | Application startup persists the baseline. Construction writes no files and starts no automatic acquisition or refresh. Explicit caller-supplied storage reads may use the network. | User clarified on 2026-09-06 |
| D7 | A complete published catalog replaces the fallback baseline. Source authority governs subsequent field updates. | Engineering default selected at activation, 2026-09-05 |
| D8 | Full operator documentation ships inside Starport and on a public documentation site from the same versioned content. | User confirmed |
| D9 | Valkey plus PostgreSQL is the primary replicated deployment recipe. Redis and MySQL alternatives require compatibility evidence before support claims. | User confirmed |
| D10 | Initial production support covers one region and tested disaster recovery. Multi-region active-active operation needs a separate design and validation. | User confirmed |
| D11 | Starport manages its own configuration. Upstream Starmap administration remains separate. | Engineering default selected at activation, 2026-09-05 |
| D12 | Local deployments use local configuration files. Initialized shared deployments use shared configuration. UI changes follow that authority. | User clarified storage-based authority on 2026-09-05 |
| D13 | A replica that cannot enforce an internal permission withdrawal blocks new inference and keeps diagnostics available. | User confirmed during audit |
| D14 | Production requests receive a retryable refusal when Starport cannot determine whether required budgets permit them. Confirmed absence permits normal processing. | User selected option 1 on 2026-09-05 |
| D15 | Bootstrap stays node-local. Shared PostgreSQL stores deployment configuration revisions and their audit records. Valkey retains catalog state and coordination. | Engineering default selected at activation, 2026-09-05 |
| D16 | Retain Starmap `config.yaml` and Starport `config.env` as their primary local files. Separate config, data, state, and cache roots. | Storage review recommendation adopted into the draft |
| D17 | Badger and Valkey hold durable KV records. Disposable caches need bounded, separate policies and must preserve expiry. | Storage review recommendation adopted into the draft |
| D18 | Temporary development isolates all product stores. Persistent external storage requires the persistent recipe. | Storage review recommendation adopted into the draft |
| D19 | Starport serves stable request data from bounded, validated process memory and minimizes repeated allocation and storage access. | User accepted latency target on 2026-09-05 |
| D20 | Strict budgets use atomic admission or reserved capacity. Optional caches and advisory refresh must not delay inference beyond explicit bounds. | User accepted latency target on 2026-09-05 |
| D21 | Performance claims require complete request measurements, declared workloads, and reviewed numeric limits. Existing narrow measurements remain historical evidence. | User accepted measurement recommendation on 2026-09-05 |
| D22 | Windows may briefly remove the optional YAML workspace path during journaled replacement. Starmap readers retry; accepted inference state remains independent. | User accepted on 2026-09-06; implementation and native qualification remain open |
| D23 | Explicitly selected service-managed primary configuration may use trusted administrator ownership and service read access. Untrusted writes remain forbidden. | User confirmed on 2026-09-06 |
| D24 | A fresh manual update resets prior local acquisition results while preserving the embedded or selected upstream baseline. Internal authority remains binding. | User confirmed on 2026-09-07 |

D23 permits checked, explicitly selected, administrator-owned primary configuration for service deployments. Catalog state and dotenv files retain private-access requirements.
The [owner decision record](../../plans/proof/starport-production-catalog/csp2/owner-decisions-2026-09-06.md) defines its implementation and qualification limits.

D6 preserves the requested inspectable baseline manifest and payload on disk.
Explicit remote storage reads may create transport workers. Default construction remains offline and starts no refresh scheduler.
Manual source refresh remains distinct from a pin that freezes the effective catalog.
D3 retains four-hour publication and default-branch promotion.

## Problem and outcomes

The products already support embedded catalogs, remote distribution, local
acquisition, and shared storage. Their contracts still differ across startup,
directories, credentials, and merge behavior. An operator cannot infer the
complete behavior from one configuration choice.

The required outcomes are:

- A developer can inspect a catalog without network access or provider keys.
- A fresh persistent installation saves its baseline before catalog refresh.
- A connected installation adopts verified catalog updates without a binary update.
- A new source checkout contains the latest catalog promoted by the publisher.
- An enterprise controls catalog authority, egress, credentials, and persistence.
- A fleet converges on one accepted catalog without sharing local database files.
- An operator can identify the source, age, contents, and routing eligibility of a catalog.
- Uncached inference avoids repeated catalog construction, key derivation, and stable-data lookups in external stores during normal operation.
- Operators can distinguish gateway delay, provider delay, optional work, and client backpressure.

An offline catalog does not supply inference compute. Remote inference still
requires provider connectivity and valid inference credentials. A configured
local provider can support offline inference separately.

## Product boundaries

| Concern | Owner |
| --- | --- |
| Provider identity, authors, model definitions, offerings, capabilities, and prices | Starmap |
| Source acquisition, evidence, reconciliation, validation, and catalog artifacts | Starmap |
| Catalog server protocol and connected catalog runtime | Starmap |
| Standalone Starmap configuration and paths | Starmap application |
| Starport paths and its catalog store adapter | Starport application |
| Inference credentials, accounts, gateway API keys, policy, and provider availability | Starport |
| Request routing, execution, protocols, rate limits, and response caches | Starport |
| Shared infrastructure, secret access policy, backups, and recovery | Deployment operator |

Starport must derive catalog facts from Starmap. It must not maintain a second
provider roster, price table, or endpoint authority. Catalog membership alone
does not grant account access or establish provider health.

## Users and deployment behavior

| Deployment | Catalog behavior | Persistence |
| --- | --- | --- |
| Offline developer | Inspect the embedded catalog. Disable external catalog acquisition. | Product-owned local state, or explicit ephemeral mode |
| Connected developer | Start from local state, follow the public channel, and acquire eligible provider evidence. | Local files, Badger, and SQLite as applicable |
| Startup on one server | Use the same catalog behavior with explicit service paths and managed secrets. | Persistent local volume with tested backup |
| Startup with several gateways | Share the accepted catalog and gateway records. Coordinate acquisition. | Valkey, PostgreSQL, shared file bytes, and private replica state |
| Enterprise with internal Starmap | Follow the configured internal authority. Keep inference credentials in Starport. | Central catalog storage and shared gateway stores |
| Restricted catalog egress | Gateways contact only internal Starmap for catalog updates. | Retained accepted internal catalog |
| Air-gapped installation | Import verified artifacts through a controlled transfer. | Persistent local or internal shared stores |

Catalog egress and inference egress are separate. A gateway can avoid public
catalog requests while it sends inference requests to approved providers.

Company size does not select a storage engine. Process count, write ownership,
availability needs, and persistence requirements select the architecture.
A small team can use one persistent host. A local developer can operate several
isolated gateways. An enterprise can start with one gateway and internal authority.
Section 8 of the specification defines the supported architecture targets.

## Functional requirements

| ID | Requirement | Acceptance evidence |
| --- | --- | --- |
| P01 | Starmap builds a catalog from every enabled, eligible source, including provider APIs. | The run report names each configured source and its result. |
| P02 | Each release carries a verified embedded catalog with stable identity and schema metadata. | An isolated binary reports the manifest and serves catalog reads. |
| P03 | Fresh persistent application startup saves the embedded baseline without external access. | A cold start creates a valid local baseline and survives restart. |
| P04 | Each product resolves configurable paths for Linux, macOS, Windows, and service deployments. | Native platform tests verify paths, access, and migration. |
| P05 | The publisher requests a run every four hours and publishes only validated results. | Workflow configuration and failure-injection tests prove the publication order. |
| P06 | Successful publication promotes the verified embedded input to the default branch. | A clean checkout builds with the promoted catalog digest. |
| P07 | Starmap and Starport can follow the public catalog without provider keys. | A public-source test updates an empty installation without provider credentials. |
| P08 | Local acquisition updates only the facts that its authority permits. | Conflicting evidence produces the specified winner and provenance. |
| P09 | Partial source failures preserve unaffected and last-known-good facts. | One failed provider cannot remove another provider's accepted records. |
| P10 | Internal Starmap authority controls catalog membership and permitted enrichment. | Lower layers cannot restore an excluded offering. |
| P11 | Catalog acquisition, inference, gateway authentication, and catalog transport use separate credential roles. | Role-isolation tests detect any cross-role lookup. |
| P12 | Environment variables and configured secret managers can supply credentials for each supported role. | Backend contracts verify resolution, rotation, and failure handling. |
| P13 | Starport supplies a documented OpenRouter-compatible replacement surface. | Raw HTTP and official SDK tests cover the declared compatibility matrix. |
| P14 | A Starport fleet shares durable catalog and gateway state with one coordinated acquisition owner. | Multi-process tests prove convergence and reject a stale owner. |
| P15 | SQLite supports one host. A fleet uses a shared relational database. | Migration and recovery tests preserve relational and KV references. |
| P16 | Operators can inspect generations, counts, source results, fallback, and failed updates. | CLI, API, and console show consistent generation metadata. |
| P17 | Operators can pin, import, and roll back verified catalogs. | Invalid imports preserve the current head. Authorized rollback survives restart. |
| P18 | Both products distinguish automatic updates, manual refresh, offline catalog access, and a pinned generation. | Egress and restart tests prove each control independently. |
| P19 | Starport exposes the supported Starmap catalog configuration as a superset with Starport deployment settings. | A shared contract verifies every setting, default, override, and intentional exclusion. |
| P20 | Operators can inspect effective configuration, its origin, pending changes, and each replica's applied revision. | CLI, API, and console reports agree without exposing secrets. |
| P21 | Local and enterprise operators can validate configuration changes before applying them through the deployment's management method. | Conflicts, permission failures, restart requirements, and failed activation preserve known state. |
| P22 | Full documentation works without internet access and matches the installed Starport and Starmap versions. | Embedded and public builds use the same content revision and pass offline navigation tests. |
| P23 | Documentation explains each supported topology, storage role, update policy, migration, backup, and recovery procedure. | Each recipe passes an isolated deployment and restore exercise. |
| P24 | Documentation and configuration screens provide readable, accessible desktop and narrow-screen layouts. | Rendered checks verify typography, contrast, keyboard access, reflow, and text enlargement. |
| P25 | A Starmap server has an explicit administration, acquisition, publication, and subscriber-access contract. | Server tests isolate subscriber reads from administrative changes and preserve authority through recovery. |
| P26 | Production claims name the tested topology, backend versions, release pair, capacity, and recovery results. | A release support matrix links each claim to recorded evidence. |
| P27 | The Starport README explains installation, immediate catalog access, and the first inference request, with links to persistent and enterprise setups. | Platform install checks and an isolated quickstart produce the documented results. |
| P28 | The README demo shows the product outcome through a reproducible recording of the selected release. | The recording shows installation, catalog discovery, provider setup, and a successful response without exposed secrets or simulated success. |
| P29 | Clients receive predictable model identity, alias, deprecation, removal, and price behavior across catalog updates. | Transition tests cover existing requests, new attempts, authority withdrawal, and documented errors. |
| P30 | Deployment configuration has one authority: a local file, initialized shared store, or designated external controller. | Conflicting replica files and shared-store outages cannot replace the selected shared revision. |
| P31 | Every managed file has a canonical name, owner, resolved location, access policy, lifetime, and recovery rule. | CLI, API, UI, and generated documentation agree across platforms, overrides, and deployment modes. |
| P32 | Storage selection preserves durable records, isolates deployments, bounds cache use, and applies every supported setting. | Backend tests prove connection security, namespace isolation, durability policy, effective options, and expiry-preserving cache refill. |
| P33 | Temporary development uses isolated memory and scratch stores without incidental persistent storage access. | Conflicting persistent settings fail before storage access. Restart loses temporary records and leaves persistent installations unchanged. |
| P34 | Starport precomputes catalog indexes and static route candidates when activating a runtime. Exact-model requests avoid work on unrelated routes. | Scaling benchmarks and profiles verify lookup cost, allocation bounds, aliases, fallback, and generation consistency. |
| P35 | Warm requests use bounded, valid configuration, authorization records, and credential material in process memory. | Tests prove no external stable-data reads or repeated key derivation while validity holds, including rotation, revocation, and missed updates. |
| P36 | Strict account, key, and team limits account for concurrent requests through atomic admission and recoverable capacity reservations. | Multi-process tests prove limits across retries, cancellation, crashes, unknown costs, and recovery without relying on lossy analytics. |
| P37 | Optional cache work and advisory fleet refresh have bounded latency and memory costs. | Cache failures, full queues, long streams, and unavailable peer storage preserve timely delivery and required admission. |
| P38 | Performance claims measure the complete gateway path and identify tested workload, topology, allocations, and timing boundaries. | Reviewed profiles verify request percentiles, stream delay, storage operations, memory, and truthful UI, documentation, and demo claims. |

Credential migration under P11 must not silently change which inference identity pays.
Existing installations need a conflict diagnostic and explicit resolution when precedence changes select different credentials.
Fresh installations follow the documented precedence without an unnecessary migration prompt.

P01 requires actual ingestion through each supported application composition.
A report listing source names does not prove that a server or embedded runtime collected their evidence.

## User journeys and delivery order

| Journey | Required result | Failure and recovery evidence |
| --- | --- | --- |
| J1: local first use | Install, inspect the catalog without keys, configure inference access, and receive a real response. | Explain temporary state, missing credentials, and connection failures. Recovery docs remain accessible. |
| J2: one server to fleet | Preserve client IDs, selected credentials, and related records while adopting a complete shared-storage recipe. | Reject incomplete migration and unknown budget permission. Show the supported recovery action. |
| J3: internal authority | Configure an internal source and identify its approved catalog, permitted egress, and replica state. | Name the approving action when no catalog exists. Block unenforceable withdrawals while retaining diagnostics. |

Deliver independent first-use and documentation fixes before fleet implementation.
Use current commands and clearly state the tested release's limits.
Final documentation and recordings must then follow the qualified release pair.
Each phase needs an observed user result beside its technical acceptance count.

Normal setup must not require users to understand every storage engine or credential role.
Offer a default recipe and show advanced controls when the operator needs them.
Effective values, source origins, and recovery actions remain available to every authorized operator.

Persistence must be visible during setup. Show whether records survive process exit and container replacement.
An operator can inspect advanced storage details without selecting each engine during ordinary first use.
Changing a backend or path starts a migration procedure. It does not copy records through a field save.

## Catalog contents and merge behavior

The catalog must distinguish providers, authors, model definitions, provider
offerings, and endpoint projections. These counts describe different entities.
Every response must calculate its counts from one immutable generation.
Starport must report routable offerings separately from catalog membership.

The proposed layer order is:

1. Use the embedded catalog when no accepted catalog exists.
2. Use the selected verified source as the complete base catalog.
3. Reconcile permitted local observations under Starmap's authority policy.
4. Apply Starport account policy, credentials, adapters, and availability to routing.

A complete source snapshot can remove a record. A union with the embedded
baseline would restore that record, so fallback and merge have different roles.
An incomplete provider reply must not act as a complete deletion list.

Newer evidence wins only within the appropriate source authority and scope.
An account-specific model list must not become a deployment-wide availability
claim. Local changes under internal authority require explicit permission.

## Startup and failure behavior

| Condition | Required behavior |
| --- | --- |
| First public-mode start without network or keys | Serve and persist the verified baseline. Report fallback. |
| Public-source outage after acceptance | Keep the accepted catalog and report source freshness separately. |
| First internal-authority start without an accepted internal catalog | Keep diagnostics available. Block inference until an approved internal generation exists. |
| Internal-authority restart during an outage | Use the retained internal generation within the operator's retention policy. |
| Invalid signature, checksum, schema, or identity | Reject the candidate. Keep the accepted generation. |
| Expired or revoked inference credential | Stop selecting that credential. Preserve catalog facts. |
| One provider acquisition failure | Retain that provider's previous evidence and publish other valid changes. |
| Unwritable required state directory | Report the path and failure. Do not claim persistent readiness. |
| Shared store outage | Keep catalog diagnostics available. Enforce gateway policy without bypassing shared authentication or budgets. |
| Duplicate local runtime identity or writable database path | Refuse the conflicting process and name the owning instance without changing its state. |
| Temporary mode with persistent storage selectors | Reject the conflicting settings before opening persistent stores. Link the persistent setup procedure. |
| Unsupported provider adapter | Show the catalog entry and its routing limitation. Do not activate an invalid route. |

Budget permission normally comes from current authenticated policy and usage records.
A failed lookup is unknown, not proof that no budget applies.
Distinguish exhausted budget, confirmed absence, and unavailable policy or usage.
Only the last condition needs the retryable refusal from D14.

Recovery cannot reconstruct revocations or spending absent from its backups.
Use independent durable evidence, or keep affected access restricted until an operator reconciles the missing interval.
Catalog publication does not independently authorize a new destination to receive inference credentials.

## Service objectives

The publisher cadence is four hours. The default public poll interval is one
hour. The proposed freshness objective is six hours under healthy dependencies.
GitHub can delay scheduled runs, so the schedule does not guarantee a maximum
catalog age. [GitHub schedule behavior](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows#schedule)

Freshness must distinguish the last successful source check, source observation
age, channel confirmation, and catalog content age. An unchanged successful run
confirms a check without inventing a new catalog generation.

Release criteria require zero invalid candidate activations, zero credential
values in catalog artifacts, and zero unauthorized authority fallback.
Performance thresholds must use recorded catalog sizes and fleet benchmarks.
This draft does not invent an unmeasured startup or throughput guarantee.

Low gateway overhead applies to ordinary uncached requests as well as cache hits.
The target keeps active catalog indexes and valid working records in memory.
Persistent storage still owns restart state, configuration authority, credentials, and required accounting evidence.
Strict shared admission can require a remote atomic operation.
Its measured round trips count toward the gateway latency budget.

Warm request processing must not rebuild the catalog, derive stored credential keys, refresh peers, or fetch catalog sources.
Bound cache fills, retained generations, tenant working sets, queued bytes, and stream accumulation.
Keep required admission synchronous. Optional analytics and cache persistence use bounded background work.
Authorization expiry, known withdrawals, and unknown required budgets still refuse new inference.

CSP0.4 records a complete baseline and a reviewed performance profile before dependent optimizations.
The profile defines numeric request percentiles, allocations, stream delay, cache deadlines, capacity, and memory limits on fixed runners.
Unset limits block qualification. The profile cannot adopt the old controller-only 50 ms threshold as full-path evidence.
The measured catalog and decryption costs are baseline defects, not production guarantees or accepted release limits.
Specification section 8.9 defines the request-path contracts and measurement boundaries.

## Scope limits

This work defines catalog lifecycle and integration. It does not add model
hosting, provider access entitlements, billing integration, or catalog-owned
inference credentials. OpenRouter compatibility covers a versioned, tested API
surface. It does not imply identical commercial services or model availability.

The initial production boundary is one region. Disaster recovery restores one
designated writer before subscribers resume. A storage adapter does not establish
active-active support. Shared local database files and untested backend combinations
must not appear as supported recipes.

## Documentation and operator experience

Starport documentation is part of the product. It must explain how to install,
configure, inspect, migrate, and recover each supported architecture.
The public site and embedded documentation must share authored content and
generated configuration reference data. The installed documentation must remain
available when catalog readiness fails or the reader has no provider key.
Static documentation must not disclose deployment configuration or secret references.

The entry page must help readers choose a topology. Each recipe must name
its catalog authority, allowed network destinations, storage roles, credential roles,
startup behavior, and failure recovery. Separate procedures must cover GitHub
updates on the central Starmap server and updates on its Starport subscribers.

The console must separate browser preferences, gateway configuration, catalog
configuration, provider acquisition credentials, and inference credentials.
An operator must see where a value comes from before attempting to change it.
A local developer with operator authority gets the same diagnostics as an
enterprise operator. Merely opening the UI must not grant administrative authority.

Long documentation uses a reading layout. Settings use grouped fields with
help links and visible ownership. Compact catalog status remains a summary,
with links to full configuration and procedures.
The engineering specification defines measurable typography and accessibility targets.

Each storage recipe must identify KV records, SQL records, blob metadata, blob bytes, runtime evidence, and disposable caches separately.
The file inventory must include configuration, database sidecars, tokens, source checkouts, audit receipts, migration journals, and backup manifests.
Runtime enforcement, diagnostics, and documentation must use one semantic policy definition for each file role.
Diagnostics must remain passive and retain uncertainty when their observations cannot establish compliance.
Editable catalog workspaces must remain separate from private accepted catalog state.
Workspace replacement must preserve operator access restrictions and editing rights, or refuse before publication.

Show effective absolute paths and their override origins for the running deployment.
Shared services still require bootstrap configuration and distinct replica state.
Container recipes must prove persistence through container replacement, not only process restart.

Explain disk-backed Badger, in-memory Badger, and Ristretto as separate concepts.
Document cache limits and data-loss bounds with measured evidence for each supported production recipe.
A cache-clear operation must never remove catalogs, credentials, accounts, budgets, or usage records.

The README must lead readers from installation to a visible result. Its demo
must show the catalog working before provider setup, followed by a successful
request through the gateway. Catalog inspection needs no provider credential.
Remote inference needs a configured provider credential and connectivity.
The recording must make that transition clear.

A short demo does not replace installation instructions. The README must retain
copyable commands, platform-specific installation paths, a static preview, and a
transcript of the recorded sequence. Starmap's README must describe the
catalog owner and link to Starport for inference.

Recording duration and frame pacing are editorial targets, subject to human review.
Readability, artifact identity, secret protection, and truthful results remain release requirements.
The public docs release needs a named hosting service, owner, version URL, and rollback procedure.

## Remaining product choices

The engineering proposals need review for field authority, path migration,
and the maximum age an enterprise permits for retained internal catalogs.
The default proposal allows retained catalogs without a hard expiry and exposes
freshness warnings. An operator can configure a hard expiry.

Upstream administration remains a separate proposed boundary under D11.
The shared SQL choice in D15 and the local-edit allowlist need engineering review.
Production performance and recovery objectives require an agreed workload and measured evidence.
D19 adopts the architecture for serving valid state from memory. Numeric limits and authorization validity intervals remain profile inputs for CSP0.4.

D12 keeps guided saves against the selected local or shared configuration authority.
Externally managed deployments receive validated diffs. Database moves remain migration procedures.
Shared-store availability must never silently switch the configuration authority back to a file.
Model deprecation intervals and internal permission freshness limits need explicit release profiles.
Normal metadata retention must not silently extend permission after a known withdrawal.

The two documents become implementation input after these choices settle.
Existing implementation does not count as evidence that a new requirement passes.

## Execution activation

The user activated the whole-plan goal on 2026-09-05.
D6, D7, D11, and D15 use the specification defaults as engineering decisions.
These selections do not relabel an engineering proposal as an explicit user confirmation.
The [activation resolution](../../plans/proof/starport-production-catalog/activation-2026-09-05/RESOLUTION.md) closes the seven audit gaps in task and acceptance contracts.
Implementation evidence remains separate from those contract corrections.
