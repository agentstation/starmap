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

The [catalog settings reference](CATALOG_SETTINGS.md) lists every catalog flag, YAML key, default, and source boundary.
Use `--env-file .env --env-file .env.local` to select dotenv files explicitly.
The command does not discover working-directory dotenv files.

### Private configuration inputs

Selected YAML configuration and explicit dotenv files must be private regular files, each at most 1 MiB.
On Linux and macOS, the effective user must own the file, with no group or other mode permissions.
Read-only `0400` files remain valid. macOS also checks native ACL grants.
Windows applies the same owner and DACL policy as private runtime records. Native Windows qualification remains pending.

The reader resolves an explicitly selected symlink and validates its target before parsing.
A broken selected symlink causes a conflict. A genuinely absent default configuration remains optional.

Linux and macOS also check the ancestors of configuration paths. Operators remain responsible for trusted mounts. Read-only configuration mounts require no file writes.

Before correcting an access refusal, verify the selected file and the process account.
Preserve the file, then correct its ownership and private permissions through the operating system.
On POSIX, use `chmod 600 /absolute/path/config.yaml` after that review. Use the same policy for each explicit dotenv file.

The loader never repairs permissions or changes file contents. Error messages report access failures without file values.
All explicit dotenv files must pass access checks and parsing before their values enter the environment.

A refused primary file also prevents commands that need its configuration, including `config paths`.
Use operating-system file inspection to diagnose that refusal. Command help remains available without loading the file.

### Node directories and identity

These settings belong to the Starmap process. The connected runtime records its product, deployment, and instance under its state directory.

| Environment | YAML | Flag |
| --- | --- | --- |
| `STARMAP_HOME` | `home` | `--home` |
| `STARMAP_CONFIG_DIR` | `config_dir` | `--config-dir` |
| `STARMAP_DATA_DIR` | `data_dir` | `--data-dir` |
| `STARMAP_STATE_ROOT` | `state_root` | `--state-root` |
| `STARMAP_CACHE_DIR` | `cache_dir` | `--cache-dir` |
| `STARMAP_DEPLOYMENT_ID` | `deployment_id` | `--deployment-id` |
| `STARMAP_INSTANCE_ID` | `instance_id` | `--instance-id` |
| `STARMAP_SCHEDULER_IDENTITY` | `scheduler_identity` | `--scheduler-identity` |
| `STARMAP_CATALOG_STORE_PATH` | `catalog_store_path` | `--catalog-store-path` |
| `STARMAP_RELATIVE_PATH_BASE` | `relative_path_base` | `--relative-path-base` |

Root selectors require absolute paths. `STARMAP_HOME` groups `config/`, `data/`, `state/`, and `cache/` under one directory.
A specific root replaces that child. Relative leaf paths use the resolved configuration root.
The default primary file is `<config>/config.yaml`. A selected file cannot change its own configuration root.

Legacy relative selectors require explicit intent before their anchor changes.
Use the previous absolute path when upgrading an existing runtime or human catalog workspace.
For new configuration-root paths, set `relative_path_base: config`, `STARMAP_RELATIVE_PATH_BASE=config`, or `--relative-path-base=config`.
An empty declaration restores refusal of ambiguous relative selectors. Other values fail validation.

This rule covers `--config`, `CONFIG`, `state_dir`, `catalog_workspace_path`, `catalog_path`, and update workspace and source overrides.
File catalog sources apply the same rule to `catalog_source_url`. Their path report includes the resolved `source-file` entry.

A relative primary file needs the declaration in flags, environment, or an explicit dotenv file before file selection.
Its own contents cannot authorize that file choice. An absolute primary file can declare the base for its relative leaf settings.
The new `catalog_store_path` setting always uses the configuration root for relative values because it has no legacy anchor.

The declaration does not move files or verify an old migration. Resolve old relative values to absolute paths before changing it.
The path report includes `relative_path_base` and its origin. Starmap never searches the current directory to guess an old anchor.

Local defaults use deployment `local` and instance `default`. Configure distinct instance names for two processes on one machine.
The default runtime directory is `<state>/catalog/runtime/<instance-id>/`. A changed port does not establish another identity.
The runtime holds `.owner.lock` and retains `owner.json` and `instance-seed` in that directory.

