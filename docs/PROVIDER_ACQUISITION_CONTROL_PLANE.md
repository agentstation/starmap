# Starmap Provider Acquisition and Credential Control Plane

Last updated: 2026-07-13 America/Chicago

This document is the durable implementation plan and execution ledger for
making provider acquisition, authentication, scope, and publication
classification explicit in Starmap. It is written so an agent can resume after
context compaction without relying on conversation history.

## Mission

Replace Starmap's provider-wide, API-key-specific catalog acquisition model
with a typed, configuration-driven source contract that supports:

- unauthenticated public model catalogs;
- public catalogs that require an API key;
- public regional catalogs that require a cloud SDK credential chain;
- sources that accept alternative authentication mechanisms;
- account-, project-, workspace-, tenancy-, subscription-, and region-scoped
  discovery;
- customer-only model availability, early access, negotiated pricing,
  account-specific limits, deployments, endpoints, aliases, and custom models;
- deterministic fixture/evidence policy derived from provider source semantics.

The result has one canonical `Catalog` containing the richest inventory
available in the current credential context. Observation scope and provenance,
not a parallel catalog type, preserve the strict boundary between globally
publishable facts and credential-scoped facts. Starmap must never serialize,
log, retain, or publish credential values.

## Repository and Worktree Baseline

- Repository family: `/Users/jack/src/github.com/agentstation/starmap`
- Implementation worktree:
  `/Users/jack/src/github.com/agentstation/starmap-worktrees/provider-expansion-wave0`
- Branch at plan creation: `codex/provider-expansion-wave0`
- HEAD at plan creation: `59a1ba23f21746f625454c2528e9842955b20b9f`
- Dirty entries at plan creation: 130 intentional tracked/untracked entries from
  the existing provider-expansion work
- Existing provider-expansion changes are user-owned and must be preserved.
- Do not duplicate an active provider-expansion branch, worktree, draft PR,
  release run, or catalog-generation run. Inspect current state before any
  external mutation.

Audit snapshot on 2026-07-13:

- Protected base `origin/main` is
  `9508ee7866e4683e001e7ad153319d348433045d`; this branch is two commits ahead
  at `59a1ba23f21746f625454c2528e9842955b20b9f`.
- Draft PR #40 is open and clean at that exact committed head. Its two hosted
  checks passed, but they do not cover the current uncommitted overlay.
- The committed PR range is 382 files with 33,606 insertions and 3,111
  deletions. The current overlay is 133 status entries; tracked changes alone
  span 90 files with 1,188 insertions and 8,857 deletions.
- A separate release worktree exists. No active release or catalog-generation
  run was found during this audit; that lane remains out of scope and untouched.
- GitHub reports protected `main` at `9508ee7866e4683e001e7ad153319d348433045d`.
  The separate local main worktree is divergent at `3787d716` (one local commit
  ahead and eleven remote commits behind) and is not an implementation or merge
  base for this plan; it was inspected read-only and left untouched.
- `pkg/sources/cloud_credentials.go` currently contains accidental prose at
  line 10 and prevents Go compilation. This is PAF-026, not a passing
  characterization result.
- No Codex goal was active at audit close. Goal activation and the immediate
  compile-baseline repair therefore remain correctly owned by pending PAC0.3.

Current implementation seams:

- Provider configuration: `pkg/catalogs/provider.go`
- Embedded provider YAML: `internal/embedded/catalog/providers.yaml`
- Provider preflight/fetching: `pkg/sources/providers.go`
- Authentication status: `internal/auth/`
- HTTP authentication: `internal/transport/auth.go`
- Test-only SDK-neutral chain helper to replace with production adapters:
  `pkg/sources/cloud_credentials.go`
- Provider composition: `internal/providers/registry/`
- Provider fan-out: `internal/sources/providers/`
- Native cloud sources: `internal/providers/{bedrock,azurefoundry,oci}/`
- Pipeline source construction: `internal/catalog/pipeline/sources.go`
- Public observation scope/kind: `pkg/catalogmeta/source_observation.go`
- Parallel private inventory to remove: `pkg/catalogs/customer_inventory.go`
- Provider fixture contracts: `internal/providers/contract_test.go`
- Provider fixture policy: `internal/providers/fixtures/policy.yaml`
- Normative provider instructions: `docs/ADDING_PROVIDERS.md`
- Provider implementation contract: `docs/PROVIDER_IMPLEMENTATION_CONTRACT.md`

### PAC0.4 Provider Credential, Scope, and Ownership Matrix

This checked-in matrix is the exhaustive current-to-target disposition. `GP`
means globally public, `RP` regionally public, and `CS` credential-scoped. A
slash separates catalog from invocation auth. `key` means the conventional
typed `api_key`; `chain` means the provider-inferred official SDK chain. Every
listed environment name has exactly one target owner. Endpoint URLs, mappings,
and governed sweep inputs are source configuration unless the row identifies an
SDK-derived invariant. The ordinary embedded path is provider fan-out to the
registry and then the named connector/adapter; the three native sources are
currently constructed directly by the pipeline and must move behind that same
configured registry seam.

| Provider / logical source | Catalog / invocation auth | Typed environment and scope ownership | Scope, topology, and publication | Connector or adapter, endpoint owner, and production disposition |
| --- | --- | --- | --- | --- |
| Alibaba Model Studio / `models` | `key / key` | credential `DASHSCOPE_API_KEY`; endpoint override `ALIBABA_MODEL_STUDIO_BASE_URL` | GP, single endpoint, publishable | Shared OpenAI connector; source YAML owns URL/path/author mappings |
| Anthropic / `models` | direct-header `key / key` | credential `ANTHROPIC_API_KEY` | GP, bounded pagination, publishable | Shared Anthropic connector owns wire protocol; source YAML owns URL and `x-api-key` override |
| Baseten / `models` | `key / key` | credential `BASETEN_API_KEY`; endpoint override `BASETEN_BASE_URL` | GP, single endpoint, publishable | Shared OpenAI connector; source YAML owns URL/path/author mappings |
| Cerebras / `public-models` | `optional / key` | credential `CEREBRAS_API_KEY` | GP for both authenticated and unauthenticated catalog results; one bounded execution; publishable | Shared OpenAI connector; source YAML owns public URL/mappings and separate invocation route |
| Cloudflare Workers AI / `catalog` | `key / key` | credential `CLOUDFLARE_API_TOKEN`; required account binding `CLOUDFLARE_ACCOUNT_ID`; endpoint override `CLOUDFLARE_API_BASE_URL` | GP authenticated catalog with account-routed request; publishable | Retained Cloudflare adapter for account path/pagination; source YAML owns base URL and binding; never `cloud_chain` |
| SambaNova / `models` | `key / key` | credential `SAMBANOVA_API_KEY` | GP, single endpoint, publishable | Deletion-test against shared OpenAI connector; source YAML owns URL |
| Cohere / `models` | `key / key` | credential `COHERE_API_KEY` | GP, bounded cursor pagination, publishable | Retained Cohere adapter only for native schema/pagination; source YAML owns URL |
| Cursor / application catalog | `none / none declared` | no executable credential or scope input | Application-only discoverability; non-acquirable and no false empty success | Registry metadata only; remove empty `applicationClient` adapter |
| Databricks / `public-foundation-models` and `workspace-endpoints` | `none`, then `key / key route` | credential `DATABRICKS_TOKEN`; workspace endpoint/binding `DATABRICKS_HOST`; workspace identity comes from request context | Public documentation source is GP and publishable; bounded workspace pagination is CS and never publicly writable | Retained Databricks adapter for HTML support matrix and workspace schema; source IDs prevent workspace token attachment to docs origin |
| DeepInfra / `account-models` with `public-models` fallback | `key` preferred, `none` fallback / `key` | one credential owns ordered `[DEEPINFRA_TOKEN, DEEPINFRA_API_KEY]`; first is canonical and differing simultaneous values fail | Authenticated account inventory is CS; public `/models/list` fallback is GP; acquisition group selects one source, never a comparison delta | Retained delta adapter only if native public/account schemas cannot use shared OpenAI; source YAML owns both official endpoints and preference |
| DeepSeek / `models` | `key / key` | credential `DEEPSEEK_API_KEY` | GP, single endpoint, publishable | Shared OpenAI connector; no provider-local protocol fixture without a proven delta |
| Fireworks AI / `models` | `key / key` | credential `FIREWORKS_API_KEY` | GP, single endpoint, publishable | Shared OpenAI connector; source YAML owns URL and mappings |
| Google AI Studio / `models` | query/direct `key / key` | credential `GOOGLE_API_KEY` | GP authenticated catalog, bounded pagination, publishable | Shared Google connector owns native wire/query-key behavior; source YAML owns URL |
| Google Vertex / regional publisher and project models | `chain / chain` | project binding owns ordered `[GOOGLE_CLOUD_PROJECT, GOOGLE_VERTEX_PROJECT]`; location binding owns `[GOOGLE_CLOUD_LOCATION, GOOGLE_CLOUD_REGION, GOOGLE_VERTEX_LOCATION]`; `GOOGLE_APPLICATION_CREDENTIALS` is advisory metadata for the Google SDK-owned ADC chain | RP governed/project iteration plus CS project models where returned; public generation rejects any CS observation | Google SDK adapter registered by provider identity; source YAML owns binding roles, publisher list, endpoint template, and unified scope precedence; SDK owns ADC credential precedence |
| Groq / `models` | `key / key` | credential `GROQ_API_KEY` | GP, single endpoint, publishable | Shared OpenAI connector; no provider-local response-schema retest |
| Hyperbolic / `models` | `key / key` | credential `HYPERBOLIC_API_KEY`; endpoint override `HYPERBOLIC_BASE_URL` | GP, single endpoint, publishable | Shared OpenAI connector; source YAML owns URL/path |
| Hugging Face / router inventory | `optional / key` | credential `HF_TOKEN` | GP regardless of credential attachment under current route; one bounded execution; publishable | Retained Hugging Face adapter for model/provider expansion and pricing shape; source YAML owns URL |
| Novita / `models` | `key / key` | credential `NOVITA_API_KEY`; endpoint override `NOVITA_BASE_URL` | GP, single endpoint, publishable | Deletion-test delta adapter against shared OpenAI; retain only exact schema normalization difference |
| NVIDIA / public catalog and customer NIM | `optional`, then source-specific `key / key` | credential `NVIDIA_API_KEY`; NIM base URL, account, deployment, and definition mapping are request-scoped typed bindings | Public catalog is GP and publishable; NIM inventory is CS and never publicly writable; two distinct logical sources | NVIDIA adapter owns public schema; NIM may reuse OpenAI connector through configured source; remove test-only exported fetch path |
| Moonshot AI / `models` | `key / key` | credential `MOONSHOT_API_KEY` | GP, single endpoint, publishable | Shared OpenAI connector; no provider-local response-schema retest |
| Mistral / `models` | `key / key` | credential `MISTRAL_API_KEY` | GP, single endpoint, publishable | Delete Mistral adapter if normalized YAML plus shared OpenAI connector expresses its collection/mappings |
| OpenAI / `models` | `key / key` | credential `OPENAI_API_KEY` | GP authenticated catalog, single endpoint, publishable | Shared OpenAI connector owns the protocol fixture and normalization implementation |
| Scaleway / `models` | `key / key` | credential `SCW_SECRET_KEY`; region binding `SCW_REGION`; endpoint override `SCW_BASE_URL` | RP, single endpoint, publishable | Shared OpenAI connector; source YAML owns region/base URL behavior |
| Snowflake Cortex / session models | `key / no route declared` | credential `SNOWFLAKE_TOKEN`; endpoint override/account binding `SNOWFLAKE_ACCOUNT_URL`; region binding `SNOWFLAKE_REGION`; typed option `SNOWFLAKE_CORTEX_CROSS_REGION` | CS session inventory, single endpoint, never publicly writable | Retained Snowflake adapter for session envelope; reviewed static price/mode facts move from mini-catalog to canonical offerings |
| Together / serverless and dedicated inventories | `key / key` | credential `TOGETHER_API_KEY` | Two GP configured logical sources with bounded request counts; both publishable | Retained Together delta adapter only for native fields; source YAML owns the serverless/dedicated query distinction and URL |
| IBM watsonx / regional foundation models and deployments | `key / key route` | credential `IBM_WATSONX_TOKEN`; endpoint override `IBM_WATSONX_BASE_URL`; region binding `IBM_WATSONX_REGION` | Authenticated RP foundation inventory is publishable; bounded deployment inventory is CS and never publicly writable | Retained watsonx adapter for regional pagination and deployment schema; both become configured production sources |
| xAI / `language-models` | `key / key` | credential `XAI_API_KEY` | GP, single endpoint, publishable | Retain delta adapter only for the nonstandard response collection/fields; otherwise shared OpenAI connector |
| Amazon Bedrock / regional foundation models and application profiles | `chain / chain` | AWS region/profile inputs remain SDK-owned; governed commercial/GovCloud region sweeps are source config, not credential names | RP foundation models/prices are publishable; application/inference profiles that reflect enabled account inventory are CS; bounded multi-region execution | Register AWS SDK adapter; source YAML owns sweep and public pricing endpoint; pipeline direct constructor is removed |
| Microsoft Foundry / regional models and deployments | `chain / chain` | bindings `AZURE_SUBSCRIPTION_ID`, `AZURE_RESOURCE_GROUP`, `AZURE_FOUNDRY_ACCOUNT`, `AZURE_FOUNDRY_LOCATION`; endpoint override `AZURE_FOUNDRY_ENDPOINT` | RP account model availability and public retail pricing may publish only when classified public; deployments are CS and never publicly writable | Register Azure SDK adapter; source YAML owns realm/source endpoints and bindings unless proven SDK invariants; pipeline direct constructor is removed |
| Oracle OCI Generative AI / regional models and endpoints | `chain / chain` | bindings `OCI_REGION`, `OCI_COMPARTMENT_ID`; realm is typed static/sweep config | RP base-model availability is publishable; dedicated/custom endpoints are CS and never publicly writable | Register OCI SDK adapter; SDK derives regional service endpoint; reviewed prices move to canonical offerings; pipeline direct constructor is removed |

Matrix invariants:

- The 27 embedded provider rows plus Bedrock, Microsoft Foundry, and OCI cover
  every current acquisition path.
- Public and credential-scoped paths sharing a provider are separate configured
  logical sources when their endpoint, topology, or publication scope differs.
- The provider fan-out/registry call path owns all embedded execution. Direct
  native construction in `internal/catalog/pipeline/sources.go` is temporary
  current reality, not the target interface.
- Each environment name is credential input/alias, binding, endpoint override,
  typed option, or SDK advisory metadata. No name is duplicated across owners.
- CS rows may populate only the caller's isolated in-memory contextual catalog.
  No durable credential-scoped store is part of this plan; every existing store
  and public generation/distribution path rejects them before writing.

Binding value-source, role, and precedence rules:

| Binding class | Value source and role | Locked precedence and failure behavior |
| --- | --- | --- |
| Credential input | Request-scoped secret or declared environment input; `required_input` only when selected auth requires it | Explicit request value, then declared environment names in order. Missing may fall through only under `optional` or required alternatives; invalid present values and differing aliases fail typed |
| Endpoint override | Declared environment input; `required_input` only when the source has no literal base URL | Request override, then environment override, then literal configured URL. The normalized source owns allowed origin/path and redirect policy |
| Account/project/subscription/workspace/compartment scope | Explicit request, ordered environment inputs, then declared cloud-profile source; `required_input` | Explicit request wins. For Vertex project: `GOOGLE_CLOUD_PROJECT`, `GOOGLE_VERTEX_PROJECT`, ADC quota project, then gcloud project. For Vertex location: `GOOGLE_CLOUD_LOCATION`, `GOOGLE_CLOUD_REGION`, `GOOGLE_VERTEX_LOCATION`, gcloud region, then the documented source default. Other providers use the single names in their matrix rows before a declared SDK/profile fallback |
| Governed region/publisher sweep | Checked-in source configuration; `iteration` | The normalized sorted sweep is authoritative for that source execution; it never falsely fails credential preflight and never derives regions from ambient secrets |
| Provider response identity/region/deployment | Provider response; `output_metadata` | It may describe the observation and canonical facts but can never satisfy a required input or alter credential selection |
| Operational option | Explicit request, declared environment input, then source default | Parsing and validation occur once during normalization; unknown or invalid values fail typed before acquisition |
| SDK credential advisory metadata | Official SDK-owned environment/profile/workload identity | Starmap declares `cloud_chain` and safe diagnostics only; the SDK owns credential search order, refresh, and caching |

Current descriptive metadata must migrate without text loss:

| Name | Current description | Typed owner |
| --- | --- | --- |
| `GOOGLE_VERTEX_PROJECT` | `GCP project ID (optional - falls back to gcloud config)` | Vertex project binding alias |
| `GOOGLE_VERTEX_LOCATION` | `GCP location/region (optional - falls back to gcloud config or us-central1)` | Vertex location binding alias |
| `GOOGLE_APPLICATION_CREDENTIALS` | `Path to service account JSON file (optional - uses Application Default Credentials)` | Non-executable Google SDK cloud-chain advisory metadata |

### PAC0.5 Locked Contract Fixture Matrix

These acceptance anchors define the exact authoring interface before schema
implementation. PAC0.6 turns them into table-driven characterization/target
tests; later phases may add cases but may not weaken or silently rename these.

