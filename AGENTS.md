# AGENTS.md

> LLM coding assistant instructions for the Starmap project

This file provides Codex with project-specific guidance for working in this repository. For technical architecture details, see **[ARCHITECTURE.md](docs/ARCHITECTURE.md)**.

---

## Go Development Standards

**Starmap supports Go 1.25 and uses Go 1.26.5 for development and releases.** Write idiomatic, thread-safe, production-ready code:

- **Simplicity over cleverness** - Follow Effective Go, prioritize readability
- **Thread safety first** - Deep copies for shared data, proper RWMutex usage
- **Typed errors only** - Use `pkg/errors` types, wrap with context, no panic/recover
- **Measure then optimize** - Profile before optimizing, understand allocations
- **Table-driven tests** - Use testdata patterns, always run with `-race`
- **Context propagation** - Cancel-aware operations, timeout handling
- **Defer cleanup** - Always close resources properly
- **Godoc everything exported** - Clear documentation on public APIs

---

## Project Overview

Starmap is a unified AI model catalog system that combines data from provider APIs, models.dev, and embedded sources into a single authoritative catalog.

**For technical deep dive:** See [ARCHITECTURE.md](docs/ARCHITECTURE.md)

## Quick Start

```bash
make all                                # Clean, format, lint, test, build
starmap update                          # Update local catalog
starmap update openai                   # Update specific provider
make testdata PROVIDER=openai SOURCE=models # Refresh one governed observation
```

## Tech Stack

- **Language floor**: Go 1.25.0 (tested with Go 1.25.12)
- **Development/release toolchain**: Go 1.26.5
- **Build System**: Make (see Makefile)
- **Key Dependencies**: zerolog (logging), cobra (CLI), goccy/go-yaml (YAML)
- **Testing**: Go testing, deterministic module-local fixtures plus separately governed provider observations, always with `-race`
- **Providers**: OpenAI, Anthropic, Google AI/Vertex, Groq, DeepSeek, Cerebras, Alibaba Cloud, Fireworks AI, DeepInfra

## ⚠️ Critical Rules (YOU MUST FOLLOW)

### Thread Safety

**See docs/ARCHITECTURE.md § Thread Safety for full details**

**Publish concrete immutable catalogs; copy values at collection boundaries:**

```go
// ❌ WRONG - publishes a mutable builder
func Catalog() *catalogs.Builder { return s.builder }

// ✅ CORRECT - atomically returns the concrete immutable catalog
func Catalog() *catalogs.Catalog {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.catalog
}
```

**Key patterns:**
- `*catalogs.Builder` is the advanced mutation type; publish via `Builder.Build()`
- Published catalogs are immutable and swapped as complete generations
- Collection reads return caller-owned deep copies
- Double-checked locking for singletons
- RWMutex for concurrent access

### Error Handling

**ALWAYS use typed errors:**

```go
// Use typed errors from pkg/errors
&errors.NotFoundError{Resource: "model", ID: modelID}
&errors.SyncError{Provider: "openai", Err: err}
&errors.APIError{Provider: "openai", Endpoint: "/models", StatusCode: 500}
```

### Constants

**NEVER hardcode values:**

```go
// ✅ CORRECT
constants.DirPermissions
constants.DefaultTimeout

// ❌ WRONG
0755
30*time.Second
```

### Provider Connectors

Use the complete role decision tree in
[docs/ADDING_PROVIDERS.md](docs/ADDING_PROVIDERS.md). OpenAI compatibility is
the first question, but configuration-only provider, connector, adapter,
regional/account-source, and live-pricing-importer roles are distinct.
Connectors live in `internal/connectors`; they own deliberately reusable
protocol behavior and must not contain named provider policy. Provider-specific
acquisition lives in `internal/providers/<provider>/client.go`. `pricing.go` is only for a
live official pricing parser/importer, and `response_schema_test.go` is
supplemental connector coverage that never replaces behavioral tests.

### Testdata Updates

After making changes to provider code:

```bash
go test -race ./internal/connectors/... ./internal/providers/<provider>
make testdata PROVIDER=<provider> SOURCE=<source> # explicit governed raw-observation refresh
make provider-contract-check
```

## Architecture Quick Reference

For detailed architecture, see [ARCHITECTURE.md](docs/ARCHITECTURE.md). Here's a quick reference:

### System Layers

```
User Interfaces (CLI, Go Package)
    ↓
Application Layer (internal/application/ interface, cmd/starmap/app/ implementation)
    ↓
Root Package (starmap.Client - public API)
    ↓
Core Packages (catalogs, catalogstore, reconciler, authority, sources)
    ↓
Internal Implementations (embedded, providers, modelsdev)
```

