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
| Singular provider auth/endpoint shape | Removed; current provider config requires credentials, logical sources, explicit auth, observation scope, and endpoint | old-shape negative YAML/JSON fixtures and exhaustive embedded-provider validation |
| Provider-wide runtime key fields and loaders | Removed; configuration stores environment names while request-scoped resolver results own values | reflection/copy/redaction/resolver tests and exact symbol search |
| `CustomerInventory` and public/catalog overlays | Removed; credential-scoped facts are ordinary definitions/offerings in one contextual catalog | contextual encode/copy/isolation tests plus fail-before-write publication gates |
| Generic callback `CloudCredentialChain` | Removed; `cloud_chain` selects one provider-inferred official-SDK adapter | SDK registry exhaustiveness and fake-session tests |
| `Precision` and `FORMAT` aliases | Removed; quantization and `OUTPUT` are the only current forms | schema/config tests and exact symbol/environment search |
| CLI `--fmt`/`--format`, `inspect`, `server`, and `STARMAP_OUTPUT_FORMAT` aliases | Removed; `--output`, `embed`, `serve`, and `OUTPUT` are the only current forms | `TestPrelaunchCommandAndOutputAliasesAreAbsent` plus exact flag/environment searches |
| Aggregate auth status and optional-state compatibility vocabulary | Removed; each logical source reports only ready, unavailable, invalid, or unauthenticated with its accepted methods and environment names | auth checker/CLI tests and exact `APIKeyDetails`/old-state symbol searches |
| Provider configuration backup and old fixture layout | Removed; `providers.yaml` and the derived fixture contract are the sole current authorities | absence of `providers.yaml.bak`, provider-local exact-protocol metadata, and the broad fixture-policy file plus provider-contract tests |
| Permissive model-filter parser | Removed; HTTP handlers expose only strict fail-closed parsing | malformed query fixtures and absence of exported permissive parser |
| Unverified models.dev cache fallback | Removed; cached bytes require exact-current metadata or the verified embedded bootstrap replaces them | HTTP failure fixture with metadata-less cache |
| Broad provider fixture policy | Removed; fixture class derives from source/auth/topology/connector behavior | exhaustive provider contract test and exact path search |

## Retained current architecture surfaces

| Surface | Why it remains current rather than compatibility debt |
| --- | --- |
| Package-private model-source reader and source projection | Checked-in YAML and provider APIs currently produce mutable `Model` ingestion records. `buildCatalog` invokes the package-private projection only while a `Builder` is assembled; no payload, manifest, remote, distribution, store, or artifact decoder can invoke it. |
| Provider and author aliases | They are canonical identity-equivalence data used by current provider resolution. Each resolves to exactly one entity and cannot encode routing policy. |
| Route aliases | They are a current Starport routing identity above ingestion. They target exact offering keys and explicitly exclude weights, fallback, tenant, and strategy policy. |
| Credential-scoped deployment aliases | They are ordinary contextual offering identity. Any observation carrying them is credential-scoped and rejects the complete public write before bytes. |
| models.dev `cache` and boolean/object input tolerance | These parse fields currently emitted by the external models.dev source. They are data-import normalization, not catalog payload or API compatibility. Cached files themselves require current integrity metadata. |
| Provider wire-format normalization | OpenAI-compatible and cloud-provider adapters accept the current upstream response variants needed to import live provider data. They never relax catalog schema validation. |
| Embedded bootstrap and last-known-good fallback | These are current operational resilience paths between valid schema-v2 generations. They do not discover or migrate an old directory or payload. |
| `sources.ID` and `sources.ResourceType` domain spellings | These are the current source framework's public vocabulary over the zero-dependency `catalogmeta` ownership layer; no superseded package or decoder is kept alive. |
| Scheduler SQL table creation | `pkg/catalogscheduler/sql_run_ledger.go` creates current operational run-ledger tables with `CREATE TABLE IF NOT EXISTS`; it never reads or transforms catalog/configuration payloads. |
| Cobra command parsing and config-file precedence | These implement the sole current CLI/config contract. No removed flag, command, environment, directory, or payload spelling is consulted as a fallback. |

## Reproducible searches

Run from the repository root after generated documentation is refreshed:

```bash
rg -n 'LegacyV0|LegacyCatalogV0|MigrateLegacySchema|MigrateLegacyDirectory|ConsumerCompatibility|consumer_compatibility|CustomerInventory|PublicCatalog|CatalogOverlay|EffectiveCatalog|CloudCredentialChain|ProviderAPIKey|APIKeyValue|LoadAPIKey|LoadEnvVars|HasAPIKey|IsAPIKeyRequired|APIKeyDetails' internal pkg cmd --glob '*.go' --glob '!**/*_test.go'
rg -n 'StateConfigured|StateMissing|StateOptional' internal/auth cmd/starmap/cmd/providers --glob '*.go' --glob '!**/*_test.go'
rg -n 'provider_models|author_models|"schema_version"[[:space:]]*:[[:space:]]*1|auth_required|STARMAP_OUTPUT_FORMAT' internal pkg cmd --glob '*.go' --glob '*.json' --glob '*.yaml' --glob '*.yml' --glob '!**/*_test.go'
rg -n 'alias for --output|Aliases:[[:space:]]*\[\]string\{"(inspect|server)"\}|Deprecated:.*(catalog|model|schema)|backward compatibility|backwards compatibility' internal pkg cmd --glob '*.go' --glob '!**/*_test.go'
test ! -e pkg/types && test ! -e pkg/catalogstore/testdata/legacy-v0 && test ! -e internal/providers/fixtures/policy.yaml && test ! -e internal/embedded/catalog/providers.yaml.bak
```

These production searches must be empty; the path test must succeed. Separate
test searches intentionally find schema-v1 and removed-shape negative fixtures.
Generic lifecycle values such as `deprecated`, provider model statuses,
operational database setup, current identity aliases, and upstream data-import
normalization are not catalog compatibility surfaces.
