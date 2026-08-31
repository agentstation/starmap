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
2. An offering owns its provider model ID, price, limits, availability, and
   regions. It also owns provider lifecycle, endpoint behavior, and request
   overrides.
3. Two providers may expose the same provider model ID. Their offering keys are
   still distinct and neither may overwrite the other.
4. One provider may expose multiple provider model IDs for one definition.
   Those IDs identify distinct offerings linked to the same definition ID.
5. Provider model IDs are opaque. Slashes, dates, namespaces, and case are data,
   not separators or normalization instructions.
6. Aliases are identity equivalence only. An alias resolves to one canonical
   entity and cannot choose among offerings.
7. Starmap materializes route aliases above ingestion. Sources report
   observations. They do not decide routing eligibility, weights, fallback,
   tenancy, or policy.
8. Every offering references exactly one existing provider and one existing
   definition. Every route target references an existing offering key.

## Canonical read boundary

`Catalog.FindModel` and `Catalog.Definition` return the provider-independent
`ModelDefinition`. Provider facts come from `Offering` and
`ProviderOfferings`. The immutable catalog does not expose a flattened
bare-model collection. Such a collection would discard provider identity. It
would make price, limits, lifecycle, and request behavior ambiguous.

`catalogs.ProviderOffering` is the first schema implementing this contract. It
uses a comparable `OfferingKey` and typed `ProviderModelID` and
`ModelDefinitionID` values. It owns all provider-specific service facts.
Request-body
overrides retain exact JSON values rather than passing through `map[string]any`.
`catalogs.ModelDefinition` is its structurally disjoint complement. It owns
canonical authorship, release metadata, typed lineage, weights, architecture,
and intrinsic capabilities. It cannot contain provider service facts.

Immutable catalogs expose canonical `Definition`, `Offering`, and
`ProviderOfferings` lookups. `DefinitionOfferings` maps a definition to its
offerings. `AuthorModel` resolves an author plus slug.

`FindModel` accepts a canonical
`author/slug`, an unambiguous bare slug, or an unambiguous exact provider model
ID. Ambiguous aliases return a typed conflict instead of selecting a winner.
Offering reads use the exact provider tuple and return caller-owned values.
Equal model IDs at two providers never share price, limits, modes, or
request state.

Starport passes `RouteAlias` values to `MaterializeRouteAlias`. Aliases are not
stored by catalog sources. Materialization separates eligible offerings from
missing, unavailable, and retired targets without carrying weights, fallback,
tenant, or strategy policy into Starmap.

## Persisted source shape

The human catalog workspace has two model roles:

- `authors/{author}/models/{slug}.yaml` owns canonical `author/slug` identity
  and intrinsic model facts.
- `providers/{provider}/models/**.yaml` owns the exact opaque provider model ID
  and an explicit `model: author/slug` link. It also owns provider-serving facts
  such as price, limits, availability, lifecycle, modes, and endpoint behavior.

Provider identity is never authorship evidence. Multiple providers may link to
one authored model, and one provider may serve models from many independent
authors. The immutable catalog validates those links and precomputes
definitions, offerings, author membership, aliases, and model-to-offering
indexes.

`endpoints.yaml` is a versioned, digest-bound generated projection of that
join. It is inspectable output, not an editable source or a third authority.
Catalog schema version 3 introduced the `author_models` and `provider_models`
construction-record collections. Schema version 4 adds provider credential
profiles and plane references. Starmap retains no reader for an earlier schema.