**Key files:**
- `starmap.go` - Public API
- `sync.go` - 13-step sync pipeline
- `internal/application/application.go` - Application interface
- `cmd/starmap/app/app.go` - App implementation

### Sync Pipeline (13 Steps)

See docs/ARCHITECTURE.md § Sync Pipeline for details:

1. Check/set context
2. Parse options
3. Setup timeout
4. Load embedded catalog
5. Validate options
6. Filter sources
7. **Resolve dependencies** (check/install missing deps, filter optional sources)
8. Setup cleanup
9. Fetch from sources (concurrent)
10. Get existing catalog
11. Reconcile catalogs
12. Log changes
13. Save if not dry-run

### Reconciliation System

See docs/ARCHITECTURE.md § Reconciliation System for details:

- **Authority Strategy**: Field-level priorities (default)
- **Source Order Strategy**: Fixed precedence
- Sources: Provider APIs, models.dev (Git/HTTP), Local, Embedded
- Field-level authority determines which source wins

## Common Development Tasks

### Add New Provider

Follow [docs/ADDING_PROVIDERS.md](docs/ADDING_PROVIDERS.md); it is the normative
decision tree and data-ownership contract.

1. Put stable endpoint/response/offering interpretation in `providers.yaml`.
2. Use YAML-only protocol-connector configuration when it is sufficient; add no provider production Go.
3. Add `adapter.go` only for irreducible provider record semantics.
4. Add provider `client.go` for single-provider acquisition, a protocol connector only for deliberate reuse, or provider `source.go` for regional/account sweeps.
5. Put reviewed prices/lifecycle/capabilities in schema-v2 catalog data, not Go tables.
6. Own connector `client_test.go`, provider `provider_test.go`/`adapter_test.go`, or `source_test.go` according to the selected role.
7. Refresh eligible global-public raw evidence with `make testdata PROVIDER=<provider> SOURCE=<source>`; credential-scoped raw captures are prohibited and use deterministic SDK/client fakes.
8. Run `make provider-contract-check` plus the focused race, catalog, docs, and repository gates.

### Modify Sync Logic

See docs/ARCHITECTURE.md § Sync Pipeline for 12-step process.

The sync pipeline is in `sync.go` with staged execution:
- Filter → Fetch (concurrent) → Reconcile → Save
- Each stage has clear purpose and error handling

### Update Reconciliation

See docs/ARCHITECTURE.md § Reconciliation System for strategy details.

- Modify authorities: `pkg/authority/authority.go`
- Strategies: `pkg/reconciler/strategy.go`
- Field patterns support wildcards: "Pricing.*"

### Working with Commands

Commands use dependency injection via `application.Application` interface:

```go
// Commands accept Application interface
func NewCommand(app application.Application) *cobra.Command {
    return &cobra.Command{
        RunE: func(cmd *cobra.Command, args []string) error {
            catalog, err := app.Catalog()  // Atomic immutable generation
            // ... use catalog
        },
    }
}
```

### Dependency Management

Sources can declare external dependencies via `Dependencies()` interface method. The dependency resolver runs in step 7 of the sync pipeline before fetch.

**Implementation:**
- Add `Dependencies() []Dependency` and `IsOptional() bool` methods to Source
- Core resolution is noninteractive by default; the CLI supplies the only prompt adapter
- Explicit noninteractive policies support auto-install or optional-source skipping
- Optional sources are gracefully skipped if deps missing
- Use `//nolint:gosec` for subprocess calls (commands from trusted source code)

See `internal/deps/checker.go` and `lifecycle.go:resolveDependencies()` for implementation.

## Code Patterns

### Functional Options

Used throughout for configuration:

```go
// Creating instances
sm, _ := starmap.New(
    starmap.WithCatalogExportPath("./exports/catalog"),
)

// Sync options
result, _ := sm.Sync(ctx,
    sync.WithProvider("openai"),
    sync.WithDryRun(true),
)
```

See examples: `starmap.New()`, `catalogs.New(catalogs.WithEmbedded())`, `catalogs.Empty()`, `sync.WithProvider()`

### Dependency Injection

See docs/ARCHITECTURE.md § Application Layer for interface pattern.

```go
// Interface defined where it's used (internal/application/)
type Application interface {
    Catalog() (*catalogs.Catalog, error)
    Starmap(...starmap.Option) (starmap.Client, error)
    Logger() *zerolog.Logger
    // ...
}

// Implementation in cmd/starmap/app/
type App struct { /* ... */ }
func (a *App) Catalog() (*catalogs.Catalog, error) { /* ... */ }
```

