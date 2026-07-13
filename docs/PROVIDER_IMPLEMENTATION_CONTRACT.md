# Provider Implementation Contract

Status: P13.37.2 working contract

This document freezes repository reality before the provider contract
correction. `docs/ADDING_PROVIDERS.md` will become the concise normative user
workflow in P13.37.8; this document retains the complete audit, ownership
decisions, exceptions, and reproducible evidence behind that workflow.

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
5. Production Go owns only irreducible behavior: transport/SDK calls,
   authentication, pagination, response decoding, validation, bounded
   orchestration, customer isolation, and typed unit conversion.
6. A live provider observation contains only records returned by that
   invocation. Embedded or filesystem baseline records remain a separately
   identified last-known-good source and never masquerade as live evidence.
7. Every provider has behavior tests and one refreshable representative raw
   fixture with adjacent integrity/freshness metadata, unless a source-specific
   exception is documented and structurally tested.

## Reproducible inventory commands

Run these commands from the repository root:

```bash
# Provider package production/test shapes.
for directory in internal/providers/*; do
  test -d "$directory" || continue
  find "$directory" -maxdepth 1 -type f -name '*.go' -exec basename {} \; \
    | sort | paste -sd, - | sed "s#^#$(basename "$directory") #"
done

# Configured provider IDs and endpoint types.
awk '/^- id:/{id=$3} /^[[:space:]]+type:/{print id, $2}' \
  internal/embedded/catalog/providers.yaml

# Provider adapters selected behind the shared OpenAI-compatible client.
sed -n '/func openAIProviderOptions/,/^}/p' \
  internal/providers/clients/provider.go

# Regional/account sources registered directly with the catalog pipeline.
sed -n '/srcs := \[\]sources.Source{/,/^\t}/p' \
  internal/catalog/pipeline/sources.go

# Mutable current-value tables and endpoint literals in production provider Go.
rg -n --glob '*.go' --glob '!**/*_test.go' \
  'var .*Prices|var .*Rates|deprecatedModels|commercialRegions|pricing_effective|https?://' \
  internal/providers

# Fixture ownership and update-helper call sites.
find internal/providers -path '*/testdata/models_list.json' -print | sort
rg -n 'SaveTestdata|SaveJSON|CompareWithTestdata|CompareJSONWithTestdata' \
  internal/providers --glob '*.go'
```

The inventory below was read from the current P13 worktree on 2026-07-12. It
classifies roles, not merely filenames.

## Provider module inventory

| Provider package | Acquisition role | Additional role | Fixture state | Required correction |
| --- | --- | --- | --- | --- |
| `anthropic` | Native client | Supplemental source-shape classifier | Versioned fixture | Retain roles; align refresh command |
| `azurefoundry` | Native ARM client | Regional/account source plus live pricing importer | Inline wire fixtures only | Add representative raw fixtures and document source/client split |
| `baseten` | YAML-only shared OpenAI client | Declarative offering defaults and catalog capability/pricing baseline | Versioned fixture | Configuration-only implementation reached; retain provider-local behavior tests and fixture |
| `bedrock` | Native AWS SDK calls embedded in source | Regional/account source plus live pricing importer | Inline SDK fixtures only | Separate SDK client locality where useful; move reviewed region inventory to config/catalog evidence; add raw/equivalent fixture exception |
| `cerebras` | YAML-only shared OpenAI client | Provider-local behavior test | Versioned fixture | Retain; align refresh command |
| `cloudflare` | Native account-scoped client | Live provider pricing normalization | Inline wire fixtures only | Add refreshable representative fixture; move common unit mapping to declarative normalization |
| `cohere` | Native paginated client | Curated Go pricing table | No testdata | Move current prices to provider catalog; add fixture |
| `databricks` | Native documentation/session inventory client | Workspace/customer isolation | Inline fixtures only | Add fixture or evidence-backed exemption |
| `deepseek` | YAML-only shared OpenAI client | Provider-local behavior test | Versioned fixture | Retain; align refresh command |
| `google` | Native AI Studio/Vertex client | Supplemental source-shape classifier | Inline fixtures only | Add representative fixtures or split justified SDK fixture policy |
| `groq` | YAML-only shared OpenAI client | Provider-local field-mapping tests | Versioned fixture | Normalize test filename and refresh contract |
| `huggingface` | Native router inventory client | Live per-upstream pricing normalization | Inline fixtures only | Add fixture; move common units/defaults to config |
| `hyperbolic` | YAML-only shared OpenAI client | Catalog-owned price/lifecycle/capability facts plus declarative offering defaults | Versioned fixture | Configuration-only implementation reached; retain explicit per-model invocation overrides in catalog data |
| `mistral` | Shared OpenAI client plus provider adapter | Native capability decoding plus curated price table | Versioned fixture | Keep only irreducible capability decoding; move prices to provider catalog |
| `moonshot-ai` | YAML-only shared OpenAI client | Provider-local behavior test | Versioned fixture | Normalize test filename and refresh contract |
| `novita` | Shared OpenAI client plus provider adapter | Strict wire validation and live fixed-point pricing conversion | Versioned fixture | Keep validation; declare unit/mode/defaults in config where bounded; remove endpoint literal |
| `nvidia` | Native public catalog client | Optional customer NIM inventory | Inline fixtures only | Add public/customer fixtures or explicit split exemption; move stable defaults to config |
| `oci` | Native OCI SDK adapter | Regional/account source plus curated Go pricing | Inline SDK fixtures only | Move current prices to provider catalog or official importer; add source fixture policy |
| `openai` | Shared OpenAI-compatible client | Generic response/field/pricing normalization and source-shape classifier | Versioned fixture | Deepen declarative config; retain no named provider policy |
| `sambanova` | Native richer OpenAI-compatible client | Live per-token pricing normalization | Inline fixtures only | Add fixture; move common unit mapping/defaults to config |
| `scaleway` | YAML-only shared OpenAI client | Catalog-owned price/lifecycle/capability facts plus declarative region/offering defaults | Versioned fixture | Configuration-only implementation reached; retain explicit non-Chat invocation overrides in catalog data |
| `snowflake` | Native session client | Curated price/credit/effective-date table and regional modes | Inline fixtures only | Move current tables to provider catalog/provenance; retain session protocol behavior |
| `together` | Native multi-inventory client | Live provider pricing and dedicated/serverless modes | Inline fixtures only | Add fixture; move common units/defaults to config where possible |
| `watsonx` | Native paginated client | Optional customer deployment inventory | Inline fixtures only | Add fixture or evidence-backed customer/source exemption |
| `xai` | Shared OpenAI client plus provider adapter | Declarative response collection plus strict live pricing validation/conversion | Versioned fixture | Keep only irreducible model-level provider pricing semantics; the shared client owns configured envelope selection |

