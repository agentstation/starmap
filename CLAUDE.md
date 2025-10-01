# CLAUDE.md

This file provides guidance to Claude Code when working in this repository.

Starmap is a unified AI model catalog system that combines data from provider APIs, models.dev, and embedded sources into a single authoritative catalog.

## Quick Start

```bash
make all                                                    # Clean, format, lint, test, build
starmap update                                              # Update local catalog
starmap update --provider openai                            # Update specific provider
go test ./internal/sources/providers/openai -update         # Update testdata
```

## Tech Stack

- **Language**: Go 1.24.6
- **Build System**: Make (see Makefile)
- **Key Dependencies**: zerolog (logging), cobra (CLI), goccy/go-yaml (YAML)
- **Testing**: Go testing, testdata pattern with `-update` flag
- **Providers**: OpenAI, Anthropic, Google AI/Vertex, Groq, DeepSeek, Cerebras

## ⚠️ Critical Rules (YOU MUST FOLLOW)

**Thread Safety - NEVER return direct references:**
```go
// ❌ WRONG - returns mutable reference
func Catalog() catalogs.Catalog { return s.catalog }

// ✅ CORRECT - returns deep copy
func Catalog() (catalogs.Catalog, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.catalog.Copy()  // Always copy
}
```

**Error Handling - ALWAYS use typed errors:**
- Use `&errors.NotFoundError{Resource, ID}`
- Use `&errors.SyncError{Provider, Err}`
- Use `&errors.APIError{Provider, Endpoint, StatusCode}`

**Constants - NEVER hardcode values:**
- Use `constants.DirPermissions` not `0755`
- Use `constants.DefaultTimeout` not `30*time.Second`

**Provider Clients - Check OpenAI-compatible first:**
- Most providers use unified OpenAI client (`internal/sources/providers/openai/client.go`)
- Only create custom client if API is incompatible
- Register in `internal/sources/clients/factory.go`

**Testdata - Update after provider changes:**
```bash
go test ./internal/sources/providers/<provider> -update
```

## Architecture

### Two-Tier System
1. **Simple Merging** (`pkg/catalogs/`) - 1-2 sources, last-write-wins
2. **Complex Reconciliation** (`pkg/reconciler/`) - 3+ sources, field-level authority via `pkg/authority`

### Sync Pipeline
Staged execution in `sync.go`: Filter → Fetch (concurrent) → Reconcile → Save

### Data Sources & Authority
- **Provider APIs**: Model existence (concurrent goroutines + channels)
- **models.dev**: Pricing, logos (Git/HTTP)
- **Embedded**: Baseline data
- **Local**: User overrides

Field-level authority in `pkg/authority` determines which source wins.

### Interface Composition
`Starmap` = Catalog + Updater + Persistence + AutoUpdater + Hooks (see `starmap.go`)

## Package Map

**Core packages**: catalogs, reconciler, authority, sources, errors, logging, constants, convert
**Internal**: embedded, sources/{providers,modelsdev,local,clients}, transport, tools/docs

## Common Tasks

### Add New Provider
1. Add to `internal/embedded/catalog/providers.yaml`
2. Check if OpenAI-compatible (most are: OpenAI, Groq, DeepSeek, Cerebras)
3. If compatible: Configure in YAML. If not: Create custom client
4. Register in `internal/sources/clients/factory.go`
5. Update testdata: `go test ./internal/sources/providers/<provider> -update`

### Modify Embedded Data
`starmap update --input ./internal/embedded/catalog --dry-run`

## Code Patterns

### Functional Options
See `starmap.New()`, `catalogs.New()`, `sync.WithProvider()` for examples.

### Concurrent Fetching
See `internal/sources/providers/providers.go` for goroutine + channel pattern.

### Merge Strategies
- `MergeReplaceAll`: Provider APIs (fresh data)
- `MergeAdditive`: models.dev (enhancements)

## Environment Variables

**Required** (per provider): `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GOOGLE_API_KEY`, `GROQ_API_KEY`, `DEEPSEEK_API_KEY`, `CEREBRAS_API_KEY`
**Google Vertex** (optional): `GOOGLE_VERTEX_PROJECT`, `GOOGLE_VERTEX_LOCATION`

## Key Files

- `sync.go` - Pipeline orchestration
- `starmap.go` - Interface composition
- `internal/sources/providers/providers.go` - Concurrent fetching
- `internal/sources/clients/factory.go` - Provider registry
- `internal/embedded/catalog/providers.yaml` - Provider configs

## Commands

### Build & Development
```bash
make all            # Full cycle: clean, fix, lint, test, build
make build          # Build binary
make install        # Install to $GOPATH/bin
make fix            # Format and tidy
make lint           # Run linters
```

### Testing
```bash
make test                                   # Run all tests
go test ./pkg/catalogs -race -v            # Race detection
go test ./... -race -short                 # All packages with race detector
```

### Catalog Management
```bash
make update-catalog                         # Update embedded catalog (all providers)
make update-catalog-provider PROVIDER=openai  # Update specific provider
make testdata-update                        # Update all testdata
make testdata-verify                        # Verify testdata validity
```

### Documentation
```bash
make generate       # Generate all docs (Go + catalog)
make godoc          # Go docs only
make catalog-docs   # Catalog docs only
make docs-check     # Verify docs current (CI)
```

## Implementation Notes

- Go embed requires binary rebuild for catalog changes
- Sync pipeline order: Filter → Fetch → Reconcile → Save
- Provider sources fetch concurrently (goroutines + channels)
- Authority system determines field-level source priority
- Always prefer editing existing files over creating new ones