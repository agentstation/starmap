# Catalog Authority Policy

Starmap persists one human-readable provider-model record. Reconciliation
selects the facts in that record once; immutable definitions, offerings, and
author membership are derived read views of the same result.

The sole executable source of field precedence is the immutable table returned
by `authority.New`. Each `authority.Policy` owns:

- the catalog resource and reflected field path;
- the stable provenance path, when it differs from the Go field path;
- one highest-to-lowest source order;
- merge and empty-value semantics; and
- the rationale for that decision.

The reconciler iterates that table directly. Nested limits, metadata, features,
modes, pricing, and source extensions use focused executors, but those
executors receive their order and semantics from the same policy. They contain
no second priority list. Tests fail when a reconciled catalog field lacks a
policy, a policy names a nonexistent field, or returned policy state aliases
the table.

## Source roles

| Fact | Normal order | Reason |
| --- | --- | --- |
| Provider price, limits, capabilities, modes, model name | provider → models.dev HTTP → models.dev Git → local | Current valid provider facts are offering-specific and lead fallback observations |
| Model description, lifecycle, lineage family, definition metadata | models.dev HTTP → models.dev Git → provider → local | Maintained upstream metadata leads sparse provider APIs; human YAML fills missing facts |
| Provider identity and discovery metadata | provider → models.dev HTTP → models.dev Git → local | Current observations lead a human fallback |
| Provider API key shape, environment variables, catalog endpoint, inference endpoint | local → provider → models.dev HTTP → models.dev Git | Connection configuration is operator-owned |
| Source extensions | local → models.dev HTTP → models.dev Git → provider | Namespaced extension leaves merge without competing with canonical fields |

Git and HTTP remain distinct evidence identities even when they carry the same
models.dev dataset.

## Merge semantics

| Policy | Meaning |
| --- | --- |
| `replace` | Select one complete value; never synthesize its subfields |
| `fill_missing` | Higher-authority leaves win and lower sources fill only absent leaves |
| `set_union` | Add unique members in authority order |
| `deep_merge` | Merge named records or a documented structured field while preserving leaf authority |

`absent` permits fallback for missing values. `authoritative` preserves a
meaningful explicit zero or `false`. Pointers and containing-record presence
currently establish explicit capability records; the dedicated presence phase
extends that distinction to every affected scalar.

## Pricing

Pricing is one atomic provider-offering fact. Selection:

1. walks the model `Pricing` policy order;
2. rejects malformed, future, or expired candidates with evidence;
3. chooses the first semantically valid, currently effective candidate; and
4. deep-copies the complete price without mixing currency, token, operation,
   tier, or effective-period subfields.

A provider price therefore wins when it is valid. models.dev and local prices
are fallback evidence, not fragments used to repair a rejected provider price.
Routing preference is not an authority input and remains owned by the caller,
such as Starport.

## Provenance identity

Model evidence is durable under the provider/model pair, never a bare model
ID. The encoded resource identity escapes the provider and opaque provider
model ID independently, so separators inside either value cannot collide.
`Catalog.Provenance().FindModel(providerID, modelID)` is the normal read API;
generic resource lookups remain available for algorithms that work across
resource types.

The flat reconciliation result also keeps provider-scoped paths. Catalog
payload encode/decode and provenance reports preserve the same two independent
resources when providers publish the same model ID. Price, each limit
dimension, and lifecycle/availability status therefore retain the evidence for
the exact offering they describe.

## Human YAML

Local YAML is evidence, not an unconditional override layer:

- current valid dynamic facts beat stale local copies for discoverable fields;
- local values fill facts missing from dynamic sources;
- operator connection configuration remains local-first; and
- later workspace provenance work distinguishes unchanged generated YAML from
  an actual semantic human edit, so generated facts are not relabeled local.
