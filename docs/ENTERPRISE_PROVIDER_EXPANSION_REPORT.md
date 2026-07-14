# Enterprise provider expansion report

Status: provider-acquisition control-plane finalization; full closeout gates pending

This report describes the enterprise catalog implemented by draft PR #40 plus
the locally verified connector-scope correction that follows its hosted-green
head. It is not release, deployment, publication, or merge evidence. Exact
hosted-check commits, runs, jobs, and the durable task/finding closeout remain in
[`STARPORT_CATALOG_CONTROL_PLANE.md`](STARPORT_CATALOG_CONTROL_PLANE.md).

## Catalog products and counts

The checked-in bootstrap is schema v2 generation
`catalog-20260714T075600Z-2a1242730cc4` with payload
`sha256:2a1242730cc447ac9be83f9b6cce186a271df34883c4f10eac46c47c8ce025aa`,
2,134,894 uncompressed bytes, and 95,133 compressed bytes. The embedded budget
reports 30 configured providers and 746 checked-in provider model files.
Complete exact-schema validation reports 85 authors and 1,121 canonical model
definitions with valid definition/offering cross-references. Provider model
files are source inputs; canonical definitions and provider offerings are the
published product.

Every run has one canonical catalog containing definitions and provider
offerings. A run with any credential-scoped observation is usable in memory but
is structurally rejected from public schema-v2 publication before bytes are
written. Provider applications are separate channels. A definition can
therefore be shared by multiple provider,
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
unit conversion. The executable implementation contract classifies
configuration-only providers, reusable protocol connectors, provider-local
clients and adapters, regional/account sources, and pricing importers. Connector
packages own the deliberately shared OpenAI, Anthropic, and Google protocol
boundaries; provider packages own single-provider acquisition, policy, adapters,
sources, pricing acquisition, and evidence. Every provider has role-appropriate
behavioral tests whose fixture class is derived from source auth, scope,
bindings, topology, connector reuse, and evidenced adapter delta. Connector
fixtures, table-driven composition tests, provider-delta fixtures, governed
global-public observations, and SDK fakes have distinct owners;
credential-scoped raw captures are prohibited.

## Provider and channel disposition

| Channel | Providers | Publication/routing boundary |
| --- | --- | --- |
| Regional cloud control planes | Amazon Bedrock, Microsoft Foundry/Azure OpenAI, Oracle OCI Generative AI | Public regional model and price facts publish; credential-scoped profiles, deployments, endpoints, compartments, and aliases remain contextual catalog facts |
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

## Public, regional, and contextual isolation

Regional offerings retain provider-native region, realm, residency, destination,
cross-region profile, deployment type/tier, and pricing-mode facts. Public
generation never requires an enterprise credential and never serializes a token,
account, organization, project, space, compartment, workspace, deployment,
endpoint, alias, custom model, private network, or served-name mapping.

Optional credential-scoped sources make zero requests unless their exact
credentials and bindings resolve. Every custom, uploaded, external, tuned, or caller-served model requires an
explicit canonical-definition mapping. Deep-copy and race fixtures cover both
public and contextual catalogs so callers cannot mutate shared state.

## Proof classes

| Proof class | Result |
| --- | --- |
| Deterministic fixtures | Native and OpenAI-compatible envelope, pagination, identity, lifecycle, modality, routing, pricing, malformed-record, duplicate, cancellation, retry, unknown-field, and cross-context isolation fixtures pass |
| Live first-party evidence | Secret-safe temporary refreshes succeeded for Cohere, Mistral, Together serverless/dedicated, Hugging Face, and NVIDIA. xAI returned HTTP 403 and is recorded as a live failure rather than success. No key, endpoint query, account value, or raw customer payload is recorded. |
| Explicit live skips | Missing account/region/context configuration is recorded for Azure, Bedrock, Cloudflare, SambaNova, Baseten, Scaleway, Hyperbolic, Novita, IBM, OCI, Snowflake, Databricks, and NIM rather than being called success |
| Schema clean break | Production old-entry-point searches return zero; schema-v1 bootstrap/distribution fixtures fail, explicit schema-v2 definitions/offerings round-trip, and credential-scoped observations fail public writes |
| Provider contract | Declarative configuration round-trips and rejects unsafe paths, units, and defaults before transport; endpoint overrides change acquisition and publication together; mutable provider facts survive credential absence as catalog baseline data; live authoritative pricing wins atomically; stale, partial, or malformed observations retain last-known-good |
| Race and repository | Focused provider, connector, auth, native-source, catalog, store, distribution, remote, and reconciliation races are recorded in the active control plane. Repository-wide short race, lint, vet, generated-doc, artifact, and `make verify` results are not claimed until the final exact worktree passes them. |
| Hosted/protected | The protected base release is green and closed. Existing draft PR #40 is the sole authorized review lane; its hosted exact-head checks must be refreshed after the final local commit. No merge, publication, release, or deployment is authorized. |

## Freshness, quarantine, and pricing age

Source policy distinguishes direct inventory/pricing, regional sweeps,
credential-scoped observations, and curated application evidence. Observations carry attempt/success,
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
  shared model service; caller containers, images, and endpoints belong in a
  credential-scoped catalog observation.
- GitHub Models may re-enter only if GitHub reverses the documented July 30,
  2026 retirement and restores an authoritative supported catalog/inference
  contract. Microsoft Foundry remains the documented successor.
- Any currently skipped contextual source gains live proof when the required
  account/region/scope and explicit canonical mappings are supplied without
weakening public/context isolation.

## Hosted verification and authorization boundary

Draft PR #40 carries the protected exact-head `Verification Gate` and
`Security & Reliability` checks. The durable ledger records the final commit,
workflow run, job IDs, and results after the evidence-only closeout head passes.
The PR remains draft. Publishing, merging, releasing, and deploying remain
separately unauthorized and did not occur.
