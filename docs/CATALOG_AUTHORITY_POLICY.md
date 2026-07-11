# Catalog Authority Policy

Starmap reconciles two canonical model resource types:

- `ModelDefinition` contains provider-independent identity, authorship, lineage,
  weights, and intrinsic capabilities.
- `ProviderOffering` contains one provider's price, limits, availability,
  regions, endpoint behavior, lifecycle, and named service modes.

The executable source of truth is `pkg/authority.CanonicalPolicies`. A
reflection gate walks every exported canonical schema leaf and fails whenever a
new field is not covered by a policy containing an authority order, merge
policy, empty policy, and rationale.

## Policy semantics

Authority order is highest to lowest. HTTP precedes Git when both represent the
same models.dev dataset; Git remains an explicit verification transport.

| Merge policy | Meaning |
| --- | --- |
| `identity` | One stable value; disagreement is invalid rather than merged |
| `replace` | Select one complete validated value; do not synthesize subfields |
| `fill_missing` | The winner remains authoritative and lower sources fill only absent leaves |
| `set_union` | Add unique values in authority order |
| `deep_merge` | Merge named records/maps, applying leaf policy recursively |

| Empty policy | Meaning |
| --- | --- |
| `reject` | Empty is invalid for the required field |
| `absent` | Nil/empty container means no claim and permits fallback |
| `authoritative` | Explicit zero or `false` is evidence and cannot be replaced as if missing |

## Model definition inventory

| Attribute family | Authority order | Merge | Empty | Rationale |
| --- | --- | --- | --- | --- |
| `ID` | local, models.dev HTTP, models.dev Git, provider | identity | reject | Definition identity is curated and provider-independent |
| `Name` | local, models.dev HTTP, models.dev Git, provider | replace | reject | Stable display names should not follow provider branding drift |
| `AuthorIDs` | local, models.dev HTTP, models.dev Git, provider | set union | absent | Curated authorship leads; discoveries may add unique evidence |
| `Description` | local, models.dev HTTP, models.dev Git, provider | replace | absent | Human-reviewed catalog copy leads |
| `Metadata.*` | local, models.dev HTTP, models.dev Git, provider | fill missing | absent | Release, cutoff, and tags are definition facts |
| `Lineage.*` | local, models.dev HTTP, models.dev Git, provider | fill missing | absent | Family and derivation are definition identity facts |
| `Weights.Open` | local, models.dev HTTP, models.dev Git, provider | replace | authoritative | Explicit `false` is meaningful |
| `Weights.Architecture.*` | local, models.dev HTTP, models.dev Git, provider | fill missing | absent | Architecture is provider-independent |
| `Capabilities.Features.*` | provider, models.dev HTTP, models.dev Git, local | fill missing | authoritative | Explicit provider `false` must survive |
| Other `Capabilities.*` | provider, models.dev HTTP, models.dev Git, local | fill missing | absent | Provider evidence leads optional intrinsic capability details |
| `CreatedAt`, `UpdatedAt` | local, models.dev HTTP, models.dev Git, provider | replace | absent | Record times follow the winner, not ingestion time |

## Provider offering inventory

| Attribute family | Authority order | Merge | Empty | Rationale |
| --- | --- | --- | --- | --- |
| `ProviderID`, `ProviderModelID` | provider, models.dev HTTP, models.dev Git, local | identity | reject | Exact provider-scoped service identity |
| `DefinitionID` | local, models.dev HTTP, models.dev Git, provider | identity | reject | Canonical resolution is curated separately from provider naming |
| `Pricing.*` | provider, models.dev HTTP, models.dev Git, local | replace | absent | Valid provider price leads and is selected atomically |
| `Limits.*` | provider, models.dev HTTP, models.dev Git, local | fill missing | absent | Provider limits lead; lower sources fill only missing dimensions |
| `Availability` | provider, models.dev HTTP, models.dev Git, local | replace | reject | Current provider observation is authoritative |
| `Regions` | provider, models.dev HTTP, models.dev Git, local | set union | absent | Provider regions lead; documented unique regions may be added |
| `Endpoint.*` | provider, models.dev HTTP, models.dev Git, local | fill missing | absent | Provider behavior leads; catalog config may fill connection details |
| `Lifecycle` | provider, models.dev HTTP, models.dev Git, local | replace | reject | Lifecycle belongs to the specific offering |
| `Modes.*` | provider, models.dev HTTP, models.dev Git, local | deep merge | absent | Named modes merge while leaf authority remains provider-first |

Pricing is an atomic offering fact. A provider observation wins only when it
passes `ModelPricing.Validate` and is effective at the reconciliation instant.
Selection then deep-copies that complete value; rejected provider price does
not partially contaminate a models.dev fallback.

Pricing semantics are explicit:

- currency is a three-letter uppercase code;
- token units are `per_token` and `per_1m`, and both must describe the same
  amount when supplied together;
- a non-nil cost pointer containing zero values is an explicit free price,
  while a nil pointer is missing evidence;
- all monetary values must be finite and non-negative;
- context tiers have a positive, unique threshold and at least one price; and
- `effective_from` and `effective_until` form an optional half-open interval.

Malformed, future, or expired observations are rejected at the pricing-field
boundary so the rest of the offering remains usable and the next source can
provide fallback evidence. Routing preference is not an authority input and
remains owned by Starport.
