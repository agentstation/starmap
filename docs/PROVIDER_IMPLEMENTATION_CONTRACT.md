# Provider Implementation Contract

Status: current schema-v2 normative contract

This document records the detailed ownership audit behind the concise normative
workflow in `docs/ADDING_PROVIDERS.md`. It describes current repository reality,
not a preserved prelaunch interface.

## Non-negotiable ownership rules

1. Catalog schema v2 definitions and offerings are the sole normalized fact
   store. Embedded, filesystem, and remote generations are representations of
   that same catalog, not separate fact schemas.
2. A manually reviewed provider price, lifecycle, capability, region, or route
   belongs in canonical provider catalog YAML. A fact returned by an official
   live source belongs in that source's typed observation.
3. Provenance records source URL, retrieval/effective time, revision, scope,
   and validation state. It references catalog facts and never duplicates their
   values.
4. Provider configuration owns stable acquisition and interpretation: identity,
   authentication, inventory endpoint, response collection, field/unit mapping,
   offering endpoint/defaults, invocation contract, and runtime override.
5. `internal/connectors` owns deliberately reusable protocol behavior.
   `internal/providers` owns provider-specific acquisition, adapters, bounded
   regional orchestration, context isolation, pricing importers, and evidence.
6. A live provider observation contains only records returned by that
   invocation. Embedded or filesystem baseline records remain a separately
   identified last-known-good source and never masquerade as live evidence.
7. Connector protocol fixtures, provider-delta fixtures, and governed raw
   observations are different artifacts. Deterministic testdata has no capture
   metadata. Genuine replay/import observations live under
   `internal/providers/fixtures/responses/<id>/<source>` with verified metadata.

## Reproducible inventory commands

Run these commands from the repository root:

```bash
# Connector and provider package production/test shapes.
for directory in internal/connectors/* internal/providers/*; do
  test -d "$directory" || continue
  find "$directory" -maxdepth 1 -type f -name '*.go' -exec basename {} \; \
    | sort | paste -sd, - | sed "s#^#$(basename "$directory") #"
done

# Configured provider IDs and endpoint types.
awk '/^- id:/{id=$3} /^[[:space:]]+type:/{print id, $2}' \
  internal/embedded/catalog/providers.yaml

# Provider adapters selected behind the shared OpenAI-compatible client.
sed -n '/func openAIProviderOptions/,/^}/p' \
  internal/providers/registry/provider.go

# Configuration-driven logical-source construction and native adapters.
rg -n 'NewConfigured|configuredConnectorSource' internal/sources/providers
rg -n 'sourceFactory|NewSource' internal/sources/nativeproviders

# Mutable current-value tables and endpoint literals in production provider Go.
rg -n --glob '*.go' --glob '!**/*_test.go' \
  'var .*Prices|var .*Rates|deprecatedModels|commercialRegions|pricing_effective|https?://' \
  internal/connectors internal/providers

# Deterministic provider testdata and governed observations.
find internal/providers -path '*/testdata/models_list.json' -print | sort
find internal/providers/fixtures/responses -mindepth 3 -maxdepth 3 -type f -print | sort
rg -n 'SaveTestdata|SaveJSON|CompareWithTestdata|CompareJSONWithTestdata' \
  internal/providers --glob '*.go'
```

The inventory below classifies current roles, not merely filenames.

## Provider implementation inventory