| Fixture anchor | Positive contract | Required negative proof | Removal or retained depth it governs |
| --- | --- | --- | --- |
| `none_prohibits_credentials` | A GP Databricks documentation source with `auth: none` performs one request without consulting or attaching `DATABRICKS_TOKEN` | Provider token present, hostile URL, and redirect fixtures still emit no auth header; `none` in a list and credential references fail validation | Removes `auth_required: false` ambiguity and ambient provider-wide attachment |
| `optional_key_chain_absence` | One source with `auth: optional` selects conventional key, then registered chain, then one unauthenticated request only when both are absent | Present-invalid key/chain fails without fallback; counters reject anonymous-plus-authenticated comparison and undeclared named credentials | Replaces boolean optional auth and prevents overlay/delta acquisition |
| `required_api_key_defaults` | `auth: api_key` plus scalar env expands to bearer `Authorization`, with DeepInfra proving ordered env aliases | Missing input fails before transport; empty/duplicate env lists and differing simultaneously set aliases fail typed | Deletes repeated pattern/header/scheme/query boilerplate and provider runtime secret fields |
| `required_cloud_chain` | `auth: cloud_chain` selects exactly one provider-registered official SDK adapter | Missing/duplicate adapter, unsupported provider, and vendor chain name in YAML fail before transport | Replaces Google special-case auth and test-only generic callback sequencing with a real multi-adapter seam |
| `ordered_required_alternatives` | `auth: [api_key, cloud_chain]` selects the first available method in order | Empty/duplicate list, `none`/`optional` in list, no available method, and present-invalid first method fail typed | Avoids speculative `any_of`/`all_of` expression implementation |
| `compound_named_method` | One future/basic/session/OAuth method owns all typed inputs behind one name | Top-level `all_of`, partial input set, unknown input, and secret-bearing serialization fail | Keeps compound mechanics behind a deep method interface rather than widening source auth |
| `cloudflare_token_account` | Cloudflare uses required token credential plus required account binding and bounded pagination | `cloud_chain`, absent account, malformed base URL, cross-origin next page, and credential logging fail | Retains Cloudflare adapter only for account-path/native response variation |
| `cerebras_catalog_invocation_split` | GP catalog is `optional`; invocation route is required `api_key`; one present credential is reused request-scoped | Removing catalog auth cannot remove invocation auth; invalid key cannot fall back anonymously | Replaces provider-wide auth while preserving inference transport |
| `anthropic_transport_override` | Conventional key overrides only header `x-api-key` and direct scheme; bounded pagination uses shared connector | Bearer default, duplicate key placement, unknown scheme, and provider-local duplicate protocol fixture fail | Retains deep Anthropic connector and deletes repeated provider mechanics |
| `vertex_profile_fallback` | Provider-inferred Google chain plus typed project/location bindings follows the locked matrix precedence | Connector/status precedence divergence, vendor chain YAML, missing required project, and credential-object serialization fail | Replaces hard-coded Google checker branches; SDK retains ADC mechanics |
| `bedrock_governed_sweep` | Configured sorted region sweep is an iteration binding; AWS chain resolves once per acquisition group and regional observations remain isolated | Sweep is not treated as a missing preflight input; duplicate/unknown regions, unbounded fan-out, and CS profile publication fail | Moves native constructor/region mini-catalog into configured registry execution |
| `modelsdev_env_advisory_only` | Imported env names/descriptions remain non-executable advisory metadata | Advisory fields cannot create credential definitions, bindings, endpoint overrides, auth readiness, or transport headers | Separates external documentation metadata from local executable configuration |
| `shared_acquisition_group` | Related sources may reuse one request-scoped SDK session/resolver result while each logical source executes once | Cross-provider/account reuse, duplicate source execution, shared mutable result, or unbounded group requests fail | Gives acquisition grouping leverage without a customer overlay or global cache |
| `credential_scoped_catalog` | Custom models, prices, limits, deployments, aliases, and endpoints normalize into one contextual Catalog with CS observation | Public bootstrap/generation/evidence/distribution/remote paths fail before bytes; no record filtering or second catalog product | Removes `CustomerInventory`, overlay, delta, and parallel store/query interfaces |
| `exact_schema_v2_break` | Current provider/source config and canonical definitions/offerings encode/decode deterministically at version 2 | Singular key/endpoint, `auth_required`, old expressions, schema-v1 payload/manifest, alias, fallback, and migration-on-read fixtures fail before publication | Deletes every unpublished compatibility path while retaining operational imports only by current-use proof |

Fixture design rules:

- Tests cross the normalized configuration and acquisition interfaces used by
  production; they do not test private parsing helpers as substitute behavior.
- Each positive fixture has a paired negative assertion and request counter or
  serialization/publication assertion where applicable.
- Exact-compatible providers compose shared connector fixtures; only a proven
  provider delta owns a provider-local response fixture.
- Absence searches support these fixtures but never replace them.

## Relationship to Existing Control Planes

- `docs/STARPORT_CATALOG_CONTROL_PLANE.md` remains the historical P0-P13
  provider-expansion and release evidence ledger. Do not rewrite or erase its
  completed evidence.
- This document is the active source of truth for the new provider acquisition,
  credential, source-scope, single-catalog publication, and derived-fixture
  work.
- When this work supersedes a prior P13 implementation claim, append a dated
  cross-reference or supersession note to the appropriate durable ledger; do
  not retroactively change the historical result that was true at that time.
- Release, publication, protected-main, and external-mutation boundaries from
  the existing control plane remain in force.

## Status Legend

- `DONE`: the row's success criteria passed and exact evidence is recorded.
- `IN_PROGRESS`: active work is underway.
- `PENDING`: not started.
- `BLOCKED`: a genuine external or authorization blocker prevents progress;
  the execution log records the exact blocker and re-entry condition.
- `REJECTED`: a finding was proven invalid or deliberately declined by the
  user with rationale and residual risk recorded.

`DEFERRED` is not a completion state for this control plane. A phase gate is an
automatic transition to the next non-`DONE` row, not a reason to stop.

## Global Constraints

1. Starmap has not launched. Implement the new provider configuration and
   canonical schema-v2 representation as a direct breaking change. Do not add
   migration-on-read, schema aliases, deprecated fields, compatibility ranges,
   dual decoders, or documentation promises for the unpublished shape.
2. Keep canonical catalog schema version `2`. Old prelaunch v2 payloads that do
   not have the exact current shape must fail rather than be mistaken for the
   new contract.
3. `providers.yaml` owns declarative provider acquisition facts. Go owns typed
   protocol/SDK behavior, credential resolvers, validation, orchestration, and
   transformations that cannot safely be expressed as data.
4. Credential definitions may contain only secret-safe metadata such as IDs,
   kinds, environment-variable names, transport placement, schemes, and
   descriptions. `cloud_chain` is a built-in auth method whose provider-specific
   adapter is inferred from the provider registry rather than named in YAML.
   Credential values and resolved SDK objects remain runtime-only.
5. Authentication describes how Starmap may call a source. Source bindings
   describe the account/project/region context required by that source.
   Observation scope describes whether the resulting catalog is globally
   public, regionally public, or credential-scoped. These axes must not be
   conflated.
6. Public data may require authentication. Authenticated data is not
   automatically customer-private.
7. Credential-scoped facts may enter the contextual `Catalog` returned by that
   acquisition, but their observation must never enter embedded bootstrap data,
   public generations, public distribution, or normalized public evidence.
8. Preserve typed errors, context cancellation, bounded retries/pagination,
   deep-copy ownership, deterministic output, and race safety.
9. Preserve unrelated dirty work. Never use destructive Git commands. Do not
   stage, commit, push, mutate a PR, merge, publish, release, or deploy unless
   the user has authorized that exact class of mutation. Reuse an existing
   authorized draft PR rather than creating a duplicate.
10. Never print credentials or raw customer identifiers. Live checks must load
    ignored local environment files without echoing values.
11. Absence searches are supporting evidence only. No cleanup row may become
    `DONE` without positive behavior, negative behavior, and focused tests.
12. Research current provider behavior only from primary provider documentation
    and first-party APIs, and record retrieval date and source.
13. Preserve amended ADR 0001's one-product boundary: `Catalog` is the only
    normalized catalog product. Remove `CustomerInventory` and do not introduce
    `CatalogOverlay`, `EffectiveCatalog`, customer deltas, or another parallel
    catalog interface. Publication eligibility is derived from observation
    scope and provenance.
14. YAML is an authoring interface. It may use documented shorthands, but one
    deep normalization module must expand them into a strict canonical in-memory
    contract before validation, resolution, connector selection, or transport.
15. Normal acquisition executes each logical source exactly once after auth is
    selected. One logical execution may issue only the source's declared,
    bounded pagination, retry, or protocol requests. Required auth uses the
    first available declared method; `auth: optional` tries the conventional API
    key, then a registered cloud chain, and uses unauthenticated transport only
    when neither is available. `auth: none` never resolves or attaches a
    credential. Do not execute anonymous and authenticated variants to construct
    a delta or overlay. Public generation starts from an isolated public
    baseline and fails if the selected acquisition resolves to a
    credential-scoped observation; it obeys the same one-execution rule.
16. Starmap owns the `cloud_chain` interface, provider-to-adapter registration,
    validation, and secret-safe diagnostics. Official cloud SDKs own credential
    search order, refresh, caching, workload/managed identity, and profile
    mechanics. Do not reimplement those chains or expose vendor chain names in
    ordinary provider YAML.
17. Authentication optionality and source optionality are independent. `auth:
    optional` permits an unauthenticated request after credential absence;
    source optionality determines whether an unavailable or failed logical
    source degrades the wider acquisition. Neither implies the other.
18. Bias the implementation toward net deletion of bespoke production code,
    exported interfaces, provider-local protocol tests, and duplicate tooling.
    Prefer normalized YAML and an existing deep connector module before adding
    a provider adapter. Every new module, interface, seam, or adapter must pass
    the deletion test, name the concrete variation it hides, and improve
    leverage or locality. Phase evidence records diffstat and exported-surface
    change; unexplained net growth is not successful completion.
19. Do not create a second provider-fact schema beside canonical provider
    source configuration and canonical catalog definitions/offerings. Acquisition
    sweep inputs and official source endpoints belong to normalized source
    configuration; reviewed model, mode, and price facts belong to canonical
    catalog YAML; provenance references those facts without duplicating them.

## Locked Architecture Decisions

| ID | Decision | Rationale | Verification |
| --- | --- | --- | --- |
| PAD-001 | A provider supports multiple named credential definitions plus built-in auth methods | One provider may expose API-key, OAuth/session, and a provider-inferred cloud-chain path | JSON/YAML round-trip, duplicate-name rejection, and built-in-method validation |
| PAD-002 | Authentication is declared per catalog source and invocation route | Public discovery and inference/customer discovery frequently use different authentication | Cerebras/public and mixed-provider fixtures |
| PAD-003 | `auth` is explicit on every source and is one of `none`, `optional`, one required method, or an ordered list of required alternatives | The interface stays shallow enough to author while preserving deterministic selection and avoiding an expression language | scalar/list normalization, ordering, duplicate, empty, and truth-table tests |
| PAD-004 | `none` stands alone and prohibits credential resolution; `optional` tries conventional `api_key`, then registered `cloud_chain`, then one unauthenticated request only when both are absent | Preserve credential-preferred optional access without treating no-auth as a credential alternative or leaking ambient provider credentials into a no-auth source | none/optional/required truth table, no-credential-lookup test, and Databricks public-source fixture |
| PAD-005 | Scope dimensions are typed and independent of credentials | Account IDs and regions are routing/scope inputs, not secrets or credential mechanisms | scope-binding validation tests |
| PAD-006 | Every source declares an observation-scope policy; the resolved observation has exactly one scope | Publication eligibility must be explicit without creating another catalog product | global/regional/credential-scoped validation and publication tests |
| PAD-007 | `Catalog` is the sole normalized product; credential-scoped models, offerings, entitlements, prices, limits, and deployments use the same canonical definitions/offerings under scoped provenance | One deep module gives callers leverage and keeps identity, reconciliation, copying, and validation local | exact symbol-removal searches, canonical encode/store tests, isolation tests, and ADR conformance |
| PAD-008 | Credential values and resolved SDK-native objects are runtime-only | Public configuration may describe authentication but must never contain secrets | reflection/serialization/logging tests |
| PAD-009 | Fixture and evidence requirements derive from source contracts | Test policy should follow acquisition reality instead of duplicating provider semantics | exhaustive derivation test; broad policy deletion |
| PAD-010 | The migration is an exact-current schema-v2 break | Prelaunch compatibility would create ambiguity and maintenance burden | old-shape negative fixtures and exact-current positive fixtures |
| PAD-011 | `none`, `optional`, `api_key`, and `cloud_chain` are built-in auth authoring forms; `api_key` defaults to `Authorization: Bearer`, while `cloud_chain` selects the provider's registered SDK adapter | Common YAML should state the auth policy without restating transport defaults or AWS/Azure/GCP/OCI identity | exhaustive default-expansion and registry-validation tests plus Anthropic/Google/Databricks overrides |
| PAD-012 | Environment-variable names remain first-class secret-safe metadata at one typed owner | CLI help, docs, validation, and runtime resolution need the names even though values remain runtime-only | exhaustive legacy-to-current env migration matrix and round-trip tests |
| PAD-013 | A logical source is executed once after auth selection: required methods choose the first available alternative; `optional` prefers available standard credentials and falls back only after absence; `none` uses unauthenticated transport | The selected execution is the available inventory; anonymous comparison would add acquisitions and overlay semantics without improving the canonical catalog, while bounded pages/retries remain legitimate parts of one execution | execution-count plus bounded transport-request fixtures for none, optional key, optional chain, optional absence, required alternatives, required absence, pagination, and retry |
| PAD-014 | The provider registry owns at most one default `cloud_chain` adapter per provider; the official SDK owns the chain's internal precedence and lifecycle | This is a deep seam across four real implementations without leaking vendor mechanics into YAML or building a second cloud-auth framework | exhaustive provider/adapter registry test, AWS/Azure/GCP/OCI SDK-fake tests, unsupported-provider negative test, and Cloudflare token-only test |
| PAD-015 | Compound authentication is one named method with multiple typed inputs, not a top-level `all_of` expression | Basic, OAuth client, session, and future compound mechanisms should hide their internal inputs behind one deep method interface | multi-input credential validation/resolution tests and exact absence search for auth-expression compatibility |
| PAD-016 | Provider acquisition configuration and canonical catalog YAML are the only fact authorities; no parallel pricing/region mini-catalog exists | A second fact schema weakens locality, duplicates validation/identity, and can drift from the canonical catalog | exact parallel-schema/file searches, canonical offering price/mode tests, source-config sweep tests, and Snowflake/OCI/Bedrock production-path tests |
| PAD-017 | Credential-scoped catalog facts and runtime-only sensitive inputs are separated by a field-level sensitivity matrix | One catalog may contain contextual deployments and endpoints without serializing credentials, tokens, or routing-only tenant identifiers or allowing durable/public publication | reflection/serialization/logging/store-rejection/publication matrix tests and amended ADR conformance |
| PAD-018 | One credential may declare an ordered scalar-or-list environment input; the first name is canonical and differing simultaneously set values fail typed | DeepInfra's first-party documentation currently uses both `DEEPINFRA_TOKEN` and `DEEPINFRA_API_KEY`; preserving both operational names must not create two auth methods or a silent precedence ambiguity | scalar/list normalization, ordered-resolution, same-value, conflicting-value, documentation, and exact env-ownership tests |

## YAML Authoring and Normalization Contract

`global_public` and `none` are not synonyms. Observation scope classifies the
facts returned by acquisition; authentication independently states which
credential may be used and whether anonymous fallback is permitted. Every
source therefore keeps explicit `observation_scope` and `auth` declarations.
Starmap must not infer publication eligibility merely from authentication: a
public catalog may require a key, while an optional-key endpoint may return
credential-scoped facts only when authenticated.

The common forms are built in and need not redefine their mechanics for every
provider:

- `auth: none` means the source must never resolve or attach credentials. It
  stands alone and is invalid inside an auth list.
- `auth: optional` means the source accepts credential-preferred acquisition:
  try conventional `credentials.api_key` when declared, then the provider's
  registered `cloud_chain` when present, then perform one unauthenticated call
  only when neither method is available.
- `auth: api_key` references the conventional `credentials.api_key` definition
  and requires it.
- `auth: cloud_chain` invokes the single cloud-chain adapter registered for the
  provider and requires it. It does not require a
  `credentials.cloud_chain` block or a vendor-specific chain name.
- `auth: [api_key, cloud_chain]` requires the first available method in declared
  order and fails before transport when none is available.
- `credentials.api_key` implies `kind: api_key` and defaults to the
  `Authorization` header with the `Bearer` scheme.
- A nonstandard API key overrides only the differing transport fields.
- `api_key` is the conventional credential and transport default, but omitted
  `auth` is invalid; no global auth default is inferred for every source.
- `cloud_chain` is invalid during provider-contract validation when the provider
  has no registered adapter. The current built-ins are AWS for Bedrock, Azure
  `DefaultAzureCredential` for Microsoft sources, Google Application Default
  Credentials for Vertex, and OCI `DefaultConfigProvider` for OCI Generative AI.
  Cloudflare's current REST inventory uses an API token plus account binding and
  must not be labeled `cloud_chain`.
- Missing methods allow ordered resolution to continue. A present but malformed
  or invalid credential fails typed and never silently falls back to another
  method or unauthenticated access.
- A compound mechanism such as OAuth client credentials is one named method
  whose definition owns multiple typed inputs. Top-level `any_of`, `all_of`, and
  `required` auth expression forms do not exist in the exact-current schema.

One normalization module expands these shorthands, applies defaults, rejects
unknown fields and references, and produces the only canonical in-memory source
contract. Connectors and provider adapters consume that normalized contract;
they do not branch on YAML spelling. This creates a deep module: a small YAML
interface hides validation, defaults, secret-safe resolution, and transport
placement while keeping security-sensitive observation/auth choices visible.

Environment-variable metadata is retained, not dropped. The broad `env_vars`
list is removed because it duplicates and conflates concerns. Each name moves
to one typed owner:

- credential input: `credentials.<name>.env`, using a scalar normally and an
  ordered list only for evidenced aliases;
- account/project/region input: a typed source scope binding;
- endpoint override: `endpoint.base_url_env`;
- operational behavior: a typed source option.

Descriptions remain adjacent to those typed declarations and available to CLI
help and generated documentation. Values remain runtime-only. Official SDK
default chains interpret their own environment, profile, workload identity,
managed identity, and metadata sources; Starmap records only the
provider-inferred method and secret-free readiness or error diagnostics.

