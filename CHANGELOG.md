# Changelog

All notable changes to Starmap will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Clients configured with `WithCatalogStore` now recover and publish the exact
  durable current generation during `starmap.New`, so server and remote updates
  survive process restart. Server updates may select a single source with the
  `source` query parameter; a configured `WithCatalogExportPath` is also the default
  sync input/output path when `sync.WithOutputPath` is omitted.
- Catalog publication observers no longer head-of-line block one another, and
  server logging middleware preserves SSE flushing and WebSocket hijacking.
  HTTP catalog responses, SSE, WebSocket, and cache state now correlate the
  same durable generation and sync-run identity after commit.

### BREAKING CHANGES
- **Canonical local storage layout**: the durable generation database now uses
  `catalog_path` and defaults to `~/.starmap/catalog`; optional editable YAML
  uses `catalog_export_path` and defaults to `~/.starmap/exports/catalog` for
  CLI updates. Configuration is read from `~/.starmap/config.yaml`.
  `WithCatalogExportPath` replaces the ambiguous `WithLocalPath`. Starmap has
  not launched, so draft path names and compatibility aliases are intentionally
  not shipped.
- **Scheduling moved above the core client**: `starmap.Client` no longer owns a
  ticker goroutine or cadence lifecycle. `AutoUpdatesOn`, `AutoUpdatesOff`,
  `WithAutoUpdatesEnabled`, `WithAutoUpdatesDisabled`,
  `WithAutoUpdateInterval`, `AutoUpdateFunc`, `AutoUpdateContextFunc`,
  `WithAutoUpdateFunc`, and `WithAutoUpdateContextFunc` were removed.
  Deployments and Starport own cadence, jitter, retry, leases, and startup
  policy and invoke the idempotent `Client.Sync` or `Client.Update` operation.
  Custom candidate construction migrates to the context-aware
  `UpdateFunc`/`WithUpdateFunc` seam:

  ```go
  sm, err := starmap.New(
      starmap.WithCatalogStore(store),
      starmap.WithUpdateFunc(updateFunc),
  )
  err = sm.Update(ctx)
  ```

- **Canonical model-definition and provider-offering lookup**:
  `catalog.FindModel(id)` now returns `catalogs.ModelDefinition`. Provider price,
  limits, availability, modes, and request behavior are read through
  `catalog.Offering(providerID, providerModelID)`. Immutable catalogs expose no
  flattened-model compatibility adapter. Canonical catalog payloads use schema
  version 2 and require explicit definitions and offerings.

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
  precompute alias-aware provider/model indexes; use `ProviderModels` or
  `ProviderModel` for provider-specific offerings rather than a lossy bare-ID
  model view.

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
  - `WithRemoteServerURL` now configures a remote endpoint without silently
    diverting `Client.Update`. Use `WithRemoteServerOnly` when updates must come
    exclusively from the configured remote endpoint.
  - Programmatic `sync.WithSources` now rejects unknown source IDs and copies
    caller input. A fresh sync rejects `local_catalog` because an existing local
    catalog cannot also be the input to a replacement generation.
  - Explicit models.dev Git verification no longer follows the floating `dev`
    branch. Supply `sync.WithModelsDevGitCommit(exactCommit)` or CLI flag
    `--models-dev-git-commit`; Starmap checks out that detached commit, installs
    with `bun install --frozen-lockfile`, and records the `bun.lock` SHA-256 in
    source-observation revision metadata.

- **Remote catalog protocol is generation-based and versioned**: the ad-hoc
  unversioned `GET /catalog` envelope was removed. Configure
  `WithRemoteServerURL`/`WithRemoteServerOnly` with the versioned API base (for
  example `https://catalog.example.com/api/v1`). Consumers now read
  `GET /catalog/manifest` and then the immutable
  `GET /catalog/generations/{generation_id}/snapshot`; schema compatibility,
  media type, size, and SHA-256 are verified before durable publication.

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
- **Deletion-correct catalog saves**: legacy builder saves now replace
  Starmap-managed YAML indexes and provider/author model trees, preventing
  deleted records from reappearing after reload while preserving unmanaged
  neighboring files. Generation stores already replace the payload as one
  immutable unit.
- **Configured local catalog failures are visible**: a missing optional path
  still falls back to the embedded bootstrap, while existing corrupt YAML,
  unreadable managed files, and invalid provider/author records now propagate
  typed errors and make `starmap.New` fail before publication.
- **Prelaunch schema-v2 clean break**: catalog payload, bootstrap, remote,
  artifact, and hosted-distribution readers accept only exact schema version 2.
  Schema-v1 payloads and manifests fail before publication; no on-read format
  migration or old-directory discovery is shipped.
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

[Unreleased]: https://github.com/agentstation/starmap/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/agentstation/starmap/releases/tag/v0.1.0
