# Starmap Architecture

This document describes the Starmap application architecture, including both the current state and the planned improvements.

## Current Architecture (As-Is)

### Directory Structure
```
/Users/jack/src/github.com/agentstation/starmap/
├── cmd/starmap/
│   ├── main.go                    # Entry point (18 lines)
│   ├── app/                       # 🆕 NEW: App package (Phase 1 complete)
│   │   ├── app.go                 # App struct, DI, lifecycle
│   │   ├── config.go              # Unified config loading
│   │   ├── logger.go              # Logger initialization
│   │   ├── context.go             # Signal handling
│   │   ├── execute.go             # Command registration
│   │   └── commands.go            # Command factory methods
│   └── cmd/
│       ├── root.go                # Cobra setup, viper, logging (227 lines)
│       ├── *.go                   # Command registration files
│       └── <command>/             # Command implementations
├── internal/
│   ├── cmd/                       # CLI utilities
│   │   ├── output/                # Output formatting
│   │   ├── table/                 # Table rendering
│   │   ├── filter/                # Filtering logic
│   │   └── <others>/              # Various utilities
│   ├── sources/                   # Data source implementations
│   ├── embedded/                  # Embedded catalog data
│   └── <others>/                  # Internal packages
├── pkg/                           # Public packages
│   ├── catalogs/                  # Catalog data structures
│   ├── reconciler/                # Data reconciliation
│   ├── authority/                 # Field-level authority
│   ├── sources/                   # Source interfaces
│   ├── errors/                    # Error types
│   ├── logging/                   # Logging utilities
│   ├── constants/                 # Constants
│   └── <others>/                  # Public utilities
├── starmap.go                     # Root package (Starmap interface)
├── sync.go                        # Sync pipeline
├── update.go                      # Update operations
├── options.go                     # Functional options
└── <others>.go                    # Lifecycle, hooks, etc.
```

## App Package Architecture (New)

The `cmd/starmap/app/` package provides the application foundation following idiomatic Go patterns.

### Core Components

#### 1. App Struct (`app.go`)
The central application context that manages all dependencies:

```go
type App struct {
    version string          // Version information
    commit  string          // Git commit hash
    date    string          // Build date
    builtBy string          // Build system

    config  *Config         // Application configuration
    logger  *zerolog.Logger // Configured logger

    mu      sync.RWMutex    // Thread safety
    starmap starmap.Starmap // Lazy-initialized singleton
}
```

**Key Methods:**
- `New(version, commit, date, builtBy, ...opts) (*App, error)` - Create app with version info
- `Catalog() (catalogs.Catalog, error)` - Get catalog (creates starmap if needed)
- `Starmap() (starmap.Starmap, error)` - Get starmap instance (thread-safe singleton)
- `StarmapWithOptions(...opts) (starmap.Starmap, error)` - Create custom starmap instance
- `Execute(ctx, args) error` - Execute CLI with args
- `Shutdown(ctx) error` - Graceful shutdown

**Functional Options:**
- `WithConfig(*Config)` - Custom configuration
- `WithLogger(*zerolog.Logger)` - Custom logger
- `WithStarmap(starmap.Starmap)` - Custom starmap (for testing)

#### 2. Configuration (`config.go`)
Unified configuration loading from multiple sources:

```go
type Config struct {
    // Global flags
    Verbose bool
    Quiet   bool
    NoColor bool
    Output  string

    // Starmap configuration
    LocalPath          string
    UseEmbeddedCatalog bool
    AutoUpdatesEnabled bool
    AutoUpdateInterval time.Duration
    RemoteServerURL    string
    RemoteServerAPIKey string
    RemoteServerOnly   bool

    // Logging
    LogLevel  string
    LogFormat string
    LogOutput string
}
```

**Configuration Sources (in order of precedence):**
1. Command-line flags (handled by cobra)
2. Environment variables
3. .env files (.env.local overrides .env)
4. Config file (~/.starmap.yaml)
5. Defaults

**Key Methods:**
- `LoadConfig() (*Config, error)` - Load from all sources
- `UpdateFromFlags(...)` - Update after flag parsing

#### 3. Logger (`logger.go`)
Logger initialization based on configuration:

```go
func NewLogger(config *Config) zerolog.Logger
```

Handles:
- Log level from config/flags/env
- Format detection (auto/json/console)
- Output destination (stderr/file)
- Caller information for debug level

#### 4. Context (`context.go`)
Signal handling for graceful shutdown:

```go
func Context() (context.Context, context.CancelFunc)
func ContextWithSignals(parent context.Context) (context.Context, context.CancelFunc)
```

Handles:
- SIGINT (Ctrl+C)
- SIGTERM (termination)

#### 5. Execute (`execute.go`)
Command registration and execution:

```go
func (a *App) Execute(ctx context.Context, args []string) error
```

- Creates root cobra command
- Registers all subcommands
- Sets up global flags
- Executes with context

#### 6. Commands (`commands.go`)
Factory methods for creating commands:

```go
func (a *App) CreateListCommand() *cobra.Command
func (a *App) CreateUpdateCommand() *cobra.Command
// ... etc
```

Currently returns existing commands, will be migrated to app pattern.

## Command Pattern (Planned)

### Current Pattern
```go
// Command defined in cmd package
func init() {
    rootCmd.AddCommand(listCmd)
}

// Implementation directly calls helpers
func listModels(...) error {
    cat, err := catalog.Load()  // Creates new starmap
    // ...
}
```

