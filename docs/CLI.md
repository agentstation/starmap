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

These short flags are **RESERVED** globally and must not be used for command-specific purposes:

| Short | Long         | Purpose                    | Notes                           |
|-------|--------------|----------------------------|---------------------------------|
| `-v`  | `--verbose`  | Enable verbose output      | Sets log level to debug         |
| `-q`  | `--quiet`    | Minimize output            | Sets log level to warn          |
| `-o`  | `--output`   | Output format              | table, json, yaml, wide         |
| `-h`  | `--help`     | Show help                  | Built-in Cobra flag             |

**Aliases**: `--format` and `--fmt` are hidden aliases for `--output` (backward compatibility).

**Why `-o` instead of `-f`?**
We use `-o` for output format to:
- Avoid conflict with embed cat's `--filename` flag
- Match common tools like `gcc -o output`
- Free up `-f` for `--force` in commands that need it

## Command-Specific Short Flags

Commands may define their own short flags that don't conflict with global flags:

### Update Command

| Short | Long              | Purpose                     |
|-------|-------------------|-----------------------------|
| `-f`  | `--force`         | Force fresh update          |
| `-y`  | `--yes`           | Auto-approve changes        |

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
starmap update openai --dry --force
starmap models list --provider openai --format json

# Positional: what (resource/identity)
# Flags: how (behavior modifiers)
```

### 3. Short Flag Priorities

When assigning short flags, follow this priority:

1. **Check global conflicts** - Never use `-v`, `-q`, `-o`, `-h`
2. **Common conventions** - Prefer industry standards:
   - `-f` for `--force` or `--file`
   - `-y` for `--yes` (auto-approve)
   - `-n` for `--dry-run` (alternative to `--dry`)
   - `-a` for `--all`
   - `-l` for `--long` or `--list`
3. **Mnemonic first letter** - Use first letter of long flag when possible
4. **Leave it out** - If conflicted or unclear, omit short flag entirely

### 4. Boolean vs Value Flags

**Boolean flags** (presence = true):
```bash
starmap update --dry          # true when present
starmap update --force        # true when present
```

**Value flags** (require argument):
```bash
starmap update --source provider-api    # requires value
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

### Flag Aliases

Support both long and short forms for common patterns:

```go
// Primary flag
cmd.Flags().BoolVar(&flags.Dry, "dry", false, "Preview changes")

// Deprecated alias for compatibility
cmd.Flags().BoolVar(&flags.Dry, "dry-run", false, "Preview changes (alias for --dry)")
_ = cmd.Flags().MarkDeprecated("dry-run", "use --dry instead")
```

Prefer **shorter primary flags** (`--dry`) with longer deprecated aliases (`--dry-run`) for backward compatibility.

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
   ./starmap <command> -v --dry

   # Test deprecated flags show warnings
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

## Examples by Command

### Good Flag Design

```bash
# Update command
starmap update                    # Update all
starmap update openai             # Positional argument for provider
starmap update openai --dry       # Short flag for common option
starmap update --force -y         # Multiple short flags

# Providers fetch command
starmap providers fetch              # Fetch all providers
starmap providers fetch anthropic    # Positional argument
starmap providers fetch --raw        # Long flag only (less common)

# Models list command
starmap models list               # List all
starmap models list -o json       # Global output format flag
starmap models list --provider openai --tag multimodal  # Filtering flags

# Embed ls command
starmap embed ls -lah             # Unix-like combined short flags
starmap embed ls -? # Custom help flag
```

### Anti-Patterns to Avoid

```bash
# ❌ Don't use global short flags for different purposes
starmap serve -v  # If -v meant "version" instead of "verbose"

# ❌ Don't make resources into flags when positional is clearer
starmap update --provider openai  # Use positional instead

# ❌ Don't create ambiguous flag names
starmap update --output catalog   # Does this mean format or directory?
# Use: --output-dir for directory, --format for style

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

### Command Aliases

Consider adding common aliases:
```bash
starmap ls             # Alias for "models list"
starmap get            # Alias for "models list"
starmap sync           # Alias for "update"
```

**Status**: Not implemented yet, under consideration for UX improvements.

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
- Dry run: `--dry` is primary, `--dry-run` deprecated

**Questions?** See examples in this document or check `cmd/starmap/cmd/*/` source code.