| Provider identity | Acquisition role | Additional role | Fixture state | Required correction |
| --- | --- | --- | --- | --- |
| `alibaba` | YAML-only exact OpenAI contract | Declarative author mapping | Table-driven composition fixture | Retain embedded-config registry test |
| `anthropic` | Dedicated Anthropic connector | Supplemental response-schema classifier | Deterministic connector fixture plus governed observation | Keep protocol behavior local; verify governed metadata before replay/import |
| `azurefoundry` | ARM acquisition local to the regional source | Regional/account source plus live pricing importer | Inline wire fixtures only | Retain source-local client because its account/realm types and orchestration are not reusable outside this source |
| `baseten` | YAML-only exact OpenAI connector | Declarative offering defaults and catalog capability/pricing baseline | Table-driven composition fixture | Shared connector owns response decoding |
| `bedrock` | AWS SDK acquisition local to the regional source | Regional/account source plus live pricing importer | SDK fakes | Retain source-local SDK seam because it implements credential-scoped regional observations, not a reusable catalog endpoint connector |
| `cerebras` | YAML-only declarative OpenAI extension | Public OpenRouter-format fields and explicit per-token units | Deterministic provider-delta fixture | Retain embedded-config registry test; no connector or adapter |
| `cloudflare` | Provider-local account-scoped client | Live provider pricing normalization | Inline deterministic wire fixtures | Derived account-scoped fixture class; raw credential-scoped captures prohibited |
| `cohere` | Provider-local paginated client | Catalog-owned pricing | Inline deterministic wire fixtures | Governed raw capture only for a stable global-public source |
| `databricks` | Provider-local documentation/session client | Workspace context isolation | Inline deterministic fixtures | Public source uses no auth; workspace raw captures prohibited |
| `deepinfra` | YAML-only declarative OpenAI extension | Metadata, limits, tags, and explicit-unit pricing mappings | Deterministic provider-delta fixture | Retain embedded-config registry test; no adapter |
| `deepseek` | YAML-only exact OpenAI contract | Composition and negative-drift test | Table-driven composition fixture | Add an adapter only for an evidenced irreducible delta |
| `fireworks-ai` | YAML-only declarative OpenAI extension | Capability and named-extension mappings | Deterministic provider-delta fixture | Retain embedded-config registry test; no adapter |
| `google` | Dedicated AI Studio/Vertex connector | Supplemental response-schema classifier | Connector fixture and SDK fakes | Class derives from connector reuse and cloud-chain topology |
| `groq` | YAML-only declarative OpenAI extension | Provider-local lifecycle, limit, and author mappings | Deterministic provider-delta fixture | Retain embedded-config registry test |
| `huggingface` | Provider-local router-inventory client | Live per-upstream pricing normalization | Inline deterministic wire fixtures | Provider-local decoder behavior justifies local fixtures |
| `hyperbolic` | YAML-only exact OpenAI contract | Catalog-owned price/lifecycle/capability facts plus declarative offering defaults | Table-driven composition fixture | Retain explicit per-model invocation overrides in catalog data |
| `mistral` | YAML-only OpenAI connector | Declarative capability projection | Provider composition fixture | Adapter deleted; YAML owns projection and catalog owns reviewed facts |
| `moonshot-ai` | YAML-only exact OpenAI contract | Embedded identity and composition test | Table-driven composition fixture | Add declarative mappings or an adapter only when evidence proves a delta |
| `novita` | OpenAI-compatible connector plus provider adapter | Strict record validation; declarative fixed-point pricing and batch scale | Deterministic provider-delta fixture | Retain validation only; YAML owns field, unit, extension, and mode projection |
| `nvidia` | Provider-local public-catalog client | Optional credential-scoped NIM inventory | Inline deterministic fixtures | Retain cross-context isolation; raw credential-scoped captures prohibited |
| `oci` | OCI SDK acquisition local to the regional source | Regional/account source plus catalog-backed pricing | SDK fakes | Retain source-local SDK seam because its region/realm/context orchestration is not reusable connector behavior |
| `openai` | Reusable OpenAI-compatible connector | Generic response/field/pricing normalization and response-schema classifier | Deterministic connector fixture plus governed observation | Keep protocol behavior local and provider-neutral; verify governed metadata before replay/import |
| `sambanova` | YAML-only OpenAI connector | Declarative richer fields and per-token pricing | Provider composition fixture | Custom client deleted; shared connector validates the configured delta |
| `scaleway` | YAML-only exact OpenAI contract | Catalog-owned price/lifecycle/capability facts plus declarative region/offering defaults | Table-driven composition fixture | Retain explicit non-Chat invocation overrides in catalog data |
| `snowflake` | Provider-local session client | Catalog-backed pricing and regional modes | Inline deterministic fixtures | Session protocol justifies local fixtures; facts stay canonical |
| `together` | Provider-local multi-inventory client | Live provider pricing and dedicated/serverless modes | Inline deterministic fixtures | Each logical source is independently bounded and observed |
| `watsonx` | Provider-local paginated client | Optional credential-scoped deployment inventory | Inline deterministic fixtures | Raw credential-scoped captures prohibited |
| `xai` | OpenAI-compatible connector plus provider adapter | Declarative response collection plus strict live pricing validation/conversion | Deterministic provider-delta fixture | Retain only irreducible model-level provider pricing semantics; the connector owns configured envelope selection |