### Concurrent Fetching

See docs/ARCHITECTURE.md § Data Sources for concurrent pattern.

Provider APIs fetched in parallel using goroutines + channels:
```go
results := make(chan Result, len(providers))
for _, provider := range providers {
    go func(p Provider) {
        models, err := p.Client.ListModels(ctx)
        results <- Result{Provider: p, Models: models, Error: err}
    }(provider)
}
```

### Merge Strategies

- `MergeReplaceAll`: Provider APIs (fresh data replaces all)
- `MergeAdditive`: models.dev (enhancements, no removal)

## Package Map

**Core packages**: catalogs, catalogstore, reconciler, authority, sources, errors, logging, constants, convert

**Internal**: embedded, server, server/handlers, sources/{providers,modelsdev,local}, connectors/{openai,anthropic,google}, providers/{registry,clients,adapters,regional-sources,fixtures}, transport

**Application**: internal/application (interface), cmd/starmap/app (implementation)

See [ARCHITECTURE.md § Package Organization](docs/ARCHITECTURE.md#package-organization) for full structure.

## Environment Variables

**Required** (per provider):
```bash
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
GOOGLE_API_KEY=...
GROQ_API_KEY=...
DEEPSEEK_API_KEY=...
CEREBRAS_API_KEY=...
DASHSCOPE_API_KEY=...
FIREWORKS_API_KEY=...
DEEPINFRA_TOKEN=...
```

`DEEPINFRA_TOKEN` is optional for DeepInfra catalog fetching because `GET /v1/openai/models` is public, but it is required for DeepInfra inference calls.

**Alibaba Cloud Model Studio** (optional non-US workspace domains):
```bash
ALIBABA_MODEL_STUDIO_BASE_URL=https://{WorkspaceId}.ap-southeast-1.maas.aliyuncs.com/compatible-mode/v1
```

**Google Vertex** (optional):
```bash
GOOGLE_VERTEX_PROJECT=my-project
GOOGLE_VERTEX_LOCATION=us-central1
GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json
```

## Key File Locations

| File | Purpose |
|------|---------|
| `starmap.go` | Public API interface |
| `sync.go` | 12-step sync pipeline |
| `internal/application/application.go` | Application interface (idiomatic location) |
| `cmd/starmap/app/app.go` | App implementation |
| `cmd/starmap/cmd/serve/command.go` | HTTP server CLI command |
| `internal/server/server.go` | Server lifecycle & dependencies |
| `internal/server/router.go` | Route registration & middleware |
| `internal/server/handlers/handlers.go` | Handler base structure |
| `pkg/reconciler/reconciler.go` | Multi-source reconciliation |
| `pkg/authority/authority.go` | Field-level authorities |
| `internal/sources/providers/providers.go` | Concurrent provider fetching |
| `internal/providers/registry/provider.go` | Provider acquisition registry |
| `internal/embedded/catalog/providers.yaml` | Provider configurations |

## Development Commands

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
make catalog-generation-check               # Verify safe download/promotion/CLI tooling
make provider-contract-check                 # Verify provider roles, fixtures, and refresh contract
make testdata                               # Refresh all checked-in governed observations
make testdata PROVIDER=openai SOURCE=models # Refresh one exact logical source
```

### Documentation

```bash
make generate       # Generate Go docs
make godoc          # Go docs only
make docs-check     # Verify docs current (CI)
```

## Implementation Notes

**Important Constraints:**
- Go embed requires binary rebuild for catalog changes
- Sync pipeline order: Filter → Fetch → Reconcile → Save
- Provider sources fetch concurrently (goroutines + channels)
- Authority system determines field-level source priority
- Always prefer editing existing files over creating new ones

**Thread Safety:**
- Collections return values (not pointers) for safety
- All catalog access returns deep copies
- See docs/ARCHITECTURE.md § Thread Safety for full guidelines

**Import Cycles:**
- Zero import cycles (validated)
- Dependency flow is unidirectional (see docs/ARCHITECTURE.md)
- Commands import `internal/application/` interface, NOT `cmd/starmap/app/`

## References

- **[ARCHITECTURE.md](docs/ARCHITECTURE.md)** - Technical deep dive (system components, thread safety, sync pipeline)
- **[CLI.md](docs/CLI.md)** - CLI implementation reference (flags, patterns, examples)
- **[README.md](README.md)** - User-facing documentation
- **[Makefile](Makefile)** - Build automation and commands
- **[pkg/*/README.md](pkg/)** - Individual package documentation
