# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Starmap is a unified AI model catalog system providing accurate, up-to-date information about AI models, their capabilities, pricing, and availability across providers. It solves the problem of fragmented model information by combining data from multiple sources into a single, authoritative catalog.

### Core Problems Solved
- **Missing Pricing**: Provider APIs often lack pricing information
- **Incomplete Metadata**: Basic specs without detailed capabilities
- **Rapid Changes**: Models launch weekly, requiring constant updates
- **Multiple Sources**: Need to combine provider APIs, community data, and manual fixes

## Architecture Overview

### Two-Tier Architecture

Starmap provides two levels of data management:

1. **Simple Catalog Operations** (`pkg/catalogs/`)
   - Two-catalog merging with strategies (EnrichEmpty, ReplaceAll, AppendOnly)
   - Multiple storage backends (memory, filesystem, embedded)
   - Use for: testing, simple tools, local overrides

2. **Complex Reconciliation** (`pkg/reconcile/`)
   - Multi-source reconciliation with field-level authority
   - Provenance tracking and audit trails
   - Use for: production sync, combining 3+ sources, different authorities per field

### Key Packages

#### pkg/catalogs
- **Purpose**: Catalog abstraction with pluggable storage
- **Interface**: Unified API regardless of storage backend
- **Backends**: Memory, Filesystem, Embedded (go:embed), Custom (any fs.FS)
- **Collections**: Thread-safe Models, Providers, Authors, Endpoints
- **Documentation**: pkg/catalogs/README.md

#### pkg/reconcile
- **Purpose**: Complex multi-source data reconciliation
- **Features**: Field-level authorities, provenance tracking, conflict resolution
- **Strategies**: AuthorityBased, Union, Custom
- **Use Case**: Production sync combining provider APIs + models.dev + embedded data
- **Documentation**: pkg/reconcile/README.md

#### pkg/sources
- **Purpose**: Abstraction for data sources
- **Interface**: Source with Name(), Setup(), Fetch(), Cleanup()
- **Sources**: Provider APIs, models.dev (Git/HTTP), Local catalog

### Data Flow

```
Provider APIs ──┐
                ├─→ Reconciliation Engine ─→ Unified Catalog ─→ Storage
models.dev ─────┤   (field-level merge)                        (embed/fs)
Embedded Data ──┘
```

## Build and Development Commands

### Essential Commands

```bash
# Build and Install
make build          # Build binary to current directory
make install        # Install to $GOPATH/bin

# Development Cycle
make fix           # Format code and tidy modules (gofmt + go mod tidy)
make lint          # Run golangci-lint and go vet
make test          # Run all tests
make all           # Complete cycle: clean, fix, lint, test, build

# Testing
go test ./...                                      # Run all tests
go test ./pkg/reconcile/...                       # Test specific package
go test -run TestReconcile ./pkg/reconcile        # Run specific test
go test ./internal/sources/providers/openai -update  # Update testdata

# Coverage
make test-coverage                                 # Generate coverage report
go test -coverprofile=coverage.out ./...          # Manual coverage
go tool cover -html=coverage.out                  # View in browser
```

### Sync Operations

```bash
# Basic Sync
starmap sync                          # Sync all providers
starmap sync -p openai                # Sync specific provider
starmap sync --dry-run                # Preview changes
starmap sync --auto-approve           # Skip confirmation

# Development Sync
starmap sync --input ./internal/embedded/catalog  # Use filesystem
starmap sync --output ./my-catalog               # Custom output
starmap sync --fresh -p groq                     # Replace all models

# New Update Command
starmap update                        # Update local catalog (~/.starmap/)
starmap update -p anthropic -y        # Update specific provider, auto-approve
```

### Testdata Management

```bash
# Testdata Commands
make testdata                         # Show help
make testdata-update                  # Update all provider testdata
make testdata-verify                  # Verify testdata validity

# Provider-specific
starmap testdata --provider groq --update
go test ./internal/sources/providers/groq -update
```

## High-Level Architecture

### Reconciliation System (pkg/reconcile)

The reconciliation system uses field-level authorities to merge data from multiple sources:

```go
// Example from sync.go
authorities := map[string]reconcile.SourceAuthority{
    "pricing": {
        Primary: sources.ModelsDevGit,
        Fallback: &sources.ProviderAPI,
    },
    "limits": {
        Primary: sources.ModelsDevGit,
        Fallback: &sources.ProviderAPI,
    },
    "model_list": {
        Primary: sources.ProviderAPI,  // Provider decides what exists
    },
}

// Using variadic options pattern (idiomatic Go)
reconciler, err := reconcile.New(
    reconcile.WithAuthorities(authorities),
    reconcile.WithProvenance(true),
)
```

### Catalog System (pkg/catalogs)

Single implementation with configurable storage backends:

```go
// Memory backend (testing)
catalog, _ := catalogs.New()  // No options = memory

// Filesystem backend (development)
catalog, _ := catalogs.New(
    catalogs.WithPath("./catalog"),
)

// Embedded backend (production)
catalog, _ := catalogs.New(
    catalogs.WithEmbedded(),
)

// Custom backend (S3, GCS, etc.)
catalog, _ := catalogs.New(
    catalogs.WithFS(customFS),  // Any fs.FS implementation
)
```

### Provider System (internal/sources/providers)

Simplified architecture without complex registries:

