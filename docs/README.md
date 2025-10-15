# Starmap Documentation

Technical documentation for the Starmap AI Model Catalog project.

## Core Documentation

### [API.md](API.md)
**Go Package API Reference**

Auto-generated API documentation for the Starmap Go package including:
- Client interface and usage
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
- Core package layer (catalogs, reconciler, authority, sources)
- Data sources and concurrent fetching
- 12-stage sync pipeline
- Authority-based reconciliation system
- Thread safety patterns
- Package organization
- Testing strategy

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
- [../pkg/errors/README.md](../pkg/errors/README.md) - Typed errors
- [../pkg/logging/README.md](../pkg/logging/README.md) - Logging utilities
- [../internal/server/README.md](../internal/server/README.md) - HTTP server implementation

## Quick Links

- [System Architecture](ARCHITECTURE.md#overview)
- [Thread Safety Guidelines](ARCHITECTURE.md#thread-safety)
- [Sync Pipeline (12 Stages)](ARCHITECTURE.md#sync-pipeline)
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
