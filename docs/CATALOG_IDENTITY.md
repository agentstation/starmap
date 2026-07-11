# Catalog Identity Contract

Starmap separates model identity from provider service identity and Starport
routing identity. These identifiers are not interchangeable.

| Term | Canonical meaning | Uniqueness and mutability |
| --- | --- | --- |
| Definition ID | Starmap-owned ID for one provider-independent model definition | Globally unique within a catalog schema; stable across providers and price changes |
| Provider ID | Canonical Starmap ID for one inference provider | Globally unique; provider aliases may resolve to it |
| Provider model ID | Exact provider-facing model identifier sent on that provider's API | Unique only within a provider; opaque and never normalized for routing |
| Offering key | Ordered pair `(Provider ID, Provider model ID)` | Globally unique; the durable identity of one provider service offering |
| Author ID | Canonical organization or author responsible for a definition | Globally unique; authorship does not imply provider availability |
| Entity alias | Alternate spelling for the same provider, author, or definition | Resolves to exactly one canonical entity; cannot encode policy or fan out |
| Route alias | Starport-facing routing name that selects eligible offering keys | Unique in a routing configuration; may fan out and change eligibility without changing catalog identity |

## Invariants

1. A model definition owns provider-independent facts: authorship, family,
   lineage, weights, and intrinsic capabilities.
2. An offering owns provider facts: provider model ID, price, limits,
   availability, regions, lifecycle at that provider, endpoint behavior, and
   request overrides.
3. Two providers may expose the same provider model ID. Their offering keys are
   still distinct and neither may overwrite the other.
4. One provider may expose multiple provider model IDs for one definition;
   those are distinct offerings linked to the same definition ID.
5. Provider model IDs are opaque. Slashes, dates, namespaces, and case are data,
   not separators or normalization instructions.
6. Aliases are identity equivalence only. An alias resolves to one canonical
   entity and cannot choose among offerings.
7. Route aliases are materialized above ingestion. Sources report observations;
   they do not decide routing eligibility, weights, fallback, tenancy, or policy.
8. Every offering references exactly one existing provider and one existing
   definition. Every route target references an existing offering key.

## Compatibility boundary

The current `catalogs.Model.ID`, provider `Models` map key, `Models`, and
`FindModel` APIs predate this split and remain compatibility surfaces until the
P4 migration is complete. New code must not treat the legacy bare-ID view as an
offering identity. Provider-scoped reads use `ProviderModel(providerID,
providerModelID)` today; P4 introduces explicit definition/offering types and a
versioned compatibility adapter.

The transition is explicit. `Catalog.FindModel` keeps the canonical consumer
syntax but returns `ModelDefinition`; provider facts come from `Offering`.
Callers that still require the pre-split `Model` use
`Catalog.LegacyV0().FindModel`, `ProviderModel`, `ProviderModels`, or `Models`.
The adapter declares schema version 0; canonical generation payloads declare
schema version 1. Direct legacy collection methods on `Catalog` remain
deprecated for source compatibility during the v1 transition.

`catalogs.ProviderOffering` is the first schema implementing this contract. It
uses a comparable `OfferingKey`, typed `ProviderModelID` and
`ModelDefinitionID`, and owns all provider-specific service facts. Request-body
overrides retain exact JSON values rather than passing through `map[string]any`.
`catalogs.ModelDefinition` is its structurally disjoint complement: it owns
canonical authorship, release metadata, typed lineage, weights/architecture,
and intrinsic capabilities, and cannot contain provider service facts.

`catalogs.MigrateLegacySchema` converts the pre-split model records without
mutating them and emits a review report for every default, missing value, or
conflicting canonical definition. The embedded baseline is locked so catalog
updates cannot silently alter the migration disposition.

Immutable catalogs expose canonical `Definition`, `Offering`, and
`ProviderOfferings` lookups. Offering reads are keyed by the exact provider
tuple and return caller-owned values; equal model IDs at two providers never
share price, limits, modes, or request state.

Starport passes `RouteAlias` values to `MaterializeRouteAlias`; aliases are not
stored by catalog sources. Materialization separates eligible offerings from
missing, unavailable, and retired targets without carrying weights, fallback,
tenant, or strategy policy into Starmap.
