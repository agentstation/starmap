# Catalog settings reference

This reference describes the Starmap catalog configuration contract in this source revision.
The public package is `github.com/agentstation/starmap/pkg/catalogs/config`.
The [descriptor export](catalog-settings-schema.json) contains the same metadata.
Starport adoption remains a separate implementation task.

The descriptors generate this reference and CLI descriptions.
To regenerate both reference files, run:

```sh
go test ./internal/catalog/settings -run '^TestCatalogSettingsReferenceIsCurrent$' -args -update-settings-reference
```

## Value selection

The Starmap command selects each independent value in this order:

1. An explicit command flag.
2. The process environment, including an explicit empty value.
3. Explicit dotenv files, with the last listed file first.
4. The selected configuration file.
5. The runtime default.

YAML uses the flat keys listed below.
Boolean, integer, and string-list values can use native YAML types.
Durations use Go duration syntax, such as `4h` or `30s`.
Unknown keys fail validation.
A valid higher-priority value can replace an invalid lower-priority scalar value.
Malformed files and wrong YAML value types fail before value selection.

All catalog CLI flags take a value, including boolean flags.
For example, `--catalog-acquisition-enabled=false` disables automatic acquisition.
An unchanged flag does not override another input.
The parser trims surrounding whitespace and preserves explicit false, zero, and permitted empty values.
An empty value fails unless its descriptor permits it.

## Authority origin configuration

An internal Starmap origin publishes one permitted catalog and issues permission receipts.
Set catalog_authority_origin as one YAML object with enabled, authority_id, policy_id, bootstrap, and permission_lifetime fields.
The environment and CLI accept the same object as JSON.
Each higher-priority declaration replaces the complete object.
Authority and policy identities never inherit from a replaced declaration.

An enabled origin requires both identities, the selected catalog store, and a configured native permission clock.
The application uses its canonical catalog store path.

Omitted bootstrap is false. Set it explicitly to adopt an existing ordinary catalog.
Omitted permission_lifetime selects five minutes. An explicit duration must exceed zero and cannot exceed five minutes.
The native clock settings require qualified bounds for the host.
Unknown time prevents permission issuance while catalog diagnostics remain available.

Set enabled to false as the object's only field to disable origin issuance.
This setting does not remove the stored authority or permit ordinary catalog fallback.
An existing authoritative store still requires a matching authority mode or an explicit transition.
Omitting the whole setting preserves origin options supplied by a library host.
A subscriber uses catalog_source_authority_id and catalog_source_policy_id instead.
It cannot also publish as an origin.

## Source and credential boundaries

The source group contains the kind, endpoint, repository, channel, signing workflow, and transport credentials.
A higher-priority source identity replaces the complete lower source group.
Supply the source kind and its required endpoint together.
Lower source credentials do not transfer to the replacement source.
A higher-priority credential can replace a credential without changing the source identity.
An explicit empty credential prevents lower credential fallback.

Poll intervals, freshness thresholds, and acquisition policy keep independent precedence.
The resolver reports selected and ignored origins without credential values.
The Starmap source API key authenticates catalog transport.
The GitHub source token authenticates GitHub access.
Neither is a provider inference key.

## Permission clock configuration

The permission clock source defaults to disabled. Public and embedded catalog use need no native clock profile.
Internal authority permissions require qualified time evidence before they can authorize new work.
Selecting native mode requires explicit cache age, refresh interval, counter drift, and counter uncertainty.
Windows also requires synchronization source age, source drift, and additional source uncertainty.

The refresh interval must be less than half the maximum cache age.
Configuration declares these bounds. It does not qualify the host, time service, or error profile.

Clock settings have node scope and require restart after changes.
Each host supplies its own qualified values. They do not inherit from another product or from shared deployment configuration.
Changing the catalog source does not reset clock settings.

An explicit disabled source clears an earlier host clock selection.
The public parser preserves each setting separately. Application composition validates the complete native profile before runtime startup.

Construction starts no clock query. Runtime startup owns background observations, and runtime shutdown cancels them.
Request admission reads cached evidence without a time-service query.
An unqualified observation invalidates that evidence while catalog diagnostics remain available.
Native clock status errors are local diagnostics and need redaction before public exposure.

## Provider binding declarations

