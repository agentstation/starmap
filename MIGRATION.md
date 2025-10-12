# Migration to Idiomatic Go Architecture

This document describes the migration plan to refactor Starmap to follow idiomatic Go CLI application patterns.

## Overview

We're refactoring from the current scattered architecture to a centralized app-based pattern that follows Go best practices.

### Current State (Before)
```
cmd/starmap/
  main.go                    # Minimal, calls cmd.Execute()
  cmd/
    root.go                  # Cobra setup, viper, logging (227 lines)
    *.go                     # 13 command stub files
    <command>/               # 10 command packages
      *.go                   # Implementation files
internal/cmd/
  <various>/                 # Utility packages (output, table, filter, etc.)
```

### Target State (After)
```
cmd/starmap/
  main.go                    # Creates app, calls app.Execute()
  app/
    app.go                   # App struct, DI, lifecycle
    config.go                # Unified config loading
    logger.go                # Logger initialization
    context.go               # Signal handling
    execute.go               # Command registration
    commands.go              # Command factory methods
  cmd/
    <command>/               # Each command package
      command.go             # NewCommand(app) factory
      *.go                   # Implementation
```

## Benefits

✅ **Single Source of Truth** - All config and dependencies in one place
✅ **Testability** - Mock app.App for unit tests
✅ **Lifecycle Control** - Centralized startup/shutdown
✅ **Thread Safety** - App manages starmap singleton properly
✅ **DRY** - No more duplicate catalog loading code
✅ **Idiomatic** - Matches standard Go CLI/server patterns

## Implementation Plan

### Phase 1: Foundation (✅ COMPLETED)

**Created `cmd/starmap/app/` package:**

- [x] `app.go` - App struct with config, logger, starmap instance
- [x] `config.go` - Unified config loading (viper + env + .env files)
- [x] `logger.go` - Logger initialization based on config
- [x] `context.go` - Context with signal handling
- [x] `execute.go` - Root command creation and registration
- [x] `commands.go` - Command factory methods (stubs)

**App struct provides:**
```go
type App struct {
    config   *Config           // Unified configuration
    logger   *zerolog.Logger   // Configured logger
    starmap  starmap.Starmap   // Lazy-initialized singleton
}

// Methods
func (a *App) Catalog() (catalogs.Catalog, error)
func (a *App) Starmap() (starmap.Starmap, error)
func (a *App) StarmapWithOptions(...) (starmap.Starmap, error)
func (a *App) Execute(ctx, args) error
func (a *App) Shutdown(ctx) error
```

### Phase 2: Command Migration (✅ COMPLETED)

All commands have been migrated to use the `appcontext.Interface` pattern:

```go
// Before (current pattern)
func listModels(cmd *cobra.Command, ...) error {
    cat, err := catalog.Load()  // Creates new starmap instance
    // ...
}

// After (app pattern)
type AppContext interface {
    Catalog() (catalogs.Catalog, error)
    Config() *Config
    Logger() *zerolog.Logger
}

func NewCommand(app AppContext) *cobra.Command {
    return &cobra.Command{
        RunE: func(cmd *cobra.Command, args []string) error {
            cat, err := app.Catalog()  // Uses app instance
            // ...
        },
    }
}
```

**Migration Completed:**
All 13 commands have been migrated to use `appcontext.Interface`:
- ✅ List command (models, providers, authors)
- ✅ Update command
- ✅ Serve command (API server)
- ✅ Fetch command
- ✅ Validate command
- ✅ Inspect command (ls, cat, tree, stat)
- ✅ Auth command (status, verify, gcloud)
- ✅ Generate command (docs, completion)
- ✅ Install command (completion)
- ✅ Uninstall command (completion)
- ✅ Version command
- ✅ Man command

**Key Changes:**
1. All commands use `NewCommand(appCtx appcontext.Interface)` factory pattern
2. Replaced `catalog.Load()` with `appCtx.Catalog()`
3. Replaced direct `starmap.New()` with `appCtx.Starmap()`
4. Use `appCtx.Logger()` for logging
5. Use `appCtx.Config()` for configuration
6. Command registration centralized in `app/execute.go`

### Phase 3: Main.go Update (✅ COMPLETED)

Replaced current main.go:
```go
// Before
func main() {
    cmd.Execute(version, commit, date, builtBy)
}

// After (✅ Implemented)
func main() {
    app, err := app.New(version, commit, date, builtBy)
    if err != nil {
        app.ExitOnError(err)
    }

    ctx, cancel := app.ContextWithSignals(context.Background())
    defer cancel()

    if err := app.Execute(ctx, os.Args[1:]); err != nil {
        _ = app.Shutdown(ctx)
        app.ExitOnError(err)
    }
}
```

### Phase 4: Cleanup (🚧 IN PROGRESS)

**Removed deprecated code:**
- [x] `internal/cmd/catalog/loader.go` - Removed (replaced by appCtx.Catalog())
- [x] `internal/cmd/globals` usage from app/commands.go - Removed
- [x] Deprecated command constructors (NewCommandDeprecated, etc.) - Removed
- [x] Deprecated function variants with "WithApp" suffix - Renamed to clean names
- [x] Empty serve.go file - Cleaned up
- [x] Unused imports - Removed
- [ ] `cmd/starmap/cmd/root.go` - TODO: Review if needed (logic moved to app)
- [ ] `internal/cmd/globals/` package - TODO: Evaluate if still needed elsewhere

**Consolidated:**
- [x] All config loading in `app/config.go`
- [x] All logging setup in `app/logger.go`
- [x] Centralized starmap instance management in app
- [x] Command registration in `app/execute.go`

## Command-Specific Migration Notes

