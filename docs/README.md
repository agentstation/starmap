# Starmap Documentation

Technical documentation for the Starmap AI Model Catalog project.

## Core Documentation

### [API.md](API.md)
**Go Package API Reference**

Auto-generated API documentation for the Starmap Go package including:
- Concrete Client API and usage
- Catalog operations
- Sync and update mechanisms
- Event hooks system
- Configuration options
- Functional options patterns

### [ARCHITECTURE.md](ARCHITECTURE.md)
**System Architecture & Technical Deep Dive**

Comprehensive technical documentation covering:
- Design principles and patterns
- System components and layers
- Application layer (dependency injection)
- CLI architecture and design decisions
- Core package layer (catalogs, reconciler, authority, sources)
- Data sources and concurrent fetching
- 13-stage sync pipeline
- Authority-based reconciliation system
- Thread safety patterns
- Package organization
- Testing strategy

### [CATALOG_AUTHORITY_POLICY.md](CATALOG_AUTHORITY_POLICY.md)
**Canonical Field Authority Policy**

Executable authority order, merge semantics, empty-value semantics, and
rationale for every model-definition and provider-offering attribute family.

### [SCHEMA_DRIFT_POLICY.md](SCHEMA_DRIFT_POLICY.md)
**Strict and Tolerant Source Schema Policy**

Executable failure scope for identities, containers, additive unknown fields,
and lossless source extensions.

### [SCHEDULED_CATALOG_GENERATION.md](SCHEDULED_CATALOG_GENERATION.md)
**Validated Catalog Publication Workflow**

Daily/manual change detection, manifest derivation, validation, attestation,
payload-digest deduplication, and immutable release publication.

### [DURABLE_SCHEDULING.md](DURABLE_SCHEDULING.md)
**Deployment-Owned HA Synchronization**

Lease/single-flight composition above explicit Sync, with deterministic and
shared-filesystem adapters.

### [REMOTE_CATALOG_PROTOCOL.md](REMOTE_CATALOG_PROTOCOL.md)
**Versioned Online Generation Protocol**

Strict current-manifest and immutable generation-snapshot routes, client
compatibility/checksum verification, and atomic remote publication semantics.

### [HOSTED_CATALOG_DISTRIBUTION.md](HOSTED_CATALOG_DISTRIBUTION.md)
**Hosted Generation and Promotion Protocol**

Verified immutable assets, schema-compatible pointers, dev/canary/stable
promotion, SLO evidence, and rollback behavior.

### [CLI.md](CLI.md)
**CLI Implementation Reference**

Command-line interface reference and implementation guidelines:
- Global flags and reserved short flags
- Command-specific flags
- Flag design principles
- Positional arguments vs flags
- Custom help flags pattern
- Flag aliases and deprecation
- Testing and migration guides
- Examples and anti-patterns

### [TESTING.md](TESTING.md)
**Testing and Verification Strategy**

Enterprise verification guidance covering:
- Full deterministic verification with `make verify`
- Critical seam coverage thresholds
- Focused package test commands
- Race detection and docs checks
- Live provider verification with credentials

### [REST_API.md](REST_API.md)
**HTTP Server API Reference**

Complete REST API documentation including:
- Getting started and configuration
- Authentication and security
- Response format and error handling
- All API endpoints (models, providers, admin, health)
- Real-time updates (WebSocket, SSE)
- Filtering and search
- Rate limiting and CORS
- Usage examples

## Project Documentation

Located in the project root:

- [../README.md](../README.md) - Project overview, installation, and quick start
- [../CLAUDE.md](../CLAUDE.md) - LLM coding assistant instructions
- [../CONTRIBUTING.md](../CONTRIBUTING.md) - Contribution guidelines and development setup
- [../CHANGELOG.md](../CHANGELOG.md) - Version history and release notes
- [../LICENSE](../LICENSE) - AGPL 3.0 license

## Package Documentation

Individual package READMEs provide implementation details:

- [../pkg/catalogs/README.md](../pkg/catalogs/README.md) - Catalog storage abstraction
- [../pkg/reconciler/README.md](../pkg/reconciler/README.md) - Multi-source reconciliation
- [../pkg/authority/](../pkg/authority/) - Field-level authority system
- [../pkg/sources/README.md](../pkg/sources/README.md) - Data source abstractions
- [../pkg/sourceevidence/README.md](../pkg/sourceevidence/README.md) - Source evidence retention and deterministic replay
- [../pkg/errors/README.md](../pkg/errors/README.md) - Typed errors
- [../pkg/logging/README.md](../pkg/logging/README.md) - Logging utilities
- [../internal/server/README.md](../internal/server/README.md) - HTTP server implementation

## Quick Links

- [System Architecture](ARCHITECTURE.md#overview)
- [CLI Architecture](ARCHITECTURE.md#cli-architecture)
- [CLI Implementation Reference](CLI.md)
- [Testing and Verification](TESTING.md)
- [Thread Safety Guidelines](ARCHITECTURE.md#thread-safety)
- [Sync Pipeline (13 Stages)](ARCHITECTURE.md#sync-pipeline)
- [Reconciliation System](ARCHITECTURE.md#reconciliation-system)
- [HTTP Server Configuration](REST_API.md#configuration)
- [Real-time Updates (WebSocket/SSE)](REST_API.md#real-time-updates)
- [Go Package Usage](API.md#client)

## Contributing

See [CONTRIBUTING.md](../CONTRIBUTING.md) for:
- Development setup
- Testing guidelines
- Coding standards
- Pull request process
- Adding new providers

## Support

- **GitHub Issues**: https://github.com/agentstation/starmap/issues
- **GitHub Discussions**: https://github.com/agentstation/starmap/discussions
- **Documentation**: You're here! 📚
