# Starmap Architecture

> Technical deep dive into Starmap's system design, components, and patterns

**Last Updated:** 2025-10-14
**Status:** Production-ready architecture following idiomatic Go patterns

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [System Components](#system-components)
- [Application Layer](#application-layer)
- [Core Package Layer](#core-package-layer)
- [Root Package (starmap.Client)](#root-package-starmapclient)
- [Data Sources](#data-sources)
- [Sync Pipeline](#sync-pipeline)
- [Reconciliation System](#reconciliation-system)
- [Thread Safety](#thread-safety)
- [Package Organization](#package-organization)
- [Testing Strategy](#testing-strategy)
- [References](#references)

## Overview

Starmap is a unified AI model catalog system that combines data from multiple sources into a single authoritative catalog. The architecture follows idiomatic Go patterns with a focus on:

- **Separation of concerns**: Clear boundaries between layers
- **Dependency injection**: Interface-based design for testability
- **Thread safety**: Value semantics and proper synchronization
- **Extensibility**: Plugin patterns for sources, strategies, and storage backends

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    User Interfaces                          │
│              CLI │ Go Package │ Future HTTP API              │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────┐
│                Application Layer (cmd/application/)          │
│              Application interface (DI pattern)              │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────┐
│               Root Package (starmap.Client)                  │
│       Sync Pipeline │ Event Hooks │ Auto-updates            │
└────┬──────────────┬──────────────┬──────────────┬───────────┘
     │              │              │              │
┌────▼────┐  ┌─────▼─────┐  ┌────▼─────┐  ┌─────▼──────┐
│Catalogs │  │Reconciler │  │Authority │  │  Sources   │
│ Storage │  │Multi-src  │  │Field-lvl │  │Interfaces  │
└─────────┘  └───────────┘  └──────────┘  └────────────┘
```

## Design Principles

### 1. Interface Segregation
- **Define interfaces where they're used** (Go proverb)
- Application interface in `cmd/application/` (reusable across binaries)
- Implementation in `cmd/starmap/app/` (concrete types)
- Commands depend only on what they need

### 2. Dependency Injection
- Constructor injection via functional options
- Interface-based contracts
- Easy mocking for tests
- Example: `NewCommand(app application.Application)`

### 3. Thread Safety
- Value semantics for collections
- Deep copy on read
- Double-checked locking for singletons
- RWMutex for concurrent access
- See [Thread Safety](#thread-safety) section for details

### 4. Single Responsibility
- Each package has one clear purpose
- Catalog: storage abstraction
- Reconciler: multi-source merging
- Authority: field-level priorities
- Sources: data fetching

### 5. Explicit Error Handling
- Typed errors in `pkg/errors`
- No panics in library code
- Errors wrap context
- Examples: `NotFoundError`, `SyncError`, `APIError`

## System Components

### Layer Responsibilities

1. **Application Layer** (`cmd/application/`, `cmd/starmap/app/`)
   - Dependency injection
   - Configuration management
   - Lifecycle control (startup/shutdown)
   - Singleton management

2. **Root Package** (`starmap.Client`)
   - Public API surface
   - Sync orchestration
   - Event hooks
   - Auto-updates

3. **Core Packages** (`pkg/`)
   - Catalog storage (`pkg/catalogs/`)
   - Multi-source reconciliation (`pkg/reconciler/`)
   - Field-level authority (`pkg/authority/`)
   - Data source abstractions (`pkg/sources/`)

4. **Internal Implementations** (`internal/`)
   - Embedded catalog data
   - Provider API clients
   - models.dev integration
   - Transport utilities

## Application Layer

### Application Interface

Location: `cmd/application/application.go`

**Design Philosophy:**
- "Accept interfaces, return structs" (Go proverb)
- "Define interfaces where they're used" (idiomatic Go)
- Located at `cmd` root for reusability across multiple binaries
- Zero import cycles (unidirectional dependency flow)

**Interface Definition:**

```go
type Application interface {
    // Catalog returns a deep copy of the current catalog
    Catalog() (catalogs.Catalog, error)

    // Starmap returns starmap instance with optional configuration
    // Without options: returns cached instance (thread-safe singleton)
    // With options: creates new instance (no caching)
    Starmap(opts ...starmap.Option) (starmap.Client, error)

    // Logger returns the configured logger
    Logger() *zerolog.Logger

    // OutputFormat returns configured output format
    OutputFormat() string

    // Version info methods
    Version() string
    Commit() string
    Date() string
    BuiltBy() string
}
```

**Dependency Flow:**
```
cmd/application/ (interface)
    ↑
cmd/starmap/cmd/* (commands use interface)
    ↑
cmd/starmap/app/ (App implements interface)
```

### App Implementation

Location: `cmd/starmap/app/app.go`

**Responsibilities:**
- Implements `Application` interface
- Manages configuration, logger, starmap singleton
- Thread-safe lazy initialization
- Graceful lifecycle management

**Key Components:**

```go
type App struct {
    version string
    commit  string
    date    string
    builtBy string

    config  *Config
    logger  *zerolog.Logger

    mu      sync.RWMutex
    starmap starmap.Client  // Lazy-initialized singleton
}
```

**Thread-Safe Singleton Pattern:**

The App uses double-checked locking for optimal performance:

```go
func (a *App) Starmap(opts ...starmap.Option) (starmap.Client, error) {
    // Fast path: read lock check
    a.mu.RLock()
    if a.starmap != nil && len(opts) == 0 {
        sm := a.starmap
        a.mu.RUnlock()
        return sm, nil
    }
    a.mu.RUnlock()

    // Slow path: write lock initialization
    a.mu.Lock()
    defer a.mu.Unlock()

    // Double-check after acquiring write lock
    if a.starmap != nil && len(opts) == 0 {
        return a.starmap, nil
    }

    // Create instance (new if opts provided)
    sm, err := starmap.New(...)
    if err != nil {
        return nil, err
    }

    // Cache only if no custom options
    if len(opts) == 0 {
        a.starmap = sm
    }

    return sm, nil
}
```

## Core Package Layer

### Catalogs Package

Location: `pkg/catalogs/`

**Purpose:** Unified storage abstraction with pluggable backends

**Key Types:**
- `Catalog` - Main interface for catalog operations
- `Model`, `Provider`, `Author`, `Endpoint` - Core data types
- Collections: `Providers`, `Authors`, `Models`, `Endpoints`

**Storage Backends:**
- Memory (testing)
- Filesystem (development)
- Embedded (production)
- Custom FS (S3, GCS, etc.)

**Thread Safety:** Value semantics, all List() methods return slices of values (not pointers)

See [pkg/catalogs/README.md](pkg/catalogs/README.md) for details.

### Reconciler Package

Location: `pkg/reconciler/`

**Purpose:** Multi-source data reconciliation with conflict resolution

**Key Components:**
- `Reconciler` interface
- `Strategy` - Defines how conflicts are resolved
- `Result` - Reconciliation outcome with changeset and metadata

**Strategies:**
1. **AuthorityStrategy** - Field-level authority priorities
2. **SourceOrderStrategy** - Fixed source precedence order

**Pipeline:**
1. Fetch catalogs from all sources
2. Merge using configured strategy
3. Detect changes vs baseline
4. Generate changeset with provenance
5. Return result

See [pkg/reconciler/README.md](pkg/reconciler/README.md) for details.

### Authority Package

Location: `pkg/authority/`

**Purpose:** Field-level source authority system

**How It Works:**
- Each field (e.g., "Pricing", "Limits") has authority configuration
- Sources ranked by priority for that field
- Pattern matching supports wildcards: "Pricing.*"
- Higher priority wins in conflicts

**Example Authorities:**

```go
// Pricing - models.dev is most reliable
{Path: "Pricing", Source: sources.ModelsDevHTTPID, Priority: 110}
{Path: "Pricing", Source: sources.ModelsDevGitID, Priority: 100}

// Availability - Provider API is truth
{Path: "Features", Source: sources.ProvidersID, Priority: 95}

// Descriptions - prefer manual edits
{Path: "Description", Source: sources.LocalCatalogID, Priority: 90}
```

See `pkg/authority/authority.go` for complete authority configuration.

### Sources Package

Location: `pkg/sources/`

**Purpose:** Abstraction for fetching data from external systems

**Source Interface:**

```go
type Source interface {
    Type() Type
    ID() ID
    Fetch(ctx context.Context, opts ...Option) (catalogs.Catalog, error)
    Cleanup() error
}
```

**Source Types:**
- **Provider APIs** (`sources.ProvidersID`) - Real-time model availability
- **models.dev Git** (`sources.ModelsDevGitID`) - Community-verified pricing/logos
- **models.dev HTTP** (`sources.ModelsDevHTTPID`) - Faster HTTP API variant
- **Local Catalog** (`sources.LocalCatalogID`) - User overrides
- **Embedded** (`sources.EmbeddedID`) - Baseline data shipped with binary

See [pkg/sources/README.md](pkg/sources/README.md) for details.

## Root Package (starmap.Client)

Location: `starmap.go`, `sync.go`, `client.go`

**Purpose:** Main public API with sync orchestration and event hooks

### Client Interface

```go
type Client interface {
    // Catalog returns a copy of the current catalog
    Catalog() (catalogs.Catalog, error)

    // Sync synchronizes with provider APIs
    Sync(ctx context.Context, opts ...sync.Option) (*sync.Result, error)

    // Event hooks
    OnModelAdded(ModelAddedHook)
    OnModelUpdated(ModelUpdatedHook)
    OnModelRemoved(ModelRemovedHook)

    // Lifecycle
    AutoUpdatesOn() error
    AutoUpdatesOff() error
    Save() error
}
```

### Functional Options Pattern

Used throughout for configuration:

```go
// Creating with options
sm, err := starmap.New(
    starmap.WithAutoUpdateInterval(30 * time.Minute),
    starmap.WithLocalPath("./catalog"),
    starmap.WithAutoUpdates(true),
)

// Sync with options
result, err := sm.Sync(ctx,
    sync.WithProvider("openai"),
    sync.WithDryRun(true),
    sync.WithTimeout(5 * time.Minute),
)
```

## Data Sources

### Source Hierarchy and Authority

```
┌─────────────────────────────────────────────────────────┐
│ Local Catalog (Highest Priority for API config)        │
│   • API keys, endpoints                                 │
│   • Provider configurations                             │
│   • User overrides                                      │
└───────────────────────┬─────────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────────┐
│ Provider APIs (Authoritative for Model Existence)       │
│   • Real-time model availability                        │
│   • Basic capabilities                                  │
│   • Fetched concurrently with goroutines               │
└───────────────────────┬─────────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────────┐
│ models.dev (Authoritative for Pricing/Metadata)        │
│   • HTTP API (faster, priority 110)                     │
│   • Git clone (fallback, priority 100)                  │
│   • Community-verified pricing                          │
│   • Provider logos (SVG)                                │
└───────────────────────┬─────────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────────┐
│ Embedded Catalog (Baseline)                             │
│   • Ships with binary (go:embed)                        │
│   • Fallback data                                       │
│   • Manual corrections                                  │
└─────────────────────────────────────────────────────────┘
```

### Concurrent Fetching

Provider APIs are fetched concurrently:

```go
// internal/sources/providers/providers.go
func fetch(ctx context.Context, providers []Provider) {
    results := make(chan Result, len(providers))

    for _, provider := range providers {
        go func(p Provider) {
            models, err := p.Client.ListModels(ctx)
            results <- Result{Provider: p, Models: models, Error: err}
        }(provider)
    }

    // Collect results...
}
```

## Sync Pipeline

Location: `sync.go`

The sync pipeline executes in 12 stages:

### Stage-by-Stage Breakdown

```go
func (c *client) Sync(ctx context.Context, opts ...sync.Option) (*sync.Result, error) {
    // Stage 1: Check and set context
    if ctx == nil {
        ctx = context.Background()
    }

    // Stage 2: Parse options with defaults
    options := sync.Defaults().Apply(opts...)

    // Stage 3: Setup context with timeout
    if options.Timeout > 0 {
        ctx, cancel = context.WithTimeout(ctx, options.Timeout)
        defer cancel()
    }

    // Stage 4: Load embedded catalog for validation
    embedded, err := catalogs.NewEmbedded()

    // Stage 5: Validate options upfront
    err = options.Validate(embedded.Providers())

    // Stage 6: Filter sources by options
    srcs := c.filterSources(options)

    // Stage 7: Setup cleanup
    defer cleanup(srcs)

    // Stage 8: Fetch catalogs from all sources
    err = fetch(ctx, srcs, options.SourceOptions())

    // Stage 9: Get existing catalog for baseline
    existing, err := c.Catalog()

    // Stage 10: Reconcile catalogs from all sources
    result, err := update(ctx, existing, srcs)

    // Stage 11: Log change summary
    logging.Info().Int("added", ...).Msg("Changes detected")

    // Stage 12: Save if not dry-run
    if !options.DryRun && result.Changeset.HasChanges() {
        c.save(result.Catalog, options, result.Changeset)
    }

    return syncResult, nil
}
```

### Key Pipeline Features

- **Staged execution**: Each stage has clear purpose
- **Error handling**: Fail fast with context
- **Concurrent fetching**: Sources fetched in parallel
- **Change detection**: Diff against baseline
- **Dry-run support**: Preview without applying
- **Event triggers**: Hooks fire on successful save

## Reconciliation System

### Authority-Based Strategy

The default reconciliation strategy uses field-level authorities:

**How it works:**
1. For each field in a model, find matching authority
2. Select value from highest-priority source
3. Track provenance (which source provided which field)
4. Generate changeset by comparing with baseline

**Example:**

```
Model "gpt-4o" exists in 3 sources:
  - Provider API: { Name: "GPT-4o", Features: {...} }
  - models.dev:   { Pricing: {...}, Limits: {...} }
  - Local:        { Description: "Custom description" }

Reconciled result:
  - Name:        "GPT-4o"         (Provider API, priority 90)
  - Features:    {...}             (Provider API, priority 95)
  - Pricing:     {...}             (models.dev, priority 110)
  - Limits:      {...}             (models.dev, priority 100)
  - Description: "Custom desc"     (Local, priority 90)
```

### Changeset Generation

The reconciler generates a comprehensive changeset:

```go
type Changeset struct {
    Models struct {
        Added   []Model
        Updated []ModelUpdate
        Removed []Model
    }
    Summary struct {
        TotalChanges int
        AddedCount   int
        UpdatedCount int
        RemovedCount int
    }
}
```

**Change Detection:**
- Compare reconciled catalog with baseline
- Track field-level changes
- Preserve attribution for each field
- Generate human-readable diffs

## Thread Safety

Starmap's catalog system is designed for thread-safe concurrent access. This section consolidates all thread safety patterns and guidelines.

### Design Philosophy

**Value Semantics Over Pointer Semantics**

The catalog system uses value semantics to prevent race conditions:

```go
// ✅ CORRECT: Returns values
func (c *Catalog) Models() []Model

// ❌ WRONG: Returns pointers (race condition risk)
func (c *Catalog) Models() []*Model
```

**Deep Copy on Read**

All catalog access methods return independent copies:

```go
// Per ARCHITECTURE.md § Thread Safety section:
// This ALWAYS returns a deep copy to prevent data races
func (a *App) Catalog() (catalogs.Catalog, error) {
    a.mu.RLock()
    defer a.mu.RUnlock()
    return a.catalog.Copy()  // Single deep copy
}
```

### Core Patterns

#### 1. Double-Checked Locking (Singleton Pattern)

Used in `App.Starmap()` for optimal performance:

```go
func (a *App) Starmap(opts ...starmap.Option) (starmap.Client, error) {
    // Fast path: read lock check (common case)
    a.mu.RLock()
    if a.starmap != nil && len(opts) == 0 {
        sm := a.starmap
        a.mu.RUnlock()
        return sm, nil  // No allocation
    }
    a.mu.RUnlock()

    // Slow path: write lock initialization (rare)
    a.mu.Lock()
    defer a.mu.Unlock()

    // Double-check after acquiring write lock
    if a.starmap != nil && len(opts) == 0 {
        return a.starmap, nil
    }

    // Initialize exactly once
    sm, err := starmap.New(buildOptions()...)
    if err != nil {
        return nil, err
    }

    a.starmap = sm  // Cache for future calls
    return sm, nil
}
```

**Why double-checked locking?**
- First check (read lock): Fast path for initialized case
- Second check (write lock): Prevent race between read unlock and write lock
- Initialization happens exactly once
- Subsequent calls are fast (read lock only)

#### 2. Value Semantics in Collections

Collections return slices of values, not pointers:

```go
// Safe: Returns copies
models := catalog.Models().List()  // []Model (values)

// Each model is an independent copy
for _, model := range models {
    model.Name = "Modified"  // Only affects local copy
}
```

#### 3. Deep Copy Helpers

Every type provides deep copy methods:

```go
func (m Model) DeepCopy() Model {
    copy := m
    // Deep copy nested pointers
    if m.Pricing != nil {
        pricingCopy := *m.Pricing
        copy.Pricing = &pricingCopy
    }
    // ... copy other pointer fields
    return copy
}
```

### Safe Usage Patterns

#### ✅ Safe Concurrent Reads

```go
// Multiple goroutines can safely read
go func() {
    models := catalog.Models().List()
    // Process models...
}()

go func() {
    providers := catalog.Providers().List()
    // Process providers...
}()
```

#### ✅ Safe Concurrent Updates

```go
// Updates are atomic and thread-safe
catalog.SetModel(model1)
catalog.SetModel(model2)

// Concurrent writes are serialized internally
go func() { catalog.SetProvider(p1) }()
go func() { catalog.SetProvider(p2) }()
```

#### ❌ Avoid: Storing References Across Goroutines

```go
// Don't do this - unnecessary
models := catalog.Models().List()
go func() {
    // models already contains values, safe to use
    fmt.Println(models[0].Name)
}()

// This is fine because models are values
models[0].Name = "Modified"  // Only affects local copy
```

### Thread Safety in Storage Layer

Collections use RWMutex for concurrent access:

```go
type ProviderCollection struct {
    mu        sync.RWMutex
    providers map[ProviderID]Provider
}

func (c *ProviderCollection) Get(id ProviderID) (Provider, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    p, exists := c.providers[id]
    if !exists {
        return Provider{}, &errors.NotFoundError{...}
    }
    return p.DeepCopy(), nil  // Return copy
}

func (c *ProviderCollection) Set(provider Provider) {
    c.mu.Lock()
    defer c.mu.Unlock()

    c.providers[provider.ID] = provider.DeepCopy()
}
```

### Performance Characteristics

**Memory Impact:**
- Value semantics increase allocation during reads
- Trade-off: Safety vs. memory efficiency
- Deep copies prevent sharing but ensure correctness

**Concurrent Performance:**
- Reads scale linearly with goroutines (RWMutex)
- Writes are serialized where necessary
- Double-checked locking minimizes lock contention

**Benchmarks:**

```
BenchmarkCatalogAccess-8              1000000    350 ns/op    10 allocs/op
BenchmarkCatalogAccessWithCopy-8      1000000    725 ns/op    18 allocs/op
BenchmarkConcurrentReads-8           10000000    120 ns/op     2 allocs/op
```

After optimization (removed redundant double-copy):
```
BenchmarkCatalogAccess-8              1000000    350 ns/op     9 allocs/op  (50% faster)
```

### Testing for Thread Safety

**Race Detector:**

```bash
# Run all tests with race detector
go test -race ./...

# Run specific package
go test -race ./pkg/catalogs -v

# Benchmark with race detection
go test -race -bench=. ./pkg/catalogs
```

**Concurrent Test Pattern:**

```go
func TestConcurrentCatalogAccess(t *testing.T) {
    catalog, _ := catalogs.New()

    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            models := catalog.Models().List()
            // Use models...
        }()
    }

    wg.Wait()
}
```

### Migration Notes

The codebase has been fully migrated to value semantics:

**Completed Changes:**
- ✅ Collections return values instead of pointers
- ✅ Client interfaces return `[]Model` not `[]*Model`
- ✅ Filters work with value types
- ✅ Deep copy helpers for all types
- ✅ Double-checked locking for singletons
- ✅ Removed redundant double-copy in App.Catalog()

**Performance Improvements:**
- 50% reduction in Catalog() overhead (removed 2nd copy)
- Reduced allocations: 18 → 9-10 per call
- Maintained thread safety guarantees

### Thread Safety Checklist

When adding new code, ensure:

- [ ] Collections return values, not pointers
- [ ] Public methods that access shared state use locks
- [ ] Deep copy methods handle all pointer fields
- [ ] Tests include `-race` detector runs
- [ ] Singletons use double-checked locking
- [ ] No direct pointer returns from getters

## Package Organization

```
starmap/
├── cmd/
│   ├── application/          # Application interface (idiomatic location)
│   │   └── application.go    # Application interface definition
│   └── starmap/              # CLI binary
│       ├── main.go           # Entry point
│       ├── app/              # App implementation
│       │   ├── app.go        # App struct and methods
│       │   ├── config.go     # Configuration loading
│       │   ├── logger.go     # Logger setup
│       │   ├── context.go    # Signal handling
│       │   └── execute.go    # Command registration
│       └── cmd/              # Command implementations
│           ├── list/         # List command
│           ├── update/       # Update command
│           ├── serve/        # API server command
│           └── ...           # Other commands
│
├── pkg/                      # Public packages
│   ├── catalogs/             # Catalog storage abstraction
│   ├── reconciler/           # Multi-source reconciliation
│   ├── authority/            # Field-level authority system
│   ├── sources/              # Source interfaces
│   ├── sync/                 # Sync options and results
│   ├── errors/               # Typed errors
│   ├── logging/              # Logging utilities
│   ├── constants/            # Application constants
│   └── convert/              # Format conversion
│
├── internal/                 # Internal packages
│   ├── embedded/             # Embedded catalog data
│   │   └── catalog/          # Embedded YAML files
│   ├── sources/              # Source implementations
│   │   ├── providers/        # Provider API clients
│   │   │   ├── openai/       # OpenAI client
│   │   │   ├── anthropic/    # Anthropic client
│   │   │   ├── google-ai-studio/
│   │   │   ├── google-vertex/
│   │   │   ├── groq/
│   │   │   ├── deepseek/
│   │   │   └── cerebras/
│   │   ├── modelsdev/        # models.dev integration
│   │   ├── local/            # Local file source
│   │   └── clients/          # Client factory
│   └── transport/            # HTTP client utilities
│
├── starmap.go                # Root package - public API
├── client.go                 # Client implementation
├── sync.go                   # Sync pipeline
├── hooks.go                  # Event hooks
├── lifecycle.go              # Auto-updates
├── options.go                # Functional options
└── persistence.go            # Save/load operations
```

### Import Cycle Prevention

**Dependency Flow (Unidirectional):**

```
cmd/application/ (interface)
    ↑
cmd/starmap/cmd/* (commands)
    ↑
cmd/starmap/app/ (implementation)
    ↑
root package (starmap)
    ↑
pkg/* (core packages)
    ↑
internal/* (implementations)
```

**Rules:**
- Never import from higher layers
- Commands import `cmd/application/` interface, not `cmd/starmap/app/`
- Root package imports pkg packages
- Internal packages can import pkg packages
- Pkg packages are fully independent

## Testing Strategy

### Unit Tests

**Package-Level Tests:**

```go
// pkg/catalogs/catalog_test.go
func TestCatalogOperations(t *testing.T) {
    catalog, _ := catalogs.New()

    // Test adding models
    err := catalog.SetModel(model)
    assert.NoError(t, err)

    // Test retrieval
    retrieved, err := catalog.Model(model.ID)
    assert.NoError(t, err)
    assert.Equal(t, model.Name, retrieved.Name)
}
```

**Command Tests with Mocks:**

```go
func TestListCommand(t *testing.T) {
    // Create mock application
    mock := &mockApp{
        catalog: testCatalog,
        logger:  testLogger,
    }

    // Create command with mock
    cmd := list.NewCommand(mock)

    // Execute and verify
    err := cmd.Execute()
    assert.NoError(t, err)
}
```

### Integration Tests

**Full Pipeline Tests:**

```bash
# Tag integration tests
go test -tags=integration ./...

# Run integration tests for specific package
go test -tags=integration ./pkg/reconciler -v
```

**Example Integration Test:**

```go
//go:build integration
func TestFullSyncPipeline(t *testing.T) {
    // Create real starmap with embedded catalog
    sm, _ := starmap.New()

    // Perform actual sync
    result, err := sm.Sync(context.Background(),
        sync.WithProvider("openai"),
        sync.WithDryRun(true),
    )

    assert.NoError(t, err)
    assert.NotNil(t, result)
}
```

### Race Detection

**Always test with race detector:**

```bash
# All tests with race detector
go test -race ./...

# Specific package with race detector
go test -race ./pkg/catalogs -v

# Benchmarks with race detector
go test -race -bench=. ./pkg/catalogs
```

### Test Coverage

```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Coverage for specific package
go test -coverprofile=coverage.out ./pkg/catalogs
go tool cover -func=coverage.out
```

### Testdata Management

Provider API responses are captured as testdata:

```bash
# Update testdata for all providers
make testdata

# Update specific provider
make testdata PROVIDER=openai

# Or directly
go test ./internal/sources/providers/openai -update
```

**Testdata Pattern:**

```go
var updateFlag = flag.Bool("update", false, "update testdata files")

func TestListModels(t *testing.T) {
    if *updateFlag {
        // Fetch from real API and save
        models, _ := client.ListModels(ctx)
        saveTestdata(models)
    } else {
        // Load from testdata
        models := loadTestdata()
        // Test with loaded data
    }
}
```

## References

### Key Files

| File | Purpose | Lines |
|------|---------|-------|
| `starmap.go` | Public API interface | ~100 |
| `sync.go` | 12-step sync pipeline | ~234 |
| `cmd/application/application.go` | Application interface | ~97 |
| `cmd/starmap/app/app.go` | App implementation | ~200 |
| `pkg/reconciler/reconciler.go` | Reconciliation engine | ~300 |
| `pkg/authority/authority.go` | Field-level authorities | ~210 |

### Package Documentation

- [pkg/catalogs/README.md](pkg/catalogs/README.md) - Catalog storage
- [pkg/reconciler/README.md](pkg/reconciler/README.md) - Multi-source reconciliation
- [pkg/sources/README.md](pkg/sources/README.md) - Data source abstractions
- [pkg/authority/](pkg/authority/) - Field-level authority system
- [pkg/errors/README.md](pkg/errors/README.md) - Error types
- [pkg/logging/README.md](pkg/logging/README.md) - Logging utilities

### Related Documentation

- [CLAUDE.md](CLAUDE.md) - LLM coding assistant instructions
- [README.md](README.md) - User-facing documentation
- [CHANGELOG.md](CHANGELOG.md) - Version history

---

**Architecture Status:** ✅ Production-ready, fully implemented

This architecture has been battle-tested and optimized for:
- Thread safety with race detector validation
- Zero import cycles
- Comprehensive test coverage
- Production use with real provider APIs
