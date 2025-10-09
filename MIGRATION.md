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
  globals/                   # Global flag handling
  catalog/loader.go          # Catalog loading helper
  <various>/                 # 12 utility packages
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

### Phase 2: Command Migration (TODO)

Each command needs to be migrated to accept `AppContext` interface:

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

**Migration Priority:**
1. ✅ List command (proof-of-concept)
2. Update command (most complex, high value)
3. Serve command (needs app for long-running server)
4. Fetch, validate, inspect commands
5. Remaining utility commands

**Per-Command Steps:**
1. Create `AppContext` interface in command package
2. Add `NewCommand(app AppContext) *cobra.Command` factory
3. Replace `catalog.Load()` with `app.Catalog()`
4. Replace direct `starmap.New()` with `app.Starmap()`
5. Use `app.Logger()` for logging
6. Use `app.Config()` for configuration
7. Update tests to use mock AppContext
8. Remove old command registration from `cmd/root.go`
9. Add new registration in `app/commands.go`

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

### Phase 4: Cleanup (TODO)

**Remove deprecated code:**
- [ ] `cmd/starmap/cmd/root.go` (logic moved to app)
- [ ] `internal/cmd/catalog/loader.go` (replaced by app.Catalog())
- [ ] `internal/cmd/globals/` (replaced by app.Config())
- [ ] Old command registration in cmd/*.go files

**Consolidate:**
- [ ] Move all config loading to `app/config.go`
- [ ] Move all logging setup to `app/logger.go`
- [ ] Centralize starmap instance management in app

## Command-Specific Migration Notes

### List Command
**Current:** Uses `catalog.Load()` in each subcommand
**New:** Accept `AppContext`, use `app.Catalog()`
**Files:** `cmd/list/models.go`, `providers.go`, `authors.go`

### Update Command
**Current:** Creates custom starmap instances with options
**New:** Use `app.StarmapWithOptions()` for custom configs
**Files:** `cmd/update/update.go`, `catalog.go`, `options.go`
**Special:** Needs dry-run support, provider filtering

### Serve Command
**Current:** Creates catalog in handler initialization
**New:** App manages catalog lifecycle, pass to handlers
**Files:** `cmd/serve/api.go`, `handlers.go`
**Special:** Long-running, needs graceful shutdown via app

### Fetch Command
**Current:** Direct API calls with manual config
**New:** Use app.Config() for API keys, app.Logger() for output
**Files:** `cmd/fetch/models.go`

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

### Commands (1/13 migrated)
- [x] list (models, providers, authors) - ✅ Fully migrated to AppContext pattern
- [ ] update
- [ ] serve (api)
- [ ] fetch
- [ ] validate
- [ ] inspect
- [ ] auth (status, verify, gcloud)
- [ ] generate
- [ ] install
- [ ] uninstall
- [ ] version
- [ ] man

### Cleanup
- [ ] Remove cmd/root.go
- [ ] Remove internal/cmd/catalog/loader.go
- [ ] Remove internal/cmd/globals (or refactor)
- [ ] Update documentation
- [ ] Update CLAUDE.md with new patterns

## Timeline

- **Week 1:** Phase 1 (Foundation) - ✅ COMPLETED
- **Week 1:** Phase 3 (Main.go Update) - ✅ COMPLETED
- **Week 2-3:** Phase 2 (Command Migration) - IN PROGRESS
- **Week 4:** Phase 4 (Cleanup) - TODO
- **Week 5:** Testing, Documentation, Review - TODO

## Success Criteria

✅ All commands migrated to app pattern
✅ All tests passing
✅ No duplicate config/DI code
✅ Improved testability demonstrated
✅ Documentation updated
✅ Performance maintained or improved

---

**Last Updated:** 2025-10-09
**Status:** Phase 1 ✅ Complete | Phase 3 ✅ Complete | Phase 2 🔄 In Progress

## Current State Summary

**✅ What's Working:**
- Full app infrastructure in place (`cmd/starmap/app/`)
- All commands wired through app (backward compatible)
- `main.go` updated to use `app.Execute()`
- Build successful, basic commands tested
- No breaking changes to existing functionality

**🔄 Next Steps:**
- Migrate individual commands to use `AppContext` pattern
- Add comprehensive tests for app package
- Gradually remove old patterns (e.g., `internal/cmd/catalog/loader.go`)
- Update documentation for new patterns