`internal/providers/registry` is a provider composition module. It composes
endpoint-type protocol connectors with provider-owned clients and adapters.
`internal/providers/fixtures` owns deterministic provider test helpers;
`internal/providers/fixtures/responses` owns governed observation integrity,
refresh, and replay/import verification.

## Registration seams

There are two production composition paths and they must not be conflated:

1. `internal/providers/registry.New` constructs a `Connector` for a configured
   provider record and calls `ListModels`.
2. `internal/sources/providers.NewConfigured` constructs one observation source
   per configured logical source and delegates official-SDK endpoint types to
   `internal/sources/nativeproviders`.

An OpenAI-compatible adapter is selected at the first seam only when YAML and
the shared client cannot yet express the wire semantics. A regional source is
owned at the second seam only when one connector call cannot represent the
bounded regional, account, or workspace execution.

## Source-evidence paths

The enterprise expansion providers bind primary-source decisions in these
checked-in documents:

| Provider scope | Evidence path |
| --- | --- |
| Amazon Bedrock | `docs/AMAZON_BEDROCK_SOURCE.md` |
| Microsoft Foundry/Azure OpenAI | `docs/MICROSOFT_FOUNDRY_SOURCE.md` |
| Mistral | `docs/MISTRAL_AI_SOURCE.md` |
| xAI | `docs/XAI_SOURCE.md` |
| Cohere | `docs/COHERE_SOURCE.md` |
| Cursor | `docs/CURSOR_COMPOSER_SOURCE.md` |
| Together | `docs/TOGETHER_AI_SOURCE.md` |
| Hugging Face | `docs/HUGGING_FACE_INFERENCE_PROVIDERS_SOURCE.md` |
| NVIDIA | `docs/NVIDIA_NIM_SOURCE.md` |
| Databricks | `docs/DATABRICKS_MODEL_SERVING_SOURCE.md` |
| Snowflake | `docs/SNOWFLAKE_CORTEX_SOURCE.md` |
| watsonx | `docs/WATSONX_SOURCE.md` |
| OCI | `docs/ORACLE_OCI_GENERATIVE_AI_SOURCE.md` |
| Cloudflare | `docs/CLOUDFLARE_WORKERS_AI_SOURCE.md` |
| SambaNova | `docs/SAMBANOVA_CLOUD_SOURCE.md` |
| Baseten | `docs/BASETEN_MODEL_APIS_SOURCE.md` |
| Scaleway | `docs/SCALEWAY_GENERATIVE_APIS_SOURCE.md` |
| Hyperbolic | `docs/HYPERBOLIC_SOURCE.md` |
| Novita | `docs/NOVITA_LLM_SOURCE.md` |

Established configured providers OpenAI, Anthropic, Google AI Studio/Vertex, Groq,
DeepSeek, Cerebras, Alibaba, DeepInfra, Fireworks, and Moonshot currently rely
on embedded catalog data, provider/model documentation links, fixtures, and the
historical source-schema control plane rather than one provider-specific source
document. Their configured source documentation and governed observation
metadata supply the required provenance without a duplicate provider dossier;
the project must not invent duplicate fact values merely
to make the files uniform.

## Confirmed duplicated endpoint authority

| Provider | Removed prelaunch shape | Current authority | Proof |
| --- | --- | --- | --- |
| Baseten | singular catalog endpoint plus client literal | `catalog.sources[].endpoint` | Acquisition and offering projection share the configured override |
| Hyperbolic | singular catalog endpoint plus client literal | `catalog.sources[].endpoint` | Acquisition and offering projection share the configured override |
| Scaleway | singular catalog endpoint plus client literal | `catalog.sources[].endpoint` | Endpoint, deployment, region, and residency derive from current configuration |
| Novita | singular catalog endpoint plus adapter literal | `catalog.sources[].endpoint` | Adapter retains only validation/conversion behavior |

Focused provider-contract tests prove both acquisition and published offering
projection change together under each provider's configured base-URL override.

## Confirmed mutable Go fact authority

