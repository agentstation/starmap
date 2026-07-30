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
meaningful explicit zero, `false`, or empty string when the source actually
supplied it.

## Presence semantics

Starmap keeps ordinary catalog reads ergonomic while retaining source
presence. `Model.Description`, feature booleans, limit integers, and
`ModelMetadata.OpenWeights` remain their natural Go scalar types. Their owning
types also expose typed presence methods:

- `DescriptionValue`;
- `ModelFeatures.Support`;
- `ModelLimits.Value`; and
- `OpenWeightsValue`.

Each returns `ValueMissing`, `ValueUnknown`, or `ValueKnown`. Direct non-zero
Go literals are known without setter boilerplate. A source adapter or catalog
author uses `SetDescription`, `SetSupport`, `ModelLimits.Set`, or
`SetOpenWeights` only when it must preserve an explicit scalar zero. The
corresponding `SetDescriptionUnknown`, `SetSupportUnknown`,
`ModelLimits.SetUnknown`, and `SetOpenWeightsUnknown` methods record an
upstream `null`; matching `Unset` methods withdraw a claim.

The human YAML remains ordinary scalars:

```yaml
description: ""
status: unknown
features:
  tool_calls: false
  tools: null
limits:
  context_window: 0
  input_tokens: null
```

Generated human YAML expands every Boolean capability into the same editable
matrix. A capability with no observed claim is displayed as the conservative
`false` default; `null` remains explicitly unknown. Provenance and the
committed baseline prevent an untouched generated default from becoming
synthetic local evidence on the next update. An actual source claim or
semantic human edit remains known and participates at that source's authority
position.

For non-Boolean fields, an omitted key is missing and makes no claim. `null`
is explicitly unknown; `0` and `""` are known values. Missing numeric limits
remain omitted rather than being fabricated as zero. Precise presence survives
immutable catalog JSON, deep copy, merge, reconciliation, baseline comparison,
and change reporting.

Provider and models.dev decoders mark presence from the upstream wire keys.
Inferred positive capabilities remain known; an unreported false or zero does
not become an authoritative negative claim. During reconciliation, a known
zero participates at its source's normal authority position, while missing and
unknown values permit lower-authority fallback.

## Observation health and absence

Field authority applies only to values a source actually supplied. Omission is
not a lifecycle claim, even when an observation reports `complete` and
`succeeded`. Reconciliation therefore seeds model identity from the immutable
last-known-good baseline and retains exact prior provenance for baseline-only
models and providers.

Valid present fields from a partial/degraded observation remain eligible at
their normal authority position. Their provenance reason records observation
status, completeness, accepted/rejected counts, and stable issue codes. A
stale-cache or embedded-bootstrap fallback is narrower: it may fill a missing
baseline fact but cannot replace a known fact.

The pipeline compares each observation with models previously attributed to
that same source. An unexplained count regression becomes a provider-scoped
`volume_collapse` issue and partial/degraded evidence; it never becomes an
implicit removal. A source error without a usable candidate becomes a bounded
partial/degraded empty observation so non-strict sync can retain the baseline
and reconcile healthy siblings. Caller cancellation stops, required-all mode
fails, and a degraded `Fresh` run cannot publish from an empty baseline.

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
- projected provenance is compared to the parsed semantic field value on
  reload. An unchanged value retains its original source, observation identity,
  revision, checksum, and timestamp;
- when the original source is observed again, its current value replaces the
  unchanged projection at that source's authority position—even for a normally
  local-first operator field;
- a semantic mismatch is a local claim and receives the current local
  observation identity; and
- an explicitly present `false`, `0`, `""`, or `null` retains its distinct
  presence state, while removing the key withdraws the local claim; and
- comments, quoting, whitespace, and key order do not participate in the
  comparison.

Model comparisons use the exact provider/model provenance identity. Provider
configuration uses provider/field identity. The legacy bare model evidence in
the current embedded bootstrap is consulted only when that model ID occurs at
exactly one provider; ambiguous evidence is never promoted across offerings.
