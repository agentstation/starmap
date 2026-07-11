# Schema Drift Policy

Starmap treats upstream schema drift by scope rather than applying one global
strict-or-tolerant decoder rule. The executable inventory is
`pkg/sources.SchemaDriftPolicies`; tests fail when a record family lacks both a
strict structural boundary and an explicit unknown-field disposition.

## Dispositions

| Disposition | Meaning |
| --- | --- |
| `reject_source` | The observation envelope or top-level catalog is unusable |
| `reject_record` | Quarantine one provider/model/definition/offering and keep valid siblings |
| `classify` | Do not promote the unknown value; retain reviewable drift evidence |
| `preserve` | Retain the exact value inside the typed source-extension boundary |

Identity fields and object/array container boundaries are strict. A missing,
null, or wrong-type identity/container is never silently coerced. Source-wide
envelope/catalog failures reject the source observation; record-local failures
quarantine only that record unless completeness policy requires the generation
to fail.

Unknown additive members are tolerant but never invisible. Members inside an
`extensions` boundary are preserved losslessly. Unknown members elsewhere are
classified with evidence and withheld from canonical promotion until reviewed.
Unknown enum values follow the same classify-before-promotion rule.

All production JSON model parsers retain additive unknowns as deterministic
`path` plus SHA-256 evidence under that source's extension bucket. Raw unknown
values are not retained, so review signals cannot leak arbitrary upstream
payload data. These extension records are explicitly excluded from field
authority. Unknown models.dev lifecycle enums use the same fingerprint format.

When a provider response fails typed decoding because a required container has
changed shape, the provider source emits a `schema_drift` issue rather than
conflating it with credentials or an ordinary fetch failure. The observation is
partial/degraded, retains valid providers, and exposes the provider subject for
alerting.

## Semantic promotion

Typed decoding is necessary but is not sufficient to publish or cache source
data. The models.dev HTTP adapter is the only durable runtime source-input
cache: provider APIs are observed directly, the pinned Git adapter rebuilds
from an exact checkout, and canonical catalog generations have their own
validate-before-publication boundary.

Every models.dev HTTP cache read and every HTTP/Git candidate therefore passes
deterministic semantic validation. Provider map identities must match their
records; provider names and model containers are required; and at least five
providers must exist. Model identity or name failures are record-local: the raw
source bytes remain available as evidence, invalid models are excluded from
accepted-model counts, and observation quarantines them with typed issues while
preserving valid siblings. Promotion reports both accepted and rejected model
counts. A newly produced candidate must contain at least 100 accepted models.
When a validated last-known-good HTTP cache exists, a candidate must also retain
at least 80 percent of its provider count and 50 percent of its accepted-model
count. These conservative floors reject truncation while allowing normal
upstream removals and isolated malformed records; changing them requires an
explicit policy and regression-test update.

Semantic rejection occurs before cache mutation. HTTP retains the checksum-bound
last-known-good payload and emits source-scoped `schema_drift` evidence plus the
typed stale/bootstrap fallback classification. The resulting observation is
partial/degraded. Pinned Git builds have no fallback generation and fail the
source load with a typed validation error. Response bodies are read through the
same 16 MiB source ceiling before any promotion decision.

## Provider record identity and accounting

Provider model IDs are opaque identifiers, not slugs. Starmap therefore permits
provider-defined punctuation such as `/`, `.`, `:`, and `@`, but quarantines an
ID that is empty, has leading/trailing whitespace, contains a control character,
or duplicates an earlier ID in the same provider observation. A model name must
contain non-whitespace text and no control characters. models.dev additionally
requires every model record ID to equal its enclosing map key.

Invalid records produce stable record-scoped `invalid_record` issues and cannot
erase valid siblings. Every live-provider and models.dev observation reports
typed accepted/rejected record counts. Non-zero rejection requires a
partial/degraded observation; the counts participate in observation identity and
are retained by minimized evidence capture/replay.

## Record scopes

The policy inventory covers source observations, decoded catalogs, provider and
model source records, and canonical model-definition/provider-offering records.
This makes failure scope explicit before parser-specific mutation, quarantine,
fuzz, and resource-bound gates are applied in P7.2-P7.11.
