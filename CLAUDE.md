# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Starmap is a comprehensive AI model catalog system that provides centralized information about AI models, their capabilities, pricing, and providers. It serves as both a reference catalog and a synchronization tool for keeping model data up-to-date across different AI service providers.

### Core Purpose
- **Model Discovery**: Find and compare AI models across different providers
- **Capability Assessment**: Understand model features, limits, and pricing
- **Catalog Maintenance**: Keep model information synchronized with provider APIs
- **Integration Planning**: Access structured data for building AI applications

## Architecture Overview

### Starmap Interface Design
The project now provides a clean interface-based API (`starmap.go`) that wraps the catalog system with additional features:

- **Starmap Interface**: Main entry point for accessing catalogs with event hooks and automatic updates
- **Thread-Safe Operations**: All catalog access returns copies to prevent data races
- **Event-Driven Architecture**: Hooks for model added/updated/removed events
- **Automatic Updates**: Configurable background synchronization with provider APIs
- **Options Pattern**: Flexible configuration using variadic options
- **Future HTTP Server Support**: Designed to support remote starmap servers

### Catalog System Design
The underlying catalog system (`pkg/catalogs/catalog.go`) uses a modular interface with multiple implementations:

- **Embedded Catalog** (`internal/catalogs/embedded/`): Production implementation using `go:embed` YAML files
- **File Catalog** (`internal/catalogs/files/`): Development-friendly implementation reading from filesystem
- **Memory Catalog** (`internal/catalogs/memory/`): In-memory implementation for testing
- **Base Catalog** (`internal/catalogs/base/`): Shared implementation providing CRUD operations

### Core Domain Types
- **Provider** (`pkg/catalogs/provider.go`): AI service providers with API configuration, headers, and policies
- **Model** (`pkg/catalogs/model.go`): Comprehensive model specifications including features, pricing, limits, capabilities
- **Author** (`pkg/catalogs/author.go`): Model creators and organizations
- **Endpoint** (`pkg/catalogs/endpoint.go`): API endpoint configurations

### Provider System
Each provider implements the `Client` interface (`pkg/catalogs/client.go`) for fetching models:
- **Transport Layer** (`internal/transport/`): HTTP client with provider-specific headers and authentication
- **Provider Clients** (`internal/sources/providers/`): Specific implementations for Anthropic, OpenAI, Google, Groq, etc.
- **Registry System** (`internal/sources/providers/register.go`): Automatic provider discovery and instantiation

## Starmap Usage Patterns

### Basic Go Package Usage

```go
// Create a starmap instance with default embedded catalog
sm, err := starmap.New()
if err != nil {
    log.Fatal(err)
}

// Get catalog (always returns a copy for thread safety)
catalog, err := sm.Catalog()
if err != nil {
    log.Fatal(err)
}

// Access models, providers, authors
models := catalog.Models()
providers := catalog.Providers()
authors := catalog.Authors()
```

### Event-Driven Architecture

```go
// Register event hooks for model changes
sm.OnModelAdded(func(model catalogs.Model) {
    log.Printf("New model added: %s", model.ID)
})

sm.OnModelUpdated(func(old, new catalogs.Model) {
    log.Printf("Model updated: %s", new.ID)
    if old.Pricing.Input != new.Pricing.Input {
        log.Printf("Price changed: %f -> %f", old.Pricing.Input, new.Pricing.Input)
    }
})

sm.OnModelRemoved(func(model catalogs.Model) {
    log.Printf("Model removed: %s", model.ID)
})
```

### Automatic Updates

```go
// Configure automatic updates with custom sync logic
updateFunc := func(currentCatalog catalogs.Catalog) (catalogs.Catalog, error) {
    // Custom sync logic - could use sync command logic
    return currentCatalog, nil
}

sm, err := starmap.New(
    starmap.WithAutoUpdateInterval(30 * time.Minute),
    starmap.WithUpdateFunc(updateFunc),
)
if err != nil {
    log.Fatal(err)
}
// Auto-updates start automatically!

// Cleanup when shutting down
defer sm.AutoUpdatesOff()
```

