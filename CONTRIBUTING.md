# Contributing to Starmap

Thank you for your interest in contributing to Starmap! We welcome contributions of all kinds, from bug reports and feature requests to code contributions and documentation improvements.

## Table of Contents

- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Development Workflow](#development-workflow)
- [Project Structure](#project-structure)
- [Adding New Providers](#adding-new-providers)
- [Testing](#testing)
- [Submitting Changes](#submitting-changes)
- [Development Guidelines](#development-guidelines)
- [Contributing to models.dev](#contributing-to-modelsdev)

## Getting Started

Before contributing, please:

1. Check [existing issues](https://github.com/agentstation/starmap/issues) to avoid duplicates
2. Read our [Code of Conduct](CODE_OF_CONDUCT.md) (if available)
3. Review the [ARCHITECTURE.md](docs/ARCHITECTURE.md) to understand the system design
4. Join our [Discord](https://discord.gg/starmap) (if available) to discuss major changes

## Development Setup

### Prerequisites

- **Go 1.21 or later** - [Install Go](https://go.dev/doc/install)
- **Make** - For build automation
- **Git** - For models.dev integration and version control

### Initial Setup

```bash
# Clone repository
git clone https://github.com/agentstation/starmap.git
cd starmap

# Install dependencies
go mod download

# Run tests to verify setup
make test

# Build binary
make build
```

### Verify Installation

```bash
# Run the built binary
./starmap version

# Run with embedded catalog
./starmap models list
```

## Development Workflow

### Common Commands

```bash
# Format and lint code
make fix
make lint

# Run tests with coverage
make test-coverage

# Update provider testdata
make testdata-update

# Generate Go documentation
make generate

# Full build cycle
make all  # clean, fix, lint, test, build
```

### Inspecting Embedded Catalog

The `starmap embed` command lets you inspect the embedded filesystem during development:

```bash
# List embedded files
starmap embed ls catalog
starmap embed ls catalog/providers

# View file contents
starmap embed cat catalog/providers.yaml
starmap embed cat sources/models.dev/api.json

# Display directory tree
starmap embed tree catalog

# Show file details
starmap embed stat catalog/providers.yaml
```

This is useful for:
- Verifying catalog structure after updates
- Debugging embedded data issues
- Understanding the catalog layout
- Checking file contents without rebuilding

### Development Cycle

1. Create a feature branch: `git checkout -b feature/your-feature`
2. Make your changes
3. Run `make fix` to format code
4. Run `make lint` to check for issues
5. Run `make test` to ensure tests pass
6. Commit with descriptive message
7. Push and create pull request

## Project Structure

Understanding the codebase organization:

```
starmap/
├── cmd/
│   ├── starmap/        # CLI application
│   │   ├── app/        # Application implementation
│   │   └── commands/   # Cobra CLI commands
│   └── application/    # Application interface (DI pattern)
│
├── pkg/                # Public API packages
│   ├── catalogs/       # Catalog abstraction and storage
│   ├── reconcile/      # Multi-source reconciliation
│   ├── sources/        # Data source abstractions
│   ├── authority/      # Field-level authority system
│   ├── errors/         # Custom error types
│   ├── constants/      # Application constants
│   ├── logging/        # Structured logging
│   └── convert/        # Format conversion utilities
│
├── internal/           # Internal implementation packages
│   ├── embedded/       # Embedded catalog data
│   ├── transport/      # HTTP client utilities
│   └── sources/        # Source implementations
│       ├── providers/  # Provider API clients
│       ├── modelsdev/  # models.dev integration
│       ├── local/      # Local file source
│       └── clients/    # Client registry
│
├── docs/               # Technical documentation
│   ├── API.md          # Go package API reference
│   ├── ARCHITECTURE.md # System design documentation
│   └── REST_API.md     # HTTP server API reference
│
├── CLAUDE.md           # LLM coding assistance guide
├── README.md           # User-facing documentation
└── scripts/            # Build and automation scripts
```

### Package Dependency Rules

- User interfaces import only the root `starmap` package
- Root package imports only `pkg/` packages
- Internal packages implement `pkg/` interfaces
- No circular dependencies (enforced by Go)

See [ARCHITECTURE.md § Package Organization](docs/ARCHITECTURE.md#package-organization) for detailed dependency rules.

## Adding New Providers

### Quick Reference

For comprehensive instructions, see the provider implementation section in [ARCHITECTURE.md](docs/ARCHITECTURE.md#data-sources).

### Basic Steps

1. **Add Provider Configuration**

   Edit `internal/embedded/catalog/providers.yaml`:

   ```yaml
   - id: newprovider
     name: New Provider
     description: Description of the provider
     api_key:
       name: NEWPROVIDER_API_KEY
       env_var: NEWPROVIDER_API_KEY
     api:
       base_url: https://api.newprovider.com/v1
       models_endpoint: /models
   ```

2. **Implement Client**

   Create `internal/sources/providers/newprovider/client.go`:

   ```go
   package newprovider

   import (
       "context"
       "github.com/agentstation/starmap/pkg/catalogs"
   )

   type Client struct {
       provider *catalogs.Provider
   }

   func NewClient(provider *catalogs.Provider) (*Client, error) {
       return &Client{provider: provider}, nil
   }

   func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
       // Implement API call and model parsing
       return nil, nil
   }
   ```

3. **Register in Provider Registry**

   Edit `internal/sources/providers/providers.go`:

   ```go
   case "newprovider":
       return newprovider.NewClient(provider)
   ```

4. **Add Tests and Testdata**

   ```bash
   # Create test file
   touch internal/sources/providers/newprovider/client_test.go

   # Generate testdata (requires API key)
   export NEWPROVIDER_API_KEY=your-key
   go test ./internal/sources/providers/newprovider -update

   # Run tests
   go test ./internal/sources/providers/newprovider -v
   ```

5. **Update Documentation**

   - Add provider to README.md if it's a major provider
   - Update docs/ARCHITECTURE.md § Data Sources if needed
   - Add yourself to CONTRIBUTORS.md

## Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./pkg/catalogs/...

# Run with race detector
go test -race ./...

# Run with coverage
make test-coverage

# View coverage in browser
go tool cover -html=coverage.out
```

### Updating Testdata

Provider tests use the `-update` flag to generate testdata:

```bash
# Update testdata for a specific provider
go test ./internal/sources/providers/openai -update

# Update all provider testdata
make testdata-update

# Update testdata for specific provider via make
make testdata PROVIDER=openai
```

**Note**: Updating testdata requires valid API keys set in environment variables.

### Integration Tests

```bash
# Run integration tests
make test-integration

# Run integration tests with specific providers
PROVIDER=openai make test-integration
```

### Test Requirements

All contributions must:

- Include unit tests for new functionality
- Maintain or improve code coverage
- Pass all existing tests
- Pass race detector checks (`go test -race`)
- Pass linting (`make lint`)

## Submitting Changes

### Pull Request Process

1. **Fork the repository** and create your branch from `master`

2. **Make your changes** following our coding guidelines

3. **Test thoroughly**:
   ```bash
   make all  # Runs: clean, fix, lint, test, build
   ```

4. **Commit your changes**:
   - Use clear, descriptive commit messages
   - Reference issues: `Fixes #123` or `Relates to #456`
   - Follow [Conventional Commits](https://www.conventionalcommits.org/) if possible

5. **Push to your fork**:
   ```bash
   git push origin feature/your-feature
   ```

6. **Open a Pull Request**:
   - Provide clear description of changes
   - Reference related issues
   - Include screenshots/examples for UI changes
   - Update documentation if needed

### Pull Request Requirements

- [ ] Code follows Go best practices
- [ ] Tests added/updated and passing
- [ ] Documentation updated
- [ ] Commits are focused and atomic
- [ ] No merge conflicts with `master`
- [ ] Passes CI checks (linting, tests, build)

### Code Review Process

1. Maintainers review PR within 2-3 business days
2. Address feedback and requested changes
3. Once approved, maintainer merges PR
4. PR author will be added to CONTRIBUTORS.md

## Development Guidelines

### Code Style

- **Follow Go conventions**: Use `gofmt`, `goimports`
- **Run linters**: `make lint` uses golangci-lint
- **Write idiomatic Go**: See [Effective Go](https://go.dev/doc/effective_go)
- **Use value semantics**: Prefer values over pointers for thread safety
- **Document exported symbols**: All exported types, functions, constants

### Architecture Patterns

- **Define interfaces where used**: Don't create interfaces "just in case"
- **Dependency injection**: Use functional options pattern
- **Thread safety**: Always return deep copies, never expose internals
- **Error handling**: Use typed errors from `pkg/errors`
- **Constants**: Use `pkg/constants`, never hardcode values

See [ARCHITECTURE.md](docs/ARCHITECTURE.md) for detailed design patterns.

### Documentation

- Update package READMEs for new features
- Use `//go:generate` comments for auto-generated docs
- Include code examples in GoDoc comments
- Link to docs/ARCHITECTURE.md for design decisions
- Update CHANGELOG.md for user-facing changes

### Commit Guidelines

- Keep commits focused and atomic
- Write clear, descriptive commit messages
- Reference issues in commits: `Fixes #123`
- Don't commit generated files (unless necessary)
- Add yourself to CONTRIBUTORS.md

## Contributing to models.dev

Starmap uses [models.dev](https://models.dev) for community-verified pricing and metadata.

### How to Contribute

1. **Visit** https://models.dev
2. **Find** the model/provider you want to update
3. **Submit** corrections via GitHub pull request
4. **Wait** for review and merge
5. **Sync** automatically happens in next starmap update

### What to Contribute

- Pricing updates (input/output token costs)
- Context window limits
- Model capabilities (vision, function calling, etc.)
- Knowledge cutoff dates
- Provider logos (SVG preferred)
- Accurate model IDs

Data from models.dev syncs automatically to starmap's embedded catalog.

---

## Questions?

- **Documentation**: See [README.md](README.md) and [ARCHITECTURE.md](docs/ARCHITECTURE.md)
- **Issues**: [GitHub Issues](https://github.com/agentstation/starmap/issues)
- **Discussions**: [GitHub Discussions](https://github.com/agentstation/starmap/discussions)
- **Discord**: [Join our community](https://discord.gg/starmap) (if available)

Thank you for contributing to Starmap! 🌟