Set `scheduler_identity` to the identity returned by an explicit runtime migration.
Select its target with `state_dir`. Keep the migrated deployment and instance values.
The owner record binds this override. Changing or clearing it requires another ownership migration.

An empty override selects the derived identity only when the directory has no conflicting owner record.
Flags override environment values, explicit dotenv files, and YAML values in that order.
Starport must select its own node identity. It must not inherit this setting from Starmap environment variables.

The product owns those identity files. Use an explicit migration or recovery operation to change their ownership.

On Linux and macOS, existing runtime directories and identity files must belong to the effective user and deny group and other mode bits.
This check covers the runtime directory, `.owner.lock`, `owner.json`, and `instance-seed`.
Application startup checks the configured runtime path before baseline export. Runtime ownership checks the selected path again before identity writes.

A refusal identifies the file role. Inspect the selected path with `config paths --inspect` before changing permissions.
Run the service under its intended account. Restore owner-only modes only after verifying that account and the selected paths.

Use `0700` for the runtime directory and `0600` for its identity files. Starmap does not change existing modes automatically.
After the correction, retry startup. The same account and permission checks apply when migration opens a runtime directory.

On macOS, startup also checks native ACL entries on those four paths.
An allow entry with nonzero rights must name the file owner, including inherited entries. Deny-only entries remain valid.
This rule refuses non-owner grants even when another deny entry could limit their effective access.

Use `ls -lde <path>` to inspect an affected ACL. Review each grant before an explicit operator correction. Starmap does not remove ACLs automatically.
The `config paths --inspect` report still shows mode bits and ownership, without ACL entries.

Linux and macOS also reject unsafe runtime ancestors. Native Windows ancestor checks and effective read and write access remain unqualified.

The Windows adapter requires the process account as owner and restricts grants to that account, SYSTEM, and Administrators.
New runtime identity paths receive protected DACLs during creation. Existing paths require an explicit operator correction.
Use `icacls <path>` to inspect Windows grants. Deny entries remain valid, but absent, null, unsupported, or broader DACLs cause refusal.

Native Windows enforcement, full file-role policies, and recovery remain unverified. Native qualification remains required before production support.

The [CSP2 checkpoint](plans/proof/starport-production-catalog/csp2.md) identifies incomplete legacy migration and native platform qualification on this branch.

### Inspect selected file locations

```sh
starmap config paths
starmap config paths --output json
starmap config paths --output yaml
starmap config paths --output wide
```

The command resolves the selected configuration without opening a catalog runtime or creating product files.
The table lists the four roots and managed file roles. JSON and YAML include origins, anchors, creation conditions, and recovery rules.
The report contains path metadata and excludes credential values.

File patterns describe possible files under each location. They do not assert file existence, access permissions, or active ownership.

`available` identifies an implemented file role. `disabled` identifies an unselected human catalog workspace.
`planned` identifies a reserved path whose writer remains unimplemented.

Each file role and external destination includes a `policy` object in JSON and YAML.
It names selectors, applicability, access requirements, retention class, and the condition for removal.
Selectors identify configuration controls or explicit destination choices. They do not contain setting values or executable commands.
Wide output adds access and retention columns. A mixed root can contain files with different policies.

| Access class | Meaning |
| --- | --- |
| `owner-only` | Private access for the operating account. Use private POSIX modes and equivalent Windows ACLs. |
| `public-read` | The exact embedded baseline permits explicit shared reads. Startup still creates its export privately. |
| `deployment-controlled` | The operator sets access for catalog facts, authoring files, or source inputs. This class does not imply public data. |
| `external-system` | The external service or tool owns access control. |

Retention classes distinguish operator files, reproducible exports, durable state, identity, recovery records, rebuildable caches, and audit records.
The `removal` field states the condition for cleanup. No retention class enables automatic deletion.
Retained runtime evidence and GitHub replay floors are durable state. Disabling downloads does not make them disposable caches.

Reserved paths cover administration, download staging, managed trust, and optional file logs.
Current logging uses streams. A reported log path does not enable file logging.
External entries explain explicit destinations for dotenv, credentials, migration operations, release artifacts, shell completion, tool caches, and future exports.

Add `--inspect` to report current filesystem observations:

```sh
starmap config paths --inspect --output json
starmap config paths --inspect --max-entries 20000 --output wide
```

