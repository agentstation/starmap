# CLI Architecture & Reference

> Comprehensive CLI reference and implementation guidelines for Starmap

This document provides detailed CLI implementation guidelines. For high-level architectural decisions, see **[ARCHITECTURE.md § CLI Architecture](ARCHITECTURE.md#cli-architecture)**.

## Overview

Starmap's CLI follows industry best practices with a focus on:
- **POSIX compliance** - Standard Unix flag conventions
- **Discoverability** - Clear help text and intuitive commands
- **Consistency** - Same patterns across all commands
- **Ergonomics** - Short flags for common operations

**Framework**: [Cobra](https://github.com/spf13/cobra)
**Pattern**: RESOURCE-FIRST command structure with subcommands
**Philosophy**: Resource commands as parents, subcommands for actions, positional arguments for identity, flags for modifiers

## Global Flags (Reserved)

The CLI reserves these short flags globally. Do not use them for a command-specific purpose:

| Short | Long         | Purpose                    | Notes                           |
|-------|--------------|----------------------------|---------------------------------|
| `-v`  | `--verbose`  | Enable verbose output      | Sets log level to debug         |
| `-q`  | `--quiet`    | Minimize output            | Sets log level to warn          |
| `-o`  | `--output`   | Output format              | table, json, yaml, wide         |
| `-h`  | `--help`     | Show help                  | Built-in Cobra flag             |

Structured output has one spelling: `--output` (short form `-o`).

**Why `-o` instead of `-f`?**
We use `-o` for output format to:
- Avoid conflict with embed cat's `--filename` flag
- Match common tools like `gcc -o output`
- Free up `-f` for `--force` in commands that need it

## Command-Specific Short Flags

Commands may define their own short flags that do not conflict with global flags:

### Update Command

| Short | Long              | Purpose                     |
|-------|-------------------|-----------------------------|
| `-f`  | `--force`         | Force fresh update          |
| `-y`  | `--yes`           | Auto-approve changes        |

### Catalog Storage Migration

```bash
starmap migrate catalog
```

This is the only command that opts into changing a detected pre-plan local
storage layout. It moves the validated immutable catalog store from the
configured `catalog_path` to `~/.starmap/state/catalog`, then materializes the
current generation at `catalog_path` as editable provider YAML. It accepts no
path arguments: `catalog_path` follows normal configuration precedence and the
machine state destination is the canonical CLI-owned path.

Stop all older Starmap processes that use `catalog_path` before running the
command, and do not restart those binaries afterward. They do not understand
the path's new human-workspace meaning and can recreate machine state there.

The command checks every retained generation, the current pointer, payload
binding, and schema compatibility before the first rename. A normal failure restores
the old store. If another actor recreates the vacated path, rollback preserves
that data and the relocated store and returns a typed conflict instead of
deleting either. Normal startup projection repair completes an interrupted
process after the atomic move. The repair publishes no new catalog generation.

### Serve Command

| Short | Long      | Purpose                          |
|-------|-----------|----------------------------------|
| None  | `--port`  | Server port (no short flag)      |

**Note**: We removed `-p` from `--port` because it conflicted with the common `--provider` pattern used in other commands.

### Embed Commands

The `embed` command family uses a **custom help flag** pattern to free up commonly needed flags:

| Short | Long      | Purpose                          | Context           |
|-------|-----------|----------------------------------|-------------------|
| `-?`  | `--help`  | Show help (custom)               | embed parent      |
| `-h`  | See below | Command-specific                 | Varies by subcommand |
| `-f`  | See below | Command-specific                 | Varies by subcommand |

#### Embed Subcommand Flags

**embed ls:**
- `-l` / `--long` - Long format listing
- `-h` / `--human-readable` - Human-readable sizes (like Unix ls)
- `-a` / `--all` - Show hidden files
- `-R` / `--recursive` - Recursive listing

**embed cat:**
- `-f` / `--filename` - Show filename before content

This pattern allows Unix-like familiarity (`ls -lah`) while avoiding global flag conflicts.

## Flag Design Principles

### 1. Positional Arguments for Resources

Use positional arguments for the primary resource or identity:

```bash
# ✅ Good - resource is positional
starmap update openai
starmap providers fetch anthropic

# ❌ Avoid - resource as flag
starmap update --provider openai
```

**Why?**
- More natural: "update openai" reads better than "update with provider openai"
- Cleaner syntax
- Consistent with industry standards (kubectl, docker, gh)

### 2. Flags for Options and Modifiers

Use flags for filtering, options, and modifiers:

```bash
# ✅ Good - options as flags
starmap update openai --dry-run --force
starmap models list --provider openai --output json

# Positional: what (resource/identity)
# Flags: how (behavior modifiers)
```

### 3. Short Flag Priorities

When assigning short flags, follow this priority:

1. **Check global conflicts** - Never use `-v`, `-q`, `-o`, `-h`
2. **Common conventions** - Prefer industry standards:
   - `-f` for `--force` or `--file`
   - `-y` for `--yes` (auto-approve)
   - use the canonical `--dry-run` spelling for previews
   - `-a` for `--all`
   - `-l` for `--long` or `--list`
3. **Mnemonic first letter** - Use first letter of long flag when possible
4. **Leave it out** - If conflicted or unclear, omit short flag entirely

### 4. Boolean vs Value Flags

**Boolean flags** (presence = true):
```bash
starmap update --dry-run      # true when present
starmap update --force        # true when present
```

**Value flags** (require argument):
```bash
starmap update --source provider-api    # requires value
starmap update --source local           # explicit human-workspace reload
starmap serve --port 8080               # requires value
```

### 5. Deprecation Strategy

When changing flags (during early development):

**Option 1: Clean Break** (preferred for young projects)
```go
// Simply remove the old flag
cmd.Flags().StringVar(&flags.NewName, "new-name", "", "Description")
```

**Option 2: Deprecation Period** (for stable projects)
```go
// Keep old flag but mark deprecated
cmd.Flags().StringVar(&flags.Name, "old-name", "", "Description")
_ = cmd.Flags().MarkDeprecated("old-name", "use --new-name instead")
```

**Current policy**: Since Starmap is young (<1.0), we prefer **clean breaks** over deprecation when the improvement is significant.

## Special Patterns

### Custom Help Flags

For command groups that need to free up `-h` or `-f`, define a custom help flag on the parent:

```go
// Parent command
cmd.PersistentFlags().BoolP("help", "?", false, "help for embed commands")

// Now subcommands can use -h and -f
lsCmd.Flags().BoolVarP(&lsHuman, "human-readable", "h", false, "...")
catCmd.Flags().StringVarP(&catFilename, "filename", "f", "", "...")
```

**Example**: `embed` command uses `-?` for help, freeing `-h` for ls (human-readable) and `-f` for cat (filename).

### Canonical Flag Names

Use one descriptive long form and add a short form only when it is conventional
and unambiguous:

```go
cmd.Flags().BoolVar(&flags.Dry, "dry-run", false, "Preview changes")
cmd.Flags().BoolVarP(&flags.Yes, "yes", "y", false, "Auto-approve changes")
```

Prefer one descriptive long flag. Do not add prelaunch compatibility aliases.

## Testing Flag Changes

Before committing flag changes:

1. **Build and test**
   ```bash
   make build
   ./starmap <command> --help
   ```

2. **Check for conflicts**
   ```bash
   # Verify global flags work
   ./starmap <command> -v --dry-run

   # Test removed flags fail with a clear unknown-flag error
   ./starmap <command> --old-flag
   ```

3. **Run full test suite**
   ```bash
   make test
   ```

4. **Update documentation**
   - Command help text
   - README.md examples
   - This policy document

## Field History Tracking

The `models history` command provides field-level source tracking for models, showing which data sources contributed to each field value.

### Purpose

- **Data Provenance**: Track which source (Provider API, models.dev, local, embedded) provided each field
- **Authority Scores**: See why a particular source was chosen (based on field-level authorities)
- **Change History**: View complete history of value changes over time
- **Debugging**: Understand where data comes from and when it changed

### Usage

```bash
# View all field history for a model
starmap models history gpt-4o

# Select the provider when the same model ID exists at multiple providers
starmap models history shared --provider=openrouter

# Filter to specific fields (case-insensitive)
starmap models history gpt-4o --fields=name
starmap models history gpt-4o --fields=Name,ID,ContextWindow

# Wildcard patterns (case-insensitive)
starmap models history gpt-4o --fields='pricing.*'  # All pricing fields
starmap models history gpt-4o --fields='features.*' # All feature flags

# Output as JSON for analysis
starmap models history gpt-4o -o json
```

### Output Format

The table output shows:
- **Field**: Field name (e.g., Name, Pricing.Input)
  - Note: Field filtering is case-insensitive for convenience
  - `--fields=name` matches "Name", `--fields=pricing.*` matches "Pricing.Input"
- **Curr**: → indicator for current value
- **Value**: Field value (formatted as YAML for complex structures)
- **Source**: Data source that provided this value
- **Authority**: Authority score (percentage)
- **Confidence**: Confidence level (percentage)
- **When**: Timestamp of last update
- **Reason**: Explanation for why this source was chosen

Scope history to one provider model. When a model ID is unique, Starmap
infers its provider. When multiple providers expose the same ID, `--provider`
must identify the provider. This prevents confusion between pricing, limits,
and lifecycle evidence.

## Examples by Command

### Good Flag Design

```bash
# Update command
starmap update                    # Update all
starmap update openai             # Positional argument for provider
starmap update openai --dry-run   # Preview without publishing
starmap update --force -y         # Multiple short flags

# Providers fetch command
starmap providers fetch              # Fetch all providers
starmap providers fetch anthropic    # Positional argument
starmap providers fetch --raw        # Long flag only (less common)

# Models list command
starmap models list               # List all
starmap models list -o json       # Global output format flag
starmap models list --provider openai --capability vision  # Filtering flags

# Models history command
starmap models history gpt-4o                        # View all field history
starmap models history gpt-4o --fields=name          # Case-insensitive field filter
starmap models history gpt-4o --fields=Name,ID       # Multiple fields
starmap models history gpt-4o --fields='pricing.*'   # Wildcard patterns (case-insensitive)
starmap models history gpt-4o -o json                # Output as JSON

# Embed ls command
starmap embed ls -lah             # Unix-like combined short flags
starmap embed ls -? # Custom help flag
```

Model list rows always identify their provider. An unfiltered list preserves
same-ID records from different providers instead of selecting an arbitrary
price or limit.

### Anti-Patterns to Avoid

```bash
# ❌ Don't use global short flags for different purposes
starmap serve -v  # If -v meant "version" instead of "verbose"

# ❌ Don't make resources into flags when positional is clearer
starmap update --provider openai  # Use positional instead

# ❌ Don't split one workspace into input and output directories
starmap update --input-dir old --output-dir new
# Use: --catalog-path for the single human read/write workspace

# ❌ Don't use short flags that aren't mnemonic without good reason
starmap update -x  # What does -x mean? Not obvious
```

## Migration Guide

When breaking changes are necessary:

1. **Document in commit message**
   ```
   BREAKING CHANGES:
   - Remove --provider flag from update command
   - Use positional argument instead: `starmap update [provider]`

   Migration:
     Before: starmap update --provider openai
     After:  starmap update openai
   ```

2. **Update CHANGELOG** (when we have one)

3. **Consider compatibility**
   - Pre-1.0: Breaking changes acceptable with clear communication
   - Post-1.0: Use deprecation period (6-12 months) before removal

## Future Considerations

### Version-Specific Behavior

When Starmap reaches 1.0, we may need:
- Semantic versioning for breaking CLI changes
- Longer deprecation periods
- Compatibility shims
- Version warnings

### Command Names

Each command has one canonical public spelling. The CLI omits prelaunch aliases
so scripts, documentation, telemetry, and support guidance share one vocabulary.

---

## Summary

**Reserved Global Short Flags**: `-v`, `-q`, `-o`, `-h`

**Key Principles**:
1. Positional arguments for resources
2. Flags for options and modifiers
3. Check global conflicts first
4. Prefer mnemonic short flags
5. Clean breaks OK for young projects

**Special Cases**:
- Embed commands: Use `-?` for help
- Update command: Removed `--provider` flag, use positional
- Dry run: `--dry-run`

**Questions?** See examples in this document or check `internal/cli/commands/*/` source code.
