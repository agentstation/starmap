# Schema Drift Policy

Starmap treats upstream schema drift by scope rather than applying one global
strict-or-tolerant decoder rule. The executable inventory is
`pkg/sources.SchemaDriftPolicies`. Tests fail when a record family lacks both a
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
envelope/catalog failures reject the source observation. Record-local failures
quarantine only that record unless completeness policy requires the generation
to fail.

Unknown additive members are tolerant but never invisible. Starmap preserves
members inside an `extensions` boundary without data loss. It classifies
unknown members elsewhere with evidence and withholds them from canonical
promotion until review.
Unknown enum values follow the same classify-before-promotion rule.

All production JSON model parsers retain additive unknowns as deterministic
`path` plus SHA-256 evidence under that source's extension bucket. Raw unknown
values are not retained, so review signals cannot leak arbitrary upstream
payload data. These extension records are explicitly excluded from field
authority. Unknown models.dev lifecycle enums use the same fingerprint format.

When a required container changes shape, the provider response fails typed
decoding. The provider source emits a `schema_drift` issue rather than
conflating it with credentials or an ordinary fetch failure. The observation is
partial/degraded, retains valid providers, and exposes the provider subject for
alerting.

## Semantic promotion

Typed decoding is necessary but is not sufficient to publish or cache source
data. Only the models.dev HTTP adapter keeps a durable runtime source-input
cache. Provider APIs produce direct observations, and the pinned Git adapter
rebuilds from an exact checkout. Canonical catalog generations have their own
validate-before-publication boundary.

Every models.dev HTTP cache read and every HTTP/Git candidate therefore passes
deterministic semantic validation. Provider map identities must match their
records. Each record requires a provider name and model container. A candidate
must also contain at least five providers. Model identity, name, or typed-field
decode failures affect only their records. The raw source bytes remain available
as evidence.

The observation excludes invalid models from accepted-model counts
and quarantines them with typed issues. Valid siblings remain available. Promotion reports both accepted
and rejected model counts. A newly produced candidate must contain at least 100
accepted models.

When a validated last-known-good HTTP cache exists, a candidate must also retain
at least 80 percent of its provider count. It must retain 50 percent of its
accepted-model count. These conservative floors reject truncation while allowing normal
upstream removals and isolated malformed records. Changing them requires an
explicit policy and regression-test update.

Semantic rejection occurs before cache mutation. HTTP retains the checksum-bound
last-known-good payload and emits source-scoped `schema_drift` evidence plus the
typed stale/bootstrap fallback classification. The resulting observation is
partial/degraded. Pinned Git builds have no fallback generation and fail the
source load with a typed validation error. The adapter reads response bodies through the
same 16 MiB source ceiling before any promotion decision.

## Provider record identity and accounting

Provider model IDs are opaque identifiers, not slugs. Starmap permits
provider-defined punctuation such as `/`, `.`, `:`, and `@`. It quarantines an
empty ID or one with leading or trailing whitespace. It also quarantines an ID
that contains a control character or duplicates an earlier ID. A model name must
contain non-whitespace text and no control characters. models.dev also
requires every model record ID to equal its enclosing map key.

Invalid records produce stable record-scoped `invalid_record` issues and cannot
erase valid siblings. Every live-provider and models.dev observation reports
typed accepted/rejected record counts. Non-zero rejection requires a
partial/degraded observation. The observation identity includes the counts.
Minimized evidence capture and replay retain them.

Local provider-model YAML follows the same record scope: one malformed model
file produces a degraded load report while valid siblings remain available.
Provider, author, and provenance indexes plus filesystem failures remain
structural and fail closed. Embedded bootstrap, legacy migration, and atomic
workspace projection require an empty load report.

Stored generation payloads expose valid siblings only as a typed partial
diagnostic. A missing or malformed collection envelope prevents activation.
Inconsistent provider or author identity also prevents activation. The same
rule applies to byte, nesting, or count limit failures and any partial payload.
The last committed generation stays active.

## Record scopes

The policy inventory covers source observations, decoded catalogs, provider and
model source records, and canonical model-definition/provider-offering records.
It defines failure scope before parser-specific mutation and quarantine. The
P4.8 and P4.10 fuzz and resource-bound gates use this scope. The remote
transport gates in P7.2-P7.11 also use it.