### Auto-Updates Configuration

By default, starmap enables automatic updates. You can control this behavior:

```go
// Default: auto-updates enabled every hour
sm, err := starmap.New()

// Customize update interval
sm, err := starmap.New(
    starmap.WithAutoUpdateInterval(15 * time.Minute),
)

// Disable auto-updates entirely
sm, err := starmap.New(
    starmap.WithAutoUpdates(false),
)

// Control at runtime
sm.AutoUpdatesOff() // Temporarily stop
sm.AutoUpdatesOn()  // Resume updates
```

### Different Catalog Sources

```go
import "github.com/agentstation/starmap/pkg/catalogs/files"

// Use file-based catalog for development
filesCatalog, err := files.New("./internal/embedded/catalog")
if err != nil {
    log.Fatal(err)
}

sm, err := starmap.New(
    starmap.WithInitialCatalog(filesCatalog),
)

import "github.com/agentstation/starmap/pkg/catalogs/memory"

// Use memory catalog for testing
memoryCatalog, err := memory.New()
if err != nil {
    log.Fatal(err)
}

sm, err := starmap.New(
    starmap.WithInitialCatalog(memoryCatalog),
)
```

### Future Remote Server Mode

```go
// Planned for future releases
apiKey := "sk-your-api-key"
sm, err := starmap.New(
    starmap.WithRemoteServer("https://api.starmap.ai", &apiKey),
    starmap.WithRemoteServerOnly(true), // If enabled, do not use any other sources for catalog updates including provider APIs
    starmap.WithAutoUpdateInterval(5 * time.Minute),
)
```

## Key Implementation Details

### Thread Safety Guarantees
- All `Catalog()` calls return deep copies of the catalog data
- Multiple goroutines can safely access catalogs concurrently
- Event hooks are called sequentially but don't block other operations
- Internal state is protected by RWMutex for efficient concurrent reads

### Catalog Factory Methods
The project provides factory methods in the respective catalog packages for creating different catalog types:
- `embedded.New()` - Production catalog with embedded YAML files (pkg/catalogs/embedded)
- `files.New(path)` - Development catalog reading from filesystem (pkg/catalogs/files)
- `memory.New()` - In-memory catalog for testing (pkg/catalogs/memory)

### Options Pattern
Configuration uses functional options in `options.go`:
- `WithEmbeddedAutoLoad(bool)` - Control embedded catalog loading
- `WithFilesAutoLoad(bool)` - Control files catalog loading  
- `WithMemoryReadOnly(bool)` - Set memory catalog as read-only
- `WithMemoryPreload([]byte)` - Preload memory catalog with data

### Registry Pattern
The provider system uses a registry pattern similar to database drivers:
- Providers register themselves in `init()` functions
- Registry maintains a map of `ProviderID -> Client`
- Dependency injection breaks circular imports
- Automatic discovery of available providers
- Clean interface through `internal/sources/providers` package with exported functions

#### Provider Registration Usage
```go
import "github.com/agentstation/starmap/internal/sources/providers"

// Get a configured client for a provider
client, err := providers.GetClient(provider)

// Check if a provider has a registered client
if providers.HasClient(providerID) {
    // Provider is available
}

// List all supported provider IDs
supportedProviders := providers.ListSupportedProviders()
```

#### Plugin-Style Explicit Initialization
The providers package supports explicit initialization for advanced use cases:

```go
import "github.com/agentstation/starmap/internal/sources/providers"

// Explicit initialization (optional - happens automatically on import)
providers.Init()

// Check if providers have been initialized
if providers.IsInitialized() {
    // Providers are ready
}

// Future extensibility for dynamic provider loading
opts := providers.InitOptions{
    // Future options for selective provider loading
}
providers.InitWithOptions(opts)
```