| Provider | Mutable facts currently in Go | Target authority |
| --- | --- | --- |
| Mistral | Per-model input/output prices | Canonical provider catalog YAML plus non-duplicating provenance |
| Cohere | Per-model input/output prices | Canonical provider catalog YAML plus non-duplicating provenance |
| Scaleway | Prices, deprecated IDs, capability/route patterns, region/residency, batch discount | Catalog facts and provider configuration; retain code only for irreducible decoding |
| Hyperbolic | Prices, tool support, deprecated IDs, Chat/Completions selection | Canonical provider catalog YAML and declarative offering config |
| OCI | Exact per-model/token-tier prices | Canonical provider catalog YAML or bounded official live importer |
| Snowflake | Credit rates, USD conversion values, effective date, routing modes | Canonical provider catalog YAML/provenance plus stable session interpretation config |
| Bedrock | Reviewed commercial/GovCloud region inventory | Provider source configuration or catalog evidence with review revision |

Repository searches cover additional tables. Values returned in a live payload
are not hardcoded facts. Named unit
conversion constants and protocol versions remain eligible Go behavior.

## Fixture reality

Connector-local deterministic fixtures cover reusable OpenAI and Anthropic
protocol behavior. Table-driven tests cover exact configuration-only providers;
provider-local deterministic fixtures cover only evidenced fields or adapters.
Governed OpenAI and Anthropic raw observations are retained separately by exact
logical source for replay/import; synthetic fixtures carry no observation
metadata.

The dead test-only save/compare API and `go test -update` promise are removed.
`make testdata PROVIDER=<id> SOURCE=<source>` invokes one production refresh command that
uses the raw-fetch seam, validates the payload through the registered provider
client, atomically promotes payload and metadata into the governed observation
store, rejects unchanged/failure/invalid/secret-bearing responses, and
prints only safe provider/source identity, byte count, and checksum. Replay and
import verify that pair before decoding. Fixture class is derived exhaustively
from source auth, scope, bindings, topology, connector reuse, and evidenced
adapter delta; there is no parallel policy registry.

## Wire and policy edge cases

- The shared OpenAI connector rejects duplicate model identities unless the
  complete raw records are byte-identical. The Mistral models endpoint has been
  observed returning exact repeated records, so exact duplicates coalesce once;
  a same-ID record with any different byte fails the whole response. This is a
  protocol-normalization rule with positive and conflicting-duplicate tests,
  not provider-specific compatibility decoding.
- A connector may derive a bounded capture URL from its configured logical
  source when the official inventory contract requires a query. Cohere owns its
  page-size query and Together's dedicated source owns `dedicated=true`.
  Callers still select only provider/source identity, never an arbitrary URL;
  source credentials cannot be redirected or disclosed in result metadata.
- NVIDIA's decoder owns only wire normalization. The two configured logical
  sources own publication and offering policy: the public API Catalog is
  global/discoverable and the authenticated NIM inventory is
  credential-scoped/routable. No connector hard-codes those outcomes.
- Official pricing service URLs and SDK service/realm endpoints retained in Go
  are protocol invariants of the minimal provider adapter. User-selectable
  inventory endpoints, regions, realms, and offering defaults remain in typed
  provider configuration.

## Module decision tree

1. Configure identity/authentication/endpoints in provider YAML.
2. If an existing connector and declarative configuration cover the inventory,
   add no provider production Go; add a table-driven configuration/composition
   case and provider-local data only for an evidenced declarative delta.
3. If acquisition is shared but bounded wire semantics are not declarative, add
   `adapter.go`/`adapter_test.go`. Delete the adapter when configuration becomes
   expressive enough and the deletion test shows no complexity would escape.
4. If acquisition is provider-specific, add
   `internal/providers/<id>/client.go`/`client_test.go` and register its provider
   ID with the provider registry. Create
   `internal/connectors/<protocol>/client.go` only when the protocol is a stable,
   reusable boundary across providers; register that connector by endpoint type.
5. If the provider requires regional sweeps, multiple control-plane calls, or
   credential-scoped context, add provider `source.go`/`source_test.go`; the
   configured source factory composes it into the pipeline. Extract transport/SDK acquisition to
   a connector only when it forms a reusable outbound boundary.
6. Add `pricing.go` only for a live official pricing importer or reusable parser.
   A current-value table is catalog data, not a pricing implementation.
7. Add connector `response_schema_test.go` only as supplemental response-field
   classification; it never replaces behavior tests or provider evidence.

## Contract verification

- This inventory covers every current provider package and both registration
  seams.
- The active provider-acquisition control plane owns every confirmed contract
  gap and ordered verification gate.
- `docs/ADDING_PROVIDERS.md`, AGENTS, architecture documentation, Make targets,
  and structural tests are mandatory before closeout.
- No prelaunch shape promise, release, deployment, or hosted mutation is part of
  this contract.
