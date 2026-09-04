# Changelog

Starmap records all notable changes in this file.

This file follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)
and [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- The catalog discovery channel moves from the `catalog-latest` release to the
  `catalog/v1` branch, and its document is now `channel.json`. This repository
  enables GitHub immutable releases, which freeze a release at creation. A
  frozen release accepts no asset replacement and no asset deletion, so the
  publisher could not advance the pointer.
- Scheduled run 33855316906 created its immutable content release. It then
  failed at `gh release upload catalog-latest --clobber` with
  `HTTP 422: Cannot delete asset from an immutable release`. The channel stuck
  at its first sequence. The content now stays immutable in a
  `catalog-<digest>` release, and the pointer stays mutable on a branch.
- A connected runtime with a catalog store now serves the generation ID that
  its retained layers derive. The durable commit previously replaced that
  identity with a fresh one. A served generation ID then named bytes that no
  store held. An in-memory runtime and a durable runtime now report one
  identity for one set of layers. A restart reports that identity again.
- A derived effective generation ID now separates the upstream identity from
  the local digest with `.local.` instead of `+local.`. A published identity
  travels as one URL path segment. The remote catalog protocol accepts only
  letters, digits, dot, dash, and underscore there.
- A durable runtime without an upstream layer now derives its identity from
  the root generation. Its restart baseline is the generation that the
  previous run committed, so a derivation from that identity nested one more
  suffix and committed one more generation on every restart.

### Added

- `starmap.WithCandidateGenerationID` binds one publication candidate to a
  generation ID that the caller derives. `Client.Update` publishes that
  identity. A candidate without the option still gets one fresh identity.

### Changed

- The GitHub catalog source reads the channel document from the repository
  contents endpoint at the channel branch ref. The request is conditional. The
  source no longer reads a channel release or its asset. One changed discovery
  cycle therefore drops from eight requests to seven.
- `STARMAP_CATALOG_SOURCE_CHANNEL` now names a branch ref. Its default is
  `catalog/v1`.
- Public catalog concepts now use one concept-owned package tree under
  `pkg/catalogs`. Evidence, projection, storage, artifact, and remote protocol
  behavior have separate child packages.
- Immutable `Generation` values and canonical catalog payload codecs now
  belong to the `catalogs` root. Artifact and remote protocol packages no
  longer depend on storage to exchange a catalog generation.

### BREAKING CHANGES

- The catalog channel is a branch, and the source keeps no path to the retired
  `catalog-latest` release. The channel protocol stays at v1, because no
  external consumer reads it yet. A caller that pinned `catalog-latest` must
  set `STARMAP_CATALOG_SOURCE_CHANNEL` to `catalog/v1`.
- Follow the [v0.5.0 migration guide](docs/MIGRATING_TO_V0.5.md). The release
  removes `pkg/catalogmeta`, `pkg/catalogstore`, `pkg/catalogartifact`, and
  `pkg/catalogremote` without wrappers or compatibility aliases.
- Generation stores move to `pkg/catalogs/storage`. The S3 adapter moves to
  `pkg/catalogs/storage/s3`.
- Portable artifact and versioned wire clients move to
  `pkg/catalogs/artifact` and `pkg/catalogs/remote`.
- This release does not change catalog schema 5, generation manifest 2,
  artifact bytes, stored generations, or the remote wire protocol.

## [0.4.0] - 2026-08-11

### Added

- **Catalog-owned credential contracts** define credential fields, profiles,
  request placement, endpoint bindings, and separate credential planes.
  Provider records declare conventional environment names. Starmap derives
  its product-specific names from the same field IDs.
- **Credential source lifecycle** resolves ambient environment values and
  `env:` and `file:` references. It also resolves default cloud chains and
  direct reads from the five supported secret stores. Values remain outside
  catalog generations.
- **Durable model review candidates** preserve each excluded provider
  offering. Each record contains the exact provider model ID, source
  observation, revision, checksum, reason, and prior reviewed model link.
  Evidence-only updates can publish a new immutable generation without a
  canonical catalog change.
- **Typed provider acquisition mappings** let provider YAML select fields,
  authors, and documented capability predicates from a compiled transport
  vocabulary. Unknown provider model IDs remain review candidates until an
  offering links them to an authored model.
- **Durable remote subscriber state** uses a caller-owned catalog store and an
  optional pinned bootstrap. A context-aware constructor loads one atomic
  catalog identity that includes its payload checksum.

### Changed

- OpenAI-compatible, Anthropic, and Google acquisition adapters now project
  only facts present in provider responses or provider YAML. Catalog author
  mappings replace adapter-local author fallbacks. Google Vertex no longer
  adds a compiled Model Garden roster.
- Provider acquisition resolves credentials from catalog metadata instead of
  provider-specific switches. The public source interfaces now receive
  request-scoped credential material.
- Remote subscribers retain a verified durable or pinned catalog during
  nonterminal startup failures and recover through the normal stream lifecycle.
  HTTP 401 and 403 responses remain terminal.
- Shared dependency minima now cover every newer version in the Starport
  consumer graph. These include Google, file notification, and telemetry
  modules.

### BREAKING CHANGES

- Follow the [v0.4.0 migration guide](docs/MIGRATING_TO_V0.4.md). Complete it
  before you reuse custom YAML, durable data, fetchers, or remote subscribers.
- Catalog schema version 5 replaces provider `api_key`, `env_vars`, and
  `catalog.auth` fields with `credentials`. It removes endpoint
  `base_url_env_var`, `path`, and provider `catalog.authors` fields. Endpoint
  URL templates and credential endpoint bindings now own variable expansion.
- Catalog schema version 5 removes provider `feature_rules`. Model-family
  strings no longer infer capabilities in acquisition code.
- Generation manifest version 2 requires the ordered `review_candidates`
  array. Each candidate must match a linked source observation.
- `starmap.NewCandidate` now accepts `starmap.CandidateEvidence`. This change
  removes the prior variadic source-observation argument.
- `remote.Config` now requires a caller-owned `CatalogStore`. Use
  `remote.NewContext` when store I/O needs cancellation or a deadline.
- `sources.ProviderClient.ListModels` and `sources.ProviderRawFetcher` now
  receive `sources.ProviderCredentialMaterial`. Inject a
  `sources.ProviderCredentialResolver`. This release removes the old
  credential-loading and missing-key options.
- This release removes the provider API-key and environment-value types. It
  also removes the provider validation report functions. Read the typed
  `Provider.Credentials` contract or use the provider CLI for readiness.

## [0.3.0] - 2026-08-03

### Added

- **Provider inference contracts** define base URLs, operation paths, stream
  paths, author protocols, status sources, and offering capabilities. The
  embedded provider catalog now includes Mistral, Azure OpenAI, and Ollama.
- **Catalog-acquisition authentication contracts**: each provider declares a
  typed acquisition method independently from its endpoint protocol. Supported
  methods include API keys and cloud default credential chains. A provider can
  also require no credentials.
- **Cloud workload identity support**: Google catalog acquisition accepts
  external-account credentials. It also accepts the SDK default credential
  chain without a local credential file.
- **Tenant observation inputs**: callers can add installation-specific provider
  definitions and offerings to one immutable catalog generation. Examples
  include Azure deployments and local Ollama models.

### Changed

- Endpoint projections use schema version 2 and preserve each offering's exact
  operations, service endpoints, stream endpoints, and prompt-cache support.
- Provider and offering read views, copies, validation, diffs, CLI output, and
  generated OpenAPI documents include the new provider contracts.
- Catalog authentication checks select a credential resolver from the typed
  provider contract. Endpoint type no longer selects credentials.

### BREAKING CHANGES

- `ProviderCatalog` now owns a required `Auth` contract. This release removes
  `ProviderEndpoint.AuthRequired`.
- The operation-based `Provider.Inference` service contract replaces
  `Provider.ChatCompletions`.
- `ProviderOffering.Endpoints` replaces `ProviderOffering.Endpoint`. Offerings
  now declare service capabilities.
- The generated endpoint projection format changes from schema version 1 to
  schema version 2.

## [0.2.0] - 2026-07-30

### Added

- **Fresh reviewed embedded catalog**: refreshed provider and models.dev source
  observations, added Moonshot AI's Kimi K3 authored definition and serving
  record, and refreshed Google Vertex through local Application Default
  Credentials. The immutable embedded generation contains reviewed canonical
  author links for provider offerings and passes generation, schema, freshness,
  size, and cross-reference validation.
- **Human-editable capability checklist**: model YAML now renders every Boolean
  capability, using conservative `false` defaults for missing observations
  while preserving explicit `null`, `false`, and `true` distinctions in the
  immutable catalog. Untouched projection defaults do not override later
  authoritative source observations.
- **Safer live-source admission**: live provider observations must join a
  reviewed `author/model` definition before publication; unreviewed records are
  quarantined with actionable observation evidence. Provider aliases are
  resolved consistently when models.dev observations are reconciled.
- **CLI and server reliability fixes**: structured provider-test output keeps
  progress on stderr, first-run workspace parents are created safely, model
  search uses its documented route, configured CORS origins enable CORS,
  synchronous updates receive the full acquisition/projection deadline, and
  provider-specific token price units are normalized explicitly.
- **Pure-Go release contract**: required verification executes the external
  library, store, server, remote, and CLI compositions with `CGO_ENABLED=0`;
  race verification remains explicitly cgo-enabled. GoReleaser archives,
  container images, and Homebrew installs now verify cgo-disabled metadata and
  target-appropriate static/system-only linkage.
- **Embeddable Go server**: the public `server` package accepts
  `*starmap.Client`, returns a concrete server, and exposes caller-owned
  `Serve`, `Handler`/`Start`, and draining `Shutdown` lifecycles. Read-only
  serving does not import provider clients or acquisition implementations;
  `server.WithSyncer` explicitly enables the update route when a deployment
  wants live source acquisition. A real external `GOWORK=off` module constructs,
  serves, reaches, drains, and stops it.
- **Caller-owned construction lifecycle**: `starmap.NewContext` propagates the
  caller's cancellation and deadline through storage-backed generation loading
  and workspace repair. `starmap.New` remains the background-context
  convenience constructor. `(*Client).Catalog()` now has explicit nil-receiver
  semantics: it returns nil for a nil client while remaining non-failing,
  non-nil, O(1), and allocation-free after successful construction.
- **Explicit local layout migration**: `starmap migrate catalog` validates and
  locks the complete pre-plan generation store at `catalog_path`, moves it to
  the canonical machine state root, and projects the exact current generation
  back as editable provider YAML. Failures roll back the move; restart repairs
  a projection interrupted after relocation. Newer schema generations are
  rejected before filesystem mutation.
- Clients configured with `WithCatalogStore` now recover and publish the exact
  durable current generation during `starmap.New`, so server and remote updates
  survive process restart. Server updates may select a single source with the
  `source` query parameter; a configured `WithCatalogPath` is also the default
  sync workspace when `sync.WithCatalogPath` is omitted.
- Catalog publication observers no longer head-of-line block one another.
  Server logging middleware preserves error-reporting SSE flushes, while HTTP
  catalog responses, SSE publication hints, and cache state correlate the same
  durable generation identity after commit.
- Catalog publication callbacks now begin in generation order and use a
  bounded newest-pending coalescing policy instead of silently dropping a
  complete generation when fixed delivery slots are saturated. Atomic catalog
  state supplies the event sequence directly; sequence gaps and
  `HookStats().Coalesced` expose overload without delaying the durable commit.
- **One reactive transport**: SSE is the sole server-to-client publication
  transport. Each connection has one serialized request-owned writer for
  `catalog.published` hints and flushed comment heartbeats. The default
  heartbeat interval is 20 seconds, every frame has a 10-second write deadline,
  and a backpressured or failed connection is terminated so reconnect catch-up
  can recover. The unused WebSocket, generic broker, model-event adapters, and
  their public route were removed before launch.
- **Verified remote publisher boundary**: reactive and low-level remote clients
  require non-loopback publishers to use HTTPS, require every HTTPS response to
  retain a standard verified certificate chain, and reject cross-origin
  redirects. Manifest-declared media and body limits are checked before the
  immutable payload is downloaded; any identity, size, digest, schema, or
  stale-generation failure leaves the active catalog unchanged.
- **Explicit polling fallback**: reactive subscribers never poll during a
  healthy heartbeat/event stream and keep polling disabled by default.
  `PollingFallbackPolicy` can opt into a consecutive-stream-failure threshold,
  and a minimum request interval. Requests use `If-None-Match`,
  current fallback state is observable, and verified reconnect catch-up stops
  fallback before event consumption resumes.
- **Terminal remote authentication failures**: HTTP 401 and 403 stop the
  one-shot subscriber lifecycle across stream, catch-up, addressed fetch, and
  conditional fallback paths. They never trigger hidden retry or polling;
  callers construct a new subscriber after correcting credentials or policy.
- **Distinct catalog and stream health**: Publishers and subscribers expose
  active generation freshness independently from heartbeat/event liveness,
  with retry/catch-up/error state plus observable hook coalescing and SSE
  backpressure or write termination.

### BREAKING CHANGES
- **One canonical human catalog workspace**: `catalog_path`,
  `WithCatalogPath`, and CLI `--catalog-path` all name the same provider-YAML
  read/write workspace, defaulting to `~/.starmap/catalog`. Machine-owned
  immutable generations are separate at `~/.starmap/state/catalog` in the CLI
  composition. `catalog_export_path`, `WithCatalogExportPath`, `--input-dir`,
  and `--output-dir` were removed before launch rather than retained as
  compatibility aliases. A pre-plan generation-store layout at the new
  workspace path returns `errors.LegacyCatalogLayoutError` before mutation and
  must be moved through the explicit transactional migration flow.
- **Scheduling moved above the core client**: `starmap.Client` no longer owns a
  ticker goroutine or cadence lifecycle. `AutoUpdatesOn`, `AutoUpdatesOff`,
  `WithAutoUpdatesEnabled`, `WithAutoUpdatesDisabled`,
  `WithAutoUpdateInterval`, `AutoUpdateFunc`, `AutoUpdateContextFunc`,
  `WithAutoUpdateFunc`, and `WithAutoUpdateContextFunc` were removed.
  Deployments and Starport own cadence, jitter, retry, leases, and startup
  policy. Provider/source synchronization is the explicit opt-in
  `acquisition.Syncer`; custom publication passes a context-aware callback
  directly to `Client.Update`:

  ```go
  publication, err := sm.Update(ctx, func(
      ctx context.Context,
      current *catalogs.Catalog,
  ) (*starmap.Candidate, error) {
      return starmap.NewCandidate(updatedCatalog, starmap.CandidateEvidence{})
  })
  ```

- **Canonical model-definition and offering lookup**:
  `catalog.FindModel(id)` now returns `catalogs.ModelDefinition`. Provider price,
  limits, availability, modes, and request behavior are read through
  `catalog.Offering(providerID, providerModelID)`. The prelaunch flattened
  `Models`, `ProviderModel`, `ProviderModels`, and `LegacyV0` reads were deleted
  because they discard provider identity. Canonical catalogs are schema
  version 3.
  The duplicate nested `tokens.cache` representation and unused
  `architecture.precision` alias were also removed; cache pricing persists only
  as `cache_read`/`cache_write`, and quantization has one typed field.
- **One canonical CLI spelling**: structured output uses `--output`/`-o` and
  update previews use `--dry-run`. The prelaunch `--format`, `--fmt`, `--dry`,
  `FORMAT`, `inspect`, and `server` aliases were removed rather than becoming
  permanent compatibility surface.

- **`Client.Catalog()` now returns a concrete immutable catalog**: the old
  `Catalog() (catalogs.Snapshot, error)` signature is replaced by
  `Catalog() *catalogs.Catalog`. After `starmap.New` succeeds, catalog access is
  non-failing, non-nil, O(1), and safe to retain across goroutines:

  ```go
  catalog := sm.Catalog()
  model, err := catalog.FindModel("gpt-4o")
  ```

  `catalogs.Catalog` has unexported state and read-only methods; its collection
  readers expose no set, delete, clear, merge, copy, or save operations. The
  former exported `catalogs.Snapshot` lifecycle interface was removed. Advanced
  catalog producers may use the concrete `*catalogs.Builder` and call
  `Builder.Build()`; create a new draft from an immutable catalog with
  `catalogs.NewBuilderFrom(catalog)`. Builder remains public for custom update
  callbacks and source/plugin authors, not for ordinary read consumers.

  `starmap.New` now returns concrete `*starmap.Client`. The one-implementation
  root `Client`, `Updater`, `AutoUpdater`, `Hooks`, and `Persistence` interfaces
  were removed; consumers should define narrow interfaces at their own use
  sites when substitution is needed.

  The client deep-copies a builder once, validates/builds the immutable catalog,
  and atomically swaps one complete generation after persistence. Catalogs
  precompute alias-aware definition and offering indexes; use `Offering` or
  `ProviderOfferings` for provider-specific service facts rather than a lossy
  bare-ID model view. Advanced builders retain provider-scoped
  `ProviderModel`/`ProviderModels` reads while constructing candidates.

- **Sync option contract corrected**:
  - Removed `sync.WithAutoApprove`; confirmation belongs to the CLI and core
    synchronization never prompts. Remove this option from programmatic calls.
  - Removed `sync.WithFailFast`; it was stored but never affected concurrent
    source fetching. Remove this option; existing source errors remain typed and
    fatal until the source-observation policy provides explicit partial-success
    semantics.
  - Removed `sources.WithSafeMode` and `sources.WithFresh`; neither source-level
    option had an implementation. Use `sync.WithFresh(true)` for an explicitly
    destructive replacement sync. Default reconciliation remains non-destructive
    according to source merge and field-authority policy.
  - Removed the implicit root remote options. Low-level protocol consumers may
    construct `catalogremote.Client` explicitly and pass a verified generation
    to `starmap.Client.Activate`; normal reactive consumers use the opt-in
    public `remote` package.
  - Programmatic `sync.WithSources` now rejects unknown source IDs and copies
    caller input. A fresh sync rejects `local_catalog` because an existing local
    catalog cannot also be the input to a replacement generation.
  - Explicit models.dev Git verification no longer follows the floating `dev`
    branch. Supply `sync.WithModelsDevGitCommit(exactCommit)` or CLI flag
    `--models-dev-git-commit`; Starmap checks out that detached commit, installs
    with `bun install --frozen-lockfile`, and records the `bun.lock` SHA-256 in
    source-observation revision metadata.

- **Remote catalog protocol is generation-based and versioned**: the ad-hoc
  unversioned `GET /catalog` envelope was removed. Construct
  `catalogremote.Client` with the versioned API base (for example
  `https://catalog.example.com/api/v1`). Consumers now read
  `GET /catalog/manifest` or an immutable addressed manifest and then
  `GET /catalog/generations/{generation_id}/snapshot`; schema compatibility,
  media type, size, and SHA-256 are verified before durable publication. The
  public `remote` subscriber performs the verified initial fetch, listens for
  the sole `catalog.published` SSE hint, deduplicates immutable generations,
  and performs mandatory catch-up after every reconnect. Validated heartbeat
  and liveness settings detect silent streams, while caller cancellation and a
  bounded `Close` own and join initial fetch, stream reading, retry, and
  activation lifecycles.

- **Restructured Auth Commands**: Simplified authentication command structure
  - **Removed**: `starmap providers auth` (entire subcommand tree)
  - **New**: `starmap auth` (top-level command in "setup" group, alongside `starmap deps`)
  - **Available**: `starmap auth gcloud` (Google Cloud authentication helper)
  - **Migration**:
    - For auth status: Use `starmap providers` (shows auth status with all provider info)
    - For credential testing: Use `starmap providers --test` instead of `starmap providers auth test`
    - For Google Cloud auth: Use `starmap auth gcloud` instead of `starmap providers auth gcloud`
  - **Rationale**: Consolidate provider information and simplify command hierarchy

### Added
- **Catalog generation manifest contract**: added the transport-neutral
  `catalogs.GenerationManifest`, a checked-in JSON Schema and payload fixture,
  exact SHA-256/size verification, validator/check results, sync-run and source
  observation correlation, completeness/degradation state, and catalog-schema
  compatibility independent of binary versions. Atomic store activation follows
  in the transactional catalog-store work.
- **Generation-oriented CatalogStore**: added one CAS-based store contract and
  a shared conformance suite covering memory, filesystem, SQLite, and
  conditional object-storage adapters. Generations are validated before commit,
  retained immutably, defensively copied, and activated only when the expected
  current ID matches; identical retries are idempotent.
- **Deletion-correct catalog saves**: builder saves replace Starmap-managed YAML
  indexes and provider model trees and remove obsolete author-model
  directories, preventing deleted records from reappearing after reload while
  preserving unmanaged neighboring files. Generation stores already replace
  the payload as one immutable unit.
- **Configured local catalog failures are visible**: a missing optional path
  still falls back to the embedded bootstrap, while existing corrupt YAML,
  unreadable managed files, and invalid provider/author records now propagate
  typed errors and make `starmap.New` fail before publication.
- **Single persisted model source**: catalog schema version 2 persists provider
  model records and provenance only. Definitions, offerings, and author
  membership are immutable validated read views; the duplicate author-model
  tree and its prelaunch migration adapter were removed.
- **New `starmap auth` Command**: Top-level authentication helper in "setup" group
  - `starmap auth gcloud` → Google Cloud authentication setup (ADC configuration)
  - Provides guidance to use `starmap providers` for viewing auth status
  - Provides guidance to use `starmap providers --test` for testing credentials

- **`--test` Flag for Providers**: New flag to test provider credentials via API calls
  - `starmap providers --test` → Test all configured providers
  - `starmap providers openai --test` → Test specific provider
  - `--timeout` flag controls API call timeout (default: 10s)
  - Runs concurrent tests in TTY mode for faster execution
  - Shows response time, model count, and detailed errors

### Changed
- **Dependency prompts are CLI-owned**: Library, server, scheduler, and other
  non-CLI sync calls no longer read stdin. Optional sources with missing tools
  are skipped by default, required sources return `DependencyError`, and the
  update command supplies an explicit interactive decision adapter.

- **Enhanced `starmap providers` Output**: Now shows comprehensive provider information in unified table
  - Added columns: TYPE (endpoint type), ENV KEY, KEY (masked), MODELS (count)
  - Reordered columns: NAME, ID, LOCATION, TYPE, ENV KEY, KEY, MODELS, STATUS
  - Combines functionality of both `providers` and `providers auth` commands
  - All existing flags (--search, --limit, --output) continue to work
  - Detail view for single provider preserved

### Improved
- **Stderr Suppression**: Replaced platform-specific implementation with idiomatic cross-platform solution
  - Removed build tags (`//go:build darwin` and `//go:build !darwin`)
  - Removed syscall manipulation (no more `syscall.Dup`, `syscall.Dup2`)
  - Single `stderr.go` file instead of `stderr_darwin.go` and `stderr_other.go`
  - Pure Go implementation using `os.Pipe()` and `io.Copy(io.Discard)`
  - Works on all platforms (Darwin, Linux, Windows), not just macOS
  - No linter exceptions needed (removed `//nolint:gosec`)
  - Cleaner, more maintainable code following Go best practices

## [0.0.24] - 2025-10-21

### BREAKING CHANGES
- **Authentication Command Rename**: `starmap providers auth verify` → `starmap providers auth test`
  - Old: `starmap providers auth verify`
  - New: `starmap providers auth test`
  - Rationale: "test" is more accurate - command actually tests credentials by making API calls
  - Migration: Update scripts/docs using `auth verify` to use `auth test`

### Added
- **Concurrent Provider Testing**: Tests now run in parallel for significantly faster execution
  - TTY mode: All provider APIs tested concurrently using goroutines
  - Non-TTY mode: Sequential testing preserved for clear line-by-line output
  - Total test time reduced from sum of all tests to max of slowest test
  - Three-phase architecture: pre-flight checks → concurrent API calls → result collection
  - Proper error handling with panic recovery in goroutines

### Changed
- **Improved Auth Status Output**:
  - Reordered columns: PROVIDER, AUTH SOURCE, ENV KEY, KEY (preview), STATUS
  - Added masked key preview in status table
  - Removed redundant summary table (kept helpful hints)
- **Default Auth Behavior**: `starmap providers auth` now defaults to showing status (same as `auth status`)
- **ASCII Symbols**: Replaced emojis with universally-compatible ASCII symbols
  - Success: ✓ (check mark)
  - Error: ✗ (ballot X)
  - Warning: ! (exclamation)
  - Optional: - (dash)
  - Unsupported: × (multiplication)
  - Unknown: ? (question mark)
- **Simplified Test Output**: Clean progress message → concurrent testing → final results table

### Fixed
- **Concurrent stderr Suppression**: Fixed SDK warnings appearing during parallel testing
  - Root cause: Multiple goroutines manipulating same stderr file descriptor
  - Solution: Single stderr suppression wrapping all concurrent operations
  - Result: Clean output without SDK warnings
- **Code Quality**: Removed unused parameters and imports throughout auth package

### Technical Details
- Pre-allocated slices for better performance
- Proper context cancellation in all goroutines
- Buffered channels sized to number of providers
- Thread-safe result collection with WaitGroup synchronization
- All tests passing, linter clean (0 issues)

## [0.0.23] - 2025-10-21

### Changed
- **CLI Improvement**: Renamed `models provenance` command to `models history` for better user experience
  - Old: `starmap models provenance gpt-4o`
  - New: `starmap models history gpt-4o`
  - Rationale: "history" is more intuitive terminology for field-level source tracking
- **Enhanced History Command**: Improved field filtering with multiple fields and case-insensitive matching
  - Renamed `--field` → `--fields` (plural, more intuitive)
  - Support multiple fields: `--fields=Name,ID,Pricing.Input`
  - Case-insensitive matching: `--fields=name` matches "Name" field
  - Wildcard patterns now case-insensitive: `--fields='pricing.*'` matches "Pricing.Input"

### Removed
- Removed `starmap providers provenance` command (provider-level tracking no longer needed)
- Removed `starmap authors provenance` command (author-level tracking no longer needed)
- Only model-level field history tracking is retained as it's the primary use case

## [0.0.22] - 2025-10-20

### BREAKING CHANGES
- **CLI Restructuring**: Migrated from verb-first to resource-first command structure for improved discoverability and consistency
  - `starmap list models` → `starmap models list`
  - `starmap fetch models` → `starmap providers fetch`
  - `starmap auth verify` → `starmap providers auth verify`
  - `starmap auth status` → `starmap providers auth status`
  - `starmap auth gcloud` → `starmap providers auth gcloud`
  - See commit 2015cd0d for complete migration guide and rationale

### Changed
- **Documentation**: All markdown documentation updated to reflect new CLI structure
  - Updated README.md with new command examples
  - Updated CONTRIBUTING.md with new development patterns
  - Updated docs/CLI.md with new command reference
  - Updated docs/ARCHITECTURE.md with new CLI architecture
  - Updated scripts/demo.tape VHS demo script
- **Makefile**: Fixed completion installation command (`starmap completion install`)
- **Internal References**: Updated all error messages, hints, and code comments with new command patterns

### Fixed
- Lint error in `cmd/starmap/cmd/embed/ls.go` (unused parameter)
- Shell completion installation now uses correct command order
- Contextual hints now reference correct command paths

### Technical Details
- No functionality removed - 100% feature parity maintained
- All 27 flags preserved across commands (20 command-specific + 7 global)
- Auth commands reused directly from old structure (zero implementation changes)
- GoReleaser configuration updated for new command structure

## [0.0.15] - 2025-10-15

### Added
- **Production Logging & Metrics** - Comprehensive observability following industry best practices
  - Runtime metrics: uptime, goroutines, memory usage
  - Event metrics: events published, dropped, queue depth
  - Enhanced `/api/v1/stats` endpoint with structured metric grouping
  - Follows Prometheus/Grafana/Kubernetes patterns for monitoring

- **CLI Logging Enhancements** - Hybrid logging pattern with clear precedence
  - `--log-level` flag for explicit level control (trace, debug, info, warn, error)
  - `-v/--verbose` shortcut for debug level
  - `-q/--quiet` shortcut for warn level
  - Clear precedence hierarchy following kubectl/docker patterns
  - Comprehensive validation with user-friendly warnings

### Fixed
- **Embedded Catalog Loading** - Critical fix for immediate catalog availability
  - Fixed empty catalog issue on startup (was showing 0 models/providers)
  - Embedded catalog now loads immediately instead of waiting for auto-update
  - Main catalog populated with embedded data during client initialization
  - Users now see 436+ models and 7 providers instantly

- **Server Logging** - Production-ready log levels and clarity
  - Internal subscriber registration moved from INFO to DEBUG level
  - Removed confusing "Subscriber registered" messages from production logs
  - Added descriptive transport subscription messages (WebSocket/SSE)
  - Improved startup log ordering for better readability

- **Server Stability** - Deadlock prevention and clean operations
  - Buffered broker event channels to prevent startup deadlocks
  - Buffered WebSocket hub channels for reliable message delivery
  - Buffered SSE broadcaster channels for stable streaming
  - Added favicon handler (returns 204 No Content, eliminates 404 spam)

### Changed
- Event metrics no longer expose internal subscriber count (implementation detail)
- Log levels now follow industry standard: DEBUG for internal wiring, INFO for user-facing events

## [0.0.14] - 2024-10-09

### Added
- **HTTP Server** - Production-ready REST API with real-time updates
  - RESTful endpoints for models, providers, and catalog operations
  - WebSocket support for real-time catalog updates (`/api/v1/updates/ws`)
  - Server-Sent Events (SSE) streaming (`/api/v1/updates/stream`)
  - Unified event broker system for transport-agnostic notifications
  - Event adapters for SSE and WebSocket subscribers
  - OpenAPI 3.1 specification with Swag v2 annotations
  - Comprehensive HTTP handler suite (models, providers, admin, health, realtime)
  - Advanced filtering and search capabilities
  - Pagination support for large result sets

- **Server Infrastructure**
  - Modular middleware system (auth, CORS, rate limiting, logging, recovery)
  - In-memory caching with configurable TTL
  - Per-IP token bucket rate limiting
  - Optional API key authentication with public/private path support
  - CORS configuration with wildcard and specific origin support
  - Request logging with structured zerolog integration
  - Panic recovery with graceful error handling
  - Response wrapper for consistent API format

- **Testing & Quality**
  - Comprehensive test coverage (>85%) across all server packages:
    - Middleware: 94.1% coverage
    - SSE broadcaster: 96.5% coverage
    - WebSocket hub: 86.8% coverage
    - Event adapters: 100% coverage
  - Race detector validation on all tests
  - Context-based timeouts for async operations
  - Production-ready WebSocket/SSE with critical bug fixes

- **Initial Core Features**
  - Command-line interface for model discovery and comparison
  - Support for multiple AI providers (OpenAI, Anthropic, Google, Groq, DeepSeek, Cerebras)
  - Embedded catalog with 500+ AI models
  - Real-time synchronization with provider APIs
  - Multi-source reconciliation engine with field-level authority
  - Provider API client implementations
  - Model comparison and filtering capabilities
  - Pricing and capability information
  - Export functionality (OpenAI/OpenRouter formats)

### Changed
- **OpenAPI Migration** - Upgraded from Swagger 2.0 to OpenAPI 3.1
  - Migrated to Swag v2 for native OpenAPI 3.1 generation
  - Removed Node.js dependency (@redocly/cli)
  - Embedded OpenAPI specs in binary via go:embed
  - Simplified CLI: `starmap serve` (removed `api` subcommand)

- **Architecture Improvements**
  - Refactored HTTP server with clear separation of concerns
  - Separated CLI command from server implementation
  - Moved OpenAPI annotations to server package
  - Consolidated serve package utilities
  - Removed dead code and Hugo/Git submodule infrastructure

### Infrastructure
- GitHub Actions workflow for documentation
- GoReleaser configuration for multi-platform releases
- Docker support with automated image builds
- Homebrew tap for macOS/Linux installation

## [0.1.0] - TBD

Initial public release. See Unreleased section for features.

[Unreleased]: https://github.com/agentstation/starmap/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/agentstation/starmap/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/agentstation/starmap/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/agentstation/starmap/compare/v0.1.2...v0.2.0
[0.1.0]: https://github.com/agentstation/starmap/releases/tag/v0.1.0