**Issues:**
- Duplicate starmap creation
- No dependency injection
- Hard to test
- Scattered configuration

### New Pattern (After Migration)
```go
// Command package defines interface
type AppContext interface {
    Catalog() (catalogs.Catalog, error)
    Config() *Config
    Logger() *zerolog.Logger
}

// Factory function accepts app
func NewCommand(app AppContext) *cobra.Command {
    return &cobra.Command{
        RunE: func(cmd *cobra.Command, args []string) error {
            cat, err := app.Catalog()  // Uses app instance
            // ...
        },
    }
}
```

**Benefits:**
- Single starmap instance
- Proper dependency injection
- Easy to test (mock AppContext)
- Centralized configuration

## Data Flow

### Startup Flow
```
1. main.go
   ├─> app.New(version, ...)
   │   ├─> LoadConfig()
   │   │   ├─> Load .env files
   │   │   ├─> Setup viper
   │   │   └─> Return Config
   │   ├─> NewLogger(config)
   │   └─> Return App
   │
   ├─> app.Context()  # Signal handling
   │
   └─> app.Execute(ctx, args)
       ├─> createRootCommand()
       │   ├─> registerCommands()
       │   │   ├─> CreateListCommand()
       │   │   ├─> CreateUpdateCommand()
       │   │   └─> ... (all commands)
       │   └─> Return rootCmd
       │
       └─> rootCmd.ExecuteContext(ctx)
```

### Command Execution Flow
```
1. User runs: starmap list models

2. Cobra routes to list.ModelsCmd.RunE

3. Command implementation:
   ├─> app.Catalog()           # Get catalog
   │   └─> app.Starmap()       # Lazy init if needed
   │       ├─> buildStarmapOptions()
   │       ├─> starmap.New(opts...)
   │       └─> Cache instance
   │
   ├─> Filter models
   ├─> Format output
   └─> Return

4. Shutdown:
   └─> app.Shutdown(ctx)
       └─> sm.AutoUpdatesOff()
```

## Thread Safety

### Starmap Singleton
The App struct ensures thread-safe lazy initialization of the starmap instance:

```go
func (a *App) Starmap() (starmap.Starmap, error) {
    // Fast path: read lock check
    a.mu.RLock()
    if a.starmap != nil {
        sm := a.starmap
        a.mu.RUnlock()
        return sm, nil
    }
    a.mu.RUnlock()

    // Slow path: write lock initialization
    a.mu.Lock()
    defer a.mu.Unlock()

    // Double-check after acquiring write lock
    if a.starmap != nil {
        return a.starmap, nil
    }

    // Create instance
    sm, err := starmap.New(...)
    if err != nil {
        return nil, err
    }

    a.starmap = sm
    return sm, nil
}
```

This follows the double-checked locking pattern for optimal performance.

## Testing Strategy

### Unit Tests
```go
func TestCommand(t *testing.T) {
    // Create test app with mocks
    app := &App{
        config: testConfig,
        logger: testLogger,
        starmap: mockStarmap,
    }

    // Create command
    cmd := NewCommand(app)

    // Test command
    err := cmd.Execute()
    assert.NoError(t, err)
}
```

### Integration Tests
```go
func TestFullExecution(t *testing.T) {
    app, _ := New("test", "test", "test", "test")

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    err := app.Execute(ctx, []string{"list", "models"})
    assert.NoError(t, err)
}
```

## Migration Status

See [MIGRATION.md](./MIGRATION.md) for detailed migration plan.

### Phase 1: Foundation ✅ COMPLETE
- [x] Create app package
- [x] Implement App struct
- [x] Implement Config loading
- [x] Implement Logger setup
- [x] Implement Execute() method
- [x] Document architecture

### Phase 2: Command Migration ✅ COMPLETE
- [x] Migrate all commands to app pattern (using appcontext.Interface)
- [x] Update command implementations to use app.Catalog(), app.Logger(), app.Config()
- [x] Remove deprecated command constructors
- [x] All commands now use dependency injection

### Phase 3: Cleanup 🚧 IN PROGRESS
- [x] Remove internal/cmd/catalog/loader.go (deprecated catalog loading)
- [x] Remove internal/cmd/globals usage from app/commands.go
- [x] Remove deprecated function variants (WithApp suffix, old constructors)
- [x] Clean up unused imports
- [x] Update documentation (ARCHITECTURE.md, MIGRATION.md)
- [ ] Performance testing with race detector
- [ ] Final validation (lint, build, smoke tests)

## Key Principles

1. **Single Responsibility**: Each package has one clear purpose
2. **Dependency Injection**: Dependencies passed via constructor, not globals
3. **Interface Segregation**: Commands only depend on what they need (AppContext)
4. **Thread Safety**: Proper synchronization for shared state
5. **Testability**: All components mockable via interfaces
6. **Configuration**: Unified, precedence-based config loading
7. **Lifecycle**: Explicit startup/shutdown for clean resource management

## References

- [cmd/starmap/app/](./cmd/starmap/app/) - App package implementation
- [MIGRATION.md](./MIGRATION.md) - Migration plan and progress
- [CLAUDE.md](./CLAUDE.md) - Development guidelines
- [THREAD_SAFETY.md](./THREAD_SAFETY.md) - Thread safety patterns

---

**Last Updated:** 2025-10-12
**Status:** Phase 2 Complete, Phase 3 In Progress (Cleanup)