```go
// Direct client creation via switch statement
func getClient(provider *catalogs.Provider) (Client, error) {
    switch provider.ID {
    case "openai":
        return openai.NewClient(provider)
    case "anthropic":
        return anthropic.NewClient(provider)
    // ... other providers
    }
}
```

Adding a new provider:
1. Add to `internal/embedded/catalog/providers.yaml`
2. Create client in `internal/sources/providers/<provider>/`
3. Add case to switch in `providers.go`
4. Update testdata: `starmap testdata --provider <provider> --update`

## Data Sources and Their Roles

### Provider APIs
- **Authority**: Model existence, availability, deprecation
- **Provides**: Real-time model list, basic capabilities
- **Missing**: Pricing (most providers), detailed metadata

### models.dev Repository
- **Authority**: Pricing, accurate limits, provider logos
- **Provides**: Community-verified pricing, SVG logos, enhanced metadata
- **Source**: https://models.dev (cloned/updated during sync)

### Embedded Catalog
- **Authority**: Baseline data, manual corrections
- **Provides**: Starting point shipped with binary
- **Location**: `internal/embedded/catalog/`

### Local Files
- **Authority**: User customizations
- **Provides**: Local overrides, custom models
- **Usage**: `--input` flag for development

## Critical Implementation Notes

### Go Embed Constraints
- Cannot use `**` patterns (no recursive glob)
- Changes require binary rebuild
- Workaround: Use `--input ./internal/embedded/catalog` for development

### Thread Safety
- All catalog operations return copies
- Collections (Models, Providers, etc.) are thread-safe
- Event hooks called sequentially
- RWMutex protects internal state

### Provider Isolation
- Models compared only within provider boundaries during sync
- Cross-provider hosting supported (e.g., OpenAI models on Groq)
- Each provider's models synced against that provider's API only

### Merge vs Reconcile Decision

Use **pkg/catalogs** merge when:
- Working with 1-2 sources
- Simple last-write-wins is sufficient
- Testing or development

Use **pkg/reconcile** when:
- Combining 3+ sources
- Different sources authoritative for different fields
- Need provenance tracking
- Production systems

## Environment Variables

### Provider API Keys
```bash
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
GOOGLE_API_KEY=...
GROQ_API_KEY=...
DEEPSEEK_API_KEY=...
CEREBRAS_API_KEY=...
```

### Google Vertex AI (Alternative Auth)
```bash
# Option 1: Environment variables
GOOGLE_VERTEX_PROJECT=my-project
GOOGLE_VERTEX_LOCATION=us-central1
GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json

# Option 2: gcloud CLI (recommended for dev)
gcloud config set project my-project
gcloud auth application-default login
```

## Code Patterns and Conventions

### Variadic Options Pattern
```go
// Preferred (idiomatic Go)
obj, err := New(
    WithOption1(value1),
    WithOption2(value2),
)

// NOT method chaining
obj := New().WithOption1(value1).WithOption2(value2)
```

### Error Handling
```go
// Multiple errors
return errors.Join(err1, err2)

// Wrapping with context
return fmt.Errorf("reconciling catalogs: %w", err)
```

### Testing Patterns
```go
// Use testdata for API responses
response := loadTestdataResponse(t, "models_list.json")

// Update testdata with -update flag
if *update {
    saveTestdataResponse(t, response, "models_list.json")
}
```

## Project Structure

```
starmap/
├── pkg/                    # Public packages
│   ├── catalogs/          # Catalog abstraction
│   ├── reconcile/         # Multi-source reconciliation
│   └── sources/           # Source interfaces
├── internal/              # Private implementation
│   ├── embedded/          # Embedded catalog data
│   │   └── catalog/       # YAML files and logos
│   └── sources/           
│       ├── providers/     # Provider API clients
│       └── modelsdev/     # models.dev integration
├── cmd/starmap/           # CLI implementation
└── docs/                  # Generated documentation
```

## Common Development Tasks

### Adding Provider Support
1. Edit `internal/embedded/catalog/providers.yaml`
2. Create `internal/sources/providers/<provider>/client.go`
3. Update switch in `internal/sources/providers/providers.go`
4. Run `starmap testdata --provider <provider> --update`
5. Add tests in `client_test.go`

### Modifying Embedded Data
```bash
# Edit YAML directly
vim internal/embedded/catalog/providers/openai/gpt-4o.yaml

# Test without rebuild
starmap sync --input ./internal/embedded/catalog --dry-run

# Apply changes
starmap sync --input ./internal/embedded/catalog --auto-approve
```

### Debugging Reconciliation
```go
// Enable provenance tracking
reconciler, _ := reconcile.New(
    reconcile.WithProvenance(true),
)

// Check provenance in result
result, _ := reconciler.ReconcileCatalogs(ctx, sources)
for _, prov := range result.Provenance {
    log.Printf("Field %s from %s", prov.Field, prov.Source)
}
```

## Key Files to Understand

### Core Interfaces
- `pkg/catalogs/catalog.go` - Catalog interface definition
- `pkg/sources/source.go` - Source interface definition  
- `pkg/reconcile/reconcile.go` - Reconciliation interface

### Implementation Entry Points
- `sync.go` - Main sync command using reconciliation
- `starmap.go` - Starmap interface with event hooks
- `internal/sources/providers/providers.go` - Provider source implementation

### Configuration
- `internal/embedded/catalog/providers.yaml` - Provider definitions
- `internal/embedded/catalog/authors.yaml` - Author metadata
- `internal/embedded/catalog/providers/*/` - Model YAML files