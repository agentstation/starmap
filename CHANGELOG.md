# Changelog

All notable changes to Starmap will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.0.23] - TBD

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