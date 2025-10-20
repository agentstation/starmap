# Starmap CLI Analysis & Improvement Plan

Generated: 2025-10-20

## Current Flag Usage Matrix

### Global Flags (Persistent across all commands)
| Short | Long | Type | Source | Usage |
|-------|------|------|--------|-------|
| -o | --format | string | app/execute.go | Output format |
| -v | --verbose | bool | app/execute.go | Verbose output |
| -q | --quiet | bool | app/execute.go | Minimal output |
| | --no-color | bool | app/execute.go | Disable colors |
| | --log-level | string | app/execute.go | Log level |
| | --config | string | app/execute.go | Config file |

### Resource Filter Flags (Added by globals.AddResourceFlags)
| Short | Long | Type | Commands | Usage |
|-------|------|------|----------|-------|
| -p | --provider | string | list models, list providers | Filter by provider |
| | --author | string | list models | Filter by author |
| | --search | string | list models, list providers | Search term |
| -l | --limit | int | list models, list providers | Limit results |
| | --filter | []string | list models | Complex filter expressions |
| | --all | bool | list models | Include all results |

### Command-Specific Flags

#### update command
| Short | Long | Type | Conflict? | Notes |
|-------|------|------|-----------|-------|
| -p | --provider | string | ⚠️ YES | Same as resource filter! |
| -f | --force | bool | No | Force update |
| -y | --yes | bool | No | Auto-approve |
| | --dry-run | bool | No | Preview changes |
| | --dry | bool | No | Alias for --dry-run |
| | --output | string | ⚠️ CONFUSING | Directory, not format! |
| | --input | string | No | Input directory |
| | --source | string | No | Specific source |
| | --cleanup | bool | No | Cleanup temp files |
| | --reformat | bool | No | Reformat files |
| | --sources-dir | string | No | Sources directory |
| | --auto-install-deps | bool | No | Auto install |
| | --skip-dep-prompts | bool | No | Skip prompts |
| | --require-all-sources | bool | No | Require all sources |

#### serve command
| Short | Long | Type | Conflict? | Notes |
|-------|------|------|-----------|-------|
| -p | --port | int | ⚠️ YES | Same short flag as --provider! |
| | --host | string | No | Bind address |
| | --cors | bool | No | Enable CORS |
| | --cors-origins | []string | No | CORS origins |
| | --auth | bool | No | Enable auth |
| | --auth-header | string | No | Auth header name |
| | --rate-limit | int | No | Rate limit |
| | --cache-ttl | int | No | Cache TTL |
| | --read-timeout | duration | No | Read timeout |
| | --write-timeout | duration | No | Write timeout |
| | --idle-timeout | duration | No | Idle timeout |
| | --metrics | bool | No | Enable metrics |
| | --prefix | string | No | API prefix |

#### fetch models command
| Short | Long | Type | Notes |
|-------|------|------|-------|
| | --timeout | int | API timeout |
| | --raw | bool | Raw JSON response |
| | --stats | bool | Show stats |

#### list models command
| Short | Long | Type | Notes |
|-------|------|------|-------|
| | --details | bool | Show details |
| | --capability | string | Filter by capability |
| | --min-context | int64 | Min context size |
| | --max-price | float64 | Max price |
| | --export | string | Export format |

#### embed commands (ls, cat, tree, stat)
| Short | Long | Type | Command | Notes |
|-------|------|------|---------|-------|
| -l | --long | bool | ls | Long format |
| -h | --human-readable | bool | ls | ⚠️ Conflicts with global help! |
| -a | --all | bool | ls, tree | Show hidden |
| -R | --recursive | bool | ls | Recursive |
| -f | --filename | bool | cat | Show filename |
| -n | --number | bool | cat | Number lines |
| -c | --format | string | stat | Custom format |
| -L | --level | int | tree | Max depth |
| -s | --sizes | bool | tree | Show sizes |

#### auth verify command
| Short | Long | Type | Conflict? | Notes |
|-------|------|------|-----------|-------|
| -v | --verbose | bool | ⚠️ YES | Conflicts with global -v! |
| | --timeout | duration | No | Verification timeout |

## Identified Issues

### 1. Critical Flag Conflicts

**Short Flag `-p` Collision:**
- `update --provider` uses `-p`
- `serve --port` uses `-p`
- `list --provider` uses `-p` (via globals)
- **Impact**: Cannot use `-p` portably across commands
- **Resolution**: Remove `-p` from serve, use full `--port`

**Short Flag `-h` Collision:**
- Global `-h` for `--help` (Cobra default)
- `embed ls -h` for `--human-readable`
- **Impact**: Cannot get help for ls command with `-h`
- **Resolution**: Remove `-h` from ls, use full `--human-readable`

**Short Flag `-v` Collision:**
- Global `-v` for `--verbose`
- `auth verify -v` for `--verbose` (redundant)
- **Impact**: Redundant definition can cause confusion
- **Resolution**: Remove local `-v` definition, use global