Inspection reports present, absent, unavailable, and inapplicable locations. It reads matching directory entries without creating product files or opening a catalog runtime.
The default budget is 10,000 entries. `--max-entries` requires `--inspect` and accepts 1 through 100,000.
Unmatched directory entries also consume the budget. A scan that reaches its budget can report `complete: false` even at the directory boundary.
The budget bounds metadata entries, not elapsed time on slow storage. Inspection is not an atomic snapshot or a readiness verdict.

Linux and macOS report POSIX mode bits, UID, and GID. These observations do not establish effective access or ACL safety.
Windows inspection now reports `windows-owner-and-dacl` with a `windows_security` observation. Native execution remains UNVERIFIED until the Windows matrix runs.

Inspection adds `access_policy`, `access_status`, and `access_reason` for declared file roles.
An `owner-only` POSIX file or directory reports `conflict` when its mode grants group or other access, or its owner differs from the operating account.
The filesystem catalog store uses this class for its root, pointer, lock, generation files, and staging entries.

Windows observations include owner and process SIDs, DACL state, entry count, and private-policy status.
An `owner-only` entry reports a conflict when its observed descriptor violates the shared private DACL policy.
Compatible descriptors retain an unverified access status. Deny entries can still prevent the process from reading or writing the file.
Unavailable metadata, unsupported descriptors, changed identities, and symbolic links retain explicit uncertainty.
The command does not follow selected symbolic links, read payload bytes, change permissions, or resolve account names through a directory service.

Table output shows the owner SID. Wide output also shows DACL state and the native observation reason.
JSON and YAML retain the complete structured observation. The report does not establish ancestor safety or a stable security snapshot.
These conflicts concern the declared policy. They do not establish actual exposure through parent directories or ACLs.

A present file without such a conflict still reports `unverified`. Inspection does not qualify effective access, including Windows ACLs.
Absent, disabled, and planned entries report `not-assessed`. Wide inspection output includes these fields.
The command never repairs permissions or deletes files. Existing startup permission guards remain separate.

The runtime owner observation also compares a bounded `owner.json` record against the configured product, deployment, instance, and identity override.
This comparison reads at most 4,097 bytes separately from the metadata budget. It reports `matches`, `absent`, `conflict`, `unavailable`, or `unverified`.
The Windows owner comparison currently reports `unverified`. A match does not verify the instance seed, live directory lock, or fleet identity fencing.

Inspection skips observed symbolic-link targets and does not read catalog payloads or credential files.
Ordinary configuration resolution still reads the selected configuration and explicit dotenv inputs before inspection.

### Private retained evidence and discovery files

Retained source and provider records use private files under the selected runtime directory.
GitHub discovery state follows the same rule. POSIX directories use `0700`, and record files use `0600`.
Windows creation applies an explicit owner and protected DACL. Native Windows qualification remains pending.

The writers check existing directory identity, ownership, and supported ACLs before access.
They reject linked directories, linked records, nonregular records, and exposed existing files.
Startup fails when retained-layer access fails. It does not treat an unsafe record as an empty catalog source.
A lost directory binding also reports a conflict instead of an absent record.

Older versions can leave evidence directories with `0755` and records with `0644`.
Before correcting a refusal, stop the affected process and preserve a consistent backup.
Use `starmap config paths --inspect --output wide` to locate the managed files and visible policy conflicts.
Review the service account, ownership, and ACLs. Correct only the affected private paths, then restart with the same configuration.
No automatic permission repair occurs on refused paths.

New writes use unique `.layer-<id>` or `.state-<id>` temporary files.
The writer flushes each complete record before publication and flushes the directory afterward.
Legacy `.tmp` files remain untouched. Preserve orphan files until an explicit recovery or cleanup procedure identifies their owner.
These operations do not qualify power-loss durability for an untested filesystem.

### Windows workspace replacement and recovery

The optional YAML workspace uses a journal when Windows replaces an existing directory.
Starmap stages and validates the new tree, records intent, moves the old tree aside, and installs the candidate.
The workspace path can be absent between those moves. A crash can extend that interval until recovery.

For a workspace named `workspace`, the parent directory can contain these files:

| File or directory | Purpose |
| --- | --- |
| `.workspace.starmap-replacement.json` | Recovery intent, directory identities, file checksums, and expected projection receipt. |
| `.workspace.candidate-<id>/` | Validated replacement awaiting installation. |
| `.workspace.backup-<id>/` | Previous workspace retained until the replacement receipt is saved. |
| `..workspace.starmap-replacement.json.<id>` | Temporary journal write before intent publication. |