DeepInfra is the current evidenced alias case. Its [API introduction](https://docs.deepinfra.com/api-reference/introduction)
and [quickstart](https://docs.deepinfra.com/quickstart) use
`DEEPINFRA_TOKEN`, while its [authentication guide](https://docs.deepinfra.com/account/authentication)
also contains `DEEPINFRA_API_KEY` examples. The normalized `api_key` credential
therefore owns ordered inputs `[DEEPINFRA_TOKEN, DEEPINFRA_API_KEY]`, documents
the first as canonical, accepts equal duplicate values, and fails typed when
both are set differently.

Primary documentation retrieved 2026-07-13: DeepInfra sources linked above;
Google's [Vertex AI environment sample](https://docs.cloud.google.com/vertex-ai/generative-ai/docs/samples/googlegenaisdk-textgen-with-txt)
uses `GOOGLE_CLOUD_PROJECT`/`GOOGLE_CLOUD_LOCATION`, and Google's
[ADC search-order documentation](https://docs.cloud.google.com/docs/authentication/application-default-credentials)
confirms that the official authentication library, rather than Starmap, owns
credential-file, well-known-file, and metadata-server precedence.

## Target Contract

The exact Go names may change during implementation, but the semantics below
are normative.

Common bearer API key:

```yaml
- id: openai
  credentials:
    api_key:
      env: OPENAI_API_KEY
      description: OpenAI API key
  catalog:
    sources:
    - id: models
      observation_scope: global_public
      auth: api_key
      endpoint:
        type: openai
        url: https://api.openai.com/v1/models
  invocation:
    routes:
    - id: chat-completions
      api: chat_completions
      auth: api_key
      endpoint: https://api.openai.com/v1/chat/completions
```

Credential-prohibited public source:

```yaml
- id: databricks
  credentials:
    api_key:
      env: DATABRICKS_TOKEN
  catalog:
    sources:
    - id: public-foundation-models
      observation_scope: global_public
      auth: none
      endpoint:
        type: databricks
        url: https://docs.databricks.com/aws/en/machine-learning/model-serving/foundation-model-overview
```

The provider-level token may serve a different credential-scoped source or
invocation route. `auth: none` guarantees it is never sent to this public
documentation origin.

Optional-key discovery with required-key invocation:

```yaml
- id: cerebras
  credentials:
    api_key:
      env: CEREBRAS_API_KEY
  catalog:
    sources:
    - id: public-models
      observation_scope: global_public
      auth: optional
      endpoint:
        type: openai
        url: https://api.cerebras.ai/public/v1/models?format=openrouter
  invocation:
    routes:
    - id: chat-completions
      api: chat_completions
      auth: api_key
      endpoint: https://api.cerebras.ai/v1/chat/completions
```

Override only a nonstandard API-key transport:

```yaml
- id: anthropic
  credentials:
    api_key:
      env: ANTHROPIC_API_KEY
      transport:
        header: x-api-key
        scheme: direct
```

Cloud chain, typed scopes, endpoint override, and auth alternatives:

```yaml
- id: example-cloud
  credentials:
    api_key:
      env: EXAMPLE_API_KEY
  catalog:
    sources:
    - id: public-models
      observation_scope: regional_public
      auth: [api_key, cloud_chain]
      scopes:
        account:
          source: env
          name: EXAMPLE_ACCOUNT_ID
          role: required_input
        region:
          source: governed_sweep
          role: iteration
      endpoint:
        type: openai
        url: https://api.example.invalid/v1/models
        base_url_env: EXAMPLE_BASE_URL
```

This illustrative provider is valid only when its provider implementation
registers one cloud-chain adapter. Ordinary YAML never says `aws_default`,
`azure_default`, `google_adc`, or `oci_default`; provider identity supplies that
choice. A future provider with two genuinely different cloud-chain families
would require a reviewed extension to this contract rather than speculative
named-chain configuration now.

One endpoint whose inventory becomes credential-scoped when authenticated:

```yaml
- id: example-mixed
  credentials:
    api_key:
      env: EXAMPLE_MIXED_API_KEY
  catalog:
    sources:
    - id: models
      observation_scope:
        anonymous: global_public
        authenticated: credential_scoped
      auth: optional
      endpoint:
        type: openai
        url: https://api.example.invalid/v1/models
```

Required semantics:

- Every source declares `auth`; omission is an exact-current validation error.
- Scalar `auth` is `none`, `optional`, `cloud_chain`, or one required named
  credential reference. A sequence is an ordered list of required alternatives.
- `auth: none` stands alone and prevents environment lookup, cloud-chain
  resolution, credential attachment, and credential-derived diagnostics.
- `auth: optional` tries conventional `credentials.api_key`, then the registered
  `cloud_chain`, and selects unauthenticated transport only when both are absent.
  It does not automatically try arbitrary named credentials.
- Required alternatives are evaluated in declared order. Missing methods allow
  the next method; a present invalid method fails typed and stops resolution.
- Normal acquisition executes the logical source once after auth selection; the
  execution may issue only declared bounded pages, retries, or protocol
  requests. It never executes both authenticated and unauthenticated forms or
  computes a delta.
- One compound mechanism owns its required typed inputs in one named credential
  definition; auth has no `all_of`, `any_of`, or `required` expression form.
- Source optionality is separate: it controls wider acquisition degradation,
  while auth optionality controls whether this source may call unauthenticated.
- Sources reference credential IDs; they never duplicate secret-bearing
  credential configuration.
- Credential resolution returns secret-safe method/source diagnostics and an
  unexported runtime value.
- `scopes` declares typed dimensions and value bindings, never resolved
  customer-specific values.
- An invariant `observation_scope` scalar applies regardless of authentication.
  The auth-dependent mapping is used only when the same endpoint's eligibility
  changes according to whether a credential was actually attached.
- Every successful source emits ordinary canonical definitions/offerings into
  one `Catalog`. Customer models, entitlements, prices, limits, deployments,
  aliases, and endpoints are not a second product.
- Public generation accepts only `global_public` and `regional_public`
  observations and fails before writing if any `credential_scoped` observation
  is present. It never filters a mixed catalog record by record.
- `single_endpoint` is the documented topology default; sweep, paginated, or
  grouped acquisition topologies remain explicit.
- Field mappings, feature rules, author mappings, offering defaults, response
  collection, pagination, and endpoint overrides belong to the source that
  consumes them, not to a provider-wide endpoint.
- Invocation routes reference the same named credentials but own independent
  auth and endpoint declarations; removing catalog auth must not remove
  inference auth.
- A scope binding records its dimension, value source (`env`, `static`,
  `cloud_profile`, `api_result`, or `governed_sweep`), role (`required_input`,
  `iteration`, or `output_metadata`), precedence, and secret-safe description.
- Multiple logical sources may share a request-scoped acquisition group,
  adapter, SDK session, dependency check, or fetched result. Sharing must not
  cause a second anonymous/authenticated request or change the resolved
  observation scope.

### Field Sensitivity and Persistence Matrix

This matrix is normative. Publication eligibility belongs to observation
provenance, never to source kind or a parallel catalog product.

| Field class | Runtime/config owner | Canonical contextual catalog | In-memory query/copy and durable stores | Logs/errors | Public generation/distribution |
| --- | --- | --- | --- | --- | --- |
| API keys, tokens, passwords, compound-method values | Request-scoped resolver only; configuration stores IDs, env names, and placement metadata | Forbidden | Forbidden everywhere | Method/input/env names only; values redacted | Forbidden before encode |
| Official SDK credential/session/config objects | Registered cloud-chain adapter and request lifetime | Forbidden | Forbidden everywhere | Provider and `cloud_chain` only | Forbidden before encode |
| Routing-only account/project/subscription/workspace identifiers supplied as inputs | Typed source bindings resolved for one acquisition | Forbidden unless independently returned as a caller-useful provider fact | Runtime-only by default; no durable store | Dimension and binding kind only | Forbidden before encode |
| Provider-returned custom model IDs and aliases | Canonical definition/offering identity under credential-scoped observation provenance | Required when caller-useful | Caller-owned deep copies in the current context; durable stores reject | Safe identity only under contextual logging policy | Entire generation rejected; no record filtering |
| Provider-returned deployments and inference endpoints | Canonical offering/deployment/endpoint under credential-scoped observation provenance | Allowed when required for caller use | Caller-owned deep copies in the current context; durable stores reject | Redacted or omitted unless explicitly safe | Entire generation rejected; no record filtering |
| Provider-returned contextual prices, limits, lifecycle, entitlements, and regions | Canonical offering facts under credential-scoped observation provenance | Required when caller-useful | Caller-owned deep copies in the current context; durable stores reject | Aggregate/safe metadata only | Entire generation rejected; no record filtering |
| Global/regional public definitions, offerings, prices, limits, and endpoints | Canonical catalog plus public observation provenance | Required | Caller-owned copies; exact-current durable public stores allowed | Safe metadata | Allowed only after exact-current validation |
| models.dev environment names and SDK advisories | Non-executable advisory metadata | Secret-safe metadata only | Caller-owned copies; public store allowed | Names/descriptions only | Allowed; cannot satisfy auth or bindings |

Credential-scoped observations exist only as ordinary in-memory `Catalog`
observations returned to the caller. Designing an authorized durable private
store would be a separate postlaunch product decision; this plan deliberately
does not add one. Every current store and public serializer therefore rejects a
credential-scoped catalog before writing bytes.

## Before-to-After Replacement Ledger

Removal is permitted only after the replacement proves equal or stronger
behavior. These rows are design obligations, not cleanup assumptions.

| Remove | Replace with | Behavior that must be preserved or improved | Proof |
| --- | --- | --- | --- |
| Provider-wide `api_key` plus duplicate `env_vars` entry | One typed named credential with `env` and optional description | Runtime resolution, auth status, errors, CLI help, and docs retain the correct environment name | Exhaustive env migration matrix and provider-level integration tests |
| Repeated `pattern: .*`, empty query parameter, and `Authorization: Bearer` | Built-in `api_key` defaults | Common YAML shrinks without changing wire auth | Twenty-three default-provider fixtures plus Anthropic/Google overrides |
| `auth_required` boolean | Explicit per-source/per-route `none`, `optional`, required method, or ordered required alternatives | Credential-prohibited, credential-preferred optional, required, alternative, and compound-method auth become distinguishable | Resolver truth table, Databricks no-auth fixture, and old-boolean/expression negative fixtures |
| One provider catalog endpoint | Ordered logical sources with optional shared acquisition group and typed observation scope | Public and credential-scoped observations normalize into one catalog without duplicate calls | Credential-present/absent request counts, ordering, publication, and isolation tests |
| Flat provider `env_vars` | Typed credential, scope binding, endpoint override, SDK chain, operational option, or non-executable advisory metadata | No name/description is lost and no advisory name becomes executable | All 39 current entries mapped; DeepInfra conflict resolved; models.dev authority tests |
| Provider runtime fields and methods that load/store credential values | Request-scoped resolver results passed through a narrow source/route seam | Catalog configuration copies only secret-safe metadata; no serialized, retained, or multiplied runtime secrets; cancellation and typed errors improve | Reflection, copy, logging, race, and transport tests plus exact runtime-field/method searches |
| Ambient provider-wide key attachment to every provider endpoint | Source-local auth policy resolved once before transport | `none` never sends a provider token to a public documentation origin; `optional` sends only the conventional source key or registered cloud chain selected for that source | Databricks header-absence fixture, Cerebras/NVIDIA optional fixtures, hostile-origin proof, and logical-execution/bounded-request tests |
| Provider-specific Google auth checker and native cloud constructors in pipeline | Central credential resolvers plus SDK-native source adapters selected from normalized config | Official SDK behavior retained without provider branching in orchestration | Google/AWS/Azure/OCI fake-chain tests and pipeline absence search |
| Vendor-specific cloud-chain names in YAML or the unused generic `CloudCredentialChain[T]` callback sequencer | Built-in `cloud_chain` plus one provider-registered official-SDK adapter | YAML remains provider-neutral; AWS/Azure/GCP/OCI chain order, refresh, caching, and identity sources remain SDK-owned | Registry exhaustiveness, production-call-path proof, four SDK-fake tests, unsupported-provider negative fixture, and exact old-helper search |
| Hard-coded environment reads inside provider adapters | Resolved typed credentials/bindings/options | Current account, region, endpoint, and option behavior remains configurable | Exact literal search plus adapter integration fixtures; reviewed SDK-owned exceptions |
| Broad fixture policy duplicating source semantics | Derivation from normalized source contract plus narrow test-only exceptions | Evidence strength follows real acquisition semantics | Exhaustive derivation test and verified observation import |
| `CustomerInventory` and overlay-style customer result paths | Canonical `Catalog` plus observation scope/provenance | Early access, private definitions/offerings, entitlements, pricing, limits, and deployments use one identity/reconciliation/copy implementation while every durable/public write fails closed | Exact removal searches, ADR conformance, encode/copy/publication/isolation and store-rejection tests |
| Provider `Result.CustomerInventory`, `PublicCatalog`, boolean `Fetch(..., includeCustomerInventory)`, and test-only scoped fetch entry points | Configured logical sources returning `Observation{Catalog, Scope}` | Useful bounded discovery remains reachable from production without a parallel result product or dead test-only API | Production call-graph proof, request counts, provider fixtures, exact symbol searches, and focused races |
| Production-compiled but test-only-referenced `EnterpriseAttributeMatrix` authority module | The production-used `CanonicalPolicies` authority interface | One authoritative policy implementation is exercised by reconciliation rather than duplicated in a shadow module | Call-graph and exact removal searches plus policy and reconciler behavior tests |
| `internal/providerdata` `PricingCatalog`/`RegionCatalog` and provider-local `pricing.yaml`/`regions.yaml` mini-catalogs | Normalized provider source configuration for sweep/source facts plus canonical definition/offering YAML for reviewed commercial facts | Bedrock region sweeps and official price-source endpoints remain configurable; Snowflake/OCI prices and modes retain canonical identity and evidence without a second catalog schema | Exhaustive authority-location test, exact old-type/file searches, canonical generation diff, and Snowflake/OCI/Bedrock focused races |
| Ambiguous blanket treatment of customer identifiers and private endpoints | Field-level sensitivity and persistence matrix | Credentials, tokens, SDK objects, and routing-only tenant identifiers remain runtime-only; caller-useful provider-returned deployments, aliases, and endpoints may exist only in the caller's credential-scoped in-memory catalog, while every durable/public path fails closed | Marshal/copy/store-rejection/log/publication matrix, redaction fixtures, context-isolation races, and ADR conformance |

## Worktree and PR Cleanup Disposition Ledger

This ledger classifies the current branch and uncommitted overlay. `RETAIN` does
not freeze the current implementation; it means preserve the architectural
direction while completing the named revision. `REMOVE` still requires the
replacement and behavior proof in the owning task.

| Current surface | Disposition | Target and rationale | Owning proof |
| --- | --- | --- | --- |
| Shared protocol modules under `internal/connectors/{openai,anthropic,google}` | RETAIN AND DEEPEN | These are the correct connector modules: protocol parsing, normalization, and response-schema tests stay local and give every exact-compatible provider leverage through one interface | PAC2.4,PAC3.2,PAC6.1; connector positive/negative fixtures and focused races |
| Provider composition under `internal/providers/registry` | RETAIN AND NARROW | Registry remains the provider/source-to-connector or adapter seam; remove API-key-specific capability methods, the empty `applicationClient`, global test hooks, and false-success adapters | PAC2.4,PAC2.7,PAC3.2,PAC7.1; production call graph and exact symbol searches |
| Exact OpenAI-compatible provider `provider_test.go` files and provider-local response fixtures | REMOVE REDUNDANT PROTOCOL RETESTS | Shared connector owns protocol conformance; exact providers get table-driven configuration/composition assertions from canonical YAML and no duplicate response fixture | PAC6.1,PAC6.3; exact-provider inventory and absence searches plus shared connector tests |
| Provider delta adapters and delta fixtures | RETAIN ONLY WHEN IRREDUCIBLE | A provider adapter earns the seam only for wire/schema/normalization behavior not expressible through normalized YAML; each retained adapter documents the delta and tests only that delta | PAC3.2,PAC3.7,PAC6.3; deletion-test matrix and adapter-focused fixtures |
| `internal/providers/fixtures/policy.yaml` and policy parsing in `contract_test.go` | REMOVE | Source/auth/scope configuration derives fixture class; a second YAML authority harms locality and can drift | PAC6.1,PAC6.2; exhaustive derivation test and exact file/symbol searches |
| `internal/providers/fixtures/responses` governed observations | RETAIN AS RAW EVIDENCE ONLY | Governed source observations may support refresh/replay and provenance, but must not duplicate canonical provider facts or become another configuration authority | PAC6.1,PAC6.3,PAC6.7; metadata/replay tests and generated-catalog diff proof |
| `cmd/starmap-provider-testdata-refresh` and `cmd/starmap-provider-fixture-import` | CONSOLIDATE | Replace two standalone development binaries and their `httptest` decode bridges with one documented provider-fixture developer-tool interface backed by request-scoped acquisition/replay code | PAC6.7; one entry point, no provider mutation, no duplicate main packages, tool tests |
| Custom clients for Mistral, SambaNova, NVIDIA, Novita, xAI, Cohere, Together, Cloudflare, Databricks, Hugging Face, Snowflake, and watsonx | DELETION-TEST EACH | Prefer normalized YAML plus a shared connector. Retain custom code only for a concrete protocol, SDK, pagination, topology, or normalization delta; split public and credential-scoped inventories into distinct logical sources when needed | PAC3.2,PAC3.7,PAC4.1-PAC4.4; per-provider disposition matrix, production path, and delta fixtures |
| Provider-wide runtime credentials, `CustomerInventory`, generic `CloudCredentialChain[T]`, raw arbitrary-endpoint fetch, and stats-only parser | REMOVE OR REPLACE | Request-scoped auth, one canonical catalog, provider-registered SDK adapters, and one acquisition result remove shallow duplicate interfaces and unsafe authority | PAC2.1-PAC2.7,PAC5.1-PAC5.7,PAC7.1; security, behavior, isolation, and exact-search evidence |
| `source_projection` operational import plus legacy/deprecated/alias surfaces | REVIEW AND RENAME OR REMOVE | Retain only a current schema-v2 authoring/import facility used by production. It must not be reachable from bootstrap, generation, remote, distribution, or payload decoding, and legacy compatibility terminology must not conceal current behavior | PAC7.2,PAC7.5,PAC7.7; call graph, negative old-shape fixtures, and file-level rationale |
| Generated bootstrap, fixtures, provider/model YAML, hashes, and generated docs | REGENERATE | They must represent only the final exact-current schema-v2 contract; current generated churn is not authoritative until all cleanup and verification gates pass | PAC7.3,PAC7.4,PAC8.2; deterministic regeneration and clean diff checks |
| Accidental prose in `pkg/sources/cloud_credentials.go` | REMOVE IMMEDIATELY | It is non-code contamination and blocks every meaningful Go gate; removal does not imply the obsolete helper itself is retained | PAC0.3,PAC0.7,PAC7.7; compile gate plus exact prose search |
| `internal/providerdata` and `internal/embedded/catalog/providers/*/{pricing,regions}.yaml` | REMOVE OR COLLAPSE INTO CANONICAL OWNERS | The current shallow module owns parallel fact schemas. Source sweep/endpoint facts move to normalized provider configuration; reviewed prices/modes move to canonical offerings; retain only generic loading behavior that survives a deletion test and serves more than this duplicate schema | PAC4.8,PAC5.2,PAC6.5,PAC7.7; authority inventory, production call graph, exact searches, canonical generation, and focused provider races |

## Current-Reality Finding Ledger

Reviewed YAML baseline on 2026-07-13:

- 27 embedded providers;
- 20 endpoints explicitly set `auth_required: true` and 7 explicitly set it to
  `false`; omission would still decode to false because the current Go field is
  a plain boolean;
- 25 provider-wide API-key declarations, all repeating `pattern: .*`;
- 23 of those 25 repeating `Authorization: Bearer`; Anthropic and Google AI
  Studio are the two transport exceptions;
- 39 generic environment-variable entries, zero marked required, and only 3
  descriptions (all Google Vertex);
- 25 API-key environment names represented a second time through `env_vars`,
  including the conflicting DeepInfra names described in PAF-015.

The seven false booleans do not form one authentication class. Cerebras can
list public models without its inference key, Google Vertex uses a cloud
credential chain rather than an API key, and Databricks' public source reads a
documentation origin that must not receive its workspace token. This is why the
replacement makes `none`, `optional`, and required methods distinct rather than
retaining `auth_required` as a provider-API-key boolean.

The review also traced `env_vars` beyond YAML: models.dev parses and merges it,
field authority reconciles it, and the differ reports it. That incoming metadata
is advisory evidence, not trusted executable credential configuration. Its
replacement must deliberately preserve documentation/change-reporting value
while removing its authority to create runtime credentials or bindings.

Cloud-chain decision evidence reviewed on 2026-07-13:

- AWS documents `config.LoadDefaultConfig` as the entry point for its default
  credential chain across environment, shared configuration, and workload
  sources: [AWS SDK for Go v2 configuration](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-gosdk.html).
- Azure documents `DefaultAzureCredential` as a preconfigured chain spanning
  environment, workload identity, managed identity, and local developer tools:
  [Azure SDK for Go credential chains](https://learn.microsoft.com/en-us/azure/developer/go/sdk/authentication/credential-chains).
- Google documents Application Default Credentials as a platform-aware search
  strategy rather than a Starmap-defined chain:
  [Google Application Default Credentials](https://docs.cloud.google.com/docs/authentication/application-default-credentials).
- OCI exposes `DefaultConfigProvider` for its default configuration sources:
  [OCI Go SDK common package](https://docs.oracle.com/en-us/iaas/tools/go/latest/common/index.html).
- Cloudflare's current Workers AI REST contract requires an API token and
  account ID, so it is not a cloud-chain adapter:
  [Cloudflare Workers AI REST API](https://developers.cloudflare.com/workers-ai/get-started/rest-api/).

These are four real production implementations behind one small Starmap
interface, so the seam is justified. The existing generic
`CloudCredentialChain[T]` module is nevertheless shallow: it has no production
adapter or caller and merely sequences arbitrary callbacks. The target replaces
that hypothetical implementation with provider-registered adapters that
delegate chain precedence and lifecycle to the official SDKs.

| ID | Finding | Severity | Owning tasks | Verifiable closure | Status |
| --- | --- | --- | --- | --- | --- |
| PAF-001 | `Provider` has one provider-wide `APIKey` | blocker | PAC1.1,PAC4.1,PAC7.1 | Multiple credential definitions round-trip; singular API-key surface is removed; migrated providers pass | DONE |
| PAF-002 | `auth_required` means API-key-required rather than generic authentication; omission currently defaults to false | blocker | PAC1.2,PAC2.3,PAC7.1 | Explicit `none`, `optional`, required method, and ordered required-alternative cases pass; omitted auth, old boolean, and old expression objects fail exact-current decoding | DONE |
| PAF-003 | Generic `env_vars` cannot distinguish credentials, scopes, endpoint overrides, and operational options; every current entry is optional | reliability | PAC0.4,PAC1.3,PAC4.1,PAC4.5 | Every current env reference is classified, documented, round-tripped, and validated at exactly one typed destination | DONE |
| PAF-004 | Google ADC, AWS, Azure, and OCI chains are independently hard-coded | maintainability | PAC1.1,PAC2.1,PAC2.2,PAC4.4 | YAML declares only provider-inferred `cloud_chain`; one registry-backed resolver contract covers all supported official-SDK adapters with no special-case auth checker branch or Starmap-owned chain precedence | DONE |
| PAF-005 | A provider has one catalog endpoint, so public, entitlement, pricing, and deployment observations cannot be modeled together | blocker | PAC1.2,PAC3.1,PAC4.1 | Multiple ordered sources execute independently with typed observation scopes into one catalog | DONE |
| PAF-006 | Public/authenticated and public/credential-scoped distinctions are encoded in client prose and fixture exceptions | blocker | PAC1.3,PAC3.3,PAC6.1 | Observation eligibility is executable and fixture policy is derived from it | DONE |
| PAF-007 | `CustomerInventory` fragments canonical definitions/offerings and would require separate identity, validation, reconciliation, copying, storage, and query semantics | blocker | PAC5.1-PAC5.6 | The parallel type and provider result paths are removed; scoped facts round-trip in `Catalog` and fail public publication by observation scope | DONE |
| PAF-008 | Provider client interfaces expose `IsAPIKeyRequired` and `HasAPIKey` | maintainability | PAC2.3,PAC2.4,PAC7.1 | Connectors receive resolved source auth and no production API-key-specific capability interface remains | DONE |
| PAF-009 | Native cloud sources are constructed directly in the pipeline instead of from provider configuration | maintainability | PAC3.2,PAC4.4 | Source registry consumes typed provider source config; hard-coded constructors are removed from the pipeline | DONE |
| PAF-010 | Provider schema validation cannot reject unknown credential references, ambiguous auth, invalid method lists, or invalid observation-scope policies | blocker | PAC1.3-PAC1.5 | Complete negative validation matrix covers omitted auth, `none` in lists, duplicates, empty lists, unknown refs, unsupported chains, old expression objects, and invalid scope policies before resolver or transport creation | DONE |
| PAF-011 | `internal/providers/fixtures/policy.yaml` and its parsing contract duplicate acquisition/scope reasons from provider configuration | cleanup | PAC6.1-PAC6.3 | Broad policy file and parser are removed; derivation covers every provider/source; any retained test-only exception has non-duplicating evidence | DONE |
| PAF-012 | Current docs teach singular API-key/endpoint behavior, and `PROVIDER_IMPLEMENTATION_CONTRACT.md` still claims an incompatible fixture-policy/compatibility contract is normative | maintainability | PAC6.4-PAC6.6 | AGENTS, architecture, provider contract, provider-specific docs, examples, CLI help, generated docs, and structural doc tests describe the new contract only; superseded normative promises are rewritten or explicitly historical | DONE |
| PAF-013 | Old prelaunch schema-v2 provider shapes could remain readable after migration | blocker | PAC7.1-PAC7.4 | Negative old-shape payload/config/manifests fail exact-current validation before publication | DONE |
| PAF-014 | Credential-scoped facts could accidentally enter a public generation | security | PAC3.3,PAC5.3,PAC7.3 | Public generation, evidence, distribution, remote, and bootstrap paths reject any catalog/run containing credential-scoped observations before writing | DONE |
| PAF-015 | DeepInfra declares `DEEPINFRA_TOKEN` as its API key but separately advertises `DEEPINFRA_API_KEY` in `env_vars`; first-party documentation currently uses both names | reliability | PAC0.4,PAC1.1,PAC2.2,PAC4.5 | One `api_key` credential owns ordered environment inputs `[DEEPINFRA_TOKEN, DEEPINFRA_API_KEY]`, documents the first as canonical, accepts one/equal values, rejects differing simultaneously set values, and keeps runtime/help/fixture behavior consistent | DONE |
| PAF-016 | The plan and ADR introduced parallel `CustomerInventory`/overlay semantics even though one contextual `Catalog` can represent all acquired facts | architecture | PAC0.1,PAC3.3,PAC5.1-PAC5.6,PAC6.5 | Amended ADR and plan require one catalog; production `CustomerInventory`, `CustomerScope`, `CustomerDeployment`, `ObservationScopeCustomer`, `SourceKindCustomer`, `CatalogOverlay`, `EffectiveCatalog`, delta, and parallel store/query paths are absent with positive scoped-catalog and fail-closed publication proof | DONE |
| PAF-017 | Removing provider-wide `APIKey` without an invocation-route auth owner would break shared inference transport | blocker | PAC0.4,PAC1.2,PAC2.4,PAC7.1 | Every invocation route references normalized auth; catalog and invocation routes can differ; old API-key surface is deleted only after parity tests | DONE |
| PAF-018 | Current scope inputs come from env, cloud profiles, API results, static config, and governed regional sweeps, which a flat scope list cannot represent; Google connector/auth paths also use undeclared `GOOGLE_CLOUD_*` aliases with different project/location precedence | blocker | PAC0.4,PAC1.3,PAC2.3,PAC4.4 | Typed bindings encode dimension, ordered names, value source, input/iteration/output role, one provider-wide precedence, and safe diagnostics; all Google names have one owner and connector/auth status resolve identically; sweep scopes do not falsely fail preflight | DONE |
| PAF-019 | models.dev imports advisory env metadata into the same provider fields used by executable local configuration | security | PAC0.4,PAC4.5,PAC6.5 | Advisory metadata has a non-executable owner and cannot create credentials/bindings; reconciliation, diffs, and generated docs have explicit authority tests | DONE |
| PAF-020 | Multiple logical sources or anonymous/authenticated comparison could cause redundant acquisition executions | reliability | PAC2.3,PAC3.1,PAC3.3,PAC4.4 | Required methods choose one alternative, `optional` falls back only after credential absence, `none` stays unauthenticated, and counters prove one logical execution with no delta/overlay acquisition while bounded pages/retries remain allowed and tested | DONE |
| PAF-021 | Bedrock, Azure, and OCI expose parallel `CustomerInventory`/`PublicCatalog` results and boolean fetch modes that production calls with customer discovery disabled; Databricks, NVIDIA, and watsonx scoped fetch methods are test-only | blocker | PAC3.2,PAC3.3,PAC4.4,PAC5.4,PAC7.1 | Useful bounded discovery is wired through configured production sources returning scoped canonical observations or removed if redundant; parallel result fields, boolean fetch switches, and dead exported entry points are absent; request-count and provider race tests pass | DONE |
| PAF-022 | Production-compiled but test-only-referenced `EnterpriseAttributeMatrix` duplicates the production-used canonical authority policy surface | maintainability | PAC5.5,PAC5.7,PAC7.1 | Shadow authority files and symbols are removed; any required scoped rules live in production `CanonicalPolicies`; call-graph, authority, and reconciliation behavior tests prove the used implementation | DONE |
| PAF-023 | `CloudCredentialChain[T]` is a test-only callback sequencer with no production adapter, while real cloud providers call official SDK chains directly | architecture | PAC1.1,PAC2.1,PAC2.2,PAC4.4,PAC7.1 | The shallow helper is removed or replaced by a production-used provider-registry seam; Bedrock/Azure/Vertex/OCI select exactly one SDK-backed adapter from `cloud_chain`; unsupported providers fail before transport; Cloudflare remains token/account based; tests prove delegation without reimplementing SDK precedence | DONE |
| PAF-024 | Provider-wide API-key attachment can send a workspace or invocation token to an unrelated origin; Databricks combines `DATABRICKS_TOKEN` with a documentation-origin catalog endpoint, and raw fetch accepts an arbitrary caller URL before attaching provider credentials | security | PAC0.4,PAC1.2,PAC2.3,PAC2.7,PAC4.2,PAC7.1 | Databricks public acquisition declares `auth: none`; raw/debug/fixture acquisition resolves a configured source ID rather than a caller URL; transport enforces the source endpoint/origin policy and attaches only source-selected auth; header-absence, hostile-origin, redirect, and bounded execution tests pass | DONE |
| PAF-025 | `Provider.LoadAPIKey`/`LoadEnvVars` mutate catalog configuration with runtime values and `DeepCopyProvider` copies those values while transport resolves the key again | security | PAC1.4,PAC1.6,PAC2.1,PAC2.3,PAC7.1 | Provider configuration contains and copies only secret-safe metadata; runtime values exist only in request-scoped resolver results; old runtime fields/methods are absent; reflection, copy, logging, transport, and race tests pass | DONE |
| PAF-026 | `pkg/sources/cloud_credentials.go:10` contains literal user-request prose before a declaration and the worktree does not compile | blocker | PAC0.3,PAC0.7,PAC7.7 | Accidental prose is removed immediately after goal activation, exact search is empty, focused compile/race gates execute, and the obsolete helper still receives its independent remove-or-replace disposition | DONE |
| PAF-027 | CLI `--stats` bypasses the selected connector/adapter, raw-fetches separately, and parses only a reduced OpenAI shape through `parseModelsFromRaw`; custom providers can fail or return different canonical facts | blocker | PAC2.3,PAC2.7,PAC3.1,PAC6.7 | One acquisition result carries normalized models plus safe transport stats/evidence; normal and stats modes use identical connector/adapter semantics; alternate parser and second fetch are absent; exact/custom provider parity tests pass | DONE |
| PAF-028 | Mutable package-global provider hooks, unused credential-bypass options, and the empty `applicationClient` are shallow or false-success interfaces beside per-instance injection and typed non-acquirable source metadata | maintainability | PAC2.4,PAC2.7,PAC3.2,PAC7.1 | Global registration/restore hooks, unused bypasses, and empty-success adapter are absent; tests use per-instance dependencies; application-only providers are explicitly non-acquirable and cannot report successful empty inventory | DONE |
| PAF-029 | Exact OpenAI-compatible providers own provider-local response fixtures and HTTP/schema tests that retest the shared connector without a provider wire delta | cleanup | PAC6.1,PAC6.3,PAC7.7 | Shared connector owns protocol fixtures; exact providers have table-driven YAML composition tests only; provider-local response fixtures/tests remain solely for documented provider deltas | DONE |
| PAF-030 | Several custom provider clients/adapters were added before proving YAML plus a shared connector is insufficient; Mistral and SambaNova appear declarative, NVIDIA combines public and credential-scoped inventories, and Together fetches serverless plus dedicated inventories inside one `ListModels` call | architecture | PAC3.2,PAC3.7,PAC4.1-PAC4.4,PAC6.3 | Per-provider deletion-test matrix records protocol/topology delta, target connector/config, production call path, fixture owner, and retain/remove rationale; redundant clients are deleted, legitimate adapters are minimal, and NVIDIA public/NIM plus Together serverless/dedicated inventories are explicit distinct logical sources with bounded request-count tests | DONE |
| PAF-031 | Two standalone provider-fixture development binaries duplicate provider mutation and `httptest` decode bridges | maintainability | PAC6.7,PAC7.7 | One documented developer-tool interface refreshes/imports/replays fixtures through request-scoped acquisition without mutating provider secrets/config or shipping duplicate main packages; tool tests and exact searches pass | DONE |
| PAF-032 | Current-schema operational import code and prelaunch compatibility surfaces are not yet explicitly separated: `source_projection` uses `legacy`/`migrate` language, `Precision` calls itself legacy, `FORMAT` is a backwards-compatible alias, and provider YAML/docs promise older-integration compatibility | blocker | PAC7.2,PAC7.5,PAC7.7 | Call graphs prove each retained import is a current schema-v2 authoring facility unreachable from publication decoders; legacy/alias promises and fields are removed unless a current internal requirement is evidenced; every retention has file-level rationale plus positive and negative behavior tests | DONE |
| PAF-033 | `internal/providerdata` defines parallel `PricingCatalog` and `RegionCatalog` fact schemas loaded from provider-local `pricing.yaml`/`regions.yaml`; Snowflake, OCI, and Bedrock consume them outside both normalized provider source configuration and canonical definitions/offerings, while pricing endpoints and classifiers also lack an explicit YAML-versus-Go disposition | blocker | PAC3.7,PAC4.1,PAC4.4,PAC4.8,PAC5.2,PAC6.5,PAC7.7 | Every provider fact and normalization rule has one typed owner: sweep regions, official source endpoints, and data-only mapping tables are normalized source configuration; reviewed prices/modes are canonical offering facts; wire parsing, unit conversion, and irreducible classifiers remain minimal adapter behavior; provenance does not duplicate facts; old mini-catalog types/files are absent or any retained generic loader has a production call graph and deletion-test rationale; Snowflake/OCI/Bedrock races and deterministic generation pass | DONE |
| PAF-034 | PAC1.4 forbids catalog-retained customer identifiers/private endpoints while PAC5.2 and ADR 0001 require credential-scoped deployments, aliases, and endpoints to be ordinary contextual catalog facts | blocker | PAC1.4,PAC5.2-PAC5.6,PAC5.8,PAC7.3 | A field-level matrix classifies credentials/tokens/SDK objects/routing-only tenant IDs as runtime-only and classifies each caller-useful provider-returned deployment/alias/endpoint fact for contextual encode, in-memory query/copy, logging, durable-store rejection, and public rejection; plan and ADR use one non-contradictory rule; positive contextual and negative durable/public/redaction/isolation tests pass | DONE |
| PAF-035 | A durable-server restart fixture still authored a provider without the exact-current required catalog source, so the PAC1-PAC7 race gate failed after production schema-v1 compatibility was removed | reliability | PAC7.3,PAC7.6,PAC8.1 | The fixture authors an explicit application source with `global_public` scope and `none` auth; the focused durable-restart race passes; production old-shape searches remain empty | DONE |
| PAF-036 | The pinned `golangci-lint@2.5.0` binary was built with Go 1.25 and could not analyze the repository's exact Go 1.26.5 toolchain, so the lint gate had never reached source analysis | reliability | PAC8.2 | Devbox pins a linter built with Go 1.26; the lockfile is regenerated; `devbox run lint` analyzes the complete production tree and reports zero issues | DONE |
| PAF-037 | The provider differ test still expected removed schema-v1 `env_vars` and `chat_completions` change paths after production diffing moved to exact-current `credentials` and `invocation` | reliability | PAC7.3,PAC8.1,PAC8.2 | The fixture asserts only exact-current provider fields; the focused differ race passes; repository-wide race is rerun from the final tree | DONE |
| PAF-038 | Complete verification exposed 74.6% registry coverage against the required 80% gate because executable connector support and governed fixture replay selection lacked direct contract tests | reliability | PAC6.4,PAC6.6,PAC8.3 | Tests exhaustively bind `Supports` to executable endpoint types and prove positive connector-owned replay plus negative no-replay-schema rejection; registry race passes at or above 80%; complete verification is rerun | DONE |
| PAF-039 | The second complete-verification run exposed 79.9% `pkg/errors` coverage against the required 80% gate after all earlier package thresholds passed | reliability | PAC2.5,PAC8.3 | Typed merge errors are directly tested with and without conflict identities, including unwrap behavior; the package race passes at or above 80%; complete verification is rerun without lowering the threshold | DONE |
| PAF-040 | The credential-isolated verifier's table smoke reported Google Vertex `Ready`, proving that official SDK discovery read ambient ADC from the developer home even though catalog-configured environment names and dotenv were disabled | blocker | PAC2.1,PAC2.3,PAC8.3,PAC8.4 | Verification scrubs provider-inferred cloud-chain environment inputs, uses an empty temporary home for every CLI smoke, and lists providers through a non-auth-resolving output path; a zero-environment focused smoke and complete verification pass without ambient status resolution | DONE |

## Phase Ledger

| ID | Phase | Status | Verifiable phase gate |
| --- | --- | --- | --- |
| PAC0 | Control plane, baseline, and characterization | DONE | Durable plan exists; baseline is recorded; characterization tests fail for the intended missing contract before implementation |
| PAC1 | Typed configuration and exact-current schema | DONE | New credentials/sources/bindings/auth/observation-scope schema validates, deep-copies, and round-trips; old shape fails |
| PAC2 | Credential resolution and source-aware preflight | DONE | API-key and cloud-chain alternatives resolve generically; missing/invalid auth fails typed before transport |
| PAC3 | Multi-source acquisition and observation-scope enforcement | DONE | Configured sources execute independently with bounded behavior into one catalog and carry exact publication eligibility |
| PAC4 | Complete provider migration | DONE | Every embedded and native provider source is classified and migrated with no singular compatibility path |
| PAC5 | Single Catalog and publication safety | DONE | Public and credential-scoped facts use one catalog implementation; observation provenance makes public generation fail closed and contextual catalogs remain isolated |
| PAC6 | Derived fixture policy and documentation | DONE | Fixture/evidence policy is derived from source config; broad duplicate policy is gone; normative docs are current |
| PAC7 | Compatibility removal, artifacts, and schema-v2 regeneration | DONE | Old surfaces and promises are removed with positive and negative proof; bootstrap/docs/hashes are regenerated |
| PAC8 | Full verification, review, and closeout | IN_PROGRESS | Focused and repository-wide gates pass; every task/finding is terminal; external evidence is recorded without unauthorized publication |

## PAC0: Control Plane, Baseline, and Characterization

| ID | Task | Status | Verifiable success criteria |
| --- | --- | --- | --- |
| PAC0.1 | Write this control plane | DONE | File contains mission, constraints, decisions, target contract, finding/phase/task/evidence ledgers, gates, execution rules, and whole-plan `/goal` |
| PAC0.2 | Record current repository reality | DONE | Baseline records worktree/branch/HEAD/dirty count and current singular API-key, boolean auth, cloud-chain, native-source, customer-inventory, and fixture-policy seams |
| PAC0.3 | Activate the whole-plan goal and restore a compilable baseline | DONE | Active goal uses this document's `/goal` verbatim or equivalently and remains whole-plan scoped; accidental prose in `pkg/sources/cloud_credentials.go` is removed without deciding the helper's later disposition, exact prose search is empty, a focused Go compile probe runs, PAF-026 evidence is recorded, and execution advances to PAC0.4 |
| PAC0.4 | Audit all provider credential, environment, and scope behavior | DONE | Checked-in matrix covers all embedded providers plus Bedrock, Microsoft Foundry, and OCI; each path records catalog and invocation auth, every env name/description and typed destination, binding value source/role/precedence, observation-scope policy, topology/acquisition group, endpoint owner, cloud-chain adapter owner, production call path, and publication disposition; DeepInfra's conflict is resolved from first-party evidence |
| PAC0.5 | Lock and deletion-test the YAML authoring contract | DONE | Reviewed fixtures cover `none` with zero credential lookup/attachment, `optional` with key/chain/absence, required key, required cloud chain, ordered required alternatives, missing versus invalid behavior, unsupported cloud-chain provider, Cloudflare token/account auth, Databricks public docs, Cerebras split catalog/invocation auth, Anthropic override, Vertex profile fallback, Bedrock governed sweep, models.dev advisory env metadata, and shared acquisition; every proposed removal maps to positive and negative proof |
| PAC0.6 | Add characterization tests before migration | DONE | Tests capture the locked PAC0.5 cases and current invocation auth/dataflows without network calls; target behavior tests fail only for the intended missing implementation |
| PAC0.7 | Audit the full PR range and dirty overlay; lock cleanup dispositions | DONE | Exact protected base/head/PR/check evidence, committed range, dirty counts, active worktree/run inspection, architecture dataflow review, retain/revise/remove matrix, compile blocker, and passing/failing focused checks are recorded without touching release work or claiming the dirty tree green |

PAC0 gate:

```bash
go test ./pkg/catalogs ./pkg/sources ./internal/auth ./internal/providers/... ./internal/catalog/pipeline
go test ./pkg/catalogs ./pkg/sources ./internal/auth ./internal/providers/... ./internal/catalog/pipeline -race
git diff --check
```

## PAC1: Typed Configuration and Exact-Current Schema

| ID | Task | Status | Verifiable success criteria |
| --- | --- | --- | --- |
| PAC1.1 | Add typed credential definitions and standard authoring forms | DONE | Named credential map supports API keys and future explicit mechanisms; built-ins are standalone `none`, deterministic `optional`, conventional `api_key`, and provider-inferred `cloud_chain`; `api_key` implies its kind and bearer-header defaults and accepts a normal scalar or evidenced ordered environment-name list; `cloud_chain` needs no credential block and validates against exactly one provider-registered adapter; JSON/YAML, normalization, registry-contract, and copy tests pass; empty names/lists, duplicate aliases, unsupported kinds, vendor chain names, and unsupported-provider chains fail typed before transport |
| PAC1.2 | Replace one endpoint with ordered catalog sources and invocation routes | DONE | Provider owns uniquely named sources and routes with independent endpoint/auth/observation-scope declarations; source-only mappings/defaults relocate without loss; empty/duplicate/unsafe entries fail validation |
| PAC1.3 | Add typed scope bindings and the exact auth policy | DONE | Scalar `none`, `optional`, or required method and ordered required method lists normalize and round-trip; auth omission, `none` in a list, empty/duplicate lists, unknown refs, old expression objects, and bad bindings fail before transport; compound methods own multiple typed inputs; governed sweeps need no false input |
| PAC1.4 | Define secret-safe serialization and publication boundary | DONE | Provider configuration contains only credential metadata. A reviewed field-level matrix distinguishes runtime-only credentials, tokens, SDK objects, and routing-only tenant identifiers from caller-useful provider-returned contextual facts; reflection, marshal, logging, copy, and store-rejection tests prove runtime-only values have no catalog representation while credential-scoped deployments/aliases/endpoints remain caller-owned in-memory facts and every durable/public serializer rejects them |
| PAC1.5 | Restore exact-current schema-v2 validation | DONE | Provider YAML, bootstrap manifests, payload decode, generation load, remote consumption, and distribution negotiation reject unknown fields, duplicate keys, mixed old/new shapes, invalid auth policies/lists, and all non-exact shapes while retaining version `2` |
| PAC1.6 | Prove ownership and deep-copy behavior | DONE | Mutation of credential/source/auth/scope nested slices, maps, and pointers in returned values cannot mutate catalog/config internals under race tests; provider copies contain no loaded credential or environment values to alias or multiply |

PAC1 gate:

```bash
go test ./pkg/catalogs ./pkg/catalogstore ./pkg/catalogdistribution -race
go test ./internal/bootstrap ./internal/bootstrapmanifest ./pkg/catalogremote -race
go vet ./pkg/catalogs/... ./pkg/catalogstore/... ./pkg/catalogdistribution/...
git diff --check
```

## PAC2: Credential Resolution and Source-Aware Preflight

| ID | Task | Status | Verifiable success criteria |
| --- | --- | --- | --- |
| PAC2.1 | Introduce one credential resolver contract | DONE | Resolver accepts provider/source identity plus normalized auth policy without mutating catalog configuration; `none` bypasses resolution, `optional` tries conventional key then registered chain then unauthenticated, and required lists return the first available method; secret-safe diagnostics, cancellation, missing/duplicate adapter, invalid-present, and no-match behavior use typed results/errors |
| PAC2.2 | Implement API-key and cloud-chain resolvers | DONE | Default bearer plus header/query/basic/direct overrides and production-used Google/AWS/Azure/OCI SDK-native adapters pass focused tests; scalar and evidenced ordered environment inputs resolve deterministically, equal aliases agree, and differing simultaneously set aliases fail typed; Cloudflare remains API-token/account based; official SDKs own chain order, refresh, caching, and workload identity; unknown placements/schemes and the old generic callback helper are absent; runtime values and SDK-native objects are never persisted |
| PAC2.3 | Replace API-key-only provider preflight | DONE | Every acquisition mode uses the same source resolver; `none` performs no credential lookup, `optional` uses available standard credentials and falls back only after absence, a present invalid credential fails without fallback, required absence fails typed before transport, each logical source executes once with only declared bounded page/retry/protocol requests, iteration/output bindings resolve at the correct stage, and independent sources do not block usable sources; invocation routes reuse normalized auth metadata without adding an unused execution path |
| PAC2.4 | Simplify connector/client contracts | DONE | `IsAPIKeyRequired`/`HasAPIKey` are removed from production connector interfaces; connectors receive provider/source configuration plus resolved runtime auth through one typed seam |
| PAC2.5 | Replace auth checker special cases | DONE | CLI/library auth status reports each source as ready, unavailable, invalid, or unauthenticated and identifies accepted methods without provider-specific Google branching or secret output |
| PAC2.6 | Harden error and logging behavior | DONE | Typed errors include safe provider/source/credential IDs, env/header/query names, and binding kinds while redacting values, customer identifiers, tokens, and unsafe SDK diagnostics |
| PAC2.7 | Remove alternate, ambient, and false-success acquisition seams | DONE | Raw/debug/fixture/stats paths accept a configured source identity rather than an arbitrary URL, enforce endpoint/origin/redirect policy, use the same connector/adapter result as normal acquisition, and never return a closed response body; `parseModelsFromRaw`, global registration hooks, unused credential bypasses, and empty `applicationClient` success are absent; hostile-origin, stats-parity, and application-only negative tests pass |

PAC2 gate:

```bash
go test ./pkg/sources ./internal/auth ./internal/transport ./internal/providers/registry -race
go test ./internal/connectors/... ./internal/providers/... -race
go vet ./pkg/sources/... ./internal/auth/... ./internal/transport/... ./internal/providers/...
git diff --check
```

## PAC3: Multi-Source Acquisition and Observation-Scope Enforcement

| ID | Task | Status | Verifiable success criteria |
| --- | --- | --- | --- |
| PAC3.1 | Execute ordered logical sources and acquisition groups | DONE | Sources support bounded concurrency/pagination/retry, independent completeness, deterministic merge, and per-source observations while safely sharing configured adapters/sessions/results; counters prove one logical execution per source/auth selection and enforce declared bounds for pagination, retries, and multi-endpoint protocol requests |
| PAC3.2 | Make source registry configuration-driven | DONE | Registry selects reusable connectors or provider-native adapters from source type and exposes provider auth capabilities without duplicating cloud SDK construction; pipeline no longer directly constructs Bedrock/Azure/OCI sources; every retained scoped discovery adapter has a production call path |
| PAC3.3 | Normalize every result into one catalog with exact observation scope | DONE | Global-public, regional-public, and credential-scoped results all use canonical `Catalog`; the resolved observation scope matches the auth mode and declared policy; public authenticated sources remain publishable; credential-scoped observations fail public generation before any write; no delta or overlay is constructed |
| PAC3.4 | Define partial and fallback semantics | DONE | Missing credentials for one source produce classified degradation without false empty success or deletion; complete authoritative absence remains source/observation scoped |
| PAC3.5 | Preserve observation provenance | DONE | Observation/source links record provider source ID, safe auth method ID, scope kind, topology, completeness, coverage, revision, and pricing age without customer values |
| PAC3.6 | Prove concurrent isolation | DONE | Focused race tests run simultaneous public and credential-scoped acquisitions and prove no shared mutable configuration, credentials, contextual records, observation metadata, or result aliasing |
| PAC3.7 | Deletion-test every custom provider implementation | DONE | Checked-in matrix covers each custom client/adapter and records the exact protocol, SDK, pagination, topology, or normalization delta; Mistral and SambaNova are removed if normalized YAML/shared OpenAI behavior is sufficient; NVIDIA public inventory and credential-scoped NIM plus Together serverless and dedicated inventories are separate logical sources; every retained adapter is smaller than the behavior it hides and has a production call path plus delta-only tests and bounded request counts |

PAC3 gate:

```bash
go test ./pkg/sources ./internal/sources/providers ./internal/catalog/pipeline -race
go test ./pkg/catalogs ./pkg/reconciler ./pkg/sourceevidence -race
go test ./internal/providers/bedrock ./internal/providers/azurefoundry ./internal/providers/oci -race
git diff --check
```

## PAC4: Complete Provider Migration

| ID | Task | Status | Verifiable success criteria |
| --- | --- | --- | --- |
| PAC4.1 | Migrate every embedded provider configuration | DONE | Exhaustive test proves every provider has explicit source auth/observation scope, authenticated providers use normalized credentials, and none use singular `api_key`, `env_vars`, `catalog.endpoint`, `auth_required`, or old output classes; truly unauthenticated providers need no credential block |
| PAC4.2 | Migrate no-auth and optionally authenticated sources | DONE | Databricks public docs and Cursor/application-only use `none`; Cerebras, DeepInfra, Hugging Face, NVIDIA, and any similar credential-preferred sources use `optional` only when that source accepts its standard credentials; fixtures prove no ambient token attachment, credential-present, key-missing/chain-present, all-missing, invalid-present, request-count, and publication behavior |
| PAC4.3 | Migrate required API-key/session sources | DONE | OpenAI-family, Anthropic, Google AI Studio, Cloudflare, Snowflake, watsonx, Together, Cohere, and other required-auth providers preflight from YAML with correct scopes and transport behavior |
| PAC4.4 | Migrate cloud-chain regional/account sources | DONE | Bedrock, Microsoft Foundry/Azure OpenAI, Google Vertex, and OCI declare only `auth: cloud_chain` in provider source configuration and infer exactly one registry-backed official-SDK adapter with region/realm/account/project bindings; Cloudflare uses API token plus account binding; no provider YAML repeats AWS/Azure/GCP/OCI chain identity |
| PAC4.5 | Classify and preserve every environment-variable contract | DONE | All 39 current generic entries, endpoint override names, API-key names, and 3 descriptions map exactly once to credential input/alias, binding, endpoint override, SDK chain, typed option, or non-executable models.dev advisory metadata; DeepInfra's two evidenced names share one ordered credential owner with typed conflict behavior; runtime/status/errors/docs agree; no hard-coded provider env literals remain outside reviewed SDK ownership |
| PAC4.6 | Prove API-key-or-cloud-chain alternatives | DONE | Deterministic fixtures exercise required `[api_key, cloud_chain]` selection and `optional` key-first/chain-second/unauthenticated behavior, including invalid-present failure, both unavailable, and declared-order selection without network calls |
| PAC4.7 | Validate live opt-in sources where credentials exist | DONE | Secret-safe live checks record only provider/source/status/count/checksum; unavailable or unauthorized credentials are recorded as skips/failures, never deterministic success |
| PAC4.8 | Consolidate provider fact and configuration authority | DONE | Inventory maps every hard-coded acquisition/pricing endpoint, region sweep, provider-local price/mode fact, evidence record, and pricing mapping/classifier to one owner. Bedrock regions, official price-source endpoints, and data-only alias/mapping tables are typed normalized source configuration unless proven SDK invariants; Snowflake/OCI reviewed prices and modes are canonical offerings; protocol wire parsing, canonical unit conversion, and irreducible matching logic remain minimal tested adapter behavior. `PricingCatalog`, `RegionCatalog`, and provider-local pricing/region mini-catalog files are removed; any retained generic loader serves canonical owners, passes the deletion test, and has no parallel schema; production call paths, generation, and focused races pass |

PAC4 gate:

```bash
make provider-contract-check
go test ./internal/providers/... ./internal/connectors/... ./internal/sources/providers -race
go test ./pkg/catalogs ./pkg/sources -race
git diff --check
```

## PAC5: Single Catalog and Publication Safety

| ID | Task | Status | Verifiable success criteria |
| --- | --- | --- | --- |
| PAC5.1 | Remove parallel catalog products and deepen the canonical module | DONE | `CustomerInventory`, `CustomerScope`, `CustomerDeployment`, `Result.CustomerInventory`, `PublicCatalog`, `emptyPublicCatalog`, `ObservationScopeCustomer`, `SourceKindCustomer`, `CatalogOverlay`, `EffectiveCatalog`, customer-delta types, boolean customer-inventory fetch switches, and parallel store/query interfaces have no production references; source kind no longer encodes publication eligibility; deleting these surfaces does not scatter duplicate identity, validation, reconciliation, copy, or persistence implementations because those behaviors remain local to `Catalog` |
| PAC5.2 | Represent custom and early-access inventory canonically | DONE | Credential-scoped models, definitions, offerings, entitlements, pricing, limits, lifecycle, deployments, aliases, and endpoints round-trip through exact schema-v2 catalog structures, extending canonical offering identity only where multiple scoped deployments require it; identity collisions and cross-context references fail typed |
| PAC5.3 | Derive publication eligibility from observation provenance | DONE | Public generation starts from an isolated public-only baseline; public-only authenticated observations can generate; a catalog/run containing any credential-scoped observation is rejected by bootstrap, public generation/evidence, distribution, remote publication, and content negotiation before bytes are written; no record-level mixed-catalog filtering exists |
| PAC5.4 | Migrate existing credential-scoped discovery | DONE | Bedrock application profiles, Azure deployments, OCI endpoints, Databricks workspace endpoints, NVIDIA NIM, watsonx deployments, and analogous paths are reachable from configured production sources and emit ordinary canonical definitions/offerings plus credential-scoped observations rather than separate result fields or test-only exported methods; redundant fetchers are removed with rationale |
| PAC5.5 | Reconcile contextual facts without overlays | DONE | Public and credential-scoped facts acquired for one run reconcile deterministically into that run's `Catalog`; no anonymous/authenticated delta is computed, no shared public catalog is mutated, and no overlay read path is required |
| PAC5.6 | Prove context isolation and deep copies | DONE | Cross-account/project/workspace/subscription state cannot leak between client/store contexts; returned nested catalog and observation records are caller-owned; concurrent reads/writes pass focused race tests |
| PAC5.7 | Remove the shadow authority module | DONE | Production-compiled but test-only-referenced `EnterpriseAttributeMatrix` files and symbols are absent; any required credential-scoped authority rules are local to the production-used `CanonicalPolicies` implementation; call-graph, focused authority, and reconciler tests prove the production policy path and reject cross-context leakage |
| PAC5.8 | Lock contextual sensitivity and persistence policy | DONE | One reviewed matrix covers credentials, tokens, SDK objects, account/project/subscription/workspace IDs, custom model IDs, deployment names, aliases, endpoints, prices, limits, and entitlements across acquire, normalize, encode, in-memory query/copy, durable-store rejection, log/error, and public generation/distribution. The ADR and implementation retain only caller-useful facts, redact or keep routing-only identifiers runtime-only, reject credential-scoped material before every durable/public write, and prove cross-context isolation under race |

PAC5 gate:

```bash
go test ./pkg/catalogs ./pkg/catalogstore ./pkg/catalogdistribution ./pkg/authority ./pkg/reconciler -race
go test ./internal/providers/{bedrock,azurefoundry,oci,databricks,nvidia,watsonx} -race
go test ./internal/catalog/remote ./internal/server/... -race
git diff --check
```

## PAC6: Derived Fixture Policy and Documentation

| ID | Task | Status | Verifiable success criteria |
| --- | --- | --- | --- |
| PAC6.1 | Derive fixture/evidence class from provider sources | DONE | Deterministic function maps observation scope/auth/bindings/topology and connector/adapter delta to shared connector fixture, table-driven provider composition test, provider-delta fixture, governed raw public observation, SDK fake, or prohibited credential-scoped capture; exact OpenAI-compatible providers do not own response-schema fixtures |
| PAC6.2 | Delete the broad duplicate fixture policy | DONE | `internal/providers/fixtures/policy.yaml`, its duplicated role/reason parser in provider contract tests, and all references are removed; exhaustive derivation covers every configured source |
| PAC6.3 | Retain only genuine test-specific exceptions and deltas | DONE | Any exception or provider-local fixture that cannot be derived has a narrow checked-in record with concrete wire/topology evidence and rationale; exact OpenAI providers and ordinary Cerebras source semantics require no response-schema retest; retained adapters test only their delta |
| PAC6.4 | Update provider-authoring documentation | DONE | `docs/ADDING_PROVIDERS.md` explains standalone `none`, deterministic `optional`, conventional required `api_key`, provider-inferred required `cloud_chain`, ordered required alternatives, compound named methods, missing-versus-invalid behavior, source optionality, bindings, observation scopes, invocation metadata, connectors/adapters, official-SDK chain ownership, Cloudflare token/account auth, Databricks no-auth safety, the one-logical-execution rule with bounded pagination/retry, one contextual catalog, fixture ownership, and live checks with before/after examples |
| PAC6.5 | Update architecture, advisory metadata, and contributor contracts | DONE | AGENTS, amended ADR/architecture, models.dev authority/reconciliation/diff behavior, testing, `PROVIDER_IMPLEMENTATION_CONTRACT.md`, provider-specific docs, control-plane links, examples, CLI help, and generated docs use only single-catalog plus observation-scope terminology and preserve env-name documentation without making advisory metadata executable; the formerly normative fixture-policy/compatibility contract is rewritten or explicitly historical |
| PAC6.6 | Add structural documentation tests | DONE | Tests fail if singular API-key/endpoint guidance, broad fixture policy, compatibility promises, or customer/public conflation returns |
| PAC6.7 | Consolidate provider-fixture developer tooling | DONE | Refresh/import/replay share one documented developer-tool interface backed by `internal/providers/fixtures`; no duplicate standalone main packages, `httptest` production bridge, provider secret/config mutation, alternate parser, or canonical-fact authority remains; governed raw responses retain provenance and deterministic tool tests pass |

PAC6 gate:

```bash
make provider-contract-check
make docs-check
go test ./internal/providers/... ./internal/connectors/... -race
git diff --check
```

## PAC7: Compatibility Removal, Artifacts, and Schema-v2 Regeneration

| ID | Task | Status | Verifiable success criteria |
| --- | --- | --- | --- |
| PAC7.1 | Remove old production credential/configuration entry points | DONE | Exact searches find no singular `ProviderAPIKey`, `apiKeyValue`, `EnvVarValues`, `APIKeyValue`, `LoadAPIKey`, `LoadEnvVars`, `HasAPIKey`, `IsAPIKeyRequired`, `auth_required`, old auth expression objects, old provider endpoint decoding, `CustomerInventory`, `PublicCatalog`, boolean customer-inventory fetch switches, dead scoped fetch entry points, `EnterpriseAttributeMatrix`, obsolete `CloudCredentialChain`, global provider registration hooks, unused credential bypasses, empty `applicationClient`, arbitrary-endpoint raw fetch, or alternate raw model parser outside explicit negative fixtures/history |
| PAC7.2 | Remove migration and compatibility behavior | DONE | No alias, fallback decoder, migration-on-read, deprecated method, compatibility range, legacy directory migration, legacy `Precision`, `FORMAT` compatibility alias, older-integration provider promise, or documentation promise accepts or advertises the replaced prelaunch shape unless a current internal requirement is proven and renamed precisely |
| PAC7.3 | Add required negative and positive fixtures | DONE | Old provider YAML/payload/manifest and old auth expressions fail before publication; new `none`/`optional`/required-method/ordered-list encode-decode, credential selection, public authenticated publication, credential-scoped publication rejection, canonical definition/offering, context isolation, secret-free copy, and deep-copy fixtures pass |
| PAC7.4 | Regenerate canonical artifacts | DONE | Embedded bootstrap, generation manifest, deterministic fixtures, artifact hashes, provider/model files, generated docs, and verification evidence match the new exact schema-v2 bytes |
| PAC7.5 | Review retained migration facilities | DONE | Every remaining data import or operational migration is proven unrelated to unpublished credential/source compatibility and recorded with file-level rationale |
| PAC7.6 | Verify repository searches with behavior evidence | DONE | Search output and corresponding positive/negative tests are appended to the evidence log; absence alone is never the closure claim |
| PAC7.7 | Remove audited worktree debris and redundant surfaces | DONE | Accidental prose compiles away; exact-compatible provider response fixtures/tests, duplicate fixture binaries, shallow globals/bypasses, obsolete adapters, and compatibility-only aliases/promises are removed according to the disposition ledger; retained governed observations, operational imports, and adapters have production call graphs, concrete rationale, and positive behavior tests rather than absence-only proof |

PAC7 gate:

```bash
make catalog-generation-check
make embedded-catalog-budget-check
make docs-check
go test ./pkg/catalogs ./pkg/catalogstore ./pkg/catalogdistribution ./internal/catalog/remote -race
go test ./pkg/catalogartifact ./cmd/starmap-catalog-release -race
git diff --check
```

## PAC8: Full Verification, Review, and Closeout

| ID | Task | Status | Verifiable success criteria |
| --- | --- | --- | --- |
| PAC8.1 | Run focused race gates | DONE | Catalogs, catalogstore, distribution, remote consumption, reconciliation, provider fan-out, auth, registry, every connector, every custom provider, and Bedrock pass with `-race` |
| PAC8.2 | Run repository-wide static and deterministic gates | DONE | Repository-wide short race, vet, pinned lint, generated docs, provider contract, catalog generation, embedded budget, artifact, validation, and diff gates pass |
| PAC8.3 | Run complete repository verification | DONE | `make verify` passes from the exact final worktree with secrets isolated; exact toolchain and duration are recorded |
| PAC8.4 | Perform final architecture/security review | DONE | Review confirms no secret persistence or copied runtime values, ambient credential attachment, customer/public crossover, ambiguous auth, source/auth optionality conflation, unbounded acquisition, shared mutable results, compatibility remnants, duplicate config authority, speculative seam, pass-through adapter, alternate parser, or redundant provider-local protocol fixture; every retained/new module passes the deletion test and findings are fixed or mapped |
| PAC8.5 | Record Git/hosted evidence without duplicate work | PENDING | If authorized, reuse the existing branch/draft PR, verify exact-head required checks, and record IDs; do not create duplicate PR/run and do not merge/publish/release/deploy |
| PAC8.6 | Close ledgers and goal | PENDING | Every PAC task and PAF finding is `DONE` or user-approved `REJECTED`; final production/test/config diffstat, exported-surface change, removal/retention rationale, phase totals, and exact artifact hashes are recorded; bespoke production code and exported interfaces show net deletion or concrete evidence justifies each residual increase; active whole-plan goal is completed only now |

PAC8 gate:

```bash
go test ./... -race -short -count=1
go vet ./...
devbox run lint
make provider-contract-check
make docs-check
make catalog-generation-check
make embedded-catalog-budget-check
go test ./pkg/catalogartifact ./cmd/starmap-catalog-release -race -count=1
git diff --check
make verify
```

## Required Search Evidence

Run searches from the worktree root. Record exact output/counts in the evidence
log and pair every search with behavior tests.

```bash
rg -n 'ProviderAPIKey|APIKeyValue|LoadAPIKey|HasAPIKey|IsAPIKeyRequired' --glob '*.go' .
rg -n 'apiKeyValue|EnvVarValues|LoadEnvVars|any_of|all_of|required:[[:space:]]*false|auth_required' internal pkg docs --glob '*.go' --glob '*.yaml' --glob '*.md'
rg -n 'api_key:|auth_required:|catalog:[[:space:]]*$|endpoint:' internal/embedded/catalog/providers.yaml
rg -n 'schema.?v1|legacy.*catalog|migration.on.read|compatib|deprecated' --glob '*.go' --glob '*.md' .
rg -n 'CustomerInventory|CustomerScope|CustomerDeployment|ObservationScopeCustomer|SourceKindCustomer|CatalogOverlay|EffectiveCatalog|customer_catalog|customer_inventory|public_catalog|authenticated delta|customer overlay' internal pkg docs
rg -n 'Result\.CustomerInventory|PublicCatalog|emptyPublicCatalog|includeCustomerInventory|FetchWorkspace|FetchCustomerNIM|FetchDeployments|EnterpriseAttributeMatrix|CloudCredentialChain|NewCloudCredentialChain' internal pkg docs --glob '*.go' --glob '*.md'
rg -n 'aws_default|azure_default|google_adc|oci_default|auth:[[:space:]]*cloud_chain|cloud_chain' internal pkg docs --glob '*.go' --glob '*.yaml' --glob '*.md'
rg -n 'credentials:|auth:|observation_scope:|global_public|regional_public|credential_scoped|scopes:' internal pkg docs
rg -n 'os\.(Getenv|LookupEnv)\(|DEEPINFRA_(TOKEN|API_KEY)' internal pkg --glob '*.go' --glob '*.yaml'
rg -n 'fixtures/policy.yaml|fixture exception|evidence-backed fixture exception' internal docs AGENTS.md Makefile
rg -n 'great lets do a full audit|RegisterProvider(ClientFactory|RawFetcher)|registeredProviderHooks|WithoutCredentialLoading|WithAllowMissingAPIKey|applicationClient|parseModelsFromRaw' internal pkg cmd docs --glob '*.go' --glob '*.md'
rg -n 'FetchRaw(Response|Result|\()|caller-supplied integration points|endpoint string' pkg internal cmd --glob '*.go'
rg -n 'Legacy precision|backwards-compatible alias|older integrations|legacy|migrate' pkg internal cmd docs --glob '*.go' --glob '*.yaml' --glob '*.md'
rg -n 'providerdata|PricingCatalog|PricingFacts|RegionCatalog|LoadPricingCatalog|LoadRegionCatalog|pricing.yaml|regions.yaml' internal pkg docs --glob '*.go' --glob '*.yaml' --glob '*.md'
rg -n 'customer identifiers|tenant identifiers|private endpoints|credential-scoped.*(endpoint|deployment|alias)|runtime-only|private persistence' docs pkg internal --glob '*.go' --glob '*.md'
find internal/providers \( -path '*/testdata/models_list.json' -o -name 'provider_test.go' \) -print
find cmd -maxdepth 2 -type f \( -path '*provider*fixture*' -o -path '*provider*testdata*' \) -print
```

Expected final interpretation:

- Old production entry-point searches are empty outside explicit negative
  fixtures and preserved historical evidence.
- Current-contract searches are non-empty and backed by positive tests.
- Operational data import/migration facilities, if retained, are enumerated and
  proven unrelated to old credential/source schema compatibility.

## Evidence Ledger

Append evidence after every meaningful task. Do not rewrite old evidence when a
later task supersedes it; add a new entry that identifies the supersession.

| Time | Task(s) | Evidence | Result |
| --- | --- | --- | --- |
| 2026-07-13 America/Chicago | PAC0.1,PAC0.2 | Created this control plane from live worktree inspection. Baseline: branch `codex/provider-expansion-wave0`, HEAD `59a1ba23f21746f625454c2528e9842955b20b9f`, 130 pre-existing dirty entries. Current schema has one provider `APIKey`, one endpoint `AuthRequired`, provider-wide env vars, API-key-specific preflight, a Google ADC checker branch, hard-coded native cloud source construction, deployment-only `CustomerInventory`, and a broad fixture policy. | DONE |
| 2026-07-13 America/Chicago | PAC0.1,PAF-016 | Full architecture/YAML-DX review corrected the draft from three publication products to ADR 0001's public `Catalog` plus deepened private `CustomerInventory`; standardized `none`/`api_key` authoring, retained typed env metadata, added invocation routes, typed binding sources/roles, shared acquisition groups, models.dev advisory ownership, a before/after replacement ledger, and blocker findings PAF-015 through PAF-020. No implementation was claimed complete. | DONE |
| 2026-07-13 America/Chicago | PAC0.1,PAF-002 | Auth-semantics review counted 20 explicit true and 7 explicit false endpoint booleans. Confirmed current false means an API key is optional and still sent when configured, while omission also defaults false and cloud auth is not represented. Revised the target to required-by-default named auth with explicit `required: false` anonymous fallback; retained `none` only for sources with no credential path. | DONE |
| 2026-07-13 America/Chicago | PAC0.1,PAF-007,PAF-014,PAF-016,PAF-020 | User-directed architecture correction amended ADR 0001 and the full control plane to one canonical `Catalog`. Credentials are preferred and each logical source is fetched once; optional missing credentials alone trigger anonymous fallback. All acquired facts normalize into ordinary definitions/offerings with global-public, regional-public, or credential-scoped observation provenance. Public generation must fail before writing when credential-scoped observations are present. The plan now owns removal of `CustomerInventory` and all overlay/delta equivalents with positive canonical catalog, publication, isolation, deep-copy, and request-count proof. No implementation cleanup is claimed complete. | IN_PROGRESS |
| 2026-07-13 America/Chicago | PAC0.1,PAF-004,PAF-021,PAF-022,PAF-023 | Follow-up repository and first-party cloud-auth review found parallel provider result/fetch paths that production disables or never calls, a test-only shadow authority module, and a test-only generic cloud callback sequencer. The plan now preserves `cloud_chain` as a deep provider-inferred interface backed by four official SDK adapters, removes vendor chain names from YAML, keeps Cloudflare token/account based, and owns deletion or production rewiring of every shallow/dead surface with behavioral proof. No implementation cleanup is claimed complete. | IN_PROGRESS |
| 2026-07-13 America/Chicago | PAC0.1,PAF-002,PAF-010,PAF-020,PAF-024,PAF-025 | Current-code auth review replaced the draft expression language with explicit `none`, deterministic `optional`, required methods, and ordered required alternatives. `none` stands alone; `optional` tries conventional API key, registered cloud chain, then one unauthenticated request only after absence; invalid-present credentials fail. Source optionality remains independent. The plan now owns the concrete Databricks ambient-token risk and removal of runtime credential/environment values from copied provider configuration. No implementation cleanup is claimed complete. | IN_PROGRESS |
| 2026-07-13 America/Chicago | PAC0.7,PAF-026-PAF-032 | Audited `origin/main...HEAD`, the full dirty overlay, provider/connector/registry/fixture dataflows, normative docs, and prelaunch compatibility surfaces. Exact Git evidence: base `9508ee7866e4683e001e7ad153319d348433045d`, head `59a1ba23f21746f625454c2528e9842955b20b9f`, two branch commits, 382 committed files with +33,606/-3,111, 133 current status entries, and 90 tracked dirty files with +1,188/-8,857. Draft PR #40 is open/clean with two successful hosted checks at the committed head only. The release worktree was not touched and no active release/catalog run was found. `go test -race ./pkg/catalogs ./pkg/sources ./internal/providers/... ./internal/connectors/... ./internal/catalog/pipeline` failed before tests at `pkg/sources/cloud_credentials.go:10:1` because literal prose corrupts the file. A compile-independent subset, `go test -race ./pkg/catalogs ./internal/connectors/... ./internal/providers/registry ./internal/providers/fixtures/...`, passed. `git diff --check HEAD` and the untracked whitespace scan passed. Added a retain/revise/remove disposition ledger and blocker findings for unsafe raw origin authority, stats-parser divergence, shallow hooks/adapters, redundant exact-provider fixtures, unproven custom clients, duplicate developer binaries, and ambiguous compatibility/import surfaces. No production implementation was changed or claimed green. | DONE |
| 2026-07-13 America/Chicago | PAC0.1,PAC0.3,PAC8.4,PAC8.6,PAF-026 | Re-reviewed the whole-plan `/goal` for execution order and code-growth incentives. Corrected PAC0.3 so goal activation immediately removes accidental prose and proves a compilable baseline before characterization. Added a net-deletion constraint, deletion-test obligations for every module/interface/seam/adapter, phase diffstat and exported-surface evidence, explicit rejection of unexplained bespoke production growth, and a live-ledger rule so later findings cannot fall outside a frozen PAF range. Directly named cleanup files currently account for 935 lines before provider-specific custom-client deletions; CustomerInventory-related symbols affect 17 Go files and require behavioral migration rather than blind whole-file deletion. No production implementation was changed. | DONE |
| 2026-07-13 America/Chicago | PAC0.7,PAF-004,PAF-018,PAF-030,PAF-033,PAF-034 | Full plan-to-code audit found a shallow parallel provider-fact module, a contradictory sensitivity boundary, and an unmodeled two-inventory Together call. `internal/providerdata` defines `PricingCatalog`/`RegionCatalog`, loads three provider-local pricing/region mini-catalogs, and has 28 Go references across Snowflake, OCI, Bedrock, and tests; this conflicts with canonical provider source configuration and canonical catalog ownership. Production provider code also contains five direct HTTPS literals and three direct environment reads, now explicitly dispositioned as normalized source facts, SDK invariants, or irreducible adapter behavior. PAC1.4 forbade retained private endpoints while PAC5.2 and ADR 0001 required contextual endpoints to round-trip. Together currently fetches serverless and dedicated inventories inside one `ListModels`; the plan now requires distinct configured logical sources and bounded request counts. Added locked decisions, cleanup dispositions, PAC4.8 and PAC5.8, field-level positive/negative proof, pricing-rule YAML-versus-Go disposition, exact searches, and `/goal` obligations. Verified every named Make gate exists. `scripts/verify.sh` covers repository tests, short race, vet, optional lint, critical coverage, docs, diff, build, and credential-free CLI smokes; because its lint is skippable and it does not explicitly isolate artifact packages, PAC8 now separately runs pinned `devbox run lint` and race tests for `pkg/catalogartifact` plus `cmd/starmap-catalog-release`, in addition to provider-contract, catalog-generation, and embedded-budget gates. `go test -race ./internal/providerdata ./internal/providers/together` passed, proving the current duplicate module and two-call behavior are compilable characterization targets; `go test ./pkg/sources` still fails at the separately owned PAF-026 syntax contamination. Structural validation found 63 unique task rows, 34 unique sequential finding rows, no missing task references, the updated whole-goal range twice, no stale PAF-032 range, and no whitespace errors. Live GitHub reports protected `main` at `9508ee78`, draft PR #40 clean at committed head `59a1ba23`, and no active release/catalog run; the unrelated local main worktree is divergent at `3787d716` and was left untouched. No Codex goal is active, so PAC0.3 remains pending. No production implementation was changed or claimed green. | DONE |
| 2026-07-13 America/Chicago | PAC0.3,PAF-026 | Activated one whole-plan Codex goal covering PAC0-PAC8 and the live finding ledger without a phase or wave budget. Restored the original declaration comment in `pkg/sources/cloud_credentials.go` by removing only the accidental literal user prose; the independent PAF-023 deletion test for the shallow generic helper remains pending. Exact phrase search returned no match. `go test ./pkg/sources` and `go test -race ./pkg/sources` passed; `git diff --check HEAD -- pkg/sources/cloud_credentials.go` passed. Execution advanced automatically to PAC0.4. | DONE |
| 2026-07-13 America/Chicago | PAC0.4,PAF-004,PAF-015,PAF-018,PAF-020,PAF-021,PAF-024,PAF-030 | Added the exhaustive provider credential, scope, topology, endpoint-owner, production-path, and publication matrix. A structural check against live YAML/code passed with 27 embedded providers, 3 native providers, 30 matrix rows, 53 distinct current provider-related environment names each present once in the provider rows, and all 3 existing descriptions preserved verbatim. First-party DeepInfra documentation retrieved 2026-07-13 uses both `DEEPINFRA_TOKEN` and `DEEPINFRA_API_KEY`, so one ordered credential owner preserves both and rejects conflicting simultaneous values. First-party Google documentation confirms `GOOGLE_CLOUD_PROJECT`/`GOOGLE_CLOUD_LOCATION` and SDK-owned ADC precedence; the matrix also owns current `GOOGLE_VERTEX_*`, `GOOGLE_CLOUD_REGION`, and connector/auth precedence divergence. `go test -race ./internal/auth/... ./internal/connectors/google ./internal/providers/bedrock ./internal/providers/azurefoundry ./internal/providers/oci` passed; `git diff --check HEAD -- docs/PROVIDER_ACQUISITION_CONTROL_PLANE.md pkg/sources/cloud_credentials.go` passed. PAC0.5 is next. | DONE |
| 2026-07-13 America/Chicago | PAC0.5,PAF-002,PAF-004,PAF-007,PAF-010,PAF-011,PAF-013,PAF-014,PAF-017,PAF-019,PAF-020,PAF-023,PAF-024 | Locked 15 stable contract fixture anchors covering no-auth prohibition, optional selection, required key/chain/alternatives, compound methods, Cloudflare account auth, Cerebras catalog/invocation split, Anthropic transport override, Vertex precedence, Bedrock sweep, models.dev advisory isolation, shared acquisition, one credential-scoped catalog, and the exact schema-v2 break. Every anchor states positive behavior, paired negative proof, and the shallow/duplicate surface it permits later tasks to remove. Structural validation confirmed all 15 expected anchors exist exactly once and `git diff --check` passed. PAC0.6 is next. | DONE |
| 2026-07-13 America/Chicago | PAC0.6,PAF-002,PAF-010,PAF-013,PAF-017 | Added target-contract tests at the production `Provider` decode/validation interface. `go test ./pkg/catalogs -run '^TestTargetProviderContract' -count=1` failed only at the two intended gaps: the target `credentials`/`catalog.sources`/`invocation.routes` shape is discarded and validation reports missing old `catalog.endpoint.type`; the old singular `api_key`/`env_vars`/`catalog.endpoint.auth_required` shape remains readable. Existing no-network characterization passed under race for Cerebras, Databricks, Cloudflare, Bedrock, Anthropic, Google, and models.dev. Diff checks passed. PAC0 is DONE and execution advanced to PAC1.1; the target tests remain red until the exact-current schema implementation lands. | DONE |
| 2026-07-13 America/Chicago | PAC1.1-PAC1.3,PAC1.6,PAF-001,PAF-002,PAF-005,PAF-010,PAF-018 | Began the exact-current provider configuration module. Added typed named credentials with ordered scalar/list environment inputs and explicit bearer/basic/direct transport encoding; normalized the conventional `api_key` defaults; added scalar/list auth policies, ordered catalog sources, independent invocation routes, invariant or auth-dependent observation policies, typed binding sources/roles, bounded source topology, source-local mappings/defaults, safe identifiers and HTTPS endpoints, and nested deep copies. Focused positive/negative YAML/JSON/validation/copy tests pass normally and under race. The original singular provider fields and decoder remain only as an explicitly red migration target: `TestTargetProviderContractRejectsOldSingularShape` still fails, so no PAC1 task or schema-break claim is complete. Cloud-chain registry validation, compound methods, strict decoding, provider migration, and runtime-value removal remain owned work. | IN_PROGRESS |
| 2026-07-13 America/Chicago | PAC1.1-PAC1.4,PAC1.6,PAC2.1,PAC2.2,PAC4.1,PAF-001-PAF-005,PAF-010,PAF-015,PAF-018,PAF-025 | Migrated all 27 embedded providers from singular `api_key`/`env_vars`/`catalog.endpoint`/`auth_required`/`chat_completions` YAML to typed credentials, explicit ordered sources, observation scopes, auth policies, source-local docs/mappings/defaults/authors, typed scopes/options, environment advisories, and independent invocation routes. The migration preserved DeepInfra's ordered aliases and the three exact descriptive strings; a structural check reported `27 exact source-shaped providers`, and focused embedded provider validation plus configuration contract tests passed under race. Added a request-scoped resolver with an injectable environment, zero-look-up `none`, deterministic key/chain/anonymous `optional`, ordered required alternatives, equal-alias acceptance, typed conflicting-alias and invalid-present failure, secret-safe diagnostics, request-only transport application, and an immutable duplicate-rejecting provider cloud-chain registry contract. Resolver and registry tests pass under race. This is not a completed schema break: old Go fields/methods and runtime consumers still exist, exact mixed/old-shape tests remain red, official SDK adapters are not yet wired, and several providers still need the additional logical sources named by PAC4. | IN_PROGRESS |
| 2026-07-13 America/Chicago | PAC1.1-PAC1.6,PAC2.1,PAF-001,PAF-002,PAF-010,PAF-013,PAF-017,PAF-025 | Completed the direct provider-schema break in the Go and fixture surfaces: all repository tests now compile without `ProviderAPIKey`, `ProviderEndpoint`, `ProviderEnvVar`, runtime value fields, or schema-v1 test constructors. Added one named compound credential kind whose typed input metadata deep-copies and whose request-scoped resolver rejects partial input sets without exposing values. Added bounded recursive exact-JSON validation that rejects duplicate keys at payload, bootstrap manifest, generation manifest, remote-consumption, and hosted-distribution boundaries before typed decode. Regenerated OpenAPI and Go docs, then regenerated embedded generation `catalog-20260714T044931Z-1a432408db6b` with payload checksum `sha256:1a432408db6b06b80f55e7668f34e5498913b896405392f63fcdbd5cae4d847c` and size `2076548`. `go test ./... -run '^$'` passed; focused exact/compound tests passed normally and under race; `go test ./pkg/catalogs ./pkg/catalogstore ./pkg/catalogdistribution ./pkg/sourcepayload -race -count=1` passed; bootstrap, bootstrapmanifest, and catalogremote races, focused vet, and `git diff --check` passed. PAC1.2, PAC1.3, PAC1.5, PAC1.6 and findings PAF-001/002/010/013/017 are DONE. PAC1.1 remains open for production official-SDK cloud-chain registrations; PAC1.4 remains open for the contextual private/public field matrix and fail-closed publication proof; ambient runtime helper methods remain owned by PAC2/PAC7 and are not claimed complete. | DONE |
| 2026-07-13 America/Chicago | PAC1.1,PAC1.4,PAC3.3,PAC5.3,PAF-004,PAF-006,PAF-014,PAF-023,PAF-034 | Added the production official-SDK cloud-chain registry module with exactly one provider-inferred AWS, Azure, Google, and OCI adapter registration for Amazon Bedrock, Microsoft Foundry, Google Vertex, and OCI. The adapters delegate discovery/refresh to each official SDK and return provider-typed sessions with no serializable fields; exhaustive registry, unsupported-provider, Cloudflare-negative, error, serialization, and race tests pass. Added the normative field sensitivity/persistence matrix. Replaced the unpublished `customer_scoped` observation and `customer_inventory` source-kind compatibility surface with `credential_scoped` provenance on ordinary canonical catalog observations; source kind now describes acquisition rather than publication. Public generation and manifest validation reject credential-scoped observations before allocating a generation/sync identity, while focused positive in-memory observation and negative generation tests pass under race. PAC1.1 is DONE. PAC1.4 remains IN_PROGRESS until in-memory query/copy/log policy, durable-store rejection, and every public boundary are proven in PAC5; the old generic `CloudCredentialChain[T]`, direct native source construction, and special-case runtime auth remain explicitly open under PAC2/PAC3/PAC7. | DONE |
| 2026-07-14 America/Chicago | PAC2.1-PAC2.5,PAC2.7,PAC3.1,PAC3.3,PAF-005,PAF-008,PAF-020,PAF-023-PAF-025,PAF-027,PAF-028 | Introduced the neutral `internal/acquisition` request contract and migrated every connector plus the provider fetcher to immutable provider/source configuration, one resolved auth value, resolved bindings/options, and the exact configured endpoint. Removed connector API-key capability methods, catalog-level ambient credential/endpoint/validation APIs, the Google ADC file parser/special-case preflight, mutable global provider hooks, credential bypasses, empty application success, the test-only generic `CloudCredentialChain[T]`, arbitrary raw URLs, and `parseModelsFromRaw`. Logical sources now execute once, retain identity/auth/scope, continue independently after peer failure, and fail the aggregate observation to `credential_scoped` when any successful result is contextual; absent optional sources degrade while present-invalid credentials fail before connectors. Raw acquisition rejects cross-origin redirects before forwarding, copies response metadata instead of returning a closed body, and normal `--stats` uses the same connector execution. Exact production searches for the removed symbols and ambient connector/catalog env reads were empty. `go test ./... -run '^$'`, `git diff --check`, and `go test ./pkg/sources ./internal/auth ./internal/transport ./internal/providers/registry ./internal/acquisition ./internal/sources/providers -race -count=1` passed. The current whole-worktree tracked diff is net deletion at 5,450 insertions and 13,237 deletions before untracked new modules. PAC2.1 and PAC2.4 plus PAF-005/008/020/024/025/028 are DONE. PAC2.2/2.3/2.5/2.7 and PAC3 remain open for native cloud registry routing, stage-specific bindings, source-granular CLI presentation, one normal/raw evidence result, and configured native/cloud sources. | DONE |
| 2026-07-14 America/Chicago | PAC1.4,PAC2.2-PAC2.7,PAC3.1-PAC3.7,PAC4.1-PAC4.8,PAC5.1-PAC5.8,PAF-003,PAF-004,PAF-006,PAF-007,PAF-009,PAF-014-PAF-016,PAF-018-PAF-023,PAF-027,PAF-030,PAF-033,PAF-034 | Completed the configuration-driven acquisition and single-catalog implementation. Official SDK adapters, source-granular auth status, stage-resolved bindings, safe typed errors, one normal/raw acquisition result, configured native sources, independent bounded logical sources, exact observation provenance, degradation semantics, and concurrent context isolation are production-wired. Every provider has explicit source/auth/scope configuration; NVIDIA public/NIM and Together serverless/dedicated are distinct logical sources; Mistral and SambaNova custom clients are deleted; provider fact mini-catalogs are removed in favor of source configuration and canonical offerings. Credential-scoped definitions/offerings round-trip and deep-copy in memory while generation, store, distribution, remote, and bootstrap boundaries reject the complete contextual generation before writing. The broad focused race matrix passed for catalogs, store, distribution, remote, bootstrap, manifests, authority, reconciliation, auth, acquisition, every connector/provider, source fan-out, and catalog pipeline; one stale server fixture failed separately as PAF-035. | DONE |
| 2026-07-14 America/Chicago | PAC6.1-PAC6.7,PAC7.1-PAC7.7,PAF-011-PAF-013,PAF-019,PAF-029,PAF-031,PAF-032 | Replaced duplicate fixture policy with an exhaustive deterministic source-to-fixture-class function exercising shared connector, table composition, provider delta, governed public observation, SDK fake, and prohibited credential-scoped capture. Consolidated refresh/import/replay into `cmd/provider-fixtures` plus `internal/providers/fixtures`, removed exact-provider protocol retests and duplicate binaries, documented every retained adapter and current operational import, and removed prelaunch schema/auth/config/directory/CLI aliases with paired negative tests. Secret-safe live temporary refreshes succeeded for Cohere, Mistral, Together serverless/dedicated, Hugging Face, and NVIDIA; xAI returned HTTP 403 and is recorded as failure. `make provider-contract-check`, `make docs-check`, `make catalog-generation-check`, artifact/release races, exact catalog validation, and the embedded budget passed. Current artifact: `catalog-20260714T075600Z-2a1242730cc4`, payload `sha256:2a1242730cc447ac9be83f9b6cce186a271df34883c4f10eac46c47c8ce025aa`, 2,134,894 raw bytes, 95,133 compressed bytes, 30 providers, 746 provider model files, 85 authors, and 1,121 canonical definitions. | DONE |
| 2026-07-14 America/Chicago | PAC7.2,PAC7.3,PAC7.5,PAC7.6,PAC8.1,PAF-032,PAF-035 | The compatibility audit removed `--fmt`/`--format`, `inspect`, `server`, and `STARMAP_OUTPUT_FORMAT`, retained only current identity/routing aliases and package-private source projection with concrete architecture rationale, and added negative command-registration proof. Production exact searches were empty for old schema/catalog/auth/runtime/provider/compatibility entry points; removed directories/files were absent. The PAC1-PAC7 race exposed a durable-server test provider missing the required current source; the fixture now declares an application source with explicit `global_public`/`none`, and `go test ./internal/server -race -run '^TestDurableServerUpdatePublishesSameGenerationAfterProcessRestart$' -count=1` passed. This behavior failure, not an absence search, closes PAF-035. | DONE |
| 2026-07-14 America/Chicago | PAC8.1,PAC8.2,PAF-036,PAF-037 | Repaired the reproducible lint toolchain by moving the Devbox pin and lock from `golangci-lint@2.5.0` (built with Go 1.25 and unable to analyze toolchain Go 1.26.5) to `golangci-lint@2.12.2` built with Go 1.26.4. Consolidated repeated provider identities, validation messages, canonical authority paths, and resource names without behavioral aliases; `devbox run lint` completed with zero issues. The first broad race run also exposed a stale differ fixture still expecting removed `env_vars` and `chat_completions`; it now asserts exact-current `credentials` and `invocation`, and `go test ./pkg/differ -race -count=1` passed. The repository-wide final-tree race remains owned by PAC8.2 and is not inferred from either focused result. | DONE |
| 2026-07-14 America/Chicago | PAC6.4,PAC6.6,PAC8.3,PAF-038 | The first exact-tree `make verify` ran 330.34 seconds and passed normal tests, repository race, vet, catalog accessor performance (10.46-10.58 ns/op, zero bytes and allocations), lint, and the first four coverage packages before failing the existing 80% registry threshold at 74.6%. Added behavior tests that exhaustively bind `Supports` to the executable endpoint set and exercise positive OpenAI connector-owned fixture replay plus typed rejection for a Databricks connector without an offline replay schema. `go test ./internal/providers/registry -race -count=1 -coverprofile=...` now passes at 89.8%; no threshold was weakened. Complete verification remains pending until rerun from the exact final tree. | DONE |
| 2026-07-14 America/Chicago | PAC2.5,PAC8.3,PAF-039 | The second exact-tree `make verify` ran 292.55 seconds; the registry gate passed at 89.8% and every preceding test, race, vet, performance, lint, and coverage gate passed before `pkg/errors` reported 79.9% against its existing 80% threshold. Added direct typed `MergeError` coverage for conflict-ID and wrapped-failure forms, including `Unwrap`; `go test ./pkg/errors -race -count=1 -coverprofile=...` now passes at 83.3%. No threshold or production behavior changed. Complete verification remains pending until the next exact-tree rerun. | DONE |
| 2026-07-14 America/Chicago | PAC2.1,PAC2.3,PAC8.3,PAC8.4,PAF-040 | Manual closeout review caught ambient ADC attachment in the verifier: its credential-free table smoke reported Google Vertex `Ready` because the official Google SDK found the developer's home credential file. The verifier now unsets conventional AWS/Azure/Google/OCI cloud-chain inputs, disables AWS metadata, gives every built-CLI smoke an empty temporary home, and uses JSON provider listing so the smoke validates catalog output without intentionally resolving authentication. `bash -n scripts/verify.sh` passed, and an `env -i`/empty-home built-binary provider smoke returned one exact provider record with credential metadata but no auth status or runtime value. Complete verification remains pending until rerun with this exact script. | DONE |
| 2026-07-14 America/Chicago | PAC8.1-PAC8.4,PAF-036-PAF-040 | Final local closeout passed from the corrected exact tree. `git diff --check` and pinned `golangci-lint@2.12.2` reported zero issues; `/usr/bin/time -p make verify` passed in 296.90 seconds under Go 1.26.5, including normal tests, repository-wide `-race -short`, vet, three 0-allocation catalog-access benchmarks at 10.49-10.60 ns/op, all coverage thresholds (registry 89.8%, errors 83.3%), generated-doc checks, build, exact schema-v2 catalog validation, and isolated CLI smokes under an empty temporary `HOME`. Provider validation covered 30 providers, 85 authors, and 1,121 definitions; the provider smoke emitted one exact catalog record without auth status or runtime values. Separate focused races passed for catalogs, catalogstore, distribution, remote, reconciliation, source evidence/fan-out, auth/acquisition, registry, connectors, custom providers, and Bedrock; provider-contract, catalog-generation, embedded-budget, artifact/release, validation, docs, lint, and diff gates passed. Exact production searches found no removed schema/auth/customer-overlay entry points, removed paths were absent, no provider/connector production code read ambient environment values, runtime secrets remained private request-scoped resolver state, bounded reads/pagination/retry remained enforced, and retained aliases/import/fallback facilities matched `docs/PRELAUNCH_COMPATIBILITY_AUDIT.md` current-architecture rationales. The structured autoreview helper was attempted twice with the required branch range; it first identified and prompted removal of one fake secret-shaped test sentinel, then failed closed before reviewer invocation because the aggregate branch bundle still matched its secret-content policy. Per the skill contract it was not bypassed or rerouted; this repository-grounded architecture/security audit found and fixed PAF-040 and no further actionable issue. | DONE |
| 2026-07-14 America/Chicago | PAC8.6 | Staged-range accounting against protected `origin/main` covers 574 files, +45,125/-10,592. Exact categories are: production Go 165 files, +13,369/-4,459; tests and governed response fixtures 144 files, +8,623/-3,212; catalog/config/generated assets 198 files, +17,797/-843; documentation 64 files, +5,479/-2,020; tooling/other 9 files, +166/-70. Production growth is justified by the new exact schema-v2 provider/source/auth/observation contract, canonical definition/offering reconciliation, bounded native cloud/provider acquisition, and 30-provider implementation; it is not compatibility scaffolding. Deletion evidence includes old ADC parsing, provider client registry, ambient transport auth, schema-v0/v1 migration/adapters, provider validation reports, directory migration, alias package, duplicate clients/tools/fixtures, and parallel customer inventory. A declaration-level root/public-package audit reports 93 added and 59 removed exported declarations, with no added public interface; the net declarations are the current schema-v2 provider/offering/observation and bounded acquisition vocabulary, while removed declarations are legacy migrations, flattened reads, singular API-key configuration, compatibility ranges, and bypass hooks. Module-private interfaces add only the two request environments, two official-SDK cloud-chain contracts, three native SDK APIs, and one connector registry seam while removing the old generic provider client and transport authenticator. All PAC0-PAC7 rows are DONE; PAC8 has four DONE and two pending hosted/goal rows; all 40 findings are DONE. A deterministic temporary release build and independent reopen verified generation `catalog-20260714T075600Z-2a1242730cc4`, schema 2 payload `sha256:2a1242730cc447ac9be83f9b6cce186a271df34883c4f10eac46c47c8ce025aa` (2,134,894 bytes; 95,133-byte embedded compression), and archive `sha256:4149adc467624165d06dc9a23bae5f60b1e840063f3d2d6c5ca0569cb80968e4`. Exact hosted-head evidence and terminal goal state remain pending. | IN_PROGRESS |

Evidence entries must include, as applicable:

- exact task/finding IDs;
- commands and exit status;
- race/static/full verification results;
- generated catalog generation ID, payload checksum, artifact checksum, size,
  provider/definition/offering counts, and docs state;
- production/test/config diffstat plus exported interface/symbol change and
  deletion-test rationale for every retained or added seam/adapter;
- live-check status distinguished from deterministic fixtures;
- hosted run/PR/head IDs distinguished from local proof;
- any blocker, authorization boundary, residual risk, and re-entry condition.

## Autonomous Execution Rules

1. Start at the first non-`DONE` task in the first non-`DONE` phase.
2. Work in ledger order unless a later task is strictly required to unblock the
   current row; record the dependency before reordering.
3. Reproduce or characterize behavior before implementation.
4. Make coherent, reviewable changes. Update the task row, finding rows, and
   evidence ledger after every meaningful implementation slice.
5. Run focused tests after each slice and the entire phase gate before marking a
   phase `DONE`.
6. A completed row or phase automatically advances to the next non-`DONE` row.
   Never stop merely because one provider, task, phase, or gate is green.
7. Keep one active goal for this entire control plane. Do not replace it with a
   phase-, provider-, credential-, schema-, fixture-, or review-scoped goal.
8. When a test fails, diagnose and fix it within scope; do not weaken the gate,
   hide failure, or mark the row complete from prose.
9. If external state or authorization blocks progress, finish all independent
   local work, record exact evidence and re-entry conditions, and use `BLOCKED`
   only for the genuinely blocked rows.
10. Do not complete the goal until PAC0-PAC8 and every PAF row in the current
    ledger, presently PAF-001 through PAF-040, are terminal and every required
    local and authorized hosted gate is recorded.

## `/goal` Prompt

Use this prompt to drive the entire plan to completion:

```text
/goal Execute
/Users/jack/src/github.com/agentstation/starmap-worktrees/provider-expansion-wave0/docs/PROVIDER_ACQUISITION_CONTROL_PLANE.md
end to end and make Starmap's provider acquisition, credentials, source scopes,
single canonical Catalog, and observation-based publication contract fully
typed, configuration-driven, secret-safe, and exact-current schema-v2.

Treat that document as the durable source of truth. Preserve its locked
decisions, global constraints, finding mappings, phase order, task success
criteria, evidence ledger, and authorization boundaries. Begin at the first
non-DONE PAC task in the first non-DONE phase. Reproduce or characterize current
behavior before editing, preserve all unrelated and pre-existing dirty-worktree
changes, and update the task rows, finding rows, phase rows, and evidence ledger
after every meaningful implementation and verification slice.

Repository evidence outranks stale prose. When implementation or verification
reveals a missing finding, inaccurate criterion, unsafe ordering, or unnecessary
module, update this control plane with a new owned row and evidence before
continuing. Do not treat the current finding range as a ceiling, silently change
a locked architecture decision, or narrow the whole-plan objective to make a
gate pass; record and resolve the discrepancy in the ledger.

Immediately after activating this goal, restore a compilable baseline under
PAC0.3 by removing the accidental literal prose from
`pkg/sources/cloud_credentials.go`, recording the exact removal search, and
running a focused Go compile probe. This removal does not retain or approve the
obsolete generic helper; its independent deletion test remains owned by later
tasks. Then continue at PAC0.4.

Use one active goal for PAC0 through PAC8. Never replace or complete this goal
for a single credential kind, provider, source, item, task, phase, fixture,
review, or gate. Completion of any row or phase is an automatic transition to
the next non-DONE row. Do not skip pending rows, defer in-scope work, weaken a
gate, or claim completion from documentation, absence searches, one provider,
one wave, local-only proof, or coverage alone.

Bias toward net deletion. Prefer normalized provider YAML and an existing deep
connector module over new provider Go. Apply the deletion test to every new or
retained module, interface, seam, and adapter: name the concrete variation it
hides and the leverage or locality it creates. Do not add speculative wrappers,
one-off interfaces, alternate parsers, duplicate fixtures, global hooks, or
pass-through adapters. Record production/test/config diffstat and exported-
surface change at every phase gate. Tests and exact-current validation may grow
where they add behavior proof, but unexplained net growth in bespoke production
code or exported interfaces fails closeout.

Collapse parallel provider fact schemas as part of that deletion bias. Move
acquisition sweep inputs and official source endpoints to normalized provider
source configuration, move reviewed price and mode facts into canonical
definitions/offerings, and keep provenance referential rather than duplicative.
Delete `internal/providerdata` `PricingCatalog`/`RegionCatalog` and provider-local
pricing/region mini-catalogs unless a smaller generic loader passes the deletion
test against multiple canonical owners.

Implement the new contract as an intentional direct prelaunch break while
keeping canonical schema version 2: remove singular API-key/auth-required/one-
endpoint decoding and every migration, alias, fallback, compatibility test, or
documentation promise for the replaced unpublished shape. Pair exact removal
searches with negative old-shape fixtures and positive current-shape behavior.
Keep credential values, tokens, resolved SDK objects, and routing-only tenant
identifiers runtime-only. Before representing credential-scoped inventory,
lock and implement the PAC5.8 field-level sensitivity matrix: caller-useful
provider-returned deployments, aliases, endpoints, prices, limits, and
entitlements may exist only in the caller's in-memory contextual catalog under
credential-scoped provenance; they must be redacted from logs and rejected
before every durable/public serialization or write. Keep
authentication, scope binding, and observation scope independent. Preserve
the exact auth interface: every source declares `none`, `optional`, one required
method, or an ordered list of required alternatives. `none` stands alone and
performs no credential lookup or attachment. `optional` tries conventional
`api_key`, then a registered `cloud_chain`, and makes one unauthenticated call
only when neither is available. Missing methods may fall through; a present
invalid method fails typed. Omitted auth, old `required`/`any_of`/`all_of`
objects, duplicate or empty lists, and `none` inside a list fail exact-current
validation. Source optionality remains independent from auth optionality.
Provider/catalog copies contain only secret-safe credential metadata; resolved
values remain request-scoped and are never stored or copied. Preserve
`cloud_chain` as a built-in provider-inferred method: provider identity selects
one registered AWS/Azure/GCP/OCI official-SDK adapter, YAML does not repeat the
vendor chain, and Starmap does not reimplement SDK chain precedence, refresh,
caching, or workload identity. Treat Cloudflare as API-token plus account-bound,
not cloud-chain, under its current contract. Remove or replace the unused
generic callback sequencer only after production adapter and negative-registry
proof exists. Preserve amended ADR 0001's one-product rule: Catalog is the only
normalized catalog product. Remove CustomerInventory, CatalogOverlay, EffectiveCatalog,
authenticated-delta, and parallel customer store/query paths rather than
deepening or renaming them. When credentials are available, use them and execute
each logical source once after auth selection; permit only its declared bounded
pagination, retry, and protocol requests. Use unauthenticated access only
through `none` or the all-methods-absent fallback of `optional`; never execute
anonymous and authenticated variants for comparison. Normalize every valid
result into ordinary
canonical definitions/offerings and attach global-public, regional-public, or
credential-scoped observation provenance. Public-only authenticated facts may
publish, but any run containing a credential-scoped observation must fail
bootstrap/public generation, evidence, distribution, and remote publication
before writing; never construct an overlay or filter a mixed catalog record by
record. Preserve every current environment-variable name and description at
one typed owner, but never promote models.dev advisory metadata into executable
credentials or bindings.

Run every focused phase gate and then repository-wide short race, vet, pinned
lint, generated-doc, provider-contract, catalog-generation, embedded-budget,
artifact, diff, and full make verify gates. Record live provider checks
separately from deterministic proof and never print secrets. Before external
mutation, inspect active worktrees, branches, draft PRs, and workflow runs; reuse
the existing authorized provider-expansion lane and never duplicate or interfere
with active release/catalog work. Do not merge, publish, release, or deploy
without explicit authorization.

Treat the worktree-cleanup disposition ledger as required implementation scope.
Remove accidental non-code contamination, unsafe arbitrary-origin raw fetch,
the divergent stats parser, shallow global hooks/bypasses and empty adapters,
redundant exact-provider protocol fixtures, duplicate provider-fixture developer
binaries, and custom clients that fail their deletion test. Retain governed raw
responses, operational imports, or provider adapters only with a current
production call graph, concrete architectural rationale, and positive behavior
tests. Rewrite the formerly normative provider implementation contract and
regenerate every affected artifact only after the final schema and source
contract are stable.

Continue autonomously until every PAC0-PAC8 task and every finding in the live
ledger, presently PAF-001 through PAF-040, is DONE or explicitly user-approved
REJECTED and all required local plus authorized hosted evidence is recorded. If
a genuine external or authorization blocker remains after all independent work
is exhausted, mark only the affected rows BLOCKED with exact evidence and a
concrete re-entry condition; otherwise keep working and do not relinquish the
whole-plan goal.
```

## Plan Review Checklist

- [x] Mission covers public unauthenticated, API-key, cloud-chain, alternative
  auth, and credential-scoped models, prices, limits, and deployments in one
  contextual catalog.
- [x] Authentication, source bindings, and observation scope are independent.
- [x] Multiple required credentials are ordered alternatives; compound
  mechanisms own multiple typed inputs behind one named method rather than a
  top-level auth expression.
- [x] Customer-specific pricing, limits, early access, custom models, and
  deployments use the same canonical Catalog under credential-scoped
  provenance; no parallel inventory or overlay product remains in the target.
- [x] Built-in standalone `none`, deterministic `optional`, conventional
  required bearer `api_key`, provider-inferred required `cloud_chain`, and
  ordered required method lists reduce repetition while every source still
  declares authentication and observation scope.
- [x] `none` prohibits credential lookup and attachment; `optional` tries the
  conventional API key, then the registered cloud chain, and falls back to one
  unauthenticated call only when both are absent; present-invalid credentials
  fail typed without fallback.
- [x] `cloud_chain` is a deep provider-registry seam backed by AWS, Azure, GCP,
  and OCI official SDK adapters; vendor chain names stay out of ordinary YAML,
  SDK mechanics are not reimplemented, and Cloudflare remains token/account
  based.
- [x] Credentials are preferred when present, optional missing credentials fall
  back to anonymous, and normal acquisition never fetches both forms or builds
  a delta.
- [x] Invocation-route auth, scope value sources/roles, shared acquisition
  groups, and models.dev advisory metadata ownership are explicit.
- [x] Every proposed removal has a replacement and behavior-proof obligation.
- [x] The plan targets net deletion of bespoke production code and exported
  interfaces; validation/tests may grow only with behavior proof, and every
  retained or added module must pass the deletion test.
- [x] Every phase and task has verifiable success criteria.
- [x] Findings PAF-001 through PAF-040 map to owning tasks and objective closure
  evidence.
- [x] The current PR range and dirty overlay have an explicit retain/revise/remove
  disposition ledger, including compile corruption, raw-origin security,
  stats-parity, exact-provider fixture, custom-client deletion-test, developer
  tooling, and compatibility/import findings.
- [x] Exact-current schema-v2 break and negative old-shape proof are required.
- [x] Fixture policy is derived rather than duplicated.
- [x] Parallel provider pricing/region mini-catalogs are owned for removal into
  normalized source configuration and canonical offering facts.
- [x] A field-level sensitivity matrix resolves contextual catalog retention,
  in-memory copying, durable-store rejection, redaction, isolation, and public
  rejection without inventing a private persistence product.
- [x] Documentation, generated artifacts, hashes, race/static/full gates, and
  hosted evidence are included.
- [x] The `/goal` drives the entire plan rather than one task or phase.
- [x] Existing dirty work, active worktrees/runs, secrets, and external mutation
  boundaries are protected.
