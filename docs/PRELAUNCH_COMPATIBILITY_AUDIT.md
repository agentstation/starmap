# Prelaunch Compatibility Audit

Starmap has not launched. Catalog schema version 2 is therefore an intentional
clean break, not a migration target. Publication and consumption accept only
the exact current schema with explicit definitions and offerings.

## Removed surfaces

| Surface | Disposition | Executable evidence |
| --- | --- | --- |
| Schema-v1 payload decoding and migration-on-read | Removed; strict payload decoding requires version 2, `definitions`, and `offerings` | `TestCatalogPayloadV1FailsIncompatibly`, required-field negatives, strict unknown-field decode |
| Permissive bootstrap and generation manifests | Removed; both require exact `CurrentCatalogSchemaVersion` | `TestBootstrapManifestRequiresExactCurrentSchema`, generation-manifest schema negatives |
| Distribution compatibility ranges | Removed from manifests, pointers, artifacts, and attestations | exact-current repository/client tests and structural forbidden-field checks |
| Immutable flattened-model API | Removed: no `LegacyV0`, `Models`, `ProviderModel`, or `ProviderModels` on `Catalog` | `TestCatalogDoesNotExposeMutationInterfaces` includes forbidden-method reflection |
| Public schema migration API | Removed; there is no `MigrateLegacySchema` or exported migration report/type | repository absence search below |
| Old durable directory migration | Removed with its frozen fixture and tests | no `MigrateLegacyDirectory` or `pkg/catalogstore/testdata/legacy-v0` |
| Deprecated `pkg/types` aliases | Removed; current domain metadata lives in `pkg/catalogmeta` | package directory absent and repository builds without it |
| Deprecated models.dev constructor/option aliases | Removed: `Client`, `NewClient`, and `WithGitSourcesDir` | package compile and absence search |
| Deprecated `--dry-run` CLI alias | Removed; `--dry` is the sole flag | command tests and documentation |

## Retained current architecture surfaces

| Surface | Why it remains current rather than compatibility debt |
| --- | --- |
| `ModelSourceReader` and source projection | Checked-in YAML and provider APIs currently produce mutable `Model` ingestion records. Projection occurs only while a `Builder` is assembled; no payload, manifest, remote, distribution, store, or artifact decoder can invoke it. |
| Provider and author aliases | They are canonical identity-equivalence data used by current provider resolution. Each resolves to exactly one entity and cannot encode routing policy. |
| Route aliases | They are a current Starport routing identity above ingestion. They target exact offering keys and explicitly exclude weights, fallback, tenant, and strategy policy. |
| Customer deployment aliases | They are private customer-inventory identifiers, structurally excluded from public catalog generations. |
| models.dev `cache` and boolean/object input tolerance | These parse the currently consumed external models.dev source format. They are data-import normalization, not catalog payload or API compatibility. |
| Provider wire-format normalization | OpenAI-compatible and cloud-provider adapters accept the current upstream response variants needed to import live provider data. They never relax catalog schema validation. |
| Embedded bootstrap and last-known-good fallback | These are current operational resilience paths between valid schema-v2 generations. They do not discover or migrate an old directory or payload. |
| `sources.ID` and `sources.ResourceType` domain spellings | These are the current source framework's public vocabulary over the zero-dependency `catalogmeta` ownership layer; no superseded package or decoder is kept alive. |
| CLI command aliases such as `inspect` and `server` | These are intentional current UX names declared together on the commands, not deprecated spellings or hidden migration shims. |

## Reproducible searches

Run from the repository root after generated documentation is refreshed:

```bash
rg -n 'LegacyV0|LegacyCatalogV0|MigrateLegacySchema|MigrateLegacyDirectory|ConsumerCompatibility|consumer_compatibility' --glob '*.go' --glob '*.json' --glob '*.yaml' --glob '*.yml' .
rg -n 'provider_models|author_models|"schema_version"[[:space:]]*:[[:space:]]*1' --glob '*.json' --glob '*.go' .
rg -n 'Deprecated:.*(catalog|model|schema)|backward compatibility|backwards compatibility' --glob '*.go' .
test ! -e pkg/types && test ! -e pkg/catalogstore/testdata/legacy-v0
```

The only expected schema-v1 matches are negative-test descriptions or the
control-plane history explicitly marked as superseded. Generic lifecycle values
such as `deprecated`, provider model statuses, operational database migrations,
and upstream data-import normalization are not catalog compatibility surfaces.