**Key Benefits:**
- **Automatic Registration**: Providers register automatically on package import (backward compatible)
- **Explicit Control**: Optional explicit initialization for advanced scenarios
- **Future-Proof**: Foundation for dynamic provider loading and selective initialization
- **Thread-Safe**: All initialization functions are safe for concurrent use

This eliminates the need for blank imports (`_`) and makes the import purpose clear.

## File Organization and Key Components

### Core Interface Files
- **`starmap.go`** - Main Starmap interface and implementation with event hooks and automatic updates
- **`catalog.go`** - Factory methods for creating different catalog implementations (embedded, files, memory)
- **`options.go`** - Functional options for configuring catalog instances

### Catalog Implementation Structure
- **`pkg/catalogs/catalog.go`** - Core Catalog interface definition
- **`pkg/catalogs/models.go`** - Thread-safe Models collection with CRUD operations
- **`pkg/catalogs/providers.go`** - Providers collection and management
- **`pkg/catalogs/authors.go`** - Authors collection and management
- **`internal/catalogs/catalogs.go`** - Catalog type constants (Embedded, Files, Memory)

### Provider System
- **`internal/sources/providers/registry/registry.go`** - Provider client registry with thread-safe registration
- **`internal/sources/providers/register.go`** - Import file for all provider side-effect registration
- **`internal/sources/providers/{provider}/`** - Individual provider implementations

### CLI Integration
- **`cmd/starmap/cmd/sync.go`** - Main sync command that uses the catalog system
- **`cmd/starmap/cmd/list.go`** - List commands using the starmap interface
- **Other cmd files** - Use factory methods to create catalog instances

### Important Implementation Notes
- The CLI commands currently use the factory methods (`embedded.New()`, etc.) directly
- The new Starmap interface (`starmap.New()`) is designed for Go package integration
- All catalog access should go through the Starmap interface for new code
- The embedded catalog files are in `internal/embedded/catalog/` with go:embed constraints

## Build and Development Commands

### Core Commands
```bash
# Build binary
make build                  # Build to current directory
make install               # Install to GOPATH/bin

# Development workflow
make fix                   # Format code and tidy modules
make lint                  # Run static analysis (vet + golangci-lint)
make test                  # Run all tests
make test-coverage         # Generate coverage report

# Full development cycle
make all                   # clean fix lint test build
```

### Running Application
```bash
# Run locally
make run                   # Run with no args
make run-help             # Show help

# Sync operations (all run with --dry-run by default)
make sync                                    # Sync all providers
make sync PROVIDER=openai                    # Sync specific provider
make sync PROVIDER=groq OUTPUT=./models      # Sync to custom directory

# List operations
make list-models          # List all models
make list-providers       # List all providers
make list-authors         # List all authors
```

### Testing and Data Management
```bash
# Testdata management
make testdata                      # Show testdata help
make testdata PROVIDER=groq        # List testdata for provider
make testdata-update               # Update all testdata (requires API keys)
make testdata-verify               # Verify testdata files

# Run tests with specific updates
go test ./internal/sources/providers/groq -update
```

## Sync Command - Primary Workflow

The sync command (`cmd/starmap/cmd/sync.go`) is the core functionality for keeping catalogs current:

### Basic Usage
```bash
# Sync all providers with confirmation
starmap sync

# Sync specific provider with auto-approval
starmap sync -p anthropic --auto-approve

# Preview changes without applying
starmap sync --dry-run

# Fresh sync (destructive - replaces all models)
starmap sync -p groq --fresh
```

### Development Workflow
```bash
# Use file-based catalog (avoids binary rebuild)
starmap sync --input ./internal/embedded/catalog -p groq

# Custom output directory
starmap sync --output ./my-catalog --auto-approve

# Combined development pattern
starmap sync --input ./dev-catalog --output ./updated-catalog
```