All commands have been successfully migrated to use the `appcontext.Interface` pattern.

### List Command ✅
**Migrated:** Uses `appCtx.Catalog()` and `appCtx.Logger()`
**Files:** `cmd/list/list.go`, `models.go`, `providers.go`, `authors.go`

### Update Command ✅
**Migrated:** Uses `appCtx.StarmapWithOptions()` for custom configs
**Files:** `cmd/update/update.go`
**Special:** Supports dry-run, provider filtering via options

### Serve Command ✅
**Migrated:** Uses `appCtx.Catalog()` and `appCtx.Logger()`
**Files:** `cmd/serve/serve.go`, `api.go`, `handlers.go`
**Special:** Long-running server with graceful shutdown

### Fetch Command ✅
**Migrated:** Uses `appCtx.Config()` for API keys, `appCtx.Logger()` for output
**Files:** `cmd/fetch/fetch.go`, `models.go`

### Other Commands ✅
All remaining commands (validate, inspect, auth, generate, install, uninstall, version, man) have been migrated to use `appcontext.Interface`.

## Testing Strategy

### Unit Tests
```go
// Create test app with mocks
func newTestApp() *app.App {
    return app.New("test", "test", "test", "test",
        app.WithStarmap(mockStarmap),
        app.WithLogger(&testLogger),
    )
}

func TestListModels(t *testing.T) {
    app := newTestApp()
    cmd := list.NewCommand(app)
    // Test command...
}
```

### Integration Tests
- Test full app.Execute() flow
- Test config loading from various sources
- Test signal handling and graceful shutdown

## Rollout Strategy

### Step 1: Parallel Development
- Keep old pattern working
- Add new app package alongside
- Implement new pattern in new code

### Step 2: Gradual Migration
- Migrate one command at a time
- Keep both patterns working during transition
- Test thoroughly after each command migration

### Step 3: Deprecation
- Mark old patterns as deprecated
- Add warnings to old code
- Update documentation

### Step 4: Removal
- Remove deprecated code
- Update all documentation
- Celebrate! 🎉

## Open Questions

1. **Backward Compatibility:** Do we need to support old patterns temporarily?
   - **Answer:** Yes, during migration. Use feature flags if needed.

2. **Testing:** How do we ensure no regression during migration?
   - **Answer:** Keep existing tests working, add new app-based tests.

3. **Performance:** Does lazy starmap initialization affect performance?
   - **Answer:** No, only creates once per app lifetime. Same as before.

4. **Configuration:** Should app.Config() be mutable or immutable?
   - **Answer:** Immutable after initialization. Use functional options for variants.

## References

- [Standard Go Project Layout](https://github.com/golang-standards/project-layout)
- [Cobra Best Practices](https://github.com/spf13/cobra/blob/master/user_guide.md)
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Proverbs](https://go-proverbs.github.io/)

## Migration Checklist

### Infrastructure
- [x] Create app package
- [x] Implement App struct
- [x] Implement Config loading
- [x] Implement Logger setup
- [x] Implement Execute() method
- [x] Update main.go
- [x] Wire all existing commands through app
- [ ] Add migration tests

### Commands (13/13 migrated) ✅
- [x] list (models, providers, authors)
- [x] update
- [x] serve (api)
- [x] fetch
- [x] validate
- [x] inspect
- [x] auth (status, verify, gcloud)
- [x] generate
- [x] install
- [x] uninstall
- [x] version
- [x] man

### Cleanup 🚧
- [x] Remove internal/cmd/catalog/loader.go
- [x] Remove internal/cmd/globals usage from app/commands.go
- [x] Remove deprecated command constructors and function variants
- [x] Update ARCHITECTURE.md
- [x] Update MIGRATION.md
- [ ] Review cmd/root.go for remaining cleanup
- [ ] Evaluate internal/cmd/globals package usage elsewhere
- [ ] Performance testing with race detector
- [ ] Final validation (lint, build, smoke tests)

## Timeline

- **Week 1:** Phase 1 (Foundation) - ✅ COMPLETED
- **Week 1:** Phase 3 (Main.go Update) - ✅ COMPLETED
- **Week 2:** Phase 2 (Command Migration) - ✅ COMPLETED
- **Week 2:** Phase 4 (Cleanup) - 🚧 IN PROGRESS
- **Week 3:** Testing, Documentation, Review - 📅 PLANNED

## Success Criteria

✅ All commands migrated to app pattern
✅ No duplicate config/DI code
✅ Improved testability via dependency injection
✅ Documentation updated
⏳ All tests passing (Phase 4)
⏳ Performance maintained or improved (Phase 4)

---

**Last Updated:** 2025-10-12
**Status:** Phase 1 ✅ | Phase 2 ✅ | Phase 3 ✅ | Phase 4 🚧 In Progress

## Current State Summary

**✅ What's Complete:**
- Full app infrastructure in place (`cmd/starmap/app/`)
- All 13 commands migrated to `appcontext.Interface` pattern
- `main.go` updated to use `app.Execute()`
- Deprecated code removed (catalog/loader.go, old constructors, etc.)
- Function naming cleaned up (removed "WithApp" suffixes)
- Documentation updated (ARCHITECTURE.md, MIGRATION.md)
- Build successful

**🚧 In Progress (Phase 4 Cleanup):**
- Performance testing with race detector
- Final validation (lint, build, smoke tests)
- Review remaining deprecated patterns (cmd/root.go, internal/cmd/globals)

**🎉 Achievements:**
- Single starmap instance managed by app (no more duplicate catalog loading)
- Proper dependency injection throughout
- Thread-safe singleton pattern
- Clean, idiomatic Go architecture
- No breaking changes to existing functionality
