# Enterprise provider expansion report

Status: P13.39 complete; protected exact-head draft-PR proof green

This report describes the enterprise catalog implemented by draft PR #40. It is
not release, deployment, publication, or merge evidence. Exact hosted-check
commits, runs, jobs, and the durable task/finding closeout remain in
[`STARPORT_CATALOG_CONTROL_PLANE.md`](STARPORT_CATALOG_CONTROL_PLANE.md).

## Catalog products and counts

The checked-in bootstrap is schema v2 generation
`catalog-20260713T011645Z-3d47e10f17a4` with payload
`sha256:3d47e10f17a4a2f60f63888c808ed93b02bec4e0552ce679ebdf529c2c05db37`,
2,068,393 uncompressed bytes, and 90,992 compressed bytes. The embedded budget
reports 27 configured providers and 707 checked-in provider model files.
Complete exact-schema validation reports 85 authors and 1,074 canonical model
definitions with valid definition/offering cross-references. Provider model
files are source inputs; canonical definitions and provider offerings are the
published product.

The public generation contains canonical definitions and provider offerings.
Credential-scoped deployments are a different `CustomerInventory` product and
are structurally rejected from schema-v2 publication. Provider applications are
separate channels. A definition can therefore be shared by multiple provider,
regional, deployment-mode, aggregator, or application offerings without being
duplicated as a model.

## Provider implementation contract

Catalog schema v2 is the sole normalized fact store. Manually curated
last-known-good lifecycle, capability, region, routing, and pricing facts live in
provider catalog YAML; facts returned by an official live source enter as typed
observations and reconcile into the same definitions and offerings. Embedded,
filesystem, and remote generations are durable representations of that catalog,
not separate evidence schemas. Source evidence records provenance, observation
time, revision, scope, and validation state without copying mutable fact values.

Provider configuration owns stable acquisition and interpretation: identity,
authentication shape, inventory endpoint, response collection, field and unit
mappings, offering defaults, endpoint derivation, and runtime overrides. Go code
owns only irreducible protocol behavior such as SDK calls, pagination,
authentication, response decoding, validation, bounded orchestration, and typed
unit conversion. The executable provider contract classifies configuration-only
shared clients, provider adapters, native clients, regional/account sources, and
pricing importers; every provider package has contract-appropriate behavioral
tests and either an integrity-bound refreshable raw fixture or a concrete
source-specific fixture rationale.

## Provider and channel disposition

| Channel | Providers | Publication/routing boundary |
| --- | --- | --- |
| Regional cloud control planes | Amazon Bedrock, Microsoft Foundry/Azure OpenAI, Oracle OCI Generative AI | Public regional model and price facts publish; credential-scoped profiles, deployments, endpoints, compartments, and aliases remain customer inventory |
| Direct model authors | Mistral AI, xAI, Cohere | Author identity remains separate from the provider offering; only documented invocation contracts and prices route |
| Aggregators/routing platforms | Together AI, Hugging Face Inference Providers | Underlying author/provider identity is preserved; aggregator policy and provider selection are not model identities |
| Hosted and customer deployment channels | NVIDIA API Catalog/NIM, Databricks Mosaic AI, Snowflake Cortex AI, IBM watsonx.ai | Public or session-visible foundations remain separate from customer NIM, workspace endpoint, deployment, account, project, space, and alias facts |
| Shared serverless providers | Cloudflare Workers AI, SambaNova Cloud, Baseten Model APIs, Scaleway Generative APIs, Hyperbolic, Novita AI | Exact public model IDs, prices, lifecycle, regions, and invocation contracts publish; account IDs, dedicated endpoints, custom weights, clusters, private networks, and customer aliases do not |
| Application-only | Cursor Composer | Definitions and application variants publish as non-routable application offerings; no server inference endpoint is invented |
| Terminal direct-source rejection | Replicate, fal, Nebius AI Studio, GitHub Models | No direct enterprise LLM source is created under the evidence and re-entry conditions below |

Routability is an offering fact, not a provider-wide assumption. NVIDIA public
records that do not identify an invocation family, Databricks foundations whose
URL is workspace-specific, unknown provider task/modality records, and every
application-only record remain discoverable but ineligible for routing.

