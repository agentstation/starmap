# ADR 0001: Enterprise provider offering boundaries

Status: Accepted

Date: 2026-07-12

## Context

The existing definition/offering split correctly prevents equal model IDs at
two providers from overwriting each other. The original offering record still
uses a flat region string list and cannot state whether access is a supported
server-to-server API, an application-only channel, a cloud deployment type, a
cross-region profile, or customer-specific inventory. Adding Bedrock, Foundry,
Cursor, Hugging Face, NIM, Databricks, and Snowflake without strengthening that
boundary would recreate the flattened model/provider defect at a larger scale.

## Decision

Starmap uses the following distinct concepts and identities:

| Concept | Identity | Owner |
| --- | --- | --- |
| Model author | `AuthorID` | Public catalog |
| Canonical model definition | `ModelDefinitionID` | Public catalog |
| Inference provider | `ProviderID` | Public catalog |
| Provider-specific offering | `(ProviderID, ProviderModelID)` | Public catalog |
| Cloud region | provider-scoped region code plus realm | Offering geography |
| Geographic/data-residency boundary | stable boundary ID and kind | Offering geography |
| Provider deployment type/service tier | typed deployment contract | Offering |
| Cross-region inference profile | provider profile ID plus source/destination regions | Offering |
| Customer-specific deployment | provider plus account-scope deployment ID | Customer inventory only |
| Router/aggregator offering | provider offering plus optional upstream offering key | Public offering |
| Application-only access channel | typed access channel | Public discoverability |
| Routable versus discoverable-only | typed routability | Offering eligibility |
| Public versus credential-scoped inventory | distinct public catalog and customer inventory products | Publication boundary |

A canonical definition is never duplicated merely because it is sold by
Anthropic, Bedrock, Foundry, Vertex, Snowflake, or an aggregator. Those channels
produce offerings linked to the same definition. Provider facts remain absent
from `ModelDefinition`.

`ProviderOffering` owns access, routability, structured geography, deployment,
profiles, pricing, lifecycle, endpoint behavior, and optional aggregator
upstream identity. An application-only offering must be discoverable-only and
has no invocation API. A route alias can materialize only an explicitly
routable server-to-server offering.

`CustomerInventory` is a separate value product and is not stored by
`catalogs.Catalog`. It may contain account/project deployment IDs, aliases, and
private endpoints for an operator, but none of those fields have a path into a
public generation. Resolved credentials and tokens are runtime-only values with
no JSON or YAML representation.

## Source and authority rules

| Attribute family | Primary authority | Fallback behavior |
| --- | --- | --- |
| Definition name/family/lineage/context/modalities/intrinsic capabilities | Model author source | Reviewed curated evidence, then models.dev enrichment |
| Provider model ID/availability/lifecycle/invocation/deployment/region/provider capability | Live provider API | Last-known-good offering with explicit degradation |
| Offering price/effective interval/currency/unit/cache/batch dimensions | Provider official pricing source | Retain last-known-good; models.dev only when provider price is absent or rejected |
| Cloud region/residency/tier/profile/cloud price | Cloud provider | No cross-region inference from a different region's observation |
| Customer deployment/alias/quota/enabled access | Credential-scoped customer API | No public fallback and no embedded publication |
| Routing preference, `fastest`, `cheapest`, or `preferred` | Starport policy | Never ingested as model or offering identity |

## Source framework

Provider calls use bounded operation timeouts, cursor pagination with repeated
cursor/page/record limits, and bounded transient retry with jitter and
`Retry-After`. Authentication/configuration errors and HTTP 401, 403, 404, and
409 are terminal for the current operation. HTTP 429 and 5xx are retryable
within the operation budget. Context cancellation always wins.

Cloud SDK adapters receive a typed runtime credential chain. The chain can use
environment, workload identity, managed identity, shared configuration, CLI, or
role/session providers as supported by the official SDK. Starmap stores only a
secret-free provider/source label for diagnostics.

## Consequences

- Because Starmap is prelaunch, schema v2 is a direct clean break. Readers
  accept only exact schema v2 with explicit definitions and offerings; schema
  v1 is neither decoded nor migrated on read.
- Provider-specific source adapters become simpler because geography,
  deployment, profile, access, and scope have canonical homes.
- Customer inventory requires a separate storage and access-control decision in
  provider waves; it cannot silently piggyback on embedded generation.
- Cursor/Composer can be represented honestly without inventing an OpenAI API.
- Starport routing can fail closed by consulting one explicit routability field.
