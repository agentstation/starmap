# Changelog

All notable changes to Starmap will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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