## Public, regional, and customer isolation

Regional offerings retain provider-native region, realm, residency, destination,
cross-region profile, deployment type/tier, and pricing-mode facts. Public
generation never requires an enterprise credential and never serializes a token,
account, organization, project, space, compartment, workspace, deployment,
endpoint, alias, custom model, private network, or served-name mapping.

Optional customer readers make zero private calls unless explicitly configured.
Every custom, uploaded, external, tuned, or customer-served model requires an
explicit canonical-definition mapping. Deep-copy and race fixtures cover both
public catalogs and customer inventories so callers cannot mutate shared state.

## Proof classes

| Proof class | Result |
| --- | --- |
| Deterministic fixtures | Native and OpenAI-compatible envelope, pagination, identity, lifecycle, modality, routing, pricing, malformed-record, duplicate, cancellation, retry, unknown-field, and customer-isolation fixtures pass |
| Live first-party evidence | Bedrock public pricing, NVIDIA public inventory including a credential-configured isolated 121-record repeat, Hugging Face routing metrics, Together, Mistral, xAI, and Cohere evidence are recorded in their source reports and ledger entries; no secret value is recorded |
| Explicit live skips | Missing account/region/customer configuration is recorded for Azure, customer Bedrock, Cloudflare, SambaNova, Baseten, Scaleway, Hyperbolic, Novita, IBM, OCI, Snowflake, Databricks, and customer NIM rather than being called success |
| Schema clean break | Production compatibility-entry searches return zero; schema-v1 bootstrap/distribution fixtures fail, explicit schema-v2 definitions/offerings round-trip, and customer fields remain structurally absent |
| Provider contract | Declarative configuration round-trips and rejects unsafe paths, units, and defaults before transport; endpoint overrides change acquisition and publication together; mutable provider facts survive credential absence as catalog baseline data; live authoritative pricing wins atomically; stale, partial, or malformed observations retain last-known-good |
| Race and repository | Provider-focused race matrices and the exact final repository-wide short race pass; final `make verify` passes unit, race, vet, performance, lint, coverage, docs, diff, build, catalog validation, credential-free listing, and CLI smoke gates |
| Hosted/protected | The base release is green and closed. The provider-expansion tree has no commit or protected checks because stage/commit/push/PR actions were not authorized; this is the only remaining proof class |

## Freshness, quarantine, and pricing age

Source policy distinguishes direct inventory/pricing, regional sweeps, customer
inventory, and curated application evidence. Observations carry attempt/success,
degradation, rejection count, source age, and pricing observation time. Bounded
pagination and retry fail closed. Malformed records are quarantined when the
source contract permits record isolation; source-wide corruption preserves the
last-known-good catalog instead of publishing a partial replacement.

Prices are retained only with an exact provider/model/region/mode/unit/currency
mapping. Unknown, non-token, ambiguous, or stale price units remain absent or
source evidence rather than being converted speculatively. Each provider report
identifies its pricing authority and any live or document observation date.

## Re-entry conditions

- Replicate may re-enter as a direct source only when it publishes a stable,
  typed enterprise LLM inventory and normalized pricing contract distinct from
  arbitrary community model schemas; selected existing records can remain
  ordinary external enrichment.
- fal may re-enter only when it publishes a bounded LLM catalog with stable
  invocation schemas and comparable price units distinct from its media endpoint
  marketplace.
- Nebius may re-enter public coverage only when it operates an authoritative
  shared model service; customer containers, images, and endpoints belong in
  private deployment inventory.
- GitHub Models may re-enter only if GitHub reverses the documented July 30,
  2026 retirement and restores an authoritative supported catalog/inference
  contract. Microsoft Foundry remains the documented successor.
- Any currently skipped customer reader gains live proof when the required
  account/region/scope and explicit canonical mappings are supplied without
  weakening public/customer isolation.

## Hosted verification and authorization boundary

Draft PR #40 carries the protected exact-head `Verification Gate` and
`Security & Reliability` checks. The durable ledger records the final commit,
workflow run, job IDs, and results after the evidence-only closeout head passes.
The PR remains draft. Publishing, merging, releasing, and deploying remain
separately unauthorized and did not occur.