### Sync Process Flow
1. **Load Catalog**: Embedded or file-based catalog
2. **models.dev Integration**: Clone/update models.dev repository for enhanced data
3. **Provider Processing**: Iterate through providers with API keys
4. **API Data Fetching**: Use provider-specific clients to fetch current models
5. **Data Enhancement**: Merge with models.dev data for accurate pricing/limits
6. **Change Detection**: Compare API models with existing catalog using smart diff
7. **User Confirmation**: Show detailed changeset (unless `--auto-approve`)
8. **Apply Changes**: Write to embedded directory or custom output
9. **Asset Management**: Copy provider logos from models.dev

## Data Organization

### Embedded Catalog Structure
```
internal/embedded/catalog/
├── providers.yaml          # Provider definitions with API configuration
├── authors.yaml           # Author/organization metadata
├── endpoints.yaml         # API endpoint configurations (optional)
├── providers/             # Provider-organized models (primary organization)
│   ├── anthropic/         # Models available through Anthropic
│   │   ├── claude-3-5-sonnet-20241022.yaml
│   │   └── logo.svg
│   ├── groq/              # Models available through Groq
│   │   ├── whisper-large-v3.yaml
│   │   ├── meta-llama/    # Nested directories for hierarchical IDs
│   │   │   └── llama-guard-4-12b.yaml
│   │   └── openai/        # Cross-provider hosting
│   │       └── gpt-oss-120b.yaml
│   └── openai/
│       ├── gpt-4o.yaml
│       └── o1.yaml
└── authors/               # Author-organized models (cross-references)
    ├── openai/
    ├── anthropic/
    └── google/
```

### Directory Naming Convention
- **Simple IDs**: `gpt-4o` → `providers/openai/gpt-4o.yaml`
- **Hierarchical IDs**: `meta-llama/llama-guard-4-12b` → `providers/groq/meta-llama/llama-guard-4-12b.yaml`
- **Cross-provider**: `openai/gpt-oss-120b` → `providers/groq/openai/gpt-oss-120b.yaml`

This structure preserves exact model IDs while avoiding filesystem conflicts.

### Go Embed Limitations
```go
//go:embed catalog/*
var FS embed.FS
```

**Critical Constraints:**
- `go:embed` doesn't support `**` recursive patterns
- Changes to embedded files require binary rebuild
- Use `--input` flag for development to bypass embedding

## Testing Patterns

### Testdata System
The project uses `testdata` directories following Go conventions:

**Location**: `internal/sources/providers/<provider>/testdata/`
**Purpose**: Store raw API responses for offline testing

### Testdata Management
```bash
# Update all provider testdata
starmap testdata --update

# Update specific provider
starmap testdata --provider groq --update

# Verify testdata is current
starmap testdata --verify
```

### Test Integration
Provider tests use the `testhelper` package (`internal/sources/providers/testhelper/`):
```go
// Load raw API response
response := loadTestdataResponse(t, "models_list.json")

// Test parsing
models, err := client.parseModelsResponse(response)
```

### Update Workflow
```bash
# Update testdata during test runs
go test ./internal/sources/providers/groq -update

# This saves fresh API responses to testdata/models_list.json
```

## Provider Integration Patterns

### Adding New Providers
1. **Provider Configuration**: Add to `internal/embedded/catalog/providers.yaml`
2. **Client Implementation**: Create in `internal/sources/providers/<provider>/`
3. **Registration**: Add to `internal/sources/providers/register.go`
4. **Testdata**: Run `starmap testdata --provider <provider> --update`

### Provider Client Interface
```go
type Client interface {
    ListModels(ctx context.Context) ([]catalogs.Model, error)
}
```

### Authentication Patterns
- **API Keys**: Configured via environment variables (e.g., `OPENAI_API_KEY`)
- **Headers**: Provider-specific headers added by transport layer
- **Validation**: Pattern matching in provider configuration

## Smart Merging and Change Detection