**Short Flag `-f` Removed:**
- Was intended for `--format` globally
- Conflicted with `embed cat --filename`
- Now using `-o` for format
- **Status**: ✅ Already resolved

### 2. Semantic Inconsistencies

**`--output` Flag Ambiguity:**
- Global: `--output` deprecated alias for `--format` (output style)
- Update: `--output` for output directory (file path)
- **Impact**: Same flag name, completely different meanings!
- **Resolution**: Rename update's `--output` to `--output-dir`

**`--provider` Usage Pattern:**
- `list models --provider openai` → Filter existing data
- `fetch models openai` → Positional argument
- `update --provider openai` → Flag for limiting sync scope
- **Impact**: Inconsistent mental model for users
- **Resolution**: Standardize on positional for "what to act on", flags for "how to filter"

### 3. Discoverability Issues

**Resource Flags Not Visible:**
- `globals.AddResourceFlags()` adds flags programmatically
- Help text doesn't show where these come from
- `-p, --provider` appears without explanation
- **Resolution**: Document pattern in package, add comments

**Dry-run Aliases:**
- Both `--dry` and `--dry-run` work
- Only `--dry-run` shown in help
- **Resolution**: Show both in help with alias notation

## Recommendations

### Phase 1: Fix Critical Conflicts (Breaking Changes for v1.0)

1. **Remove `-p` from serve command**
   ```diff
   - cmd.Flags().IntP("port", "p", 8080, "Server port")
   + cmd.Flags().Int("port", 8080, "Server port")
   ```

2. **Remove `-h` from embed ls command**
   ```diff
   - LsCmd.Flags().BoolVarP(&lsHuman, "human-readable", "h", false, "...")
   + LsCmd.Flags().BoolVar(&lsHuman, "human-readable", false, "...")
   ```

3. **Remove redundant `-v` from auth verify**
   ```diff
   - cmd.Flags().BoolP("verbose", "v", false, "Show detailed verification output")
   + cmd.Flags().Bool("verbose", false, "Show detailed verification output")
   ```

4. **Rename `--output` in update to `--output-dir`**
   ```diff
   - cmd.Flags().StringVar(&flags.Output, "output", "", "Save updated catalog to directory")
   + cmd.Flags().StringVar(&flags.Output, "output-dir", "", "Save updated catalog to directory")
   ```

5. **Rename `--input` in update to `--input-dir`** (consistency)
   ```diff
   - cmd.Flags().StringVar(&flags.Input, "input", "", "Load catalog from directory instead of embedded")
   + cmd.Flags().StringVar(&flags.Input, "input-dir", "", "Load catalog from directory instead of embedded")
   ```

### Phase 2: Standardize Provider/Resource Pattern

**Current:**
```bash
starmap fetch models openai        # ✅ Good: positional
starmap list models --provider openai  # ✅ Good: filter flag
starmap update --provider openai   # ❌ Inconsistent: should be positional
```

**Proposed:**
```bash
starmap fetch models [provider]    # ✅ What to fetch
starmap list models --provider X   # ✅ How to filter
starmap update [provider]          # ✅ What to update (optional = all)
```

**Implementation:**
```go
// update command
Use: "update [provider]",
Args: cobra.MaximumNArgs(1),

// In RunE
var targetProvider string
if len(args) == 1 {
    targetProvider = args[0]
}
// Use targetProvider instead of flags.Provider
```

### Phase 3: Create Short Flag Policy Document

**Reserved Global Short Flags:**
- `-h` : --help (Cobra default, never override)
- `-v` : --verbose (global)
- `-q` : --quiet (global)
- `-o` : --format (global output format)

**Common Command Short Flags:**
- `-p` : --provider (for filter operations only)
- `-l` : --limit (for pagination)
- `-f` : --force (for destructive operations)
- `-y` : --yes (for auto-approve)
- `-a` : --all (for include-all modes)
- `-n` : --number (for numbering/limiting)

**Never Use:**
- `-h` in local context (conflicts with help)
- `-v` in local context (conflicts with verbose)
- Single-letter flags for obscure options

**Document in:** `docs/CLI-STANDARDS.md` or `CLAUDE.md`

### Phase 4: Improve Flag Documentation

Add flag source documentation:
```go
// internal/cmd/globals/globals.go
// AddResourceFlags adds common resource filtering flags to a command.
// These flags are used across list and query commands for consistent filtering.
//
// Added flags:
//   -p, --provider string    Filter results by provider ID
//   -l, --limit int          Limit number of results returned
//       --author string      Filter results by author
//       --search string      Search term to filter results
//       --filter strings     Advanced filter expressions
//       --all                Include all results without filtering
func AddResourceFlags(cmd *cobra.Command) {
    // ...
}
```

## Implementation Status

- [x] Create analysis document
- [ ] Fix critical flag conflicts
- [ ] Standardize provider argument pattern
- [ ] Create short flag policy document
- [ ] Update all help text for consistency
- [ ] Add migration guide for breaking changes
- [ ] Update tests for new patterns
- [ ] Release as v1.0 with stable CLI API