Support packages `clients` and `testhelper` are not providers. They own the
client-selection seam and fixture integrity/refresh implementation respectively.

## Registration seams

There are two production registration paths and they must not be conflated:

1. `internal/providers/clients.NewProvider` constructs a `ProviderClient` for a
   configured provider record and calls `ListModels`.
2. `internal/catalog/pipeline.createSourcesWithConfig` registers direct
   `sources.Source` implementations for regional/account observations such as
   Bedrock, Microsoft Foundry, and OCI.

An OpenAI-compatible adapter is selected at the first seam only when YAML and
the shared client cannot yet express the wire semantics. A regional source is
registered at the second seam only when one `ListModels` call cannot represent
the observation, scope, or customer/public separation.

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

Legacy configured providers OpenAI, Anthropic, Google AI Studio/Vertex, Groq,
DeepSeek, Cerebras, Alibaba, DeepInfra, Fireworks, and Moonshot currently rely
on embedded catalog data, provider/model documentation links, fixtures, and the
historical source-schema control plane rather than one provider-specific source
document. P13.37.8 must make the required provenance form explicit and either
route these providers through an existing authoritative record or record an
owned documentation exception; it must not invent duplicate fact values merely
to make the files uniform.

## Confirmed duplicated endpoint authority

| Provider | Configuration authority | Duplicate production literal | Risk |
| --- | --- | --- | --- |
| Baseten | `catalog.endpoint.url` plus `/models` inventory suffix | Removed | Effective acquisition and published offering base derive from the same configuration and override |
| Hyperbolic | `catalog.endpoint.url` plus `/models` inventory suffix | Removed | Effective acquisition and published offering base derive from the same configuration and override |
| Scaleway | `catalog.endpoint.url` plus `/models` inventory suffix | Removed | Effective acquisition, published offering base, deployment, region, and residency derive from configuration |
| Novita | `catalog.endpoint.url` plus `/models` inventory suffix | Removed | Effective acquisition and published offering base derive from the same configuration and override; adapter retains only wire validation/pricing behavior |

P13.37.6 proves both acquisition and published offering projection change
together under each provider's configured base-URL override.

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

The audit remains open to additional tables discovered by the exact searches in
P13.37.5. Values returned in a live payload are not hardcoded facts. Named unit
conversion constants and protocol versions remain eligible Go behavior.

## Fixture reality

Versioned `models_list.json` plus adjacent metadata currently exist for
Anthropic, Baseten, Cerebras, DeepSeek, Groq, Hyperbolic, Mistral, Moonshot,
Novita, OpenAI, Scaleway, and xAI.

The dead test-only save/compare API and `go test -update` promise are removed.
`make testdata PROVIDER=<id>` now invokes one production refresh command that
uses the raw-fetch seam, validates the payload through the registered provider
client, atomically promotes payload and metadata, rejects unchanged/failure/
invalid/secret-bearing responses, and prints only provider ID, byte count, and
checksum. `internal/providers/fixture_policy.yaml` identifies all refreshable
fixtures and every tested source-specific exception. Synthetic mutation
fixtures remain inline only when clearly named; they do not silently claim to
be representative captured evidence.

## Module decision tree

1. Configure identity/authentication/endpoints in provider YAML.
2. If the inventory is standard OpenAI-compatible and all semantics are
   declarative, add no provider production Go; add provider-local behavior tests
   and testdata.
3. If acquisition is shared but bounded wire semantics are not declarative, add
   `adapter.go`/`adapter_test.go`. Delete the adapter when configuration becomes
   expressive enough and the deletion test shows no complexity would escape.
4. If acquisition is incompatible, add `client.go`/`client_test.go` and register
   its endpoint type with the client factory.
5. If the provider requires regional sweeps, multiple control-plane calls, or
   public/customer separation, add `source.go`/`source_test.go` and register it
   with the catalog pipeline. Keep transport/SDK acquisition in `client.go` or
   `sdk.go` when that improves locality.
6. Add `pricing.go` only for a live official pricing importer or reusable parser.
   A current-value table is catalog data, not a pricing implementation.
7. Add `source_shape_test.go` only as supplemental response-field classification;
   it never replaces behavior tests or raw testdata.

## Completion evidence for P13.37.2

- This inventory covers every current provider package and both registration
  seams.
- F-125-F-131 own every confirmed contract gap.
- P13-PC0-P13-PC5 define ordered implementation and verification gates.
- `docs/ADDING_PROVIDERS.md`, AGENTS, architecture documentation, Make targets,
  and structural tests are mandatory before closeout.
- No compatibility promise, publication, release, deployment, or hosted mutation
  is part of this contract.