`starmap config paths --inspect` reports these roles without starting recovery.
A pending journal makes complete Starmap workspace reads return a retryable conflict.
The accepted catalog remains available in memory. Existing inference permission and budget checks still apply.

Connected runtime startup with a durable current generation attempts workspace repair under the writer lock.
Close editor handles that prevent directory renaming, then restart with the same configuration and catalog store.
Go callers can explicitly invoke `Client.RepairWorkspace(ctx)` to recover a stale or interrupted projection from the client's durable catalog.
The result reports completed changes and any preserved operator changes. Repair does not publish a new catalog generation.

`starmap.New` and `starmap.NewContext` never repair or create workspace files.
Without a durable current generation, construction still refuses a pending workspace read. With one, catalog reads remain available without changing the workspace.

If repair reports changed or unexpected files, preserve the journal, candidate, backup, and workspace together before operator recovery.
Do not delete the journal to suppress the conflict. Recovery checks directory identity as well as file contents.
A copied directory is not interchangeable with the recorded directory, even when its bytes match.

Native Windows execution and power-loss qualification remain pending. Local process-exit tests do not establish either guarantee.

### Source cache and checkout paths

The `update` command and server acquisition use the configured cache root.
HTTP source files use `<cache>/models.dev/{api.json,api.json.metadata.json}`. Git checkout files use `<cache>/sources/models.dev-git/`.
The path report includes `source_cache` and `source_checkout`. Reading the report creates no files.

`STARMAP_HOME` and `STARMAP_CACHE_DIR` select the same cache root used by these adapters.
An explicit `update --sources-dir` overrides both source parents. `STARMAP_SOURCES_DIR` remains its environment fallback.
Absolute overrides retain their exact locations. Relative overrides require the explicit configuration-root declaration above.
Use absolute paths for service configuration.
Provider and author logo projection uses the selected checkout parent.

The defaults do not read, move, or delete old `~/.starmap/cache` or `~/.starmap/sources` directories.
Accepted catalog evidence remains in the runtime and catalog store. Source caches can rebuild through permitted acquisition.
This change does not add permission to contact a source or change its authority.

### Runtime migration

Stop the source process before migration. Keep older binaries stopped afterward because they do not honor retirement records.
Keep the selected configuration file unchanged until completion finishes.
This procedure moves one runtime directory. It does not move the catalog store, workspace, or other product files.

Each operation requires `--from`, `--to`, `--operation-id`, and `--source-identity`.
Use absolute runtime paths and the identity observed in the old deployment.
The journal defaults to `<state>/migrations`. Use `--journal-root` to select another absolute directory outside both runtime trees.
Reuse the same journal root, paths, operation ID, identity, deployment, and instance for retries.

| Command | Result |
| --- | --- |
| `starmap migrate runtime prepare` | Records file checksums and reports whether the old owner record verifies the identity. |
| `starmap migrate runtime stage` | Copies and verifies files in a private staging directory. The target remains absent. |
| `starmap migrate runtime publish` | Publishes the target, records source retirement, and returns the target path and retained identity. |
| `starmap migrate runtime complete` | Verifies the saved selection, opens the replacement, records completion, and closes that runtime. |

Use `--output json` for machine-readable results. Publication includes staging when no verified stage exists.
If publication stops, retry `publish` with the same arguments. It verifies existing operation records before reuse.

Merge the following fields into the selected configuration file after publication. Replace the example values with the operation's target and ownership.

```yaml
state_dir: /absolute/replacement-runtime
scheduler_identity: observed-source-identity
deployment_id: production
instance_id: catalog-a
```

Run `complete` with the same migration arguments and `--config` pointing to that file.
Flags or environment variables alone do not satisfy the saved-selection check.
Completion refuses a changed file or disagreement between the saved and effective selections.
It verifies the required migration again under the target lock before owner, seed, or catalog initialization.
The command preserves configuration content and closes its temporary runtime before returning.

Configure the service to select the same file and roots before restarting it.
Completion verifies the command's selected configuration. It does not edit service definitions or prove filesystem power-loss durability.
Retain the migration journal and source files until the deployment's recovery procedure permits their removal.

