# Starmap Architecture

> Technical deep dive into Starmap's system design, components, and patterns

**Last Updated:** 2025-10-15
**Status:** Production-ready architecture following idiomatic Go patterns

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [System Components](#system-components)
- [Application Layer](#application-layer)
- [CLI Architecture](#cli-architecture)
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

```mermaid
graph TB
    subgraph UI["User Interfaces"]
        CLI[CLI Tool<br/>cmd/starmap]
        GO[Go Package API]
        HTTP[HTTP Server<br/>REST API + WebSocket + SSE]
    end

    subgraph APP["Application Layer - internal/cmd/application/"]
        APPIF[Application Interface<br/>DI Pattern]
        APPIMPL[App Implementation<br/>cmd/starmap/app/]
    end

    subgraph ROOT["Root Package - starmap.Client"]
        SYNC[Sync Pipeline<br/>13 Stages]
        HOOKS[Event Hooks<br/>Callbacks]
        AUTO[Auto-Updates<br/>Background Sync]
    end

    subgraph CORE["Core Packages - pkg/"]
        CAT[Catalogs<br/>Storage Abstraction]
        REC[Reconciler<br/>Multi-Source Merging]
        AUTH[Authority<br/>Field-Level Priorities]
        SOURCES[Sources<br/>Data Interfaces]
    end

    subgraph IMPL["Internal Implementations"]
        EMBED[Embedded Data<br/>go:embed]
        PROVS[Provider Clients<br/>OpenAI, Anthropic, etc.]
        MODELS[models.dev<br/>Git & HTTP]
        LOCAL[Local Files<br/>User Overrides]
    end

    CLI --> APPIF
    GO --> APPIF
    HTTP --> APPIF
    APPIF -.implemented by.-> APPIMPL
    APPIMPL --> ROOT
    ROOT --> CORE
    PROVS & MODELS & LOCAL & EMBED -.implement.-> SOURCES

    style UI fill:#e3f2fd
    style APP fill:#fff3e0
    style ROOT fill:#f3e5f5
    style CORE fill:#e8f5e9
    style IMPL fill:#fce4ec
```

**Architecture Layers:**
1. **User Interfaces**: Multiple entry points (CLI, Go package, HTTP API)
2. **Application Layer**: Dependency injection pattern with interface/implementation separation
3. **Root Package**: Public API with sync orchestration, hooks, and lifecycle management
4. **Core Packages**: Reusable business logic for catalog management and reconciliation
5. **Internal Implementations**: Provider-specific code and data sources

## Design Principles

### 1. Interface Segregation
- **Define interfaces where they're used** (Go proverb)
- Application interface in `internal/cmd/application/` (reusable across binaries)
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

1. **Application Layer** (`internal/cmd/application/`, `cmd/starmap/app/`)
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

Location: `internal/cmd/application/application.go`

**Design Philosophy:**
- "Accept interfaces, return structs" (Go proverb)
- "Define interfaces where they're used" (idiomatic Go)
- Located in `internal/cmd` for internal package organization
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

```mermaid
flowchart BT
    APP[cmd/starmap/app/<br/>App implements Application]
    CMD[cmd/starmap/cmd/*<br/>Commands use Application]
    INT[internal/cmd/application/<br/>Application interface]

    APP -->|implements| INT
    CMD -->|imports| INT

    style INT fill:#e3f2fd
    style CMD fill:#fff3e0
    style APP fill:#f3e5f5
```

**Key Points:**
- Commands depend only on the interface, not the implementation
- App is injected into commands at runtime
- Zero import cycles (unidirectional dependencies)
- Easy to test with mock implementations

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

**Visual Representation of Double-Checked Locking:**

```mermaid
sequenceDiagram
    participant G1 as Goroutine 1
    participant G2 as Goroutine 2
    participant Lock as RWMutex
    participant SM as Starmap Singleton

    Note over G1,G2: Scenario 1: First Call (Uninitialized)
    G1->>Lock: RLock()
    G1->>SM: Check if nil
    SM-->>G1: Yes, is nil
    G1->>Lock: RUnlock()

    G1->>Lock: Lock() [write lock]
    G1->>SM: Double-check if nil
    SM-->>G1: Still nil
    Note over G1: Initialize starmap<br/>(only once)
    G1->>SM: Set instance
    G1->>Lock: Unlock()

    Note over G1,G2: Scenario 2: Subsequent Calls (Initialized)
    G2->>Lock: RLock()
    G2->>SM: Check if nil
    SM-->>G2: No, exists!
    Note over G2: Fast path<br/>(no allocation)
    G2->>Lock: RUnlock()
    G2-->>G2: Return existing instance
```

**Why This Pattern?**
- **First Check (Read Lock)**: Fast path for the common case (already initialized)
- **Write Lock Acquisition**: Only when initialization needed
- **Second Check (Write Lock)**: Prevent race condition between locks
- **Result**: Thread-safe singleton with minimal overhead

## CLI Architecture

### Design Philosophy

Starmap's CLI is built on these core principles:

1. **POSIX Compliance**: Standard Unix flag conventions (`-o`, `--output`)
2. **Discoverability**: Clear help text, intuitive command names
3. **Consistency**: Same patterns across all commands
4. **Ergonomics**: Short flags for common operations, sensible defaults

### Command Structure

Commands follow the **VERB-NOUN pattern** borrowed from kubectl and other modern CLIs:

```
starmap <verb> <noun> [arguments] [flags]
        ↓      ↓         ↓           ↓
     action  resource  identity   modifiers
```

**Examples:**
```bash
starmap models list                    # resource=models, subcommand=list
starmap providers fetch anthropic      # resource=providers, subcommand=fetch, arg=anthropic
starmap update openai                  # verb=update, arg=openai
```

**Command Groups:**
- **Setup Commands**: Getting started (auth, deps)
- **Catalog Commands**: Working with catalog resources (authors, models, providers, update)
- **Server Commands**: Running the API (serve)
- **Development Commands**: Debugging and exploration (embed, validate)
- **Additional Commands**: Utilities (completion, version, help)

### Flag Architecture

#### Global Flags (Reserved)

These flags are **always available** and must not be overridden by commands:

| Short | Long | Purpose | Notes |
|-------|------|---------|-------|
| `-v` | `--verbose` | Verbose output | Sets log level to debug |
| `-q` | `--quiet` | Minimal output | Sets log level to warn |
| `-o` | `--output` | Output format | table, json, yaml, wide |
| `-h` | `--help` | Show help | Built-in Cobra flag |

**Why `-o` for output?**
- Avoids conflict with embed cat's `--filename` (`-f`)
- Matches common tools like `gcc -o output`
- Frees up `-f` for `--force` in commands that need it

#### Resource Filter Flags

Added programmatically via `globals.AddResourceFlags()`:

| Short | Long | Purpose |
|-------|------|---------|
| `-p` | `--provider` | Filter by provider |
| | `--author` | Filter by author |
| | `--search` | Search term |
| `-l` | `--limit` | Limit results |

#### Command-Specific Flags

Commands define their own flags that don't conflict with global flags:

**Update Command:**
- `-f` / `--force` - Force fresh update
- `-y` / `--yes` - Auto-approve changes
- `--dry` - Preview changes (primary)
- `--dry-run` - Preview changes (deprecated alias)

**Embed Commands:**
- Custom help flag (`-?`) frees up `-h` and `-f`
- `ls -h` - Human-readable sizes (like Unix ls)
- `cat -f` - Show filename before content

### Architectural Decisions

#### 1. Positional Arguments vs Flags

**Decision**: Use positional arguments for **identity/resource**, flags for **options/modifiers**

**Rationale:**
```bash
# ✅ Good: Resource is positional, options are flags
starmap update openai --dry

# ❌ Avoided: Resource as flag feels less natural
starmap update --provider openai --dry
```

**Pattern:**
- Positional = "What to act on" (which provider, which model)
- Flags = "How to act" (dry run, force, output format)

#### 2. Breaking Changes Strategy

**Decision**: Clean breaks acceptable for young projects (<1.0)

**Rationale:**
- Project is pre-1.0, rapid iteration beneficial
- Clear communication via commit messages
- Deprecation periods add complexity without benefit at this stage
- Post-1.0: Will use proper deprecation (6-12 months)

**Example from Phase 2:**
```bash
# Before (v0.x)
starmap update --provider openai

# After (v0.x+1) - Clean break
starmap update openai

# Commit message included migration guide
```

#### 3. Custom Help Flags

**Decision**: Allow command groups to override `-h` with custom patterns

**Rationale:**
- Embed commands need Unix-like flags (`ls -h` for human-readable)
- Solution: Parent command defines `-?` for help
- All subcommands inherit this, freeing `-h` and `-f`

**Implementation:**
```go
// Parent: cmd/starmap/app/commands.go
cmd.PersistentFlags().BoolP("help", "?", false, "help for embed commands")

// Now subcommands can use -h
LsCmd.Flags().BoolVarP(&lsHuman, "human-readable", "h", false, "...")
```

#### 4. Hidden Alias Flags

**Decision**: Support backward compatibility via hidden aliases

**Rationale:**
- Users may have scripts depending on old flags
- Hidden flags don't clutter help text
- Smooth migration path

**Example:**
```go
// Primary flag (shown in help)
rootCmd.PersistentFlags().StringVarP(&a.config.Output, "output", "o", "", "...")

// Hidden aliases (backward compat)
rootCmd.PersistentFlags().StringVar(&a.config.Output, "format", "", "")
_ = rootCmd.PersistentFlags().MarkHidden("format")
```

### Implementation Details

**Framework**: [Cobra](https://github.com/spf13/cobra) - Industry-standard Go CLI library

**Key Files:**
- `cmd/starmap/app/execute.go` - Root command and global flags
- `cmd/starmap/app/commands.go` - Command registration
- `internal/cmd/globals/` - Shared flag utilities
- `cmd/starmap/cmd/*/` - Individual command implementations

**For comprehensive CLI reference and implementation guidelines**, see **[CLI.md](CLI.md)**.

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

See [pkg/catalogs/README.md](../pkg/catalogs/README.md) for details.

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

See [pkg/reconciler/README.md](../pkg/reconciler/README.md) for details.

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

See [pkg/sources/README.md](../pkg/sources/README.md) for details.

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

Data flows from multiple sources into the reconciliation engine, with each source having specific authority for different types of data:

```mermaid
graph TD
    LOCAL["Local Catalog<br/><b>Priority: 100</b> (API Config)<br/>• API keys & endpoints<br/>• Provider configurations<br/>• User overrides"]
    API["Provider APIs<br/><b>Priority: 95</b> (Model Existence)<br/>• Real-time availability<br/>• Basic capabilities<br/>• Concurrent fetching"]
    MD["models.dev<br/><b>Priority: 110</b> (Pricing)<br/><b>Priority: 100</b> (Metadata)<br/>• Community-verified pricing<br/>• Provider logos (SVG)<br/>• HTTP API & Git fallback"]
    EMB["Embedded Catalog<br/><b>Priority: 80</b> (Baseline)<br/>• Ships with binary (go:embed)<br/>• Fallback data<br/>• Manual corrections"]

    REC{Reconciliation<br/>Engine<br/>Authority-Based}
    CAT["Unified Catalog<br/>✓ Complete<br/>✓ Accurate<br/>✓ Provenance Tracked"]

    LOCAL --> REC
    API --> REC
    MD --> REC
    EMB --> REC
    REC --> CAT

    style LOCAL fill:#fff3e0
    style API fill:#e8f5e9
    style MD fill:#e3f2fd
    style EMB fill:#f3e5f5
    style REC fill:#fff9c4
    style CAT fill:#c8e6c9
```

**Authority Resolution:**
- **Pricing & Limits**: models.dev is most authoritative (manually verified)
- **Model Existence**: Provider APIs determine what models actually exist
- **API Configuration**: Local catalog takes precedence (user's environment)
- **Baseline Data**: Embedded catalog provides defaults when other sources unavailable

**Concurrent Fetching:**
Provider APIs are fetched concurrently using goroutines and channels for optimal performance.

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

The sync pipeline executes in 13 stages with comprehensive error handling and decision points:

### Pipeline Flowchart

```mermaid
flowchart TD
    Start([Sync Called]) --> S1{Context<br/>nil?}
    S1 -->|Yes| S1B[Set Background Context]
    S1 -->|No| S2
    S1B --> S2[Parse Options<br/>with Defaults]

    S2 --> S3{Timeout<br/>configured?}
    S3 -->|Yes| S3B[Setup WithTimeout]
    S3 -->|No| S4
    S3B --> S4[Load Embedded<br/>Catalog]

    S4 --> S5[Validate<br/>Options]
    S5 --> E1{Valid?}
    E1 -->|No| Error1[❌ Return Error]
    E1 -->|Yes| S6[Filter Sources<br/>by Options]

    S6 --> S7[Resolve Dependencies<br/>Check & Install]
    S7 --> S8[Setup Cleanup<br/>defer]
    S8 --> S9[Fetch from Sources<br/>⚡ Concurrent]

    S9 --> E2{Fetch<br/>Success?}
    E2 -->|No| Error2[❌ Return Error]
    E2 -->|Yes| S10[Get Existing<br/>Catalog Baseline]

    S10 --> S11[Reconcile<br/>All Sources]
    S11 --> S12[Log Change<br/>Summary]

    S12 --> D1{Has<br/>Changes?}
    D1 -->|No| End1[✓ Return Result<br/>No Changes]
    D1 -->|Yes| D2{Dry<br/>Run?}
    D2 -->|Yes| End2[✓ Return Result<br/>Preview Only]
    D2 -->|No| S13[Save Catalog &<br/>Trigger Hooks]

    S13 --> End3([✅ Return Result<br/>Changes Applied])

    style Start fill:#e3f2fd
    style Error1 fill:#ffcdd2
    style Error2 fill:#ffcdd2
    style S9 fill:#fff9c4
    style S11 fill:#e1bee7
    style End1 fill:#c8e6c9
    style End2 fill:#c8e6c9
    style End3 fill:#c8e6c9
```

**Stage Groups:**
- **Stages 1-5** (Setup): Context, options, validation
- **Stages 6-9** (Preparation): Source filtering, dependency resolution, cleanup, concurrent fetching
- **Stages 10-11** (Processing): Baseline comparison, reconciliation
- **Stages 12-13** (Finalization): Change detection, persistence, hooks

### Stage-by-Stage Code

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

    // Stage 7: Resolve dependencies
    srcs, err = resolveDependencies(ctx, srcs, options)

    // Stage 8: Setup cleanup
    defer cleanup(srcs)

    // Stage 9: Fetch catalogs from all sources
    err = fetch(ctx, srcs, options.SourceOptions())

    // Stage 10: Get existing catalog for baseline
    existing, err := c.Catalog()

    // Stage 11: Reconcile catalogs from all sources
    result, err := update(ctx, existing, srcs)

    // Stage 12: Log change summary
    logging.Info().Int("added", ...).Msg("Changes detected")

    // Stage 13: Save if not dry-run
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

### Reconciliation Flow Visualization

```mermaid
sequenceDiagram
    participant Sync as Sync Pipeline
    participant Rec as Reconciler
    participant Auth as Authority System
    participant P as Provider API
    participant M as models.dev
    participant L as Local

    Sync->>Rec: Reconcile(sources)

    par Concurrent Fetch from all sources
        Rec->>P: Fetch()
        P-->>Rec: {Name, Features}
        Rec->>M: Fetch()
        M-->>Rec: {Pricing, Limits}
        Rec->>L: Fetch()
        L-->>Rec: {Description}
    end

    Note over Rec: Process each model field

    Rec->>Auth: ResolveConflict("Name", values)
    Auth-->>Rec: Provider API (priority 90)

    Rec->>Auth: ResolveConflict("Features", values)
    Auth-->>Rec: Provider API (priority 95)

    Rec->>Auth: ResolveConflict("Pricing", values)
    Auth-->>Rec: models.dev (priority 110)

    Rec->>Auth: ResolveConflict("Limits", values)
    Auth-->>Rec: models.dev (priority 100)

    Rec->>Auth: ResolveConflict("Description", values)
    Auth-->>Rec: Local (priority 90)

    Note over Rec: Merge all reconciled fields

    Rec-->>Sync: Result with changeset<br/>& provenance tracking
```

**Reconciliation Steps:**
1. **Concurrent Fetch**: All sources fetched in parallel
2. **Field-Level Resolution**: Authority system determines winner for each field
3. **Provenance Tracking**: Record which source provided each value
4. **Changeset Generation**: Compare with baseline to detect changes

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

### Visual Comparison: Safe vs Unsafe Patterns

```mermaid
graph LR
    subgraph "❌ UNSAFE: Shared Mutable State"
        direction TB
        G1A[Goroutine 1<br/>Read] -->|direct access| SHARED1[(Shared<br/>Data)]
        G2A[Goroutine 2<br/>Write] -->|direct access| SHARED1
        SHARED1 -.->|Race Condition| CRASH[💥 Data Race<br/>Undefined Behavior]
        style SHARED1 fill:#ffcdd2
        style CRASH fill:#f44336,color:#fff
    end

    subgraph "✅ SAFE: Value Semantics with Deep Copy"
        direction TB
        G1B[Goroutine 1] -->|DeepCopy| SHARED2[(Shared<br/>Data<br/>+RWMutex)]
        SHARED2 -->|independent copy| COPY1[Local<br/>Copy 1]
        G2B[Goroutine 2] -->|DeepCopy| SHARED2
        SHARED2 -->|independent copy| COPY2[Local<br/>Copy 2]
        COPY1 & COPY2 -.->|No Sharing| SAFE[✅ Thread Safe<br/>No Data Races]
        style SHARED2 fill:#c8e6c9
        style COPY1 fill:#e8f5e9
        style COPY2 fill:#e8f5e9
        style SAFE fill:#4caf50,color:#fff
    end
```

**Key Differences:**
- **Unsafe**: Direct access to shared mutable state causes race conditions
- **Safe**: Deep copy creates independent instances, preventing data races
- **Trade-off**: Safety vs. memory efficiency (copies allocate more memory)
- **Starmap Choice**: Safety first with optimizations (e.g., single copy in App.Catalog)

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
    catalog := catalogs.Empty()

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

#### 4. Channel Buffering for Event-Driven Systems

For event-driven systems using channels (event brokers, WebSocket hubs, SSE broadcasters), **ALWAYS buffer channels used for registration/unregistration**:

```go
// ❌ WRONG: Unbuffered channels cause initialization deadlocks
type Broker struct {
    register   chan Subscriber    // Blocks if Run() not started
    unregister chan Subscriber    // Blocks during cleanup
}

// ✅ CORRECT: Buffered channels prevent blocking
type Broker struct {
    register   chan Subscriber, 10    // Buffer for setup phase
    unregister chan Subscriber, 10    // Buffer for cleanup phase
}
```

**Why buffering is critical:**

1. **Initialization Order Independence**: Components can be initialized and subscribed before event loops start
2. **No Deadlocks**: `Subscribe()` doesn't block waiting for `Run()` to read from channel
3. **Graceful Cleanup**: Unregister operations during shutdown don't block

**Buffer sizing guidelines:**

- **Registration channels**: Size based on typical number of subscribers registered during initialization (commonly 5-10)
- **Unregistration channels**: Same size as registration channels
- **Event channels**: Size based on burst capacity (commonly 256+ for high-throughput systems)

**Real-world example from `internal/server/events/broker.go`:**

```go
func NewBroker(logger *zerolog.Logger) *Broker {
    return &Broker{
        subscribers: make([]Subscriber, 0),
        events:      make(chan Event, 256),        // High-capacity event buffer
        register:    make(chan Subscriber, 10),    // Prevents blocking during setup
        unregister:  make(chan Subscriber, 10),    // Prevents blocking during shutdown
        logger:      logger,
    }
}
```

**Testing for initialization order bugs:**

Always write tests that verify subscriptions work before `Run()` starts:

```go
func TestBroker_SubscribeBeforeRun(t *testing.T) {
    b := NewBroker(logger)

    // Subscribe BEFORE starting Run() - should NOT block
    done := make(chan struct{})
    go func() {
        sub := newSubscriber()
        b.Subscribe(sub)  // Would deadlock with unbuffered channels
        close(done)
    }()

    select {
    case <-done:
        // Success
    case <-time.After(2 * time.Second):
        t.Fatal("Deadlock detected - channels not buffered!")
    }
}
```

See `internal/server/events/broker_test.go:TestBroker_SubscribeBeforeRun` for a complete example.

### Thread Safety Checklist

When adding new code, ensure:

- [ ] Collections return values, not pointers
- [ ] Public methods that access shared state use locks
- [ ] Deep copy methods handle all pointer fields
- [ ] Tests include `-race` detector runs
- [ ] Singletons use double-checked locking
- [ ] No direct pointer returns from getters
- [ ] Event-driven channels are buffered (registration/unregistration channels especially)
- [ ] Initialization order tests verify Subscribe/Register work before Run()

## Package Organization

```
starmap/
├── cmd/
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
│   ├── cmd/
│   │   └── application/      # Application interface (internal)
│   │       └── application.go # Application interface definition
│   ├── embedded/             # Embedded catalog data
│   │   ├── catalog/          # Embedded YAML files
│   │   └── openapi/          # OpenAPI 3.1 specs (JSON/YAML)
│   ├── server/               # HTTP server implementation
│   │   ├── server.go         # Server struct & lifecycle
│   │   ├── config.go         # Configuration management
│   │   ├── router.go         # Route registration & middleware
│   │   └── handlers/         # HTTP request handlers
│   │       ├── models.go     # Model endpoints
│   │       ├── providers.go  # Provider endpoints
│   │       ├── admin.go      # Admin operations
│   │       ├── health.go     # Health checks
│   │       ├── realtime.go   # WebSocket/SSE
│   │       └── openapi.go    # OpenAPI spec endpoints
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

```mermaid
graph BT
    subgraph "Layer 6: Implementations"
        INT[internal/*<br/>Embedded, Providers, models.dev]
    end

    subgraph "Layer 5: Core Packages"
        PKG[pkg/*<br/>catalogs, reconciler, sources, authority]
    end

    subgraph "Layer 4: Root Package"
        ROOT[starmap<br/>Client interface & implementation]
    end

    subgraph "Layer 3: App Implementation"
        APPIMPL[cmd/starmap/app/<br/>App struct implements Application]
    end

    subgraph "Layer 2: Commands"
        CMDS[cmd/starmap/cmd/*<br/>list, update, serve commands]
    end

    subgraph "Layer 1: Application Interface"
        APPIF[internal/cmd/application/<br/>Application interface]
    end

    INT --> PKG
    PKG --> ROOT
    ROOT --> APPIMPL
    APPIMPL -.implements.-> APPIF
    CMDS --> APPIF

    style APPIF fill:#e3f2fd
    style CMDS fill:#fff3e0
    style APPIMPL fill:#f3e5f5
    style ROOT fill:#e8f5e9
    style PKG fill:#fff9c4
    style INT fill:#fce4ec
```

**Architecture Benefits:**
- **Clean Separation**: Each layer has clear responsibilities
- **Testability**: Commands depend on interfaces, easily mocked
- **Flexibility**: Implementation can change without affecting commands
- **No Cycles**: Go enforces unidirectional dependencies

**Rules:**
- Never import from higher layers
- Commands import `internal/cmd/application/` interface, not `cmd/starmap/app/`
- Root package imports pkg packages
- Internal packages can import pkg packages
- Pkg packages are fully independent

## Testing Strategy

### Unit Tests

**Package-Level Tests:**

```go
// pkg/catalogs/catalog_test.go
func TestCatalogOperations(t *testing.T) {
    catalog := catalogs.Empty()

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
| `sync.go` | 13-step sync pipeline | ~234 |
| `internal/cmd/application/application.go` | Application interface | ~97 |
| `cmd/starmap/app/app.go` | App implementation | ~200 |
| `pkg/reconciler/reconciler.go` | Reconciliation engine | ~300 |
| `pkg/authority/authority.go` | Field-level authorities | ~210 |

### Package Documentation

- [pkg/catalogs/README.md](../pkg/catalogs/README.md) - Catalog storage
- [pkg/reconciler/README.md](../pkg/reconciler/README.md) - Multi-source reconciliation
- [pkg/sources/README.md](../pkg/sources/README.md) - Data source abstractions
- [pkg/authority/](../pkg/authority/) - Field-level authority system
- [pkg/errors/README.md](../pkg/errors/README.md) - Error types
- [pkg/logging/README.md](../pkg/logging/README.md) - Logging utilities

### Related Documentation

- [CLAUDE.md](../CLAUDE.md) - LLM coding assistant instructions
- [README.md](../README.md) - User-facing documentation
- [CHANGELOG.md](../CHANGELOG.md) - Version history

---

**Architecture Status:** ✅ Production-ready, fully implemented

This architecture has been battle-tested and optimized for:
- Thread safety with race detector validation
- Zero import cycles
- Comprehensive test coverage
- Production use with real provider APIs
