# CLAUDE.md

Starmap is a unified AI model catalog system that combines data from provider APIs, models.dev, and embedded sources into a single authoritative catalog.

## Quick Start

```bash
# Development
make all                              # Clean, format, lint, test, build
starmap update                        # Update local catalog
starmap update --provider openai      # Update specific provider

# Testing
go test ./pkg/reconcile/...           # Test specific package
go test ./internal/sources/providers/openai -update  # Update testdata
```

## Architecture

### Two-Tier System
1. **Simple Merging** (`pkg/catalogs/`) - 1-2 sources, last-write-wins
2. **Complex Reconciliation** (`pkg/reconcile/`) - 3+ sources, field-level authority

### Data Sources & Authority
- **Provider APIs**: Model existence, availability
- **models.dev**: Pricing, limits, logos
- **Embedded**: Baseline data, manual fixes
- **Local**: User overrides (`--input` flag)

## Package Map

| Core | Internal |
|------|----------|
| `pkg/catalogs` - Catalog abstraction | `internal/embedded` - Embedded data |
| `pkg/reconcile` - Multi-source merge | `internal/sources/providers` - API clients |
| `pkg/sources` - Source interfaces | `internal/sources/modelsdev` - models.dev |
| `pkg/errors` - Typed errors | `internal/sources/local` - Local files |
| `pkg/logging` - Structured logs | `internal/sources/registry` - Client registry |
| `pkg/constants` - No magic numbers | `internal/transport` - HTTP client |
| `pkg/convert` - Format conversion | `internal/tools/docs` - Doc generation |

## Common Tasks

### Add New Provider
```bash
# 1. Add to providers.yaml
vim internal/embedded/catalog/providers.yaml

# 2. Create client
vim internal/sources/providers/<provider>/client.go

# 3. Add to switch statement
vim internal/sources/providers/providers.go  # Add case

# 4. Update testdata
starmap testdata --provider <provider> --update
```

### Key Commands
- `starmap list models`: View models in local catalog
- `starmap fetch models --provider openai`: Get models from API
- `starmap update`: Sync catalog from all sources

### Modify Embedded Data
```bash
# Edit without rebuild
starmap update --input ./internal/embedded/catalog --dry-run
```

## Code Patterns

### Error Handling
```go
// Use typed errors
return &errors.NotFoundError{Resource: "model", ID: id}

// Check types
if errors.IsNotFound(err) { /* 404 */ }
```

### Constants
```go
import "github.com/agentstation/starmap/pkg/constants"
os.MkdirAll(dir, constants.DirPermissions)  // Not 0755
```

### Options Pattern
```go
catalog, _ := catalogs.New(
    catalogs.WithPath("./catalog"),
    catalogs.WithEmbedded(),
)
```

### Interface Segregation
```go
func ProcessModels(r catalogs.Reader) { }  // Read-only
func UpdateCatalog(w catalogs.Writer) { }  // Write-only
```

## Environment Variables

```bash
# Provider API Keys
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
GOOGLE_API_KEY=...
GROQ_API_KEY=...
DEEPSEEK_API_KEY=...
CEREBRAS_API_KEY=...

# Google Vertex (optional)
GOOGLE_VERTEX_PROJECT=my-project
GOOGLE_VERTEX_LOCATION=us-central1
```

## Key Files

- `sync.go` - Core sync/update logic
- `starmap.go` - Core interface
- `cmd/starmap/cmd/update.go` - Update command implementation
- `cmd/starmap/cmd/fetch.go` - Fetch command implementation
- `cmd/starmap/cmd/list.go` - List command implementation
- `internal/sources/providers/providers.go` - Provider registry
- `internal/embedded/catalog/providers.yaml` - Provider definitions

## Build Commands

```bash
make build          # Build binary
make install        # Install to $GOPATH/bin
make fix            # Format and tidy
make lint           # Run linters
make test           # Run tests
make all            # Complete cycle: clean, fix, lint, test, build
```

## Documentation & Embedded Catalog

```bash
# Update embedded catalog data
make update-catalog                    # Update all providers in embedded catalog
make update-catalog-provider PROVIDER=openai  # Update specific provider

# Generate documentation
make generate       # Generate all docs (Go + catalog)
make godoc          # Generate Go package docs only  
make catalog-docs   # Generate catalog docs only
make docs-check     # Verify docs are current (CI)

# Testdata management
make testdata                          # Show testdata help
make testdata-update                   # Update all provider testdata
make testdata-verify                   # Verify testdata validity
```

## Implementation Notes

- Go embed requires binary rebuild for changes
- All catalog operations return copies (thread-safe)
- Models compared only within provider boundaries
- Use reconcile for 3+ sources, merge for 1-2 sources

# Important Instructions
When working in this codebase:
- ALWAYS use typed errors from pkg/errors
- NEVER hardcode values - use pkg/constants
- Prefer editing existing files over creating new ones
- Run `make lint` after changes
- Update testdata with `-update` flag when changing provider clients