### Three-Way Merge System
The sync process uses intelligent merging (`internal/catalogs/operations/merge.go`):
1. **API Data**: Latest from provider APIs (availability, basic specs)
2. **models.dev Data**: Enhanced data (accurate pricing, limits, metadata)
3. **Existing Data**: Current catalog (manual annotations, corrections)

### Priority System
- **models.dev**: Higher priority for limits, pricing, metadata
- **API Data**: Higher priority for availability, model lists
- **Existing**: Preserves manual annotations and corrections

### Change Detection
The diff system (`internal/catalogs/operations/diff.go`) categorizes:
- **Added**: New models from API
- **Updated**: Field-level changes with detailed diff
- **Removed**: Models missing from API (informational only)

## Configuration and Environment

### Configuration Files
- **Primary**: `~/.starmap.yaml` or via `--config` flag
- **Environment**: `.env` and `.env.local` files loaded automatically
- **API Keys**: Environment variables with automatic binding

### Key Environment Variables
```bash
OPENAI_API_KEY=...
ANTHROPIC_API_KEY=...
GOOGLE_API_KEY=...
GROQ_API_KEY=...
```

## Development Patterns

### File vs Embedded Catalogs
- **Embedded**: Production use, requires binary rebuild for changes
- **File-based**: Development use with `--input` flag, immediate changes

### Development Workflow
```bash
# Edit YAML files directly
vim internal/embedded/catalog/providers/groq/whisper-large-v3.yaml

# Test without rebuilding
starmap sync --input ./internal/embedded/catalog -p groq --dry-run

# Apply and review
starmap sync --input ./internal/embedded/catalog -p groq
```

### Code Conventions
- **Error Handling**: Use `errors.Join` for multiple errors
- **Type Aliases**: Use `any` instead of `interface{}`
- **Provider Isolation**: Keep provider models separate during sync operations
- **YAML Structure**: Preserve exact model IDs in file paths

## models.dev Integration

Starmap integrates with the models.dev repository for enhanced model data:

### Integration Process
1. **Repository Cloning**: Automatic clone/update of models.dev
2. **API Generation**: Build comprehensive `api.json` from TOML files
3. **Data Enhancement**: Merge models.dev data with API responses
4. **Asset Management**: Copy provider logos and metadata

### Enhanced Data Sources
- **Pricing Information**: More accurate than provider APIs
- **Context Windows**: Verified limits and capabilities
- **Metadata**: Release dates, knowledge cutoffs, open weights status
- **Provider Assets**: Logos and branding materials

## Important Limitations

### Go Embed Constraints
- **Pattern Limitation**: `go:embed` doesn't support `**` recursive patterns
- **Build Requirement**: Changes to embedded files require binary rebuild
- **Development Workaround**: Use `--input` flag for immediate feedback

### Provider Isolation Requirements
- **Sync Boundaries**: Models compared only within provider boundaries
- **Cross-Provider Hosting**: Supported but isolated during sync operations
- **Example**: Groq's OpenAI models sync against Groq API, not OpenAI API

## CLI Command Reference

### Primary Commands
```bash
starmap sync [--provider <id>] [--auto-approve] [--dry-run] [--fresh]
starmap list models|providers|authors
starmap fetch --provider <id>
starmap testdata [--update] [--provider <id>] [--verify]
starmap export
```

### Sync Flags
- `--provider <id>`: Sync specific provider
- `--auto-approve`: Skip confirmation prompts
- `--dry-run`: Preview changes only
- `--fresh`: Delete all models and write fresh (destructive)
- `--input <dir>`: Use file-based catalog
- `--output <dir>`: Custom output directory

## Dependencies

### Core Libraries
- **Cobra**: CLI framework and command structure
- **Viper**: Configuration management with environment integration
- **go-yaml**: YAML parsing and structured output generation
- **godotenv**: Environment file loading
- **agentstation/utc**: Custom UTC time handling for model timestamps

### Development Tools
- **golangci-lint**: Static analysis and linting
- **go test**: Testing framework with testdata integration
- **make**: Build automation and development workflows