`catalog_provider_bindings` selects the complete active binding set for connected-runtime acquisition.
Supply YAML as a list of binding objects.
Environment variables and CLI flags accept the same objects as a JSON array.
The value uses the `sources.ProviderAcquisitionBinding` wire contract.

Each object declares a schema version, binding ID, revision, provider, scope, API surface, region, and credential profile.
The credential role must be `catalog_acquisition`.
The declarations contain no credential material and do not prove upstream account ownership.

Schema 2 supports explicit [membership replacement authority](CATALOG_STORE_CONTRACT.md#scope-replacement-and-transport).
Omission grants no replacement authority. Schema 1 remains readable and cannot grant that authority.

An explicit `[]` permits no local provider acquisition.
Omission retains legacy unscoped acquisition. Empty text and `null` are invalid.
A higher-priority array replaces the entire lower array.
Bindings from separate configuration authorities do not combine.

Replacing the upstream catalog source does not replace this independent binding policy.
Unknown binding fields, invalid declarations, and duplicate binding IDs fail validation.
Changing the binding set requires a new runtime.

The current standalone update command and HTTP update adapter still require binding-policy integration.
Do not use those paths to enforce a scoped acquisition policy.
Starport adoption and complete product qualification remain open.

## Internal authority runtime

The require_authority startup policy keeps metadata available while permission controls new work.
Select the starmap source, its URL, and both source authority and policy IDs.
The runtime accepts only that authority's catalog and disables local acquisition.
A retained catalog does not by itself authorize requests.
Permission checks run independently of catalog checks and the shared acquisition lease.

The runtime stores its highest requirement and finite receipt in catalog-runtime/permission.json beneath the state directory.
This private checkpoint stays uncertain until shutdown finishes and retains every known requirement.
After a crash, metadata remains available, but new work requires a fresh verified receipt.
A completed shutdown permits offline restart while the retained receipt remains valid.

The library host supplies cached clock evidence through WithPermissionClockUncertainty.
Unknown clock validity blocks new work. The callback must use the same time source as WithClock.
The current standalone composition supplies no clock qualification adapter.
Authority receipt issuance and complete deployment qualification remain open.
These settings do not establish a qualified internal-server recipe by themselves.

## Explicit dotenv files

Service configuration does not discover dotenv files in the working directory.
A local operator can name files explicitly:

```sh
starmap --env-file .env --env-file .env.local version
```

Later files replace earlier file values.
The process environment takes precedence over every file, even when its value is empty.
Conflicting file values produce diagnostics with setting names and file paths.
Diagnostics omit both values.
All files must pass private-access checks and parse before any environment change occurs.

Each file permits at most 1 MiB. Private read-only files remain valid.
See [configuration access and recovery](CLI.md#private-configuration-inputs) for platform requirements.

Catalog dotenv values stay in their named file layers.
Other dotenv values enter the process environment only when the process does not already define them.
This supplies provider credentials to the existing acquisition resolver.
Do not place credentials in command arguments or commit them to a repository.

## Legacy migration

`REMOTE_SERVER_URL` and `remote_server_url` imply the Starmap source kind within their own input layer.
Their matching API key aliases are `REMOTE_SERVER_API_KEY` and `remote_server_api_key`.
A canonical source identity replaces the legacy source group within that layer.
The command reports legacy names without their values.
Migrate the complete source group together.

## Library and application responsibilities

`Parse` accepts canonical names and rejects unknown names.
`CanonicalValues` converts descriptor keys, semantic IDs, and canonical environment names.
Conflicting aliases fail.
`Load` reads a caller-supplied lookup and cannot enumerate unknown names.
`Resolve` accepts ordered layers after the host selects eligible authorities.
These APIs do not read files, inspect the environment, or start network work.

`Config.Options()` supplies runtime options for the selected values.
Absent values leave defaults with the runtime or hosting application.
`Config.Value()` reports supplied values and presence, including secret values.
Do not expose that method's output through diagnostics.

Node settings belong to this process.
Deployment settings belong to the selected deployment authority.
A change class describes the required application action.
It does not imply a live settings API in the current command.
The command applies configuration at startup.

Platform roots, primary file selection, storage migration, and Starport shared configuration need their separate implementation and qualification.
This reference does not qualify those features.

<a id="catalog-authority-origin"></a>

## catalog_authority_origin

Selects one complete authority origin declaration. Disabling issuance preserves the store's authority and requires an explicit transition before ordinary startup.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_AUTHORITY_ORIGIN` |
| CLI flag | `--catalog-authority-origin value` |
| YAML key | `catalog_authority_origin` |
| Semantic ID | `catalog.authority.origin` |
| Grammar | `authority-origin` |
| Default | omission preserves host origin options. An explicit declaration replaces the complete origin selection |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `deployment` |
| Applicability | `all` |
| Change class | `restart` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-source"></a>

## catalog_source

Selects the upstream catalog source.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_SOURCE` |
| CLI flag | `--catalog-source value` |
| YAML key | `catalog_source` |
| Semantic ID | `catalog.source` |
| Grammar | `string` |
| Accepted names | `public`, `github`, `starmap`, `file`, `embedded` |
| Default | `public` |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `deployment` |
| Applicability | `all` |
| Change class | `runtime-replacement` |
| Compatibility | `supported`, schema 1 |
| Source group | `catalog-source` |

<a id="catalog-source-url"></a>

## catalog_source_url

Names the Starmap endpoint or catalog file.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_SOURCE_URL` |
| CLI flag | `--catalog-source-url value` |
| YAML key | `catalog_source_url` |
| Semantic ID | `catalog.source.url` |
| Grammar | `string` |
| Default | required for a custom URL or file source |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `deployment` |
| Applicability | `starmap`, `file` |
| Change class | `runtime-replacement` |
| Compatibility | `supported`, schema 1 |
| Source group | `catalog-source` |

<a id="catalog-source-api-key"></a>

## catalog_source_api_key

Authenticates transport to the selected catalog source. It is separate from provider credentials.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_SOURCE_API_KEY` |
| CLI flag | `--catalog-source-api-key value` |
| YAML key | `catalog_source_api_key` |
| Semantic ID | `catalog.source.api.key` |
| Grammar | `string` |
| Default | no transport credential |
| Explicit empty | true |
| Explicit zero | false |
| Sensitive | true |
| Scope | `deployment` |
| Applicability | `starmap` |
| Change class | `runtime-replacement` |
| Compatibility | `supported`, schema 1 |
| Source group | `catalog-source` |

<a id="catalog-source-repository"></a>

## catalog_source_repository

Names the GitHub repository that publishes the catalog channel.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_SOURCE_REPOSITORY` |
| CLI flag | `--catalog-source-repository value` |
| YAML key | `catalog_source_repository` |
| Semantic ID | `catalog.source.repository` |
| Grammar | `string` |
| Default | `agentstation/starmap` |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `deployment` |
| Applicability | `public`, `github` |
| Change class | `runtime-replacement` |
| Compatibility | `supported`, schema 1 |
| Source group | `catalog-source` |

<a id="catalog-source-channel"></a>

## catalog_source_channel

Names the branch that holds the catalog channel document.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_SOURCE_CHANNEL` |
| CLI flag | `--catalog-source-channel value` |
| YAML key | `catalog_source_channel` |
| Semantic ID | `catalog.source.channel` |
| Grammar | `string` |
| Default | `catalog/v1` |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `deployment` |
| Applicability | `public`, `github` |
| Change class | `runtime-replacement` |
| Compatibility | `supported`, schema 1 |
| Source group | `catalog-source` |

<a id="catalog-source-signer-workflow"></a>

## catalog_source_signer_workflow

Pins the accepted build provenance workflow.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_SOURCE_SIGNER_WORKFLOW` |
| CLI flag | `--catalog-source-signer-workflow value` |
| YAML key | `catalog_source_signer_workflow` |
| Semantic ID | `catalog.source.signer.workflow` |
| Grammar | `string` |
| Default | source adapter's default signing workflow |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `deployment` |
| Applicability | `public`, `github` |
| Change class | `runtime-replacement` |
| Compatibility | `supported`, schema 1 |
| Source group | `catalog-source` |

<a id="catalog-source-token"></a>

## catalog_source_token

Authenticates transport to the selected catalog source. It is separate from provider credentials.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_SOURCE_TOKEN` |
| CLI flag | `--catalog-source-token value` |
| YAML key | `catalog_source_token` |
| Semantic ID | `catalog.source.token` |
| Grammar | `string` |
| Default | no transport credential |
| Explicit empty | true |
| Explicit zero | false |
| Sensitive | true |
| Scope | `deployment` |
| Applicability | `public`, `github` |
| Change class | `runtime-replacement` |
| Compatibility | `supported`, schema 1 |
| Source group | `catalog-source` |

<a id="catalog-source-refresh-mode"></a>

## catalog_source_refresh_mode

Selects automatic source refresh or explicit manual reads. Manual mode suppresses startup reads, polling, and source watchers.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_SOURCE_REFRESH_MODE` |
| CLI flag | `--catalog-source-refresh-mode value` |
| YAML key | `catalog_source_refresh_mode` |
| Semantic ID | `catalog.source.refresh.mode` |
| Grammar | `string` |
| Accepted names | `automatic`, `manual` |
| Default | `automatic` |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `deployment` |
| Applicability | `all` |
| Change class | `runtime-replacement` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-network-mode"></a>

## catalog_network_mode

Controls catalog network acquisition. Offline mode preserves local imports and does not change inference or selected storage access.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_NETWORK_MODE` |
| CLI flag | `--catalog-network-mode value` |
| YAML key | `catalog_network_mode` |
| Semantic ID | `catalog.network.mode` |
| Grammar | `string` |
| Accepted names | `configured`, `offline` |
| Default | `configured` |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `deployment` |
| Applicability | `all` |
| Change class | `runtime-replacement` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-source-poll-interval"></a>

## catalog_source_poll_interval

Sets the period between automatic catalog checks. Zero disables periodic catalog checks.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_SOURCE_POLL_INTERVAL` |
| CLI flag | `--catalog-source-poll-interval value` |
| YAML key | `catalog_source_poll_interval` |
| Semantic ID | `catalog.source.poll.interval` |
| Grammar | `duration` |
| Default | `1h0m0s` |
| Explicit empty | false |
| Explicit zero | true |
| Sensitive | false |
| Scope | `deployment` |
| Applicability | `all` |
| Change class | `runtime-replacement` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-source-startup-policy"></a>

## catalog_source_startup_policy

Selects catalog availability before the first upstream reply.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_SOURCE_STARTUP_POLICY` |
| CLI flag | `--catalog-source-startup-policy value` |
| YAML key | `catalog_source_startup_policy` |
| Semantic ID | `catalog.source.startup.policy` |
| Grammar | `string` |
| Accepted names | `prefer_source`, `require_source`, `require_authority`, `prefer_local` |
| Default | `prefer_source` |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `deployment` |
| Applicability | `all` |
| Change class | `runtime-replacement` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-source-authority-id"></a>

## catalog_source_authority_id

Pins an internal authority or permission policy. The require_authority policy needs both identities.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_SOURCE_AUTHORITY_ID` |
| CLI flag | `--catalog-source-authority-id value` |
| YAML key | `catalog_source_authority_id` |
| Semantic ID | `catalog.source.authority.id` |
| Grammar | `string` |
| Default | no internal authority selected |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `deployment` |
| Applicability | `starmap` |
| Change class | `restart` |
| Compatibility | `supported`, schema 1 |
| Source group | `catalog-source` |

<a id="catalog-source-policy-id"></a>

## catalog_source_policy_id

Pins an internal authority or permission policy. The require_authority policy needs both identities.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_SOURCE_POLICY_ID` |
| CLI flag | `--catalog-source-policy-id value` |
| YAML key | `catalog_source_policy_id` |
| Semantic ID | `catalog.source.policy.id` |
| Grammar | `string` |
| Default | no internal authority selected |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `deployment` |
| Applicability | `starmap` |
| Change class | `restart` |
| Compatibility | `supported`, schema 1 |
| Source group | `catalog-source` |

<a id="catalog-source-max-age"></a>

## catalog_source_max_age

Sets the source freshness warning threshold. Zero keeps the default channel freshness thresholds.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_SOURCE_MAX_AGE` |
| CLI flag | `--catalog-source-max-age value` |
| YAML key | `catalog_source_max_age` |
| Semantic ID | `catalog.source.max.age` |
| Grammar | `duration` |
| Default | `6h0m0s` |
| Explicit empty | false |
| Explicit zero | true |
| Sensitive | false |
| Scope | `deployment` |
| Applicability | `all` |
| Change class | `runtime-replacement` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-source-max-hops"></a>

## catalog_source_max_hops

Limits the accepted source chain length.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_SOURCE_MAX_HOPS` |
| CLI flag | `--catalog-source-max-hops value` |
| YAML key | `catalog_source_max_hops` |
| Semantic ID | `catalog.source.max.hops` |
| Grammar | `integer` |
| Default | `8` |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `deployment` |
| Applicability | `all` |
| Change class | `runtime-replacement` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-source-aliases"></a>

## catalog_source_aliases

Names other identities of this runtime for source cycle detection.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_SOURCE_ALIASES` |
| CLI flag | `--catalog-source-aliases value` |
| YAML key | `catalog_source_aliases` |
| Semantic ID | `catalog.source.aliases` |
| Grammar | `string-list` |
| Default | Empty |
| Explicit empty | true |
| Explicit zero | false |
| Sensitive | false |
| Scope | `node` |
| Applicability | `all` |
| Change class | `restart` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-acquisition-enabled"></a>

## catalog_acquisition_enabled

Enables automatic acquisition from configured provider and metadata sources.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_ACQUISITION_ENABLED` |
| CLI flag | `--catalog-acquisition-enabled value` |
| YAML key | `catalog_acquisition_enabled` |
| Semantic ID | `catalog.acquisition.enabled` |
| Grammar | `boolean` |
| Default | `true` |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `deployment` |
| Applicability | `all` |
| Change class | `runtime-replacement` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-acquisition-sources"></a>

## catalog_acquisition_sources

Selects permitted local acquisition inputs. An empty list excludes every acquisition source.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_ACQUISITION_SOURCES` |
| CLI flag | `--catalog-acquisition-sources value` |
| YAML key | `catalog_acquisition_sources` |
| Semantic ID | `catalog.acquisition.sources` |
| Grammar | `string-list` |
| Accepted names | `providers`, `local_catalog`, `models_dev_http`, `models_dev_git` |
| Default | omission keeps host acquisition defaults and existing retained source evidence |
| Explicit empty | true |
| Explicit zero | false |
| Sensitive | false |
| Scope | `deployment` |
| Applicability | `all` |
| Change class | `restart` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-models-dev-git-commit"></a>

## catalog_models_dev_git_commit

Pins models.dev Git acquisition to one exact hexadecimal commit. An empty value clears the pin.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_MODELS_DEV_GIT_COMMIT` |
| CLI flag | `--catalog-models-dev-git-commit value` |
| YAML key | `catalog_models_dev_git_commit` |
| Semantic ID | `catalog.models.dev.git.commit` |
| Grammar | `string` |
| Default | omission keeps the collector pin. Git acquisition requires an exact commit |
| Explicit empty | true |
| Explicit zero | false |
| Sensitive | false |
| Scope | `deployment` |
| Applicability | `all` |
| Change class | `restart` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-acquisition-interval"></a>

## catalog_acquisition_interval

Sets the acquisition period. Zero permits one startup pass when automatic acquisition is on.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_ACQUISITION_INTERVAL` |
| CLI flag | `--catalog-acquisition-interval value` |
| YAML key | `catalog_acquisition_interval` |
| Semantic ID | `catalog.acquisition.interval` |
| Grammar | `duration` |
| Default | `4h0m0s` |
| Explicit empty | false |
| Explicit zero | true |
| Sensitive | false |
| Scope | `deployment` |
| Applicability | `all` |
| Change class | `runtime-replacement` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-provider-bindings"></a>

## catalog_provider_bindings

Selects the complete active provider binding set. An empty array permits no local provider acquisition.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_PROVIDER_BINDINGS` |
| CLI flag | `--catalog-provider-bindings value` |
| YAML key | `catalog_provider_bindings` |
| Semantic ID | `catalog.provider.bindings` |
| Grammar | `provider-bindings` |
| Default | omission retains legacy unscoped acquisition; an explicit array selects only its declared bindings |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `deployment` |
| Applicability | `all` |
| Change class | `restart` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-coalesce-window"></a>

## catalog_coalesce_window

Bounds the wait before completed provider observations publish.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_COALESCE_WINDOW` |
| CLI flag | `--catalog-coalesce-window value` |
| YAML key | `catalog_coalesce_window` |
| Semantic ID | `catalog.coalesce.window` |
| Grammar | `duration` |
| Default | `30s` |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `deployment` |
| Applicability | `all` |
| Change class | `runtime-replacement` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-workspace-path"></a>

## catalog_workspace_path

Names the reviewed operator catalog input.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_WORKSPACE_PATH` |
| CLI flag | `--catalog-workspace-path value` |
| YAML key | `catalog_workspace_path` |
| Semantic ID | `catalog.workspace.path` |
| Grammar | `string` |
| Default | hosting application's workspace |
| Explicit empty | true |
| Explicit zero | false |
| Sensitive | false |
| Scope | `node` |
| Applicability | `all` |
| Change class | `restart` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-startup-spread"></a>

## catalog_startup_spread

Spreads cold automatic work across a stable time window. Zero disables the spread.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_STARTUP_SPREAD` |
| CLI flag | `--catalog-startup-spread value` |
| YAML key | `catalog_startup_spread` |
| Semantic ID | `catalog.startup.spread` |
| Grammar | `duration` |
| Default | `15m0s` |
| Explicit empty | false |
| Explicit zero | true |
| Sensitive | false |
| Scope | `deployment` |
| Applicability | `all` |
| Change class | `runtime-replacement` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-transfer-idle-timeout"></a>

## catalog_transfer_idle_timeout

Bounds a transfer that makes no progress.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_TRANSFER_IDLE_TIMEOUT` |
| CLI flag | `--catalog-transfer-idle-timeout value` |
| YAML key | `catalog_transfer_idle_timeout` |
| Semantic ID | `catalog.transfer.idle.timeout` |
| Grammar | `duration` |
| Default | `2m0s` |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `deployment` |
| Applicability | `all` |
| Change class | `runtime-replacement` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-transfer-max-duration"></a>

## catalog_transfer_max_duration

Bounds one finite HTTP body transfer. It does not bound an open event stream.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_TRANSFER_MAX_DURATION` |
| CLI flag | `--catalog-transfer-max-duration value` |
| YAML key | `catalog_transfer_max_duration` |
| Semantic ID | `catalog.transfer.max.duration` |
| Grammar | `duration` |
| Default | `1h0m0s` |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `deployment` |
| Applicability | `all` |
| Change class | `runtime-replacement` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-refresh-timeout"></a>

## catalog_refresh_timeout

Limits the time for one refresh. Zero adds no cap.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_REFRESH_TIMEOUT` |
| CLI flag | `--catalog-refresh-timeout value` |
| YAML key | `catalog_refresh_timeout` |
| Semantic ID | `catalog.refresh.timeout` |
| Grammar | `duration` |
| Default | `0s` |
| Explicit empty | false |
| Explicit zero | true |
| Sensitive | false |
| Scope | `deployment` |
| Applicability | `all` |
| Change class | `runtime-replacement` |
| Compatibility | `supported`, schema 1 |

<a id="state-dir"></a>

## state_dir

Names the runtime state directory for this process.

| Property | Value |
|---|---|
| Environment | `STARMAP_STATE_DIR` |
| CLI flag | `--state-dir value` |
| YAML key | `state_dir` |
| Semantic ID | `state.dir` |
| Grammar | `string` |
| Default | hosting application's state directory |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `node` |
| Applicability | `all` |
| Change class | `restart` |
| Compatibility | `supported`, schema 1 |

<a id="scheduler-identity"></a>

## scheduler_identity

Sets a stable identity for this runtime instance.

| Property | Value |
|---|---|
| Environment | `STARMAP_SCHEDULER_IDENTITY` |
| CLI flag | `--scheduler-identity value` |
| YAML key | `scheduler_identity` |
| Semantic ID | `scheduler.identity` |
| Grammar | `string` |
| Default | runtime-derived instance identity |
| Explicit empty | true |
| Explicit zero | false |
| Sensitive | false |
| Scope | `node` |
| Applicability | `all` |
| Change class | `restart` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-permission-clock-source"></a>

## catalog_permission_clock_source

Selects native permission clock evidence. Native mode requires qualified bounds for this host.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_PERMISSION_CLOCK_SOURCE` |
| CLI flag | `--catalog-permission-clock-source value` |
| YAML key | `catalog_permission_clock_source` |
| Semantic ID | `catalog.permission.clock.source` |
| Grammar | `string` |
| Accepted names | `disabled`, `native` |
| Default | no native observations |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `node` |
| Applicability | `` |
| Change class | `restart` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-permission-clock-refresh-interval"></a>

## catalog_permission_clock_refresh_interval

Sets the background observation period. It must be less than half the maximum cache age.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_PERMISSION_CLOCK_REFRESH_INTERVAL` |
| CLI flag | `--catalog-permission-clock-refresh-interval value` |
| YAML key | `catalog_permission_clock_refresh_interval` |
| Semantic ID | `catalog.permission.clock.refresh.interval` |
| Grammar | `duration` |
| Default | operator-qualified bound |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `node` |
| Applicability | `native permission clock` |
| Change class | `restart` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-permission-clock-max-age"></a>

## catalog_permission_clock_max_age

Bounds cached observation age to at most five minutes.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_PERMISSION_CLOCK_MAX_AGE` |
| CLI flag | `--catalog-permission-clock-max-age value` |
| YAML key | `catalog_permission_clock_max_age` |
| Semantic ID | `catalog.permission.clock.max.age` |
| Grammar | `duration` |
| Default | operator-qualified bound |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `node` |
| Applicability | `native permission clock` |
| Change class | `restart` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-permission-clock-max-drift-ppm"></a>

## catalog_permission_clock_max_drift_ppm

Bounds elapsed-counter rate error in parts per million, below one million.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_PERMISSION_CLOCK_MAX_DRIFT_PPM` |
| CLI flag | `--catalog-permission-clock-max-drift-ppm value` |
| YAML key | `catalog_permission_clock_max_drift_ppm` |
| Semantic ID | `catalog.permission.clock.max.drift.ppm` |
| Grammar | `integer` |
| Default | operator-qualified bound |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `node` |
| Applicability | `native permission clock` |
| Change class | `restart` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-permission-clock-counter-uncertainty"></a>

## catalog_permission_clock_counter_uncertainty

Bounds each elapsed-counter reading error. Supply a positive duration, at most thirty seconds.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_PERMISSION_CLOCK_COUNTER_UNCERTAINTY` |
| CLI flag | `--catalog-permission-clock-counter-uncertainty value` |
| YAML key | `catalog_permission_clock_counter_uncertainty` |
| Semantic ID | `catalog.permission.clock.counter.uncertainty` |
| Grammar | `duration` |
| Default | operator-qualified bound |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `node` |
| Applicability | `native permission clock` |
| Change class | `restart` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-permission-clock-windows-max-source-age"></a>

## catalog_permission_clock_windows_max_source_age

Bounds Windows synchronization age to at most one day.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_PERMISSION_CLOCK_WINDOWS_MAX_SOURCE_AGE` |
| CLI flag | `--catalog-permission-clock-windows-max-source-age value` |
| YAML key | `catalog_permission_clock_windows_max_source_age` |
| Semantic ID | `catalog.permission.clock.windows.max.source.age` |
| Grammar | `duration` |
| Default | operator-qualified bound |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `node` |
| Applicability | `native permission clock on Windows` |
| Change class | `restart` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-permission-clock-windows-max-source-drift-ppm"></a>

## catalog_permission_clock_windows_max_source_drift_ppm

Bounds Windows synchronization source drift in parts per million, below one million.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_PERMISSION_CLOCK_WINDOWS_MAX_SOURCE_DRIFT_PPM` |
| CLI flag | `--catalog-permission-clock-windows-max-source-drift-ppm value` |
| YAML key | `catalog_permission_clock_windows_max_source_drift_ppm` |
| Semantic ID | `catalog.permission.clock.windows.max.source.drift.ppm` |
| Grammar | `integer` |
| Default | operator-qualified bound |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `node` |
| Applicability | `native permission clock on Windows` |
| Change class | `restart` |
| Compatibility | `supported`, schema 1 |

<a id="catalog-permission-clock-windows-source-uncertainty"></a>

## catalog_permission_clock_windows_source_uncertainty

Adds qualified Windows source error. Supply a positive duration, at most thirty seconds.

| Property | Value |
|---|---|
| Environment | `STARMAP_CATALOG_PERMISSION_CLOCK_WINDOWS_SOURCE_UNCERTAINTY` |
| CLI flag | `--catalog-permission-clock-windows-source-uncertainty value` |
| YAML key | `catalog_permission_clock_windows_source_uncertainty` |
| Semantic ID | `catalog.permission.clock.windows.source.uncertainty` |
| Grammar | `duration` |
| Default | operator-qualified bound |
| Explicit empty | false |
| Explicit zero | false |
| Sensitive | false |
| Scope | `node` |
| Applicability | `native permission clock on Windows` |
| Change class | `restart` |
| Compatibility | `supported`, schema 1 |