Completion also writes `.migration-completed.json` in the target runtime directory.
If completion stops, retry `complete` with the same arguments and saved configuration.
A retry repairs a missing record after the final journal event. It refuses conflicting records.

Default-root startup accepts the preserved legacy runtime only when this record verifies its source files, retirement, target, owner, and identity.
Keep `scheduler_identity` set to the retained identity. Another recognized legacy runtime still causes refusal.
Later catalog updates in the target do not invalidate the record. Verification does not require the operation journal.
Keep the old source intact while default-root startup needs this acknowledgement.


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
configured `catalog_path` to the resolved catalog store, then materializes the
current generation at `catalog_path` as editable provider YAML. It accepts no
path arguments: `catalog_path` follows normal configuration precedence and the
default machine state destination is `<state>/catalog`. An explicit `STARMAP_CATALOG_STORE_PATH` overrides it.

This command handles the catalog-store versus workspace layout. Product-root and runtime-identity migration remain separate, incomplete CSP2 operations.

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

## Catalog Source Settings

Every command that opens a connected runtime reads the same catalog settings.
`internal/catalog/settings` owns the canonical names. Each name has one
kebab-case flag that carries the same value grammar, so one parser reads the
environment value and the flag value.

```bash
starmap serve --catalog-source starmap \
  --catalog-source-url https://catalog.example.com/api/v1
STARMAP_CATALOG_SOURCE=embedded starmap serve
```

A flag wins over its environment name only when the operator changed the flag.
An untouched flag never replaces an environment value.
[ARCHITECTURE.md](ARCHITECTURE.md#catalog-settings) lists every name, every
flag, and every default. Starport reads the same suffixes under the
`STARPORT_` prefix.

The credential settings stay separate. `--catalog-source-api-key` and
`--catalog-source-token` reach the catalog source only. `API_KEY` is the server
credential of `starmap serve --auth`. Provider credentials belong to
acquisition. No setting is an alias of another.

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

### Ancestor access on Linux and macOS

Private runtime, retained-file, discovery, and configuration paths require ancestors that protect their directory entries.
Each existing ancestor and selected symlink must belong to root or the effective user.
Other accounts can read and search trusted ancestors. Group or other write permissions require the sticky bit on a trusted ancestor.
The sticky rule permits normal temporary directories while the child ownership checks protect the selected path.

Starmap checks every intermediate directory in each symlink route.

Intermediate directories and final targets must pass their applicable checks.

macOS also refuses ancestor ACL grants that permit other accounts to change directory entries, permissions, ownership, or attributes.
Root and the effective user remain trusted. Read-only grants and deny entries remain valid.

A refused operation preserves existing file bytes and modes. Runtime checks run before baseline export, and configuration checks run before parsing.
Private directory creation uses verified parent handles and checks child identity before descent.
Retained-file publication checks ancestor access again before replacing its record.

Inspect the selected path, its ancestors, and the service identity before correcting ownership or permissions.
Do not change shared system directory permissions without reviewing their other users.
Windows ancestor enforcement, managed-service ownership exceptions, and full filesystem qualification remain open.


### Ancestor access on Windows

Private-file operations now check existing Windows ancestors before access. This implementation still requires native Windows qualification.
The guard accepts ancestors owned by the process account, SYSTEM, Administrators, or the Windows Modules Installer service.
Private leaf files retain their stricter process-account ownership rule.

Ancestors may grant shared read and traversal access. They may also grant subdirectory creation while child ownership and protected private creation remain mandatory.
Grants that permit deletion, file creation, attribute changes, or security changes require a trusted host principal.
The guard rejects absent or null DACLs and unsupported ACE types or flags.
It does not compute ordered effective access, so a deny entry does not excuse an unsafe allow entry.

Inheritance-only entries do not grant access to the ancestor itself. New private children receive protected ACLs, and existing children receive their own checks.
The guard validates selected symlinks and their target ancestors. It inspects each component before parent traversal can remove it from the path.

Drive paths, UNC shares, extended drive and UNC forms, and volume GUID paths enter the filesystem validator.
Physical devices and unsupported reparse points cause refusal. Acceptance by the parser does not qualify every filesystem or mount arrangement.

On refusal, inspect the ancestor path in the error and the intended service identity. Starmap does not change existing permissions automatically.
Native tests cover permission correction and retained-file preservation, but their Windows execution remains UNVERIFIED.
