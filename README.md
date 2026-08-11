# Starmap ⭐🗺️

> A versioned AI model catalog available as a Go package, CLI tool, or HTTP server with REST and SSE.

<div align="center">

```
                             ____  _                                 
                            / ___|| |_ __ _ _ __ _ __ ___   __ _ _ __  
                            \___ \| __/ _` | '__| '_ ` _ \ / _` | '_ \ 
                             ___) | || (_| | |  | | | | | | (_| | |_) |
                            |____/ \__\__,_|_|  |_| |_| |_|\__,_| .__/ 
                                                                |_|    
```

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-AGPL%203.0-blue)](LICENSE)

[Installation](#installation) • [Quick Start](#quick-start) • [API Reference](docs/API.md) • [Contributing](CONTRIBUTING.md)

</div>

## Table of Contents

- [Why Starmap?](#why-starmap)
- [Key Features](#key-features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Architecture](#architecture)
- [Core Concepts](#core-concepts)
- [Project Structure](#project-structure)
- [Choosing Your Approach](#choosing-your-approach)
- [CLI Usage](#cli-usage)
- [Go Package](#go-package)
- [Data Sources](#data-sources)
- [Model Catalog](#model-catalog)
- [HTTP Server](#http-server)
- [Configuration](#configuration)
- [Development](#development)
- [Contributing](#contributing)
- [API Reference](#api-reference)
- [License](#license)

## Why Starmap?

### The Problem

Building AI applications requires accurate information about models across multiple providers, but:

- **Fragmented Information**: Each provider has different APIs, documentation formats, and update cycles
- **Missing Pricing Data**: Many providers don't publish pricing through their APIs
- **Rapid Changes**: New models launch weekly, capabilities change, prices update
- **Integration Complexity**: Each provider requires custom code to fetch and parse model data
- **No Single Source of Truth**: Developers must check multiple sources for complete information

### The Solution

Starmap provides:

- **Unified Catalog**: Single interface for all AI model information
- **Multi-Source Reconciliation**: Combines provider APIs with community data for completeness
- **Explicit Synchronization**: Refresh from provider APIs and models.dev when
  your application, CLI, or repository workflow chooses
- **Flexible Storage**: From in-memory for testing to persistent for production
- **Event-Driven Updates**: React to model changes in real-time
- **Type-Safe Go API**: Strongly typed models with comprehensive metadata

### Who Uses Starmap?

- **AI Application Developers**: Discover and compare models for your use case
- **Platform Engineers**: Maintain accurate model catalogs for your organization
- **Tool Builders**: Integrate comprehensive model data into your products
- **Researchers**: Track model capabilities and pricing trends
- **Cost Optimizers**: Find the best price/performance for your workloads

## Key Features

✅ **Comprehensive Coverage**: 500+ models from 10+ providers
✅ **Accurate Pricing**: Valid provider-offering prices first, with models.dev fallback
✅ **Reactive Distribution**: Verified server generations activate from SSE
publication hints; source acquisition remains an explicit operation
✅ **Flexible Architecture**: Simple merging or complex reconciliation
✅ **Multiple Interfaces**: CLI, Go package, and HTTP server (REST + SSE)
✅ **Production Ready**: Thread-safe, well-tested, actively maintained  

## Installation

### CLI Tool

```bash
# Homebrew (macOS/Linux)
brew install agentstation/tap/starmap

# Or install from source
go install github.com/agentstation/starmap/cmd/starmap@latest

# Verify installation
starmap version
```

### Go Package

The library requires Go 1.25 or newer. Releases are built and verified with Go
1.26.5, while required CI also tests the latest patched Go 1.25 toolchain.
Supported library, CLI, server, and remote-consumer compositions require no C
toolchain; release archives and containers are built with `CGO_ENABLED=0`.
The external offline composition also verifies a compile-time-pinned catalog
artifact and activates it without network access, provider credentials, or
acquisition/server dependencies.

```bash
# Add to your project
go get github.com/agentstation/starmap
```

### Docker

Starmap release images are built with [ko](https://ko.build), `CGO_ENABLED=0`,
and a digest-pinned Chainguard static base. Verify and scan the exact image
digest, SBOM, and release attestation under your deployment policy.

**Quick Start:**

```bash
# Pull and run the HTTP server
docker run -p 8080:8080 ghcr.io/agentstation/starmap:latest serve --host 0.0.0.0

# Or use docker-compose (recommended)
docker-compose up
```

**Using Docker Compose:**

```bash
# 1. Copy environment template
cp .env.example .env

# 2. Edit .env with your API keys (optional)
nano .env

# 3. Start the server
docker-compose up -d

# 4. Check health
curl http://localhost:8080/api/v1/health
```

**Available Images:**

- `ghcr.io/agentstation/starmap:latest` - Latest stable release
- `ghcr.io/agentstation/starmap:v<version>` - Pinned application version
- `ghcr.io/agentstation/starmap@sha256:<digest>` - Immutable production pin

**Supported Platforms:**
- `linux/amd64` (x86_64)
- `linux/arm64` (ARM 64-bit)

See [docs/DOCKER.md](docs/DOCKER.md) for detailed deployment guides including Kubernetes, security hardening, and production best practices.

## Quick Start

### CLI: List Available Models

```bash
# List all models
starmap models list

# Filter by provider
starmap models list --provider openai

# Search by capability
starmap models list --capability vision

# Export as JSON
starmap models list --output json > models.json
```

Unfiltered model rows include `provider_id`; equal model IDs at different
providers remain separate so provider-specific prices and limits are never
silently collapsed.

### Go Package: Basic Usage

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/agentstation/starmap"
)

func main() {
    // Create starmap with embedded catalog
    sm, err := starmap.New()
    if err != nil {
        log.Fatal(err)
    }
    
    // Get the concrete immutable catalog
    catalog := sm.Catalog()
    
    // Find the canonical GPT-4o definition
    model, err := catalog.FindModel("gpt-4o")
    if err == nil {
        fmt.Printf("Model: %s\n", model.Name)
        fmt.Printf("Model ID: %s\n", model.ID)
    }

    // Provider price and service facts live on offerings.
    offerings, err := catalog.DefinitionOfferings(model.ID)
    if err == nil {
        for _, offering := range offerings {
            fmt.Printf("%s serves %s as %s\n",
                offering.ProviderID, model.ID, offering.ProviderModelID)
        }
    }
}
```

### Sync with Provider APIs

```bash
# Set up API keys
export OPENAI_API_KEY=sk-...
export ANTHROPIC_API_KEY=sk-ant-...

# Update catalog from all providers
starmap update

# Update specific provider with auto-approve
starmap update openai -y
```

## Architecture

Starmap uses a layered architecture with clean separation of concerns:

- **User Interfaces**: CLI, Go package, and HTTP server (REST + SSE)
- **Core System**: Catalog management, reconciliation engine, and event hooks
- **Data Sources**: Provider APIs, models.dev, embedded catalog, and local files
- **Generation Stores**: Memory, filesystem, or conditional object storage;
  embedding applications can inject their own store

For detailed architecture diagrams, design principles, and implementation details, see **[ARCHITECTURE.md](docs/ARCHITECTURE.md)**.

## Core Concepts

Starmap's core abstractions provide a clean separation of concerns:

### 1. Catalog
The concrete immutable product for model data access. Advanced producers use a separate builder; ordinary consumers retain and share the catalog safely. See [Catalog Package Documentation](pkg/catalogs/README.md).

### 2. CatalogStore
A generation-oriented commit/read/CAS boundary. The same conformance contract
covers Starmap's memory, filesystem, and conditional object-storage adapters
while retaining old immutable generations. Embedding applications such as
Starport may inject a database-backed implementation, but own its driver,
schema, migrations, credentials, connection pool, backups, and lifecycle.
The exact concurrency, idempotency, failure, rollback, ownership, and error
requirements are defined in the
[Catalog Store Contract](docs/CATALOG_STORE_CONTRACT.md).
For servers without a durable filesystem, `pkg/catalogstore/s3` adapts a
caller-owned AWS SDK v2 S3 client to the same object-generation contract:

```go
backend, err := s3store.New(callerOwnedS3Client, s3store.Config{
    Bucket: "starmap-catalogs",
})
if err != nil {
    return err
}
store, err := catalogstore.NewObject(backend, "production")
```

The caller configures and owns the client, endpoint, credentials, transport,
retries, and lifecycle. Construction performs no network request, and every
write requires an S3-compatible `If-None-Match` or `If-Match` conditional
write; there is no last-writer-wins fallback.

The standalone `starmap serve` command uses the CLI's filesystem generation
store by default. An embedding server selects storage before it constructs the
Starmap client: use `catalogstore.NewFilesystem` for a persistent local path, or
compose `s3store.New` with `catalogstore.NewObject` for an S3-compatible bucket,
then inject the selected store through `starmap.WithCatalogStore`. Storage mode,
paths, bucket, prefix, client, credentials, and lifecycle remain deployment
configuration; `server.New` receives the already-constructed client and does
not open storage.

When a client starts with a configured store, it validates and publishes that
store's current generation before returning from `starmap.New`; an empty store
uses either the exact configured human workspace or the verified embedded
bootstrap until its first successful commit.

Validated generations use a deterministic archive and detached in-toto
statement for release/hosted distribution. See the
[Catalog Artifact Format](docs/CATALOG_ARTIFACT_FORMAT.md).

### 3. Provider Offering

The provider-scoped service contract for a model definition. Its key combines
the provider ID with the provider's exact opaque model ID, so equal model IDs at
different providers retain independent pricing, limits, availability, regions,
endpoint behavior, lifecycle, modes, and request overrides.

### 4. Source
Abstraction for fetching data from external systems (provider APIs, models.dev, local files). Each implements a common interface for consistent data access.

### 5. Reconciliation
The acquisition pipeline combines source observations with field-level
authority, provenance tracking, and conflict resolution before publishing one
validated generation. Consumers compose this through
[`acquisition.Syncer`](acquisition/), while the reconciliation engine remains
an internal implementation detail.

Provider adapters project only directly observed model facts and catalog-
declared mappings. They do not infer capabilities or authors from model-family
names. An offering without a reviewed canonical model link stays out of the
catalog and becomes a durable review candidate in the generation manifest.
The candidate retains the exact provider model ID and its source revision and
evidence checksum.

### 6. Model Definition

The canonical provider-independent model record: authorship, lineage,
weights/architecture, release metadata, and intrinsic capabilities. Provider
pricing, limits, availability, regions, lifecycle, modes, endpoints, and
request behavior belong to provider offerings. See
[pkg/catalogs/README.md](pkg/catalogs/README.md) for the schema reference.

### Add a provider with YAML

A provider needs no Go change when it uses a compiled catalog transport and
authentication primitive. Its response fields must also fit that transport's
typed source vocabulary.

1. Add the canonical author to `internal/embedded/catalog/authors.yaml` when it
   does not exist.
2. Add intrinsic model facts to
   `internal/embedded/catalog/authors/<author>/models/<slug>.yaml`.
3. Add the provider record to `internal/embedded/catalog/providers.yaml`.
   Declare credentials, the catalog endpoint, inference endpoints, and status
   sources there.
4. Add each reviewed offering below
   `internal/embedded/catalog/providers/<provider>/models/`. Preserve the
   provider's exact `id`. Set `model: <author>/<slug>` explicitly.
5. Run `make validate`.
6. Run `make update-catalog-provider PROVIDER=<provider>` when the provider
   API is available.
7. Run `make verify` before publication.

`field_mappings` select typed wire fields and canonical destinations. The first
present source for one destination wins in YAML order. OpenAI-compatible
sources include `context_window` and `max_completion_tokens`. Their canonical
destinations are `limits.context_window` and `limits.output_tokens`.

`author_mapping` selects a transport-supported author field and maps exact,
case-insensitive, or glob values to an existing canonical author. OpenAI-
compatible acquisition supports `id` and `owned_by`. Google acquisition
supports `publisher` where its transport contract declares it.

`capability_mappings` use exact typed provider predicates. Each mapping cites
an HTTPS provider contract and names each entailed canonical target. A mapping
can use `conflict`, `first-known`, `any`, or `all` combination behavior. Model
IDs, author names, family names, and free text cannot create capability facts.

A known offering publishes only when its exact provider ID has an explicit
canonical model link. An unknown discovered ID stays out of the catalog and
becomes a durable review candidate. Add its authored model and offering record
before the next publication. The generator derives the endpoint
projections. Do not use them as an authoring source.

For detailed component design and interaction patterns, see **[ARCHITECTURE.md § System Components](docs/ARCHITECTURE.md#system-components)**.

## Project Structure

Starmap follows Go best practices with clear package separation:

- **`pkg/`** - Focused public contracts ([catalogs](pkg/catalogs/), [catalogstore](pkg/catalogstore/), [sources](pkg/sources/), [errors](pkg/errors/), etc.)
- **`internal/`** - Internal implementations (reconciliation, CLI, providers, embedded data, transport)
- **`cmd/starmap/`** - CLI application

See [CONTRIBUTING.md § Project Structure](CONTRIBUTING.md#project-structure) for detailed directory layout and dependency rules.

## Choosing Your Approach

Starmap provides two composition levels:

**Use [Catalog Package](pkg/catalogs/README.md) (Simple) When:**
- ✅ Constructing or reading one provider-YAML catalog
- ✅ Combining two provider responses
- ✅ Testing with mock data
- ✅ Building simple tools

**Use [`acquisition.Syncer`](acquisition/) When:**
- ✅ Syncing with multiple provider APIs
- ✅ Integrating models.dev for pricing
- ✅ Importing a publisher-verified portable catalog release
- ✅ Different sources own different fields
- ✅ Need audit trail of data sources
- ✅ Building production systems

For architecture details and the internal reconciliation algorithm, see
**[ARCHITECTURE.md § Reconciliation System](docs/ARCHITECTURE.md#reconciliation-system)**.

## CLI Usage

### Core Commands

```bash
# Discovery
starmap models list              # List all models
starmap providers                # List all providers
starmap authors                  # List all authors

# Model field history
starmap models history gpt-4o                    # View field provenance
starmap models history shared --provider=openrouter # Select a provider offering
starmap models history gpt-4o --fields=Name      # Filter to specific field
starmap models history gpt-4o --fields=Name,ID   # Multiple fields

# Update catalog
starmap update                  # Update all providers
starmap update openai           # Update specific provider
starmap update --dry-run        # Preview changes
starmap migrate catalog         # Explicitly migrate the former local store layout

# Development
starmap validate                # Validate configurations
starmap deps check              # Check dependency status
starmap completion bash         # Generate shell completion
```

### Advanced Update Workflows

```bash
# Development: Use a custom human workspace
starmap update groq --catalog-path ./catalog --dry-run

# Production: Fresh update with auto-approval
starmap update --force -y

# Specific sources only
starmap update --source models.dev

# Reload semantic edits from the human workspace; no filesystem watcher runs
starmap update --source local

# Reproducible Git verification requires an exact commit
starmap update --source models.dev-git --models-dev-git-commit <40-or-64-hex-commit>
```

### Dependency Management

Some data sources require external tools. Starmap handles missing dependencies gracefully:

```bash
# Interactive (default) - Prompts to install or skip
starmap update

# CI/CD - Skip sources with missing dependencies
starmap update --skip-dep-prompts

# Strict mode - Require every configured source to be healthy and nonempty
starmap update --require-all-sources --skip-dep-prompts

# Auto-install - Install dependencies automatically
starmap update --auto-install-deps
```

The `starmap update` command owns the interactive prompt adapter. Go library,
server, repository-job, and other non-CLI sync calls never read stdin: they skip
an optional source with missing dependencies and return a typed error for a
required source unless an explicit noninteractive dependency policy is
configured.

**Available Flags:**
- `--auto-install-deps` - Automatically install missing dependencies
- `--skip-dep-prompts` - Skip sources with missing dependencies without prompting
- `--require-all-sources` - Fail before reconciliation unless every configured
  source is available, complete, successful, and returns at least one model
  (CI/CD mode)

**Common Scenario:** The `models_dev_git` source requires `bun` for building. If missing, Starmap offers to install it or falls back to `models_dev_http` which provides the same data without dependencies.

#### Checking Dependencies

Use `starmap deps check` to verify dependency status before running updates:

```bash
# Check all dependencies
starmap deps check

# JSON output for tooling
starmap deps check --output json

# YAML output
starmap deps check --output yaml
```

The command shows:
- ✅ Available dependencies with version and path
- ❌ Missing dependencies with installation instructions
- ℹ️  Sources that don't require any dependencies

Example output:
```
Dependency Status:

┌────────────────────────────┬────────────────────────┬──────────────────┬─────────┬───────────────────────┐
│           SOURCE           │       DEPENDENCY       │      STATUS      │ VERSION │         PATH          │
├────────────────────────────┼────────────────────────┼──────────────────┼─────────┼───────────────────────┤
│ embedded_catalog           │ -                      │ ✅ None required │ -       │ -                     │
│ local_catalog (optional)   │ -                      │ ✅ None required │ -       │ -                     │
│ providers                  │ -                      │ ✅ None required │ -       │ -                     │
│ models_dev_git (optional)  │ Bun JavaScript runtime │ ✅ Available     │ 1.2.21  │ /opt/homebrew/bin/bun │
│                            │ Git version control    │ ✅ Available     │ 2.51.0  │ /opt/homebrew/bin/git │
│ models_dev_http (optional) │ -                      │ ✅ None required │ -       │ -                     │
└────────────────────────────┴────────────────────────┴──────────────────┴─────────┴───────────────────────┘

Additional Information:

Bun JavaScript runtime (models_dev_git):
  Description: Fast JavaScript runtime for building models.dev data
  Why needed:  Builds api.json from models.dev TypeScript source

Summary:
┌────────────────────────────────┬───────┐
│             STATUS             │ COUNT │
├────────────────────────────────┼───────┤
│ ✅ Available                   │ 2     │
│ ℹ️ Sources without dependencies │ 3     │
└────────────────────────────────┴───────┘
✅ All required dependencies are available.
```

### Environment Setup

```bash
# Required for provider syncing
export OPENAI_API_KEY=sk-...
export ANTHROPIC_API_KEY=sk-ant-...
export GOOGLE_API_KEY=...
export GROQ_API_KEY=...
export DEEPSEEK_API_KEY=...
export CEREBRAS_API_KEY=...
export DASHSCOPE_API_KEY=...
export FIREWORKS_API_KEY=...

# Optional for DeepInfra catalog fetch; required for inference calls
export DEEPINFRA_TOKEN=...

# Optional for Alibaba Cloud Model Studio regions that use workspace domains
export ALIBABA_MODEL_STUDIO_BASE_URL=https://{WorkspaceId}.ap-southeast-1.maas.aliyuncs.com/compatible-mode/v1

# Optional for Google Vertex
export GOOGLE_VERTEX_PROJECT=my-project
export GOOGLE_VERTEX_LOCATION=us-central1
```

## Go Package

### Installation and Setup

```go
import (
    "github.com/agentstation/starmap"
    "github.com/agentstation/starmap/pkg/catalogs"
)
```

### Basic Usage Patterns

#### Simple Catalog Access
```go
// Default embedded catalog; construction starts no background work.
sm, err := starmap.New()
if err != nil {
    return err
}
catalog := sm.Catalog()

// Query canonical model definitions
model, err := catalog.FindModel("gpt-4o")
if err != nil {
    return err
}
fmt.Printf("Model: %s\n", model.Name)

// Provider-specific facts remain exact and separately addressable. Provider
// identity is independent from the author/lab that created the model.
offerings, err := catalog.DefinitionOfferings(model.ID)
if err != nil {
    return err
}
for _, offering := range offerings {
    if offering.Pricing != nil {
        fmt.Printf("%s input price: %v\n",
            offering.ProviderID, offering.Pricing.Tokens.Input.Per1M)
    }
}
```

`New` is the convenient read-only constructor and uses a background context.
When construction reads a configured generation store, use `NewContext` so the
caller owns cancellation and deadlines:

```go
sm, err := starmap.NewContext(ctx,
    starmap.WithCatalogStore(store),
    starmap.WithCatalogPath("./catalog"),
)
if err != nil {
    return err
}
catalog := sm.Catalog() // non-nil after successful construction
```

Calling `Catalog` on a nil `*starmap.Client` returns nil. For every successfully
constructed client the accessor is non-failing, non-nil, O(1), allocation-free,
and safe to retain across goroutines.

#### Event-Driven Updates
```go
// React to catalog changes
sm.OnModelAdded(func(model catalogs.Model) {
    log.Printf("New model: %s", model.ID)
})

sm.OnModelUpdated(func(old, new catalogs.Model) {
    if old.Pricing.Input != new.Pricing.Input {
        log.Printf("Price changed for %s", new.ID)
    }
})

// Durable publication callbacks run asynchronously after Store.Commit.
sm.OnCatalogPublished(func(event starmap.CatalogPublishedEvent) error {
    log.Printf("catalog generation %s from sync %s", event.GenerationID, event.SyncRunID)
    return nil
})

stats := sm.HookStats() // failures, panics, coalesced generations, and callback latency
```

Generation-store CAS is the durable commit point. The immutable catalog,
generation identity, and sequence become visible atomically before callbacks
begin. Callback delivery never lets a later generation overtake an earlier
one. If callbacks lag, Starmap keeps the running generation plus the newest
pending generation; skipped intermediate sequences are observable through the
event sequence and `HookStats().Coalesced`.

#### Advanced Catalog Construction
```go
// Builders are for custom source/plugin authors and update pipelines.
builder, err := catalogs.New(
    catalogs.WithPath("./my-catalog"),
)
if err != nil {
    return err
}
catalog, err := builder.Build()
if err != nil {
    return err
}
```

The human workspace keeps canonical authored facts and provider-serving facts
readable but separate:

```yaml
# authors/moonshot-ai/models/kimi-k2.5.yaml
id: kimi-k2.5
name: Kimi K2.5
authors:
  - id: moonshot-ai
    name: Moonshot AI
```

```yaml
# providers/alibaba/models/kimi-k2.5.yaml
id: kimi-k2.5
model: moonshot-ai/kimi-k2.5
name: Kimi K2.5
pricing:
  currency: USD
  tokens:
    input:
      per_1m: 0.60
```

Any number of providers may link to the same `author/slug`; a provider may also
serve models from many unrelated authors. Starmap generates `endpoints.yaml`
from these validated links. That file is a digest-bound inspection/export
projection, not an editable source of truth.

#### Syncing with Provider APIs
```go
// Non-dry mutation requires an explicit writable generation store.
store, err := catalogstore.NewFilesystem("./state/catalog")
if err != nil {
    return err
}
sm, err := starmap.New(
    starmap.WithCatalogStore(store),
    starmap.WithCatalogPath("./catalog"),
)
if err != nil {
    return err
}
syncer, err := acquisition.New(sm)
if err != nil {
    return err
}

// Sync a selected provider API.
result, err := syncer.Sync(ctx,
    sync.WithProvider("openai"),
    sync.WithDryRun(false),
)

if err != nil {
    log.Fatal(err)
}

fmt.Printf("Added: %d models\n", result.Added)
fmt.Printf("Updated: %d models\n", result.Updated)
fmt.Printf("Removed: %d models\n", result.Removed)
```

#### Importing a Verified Catalog Release

An importer supplies the three immutable release assets and a
channel-specific `catalogartifact.PublisherVerifier`. For GitHub Releases, that
verifier should require the exact `agentstation/starmap` repository and catalog
generation workflow identity. Starmap verifies all trust inputs before
mutation, reconciles release facts below human workspace evidence, and retains
the prior generation for rollback:

```go
result, err := syncer.ImportRelease(ctx, catalogartifact.Release{
    Archive:     archive,
    Checksum:    checksum,
    Attestation: statement,
}, publisherVerifier)
if err != nil {
    return err
}
if result.Publication.Published {
    fmt.Println(result.Publication.GenerationID)
}
```

The deterministic detached statement authenticates content structure, not
publisher identity. Credentials, network clients, and trust policy remain
caller-owned.

### Advanced Patterns

#### Explicit Updates with Custom Logic
```go
publication, err := sm.Update(ctx, func(
    ctx context.Context,
    current *catalogs.Catalog,
) (*starmap.Candidate, error) {
    builder, err := catalogs.NewBuilderFrom(current)
    if err != nil {
        return nil, err
    }
    // Apply custom observations to builder while honoring ctx.
    updated, err := builder.Build()
    if err != nil {
        return nil, err
    }
    return starmap.NewCandidate(updated, starmap.CandidateEvidence{})
})
if err != nil {
    return err
}
if publication.Published {
    log.Printf("published catalog generation %s", publication.GenerationID)
}
```

#### Filtering and Querying
```go
// Definitions describe the model; offerings preserve each provider's exact
// service facts. Equal model IDs at two providers never overwrite each other.
for _, definition := range catalog.Definitions() {
    if definition.Capabilities.Features == nil ||
        !slices.Contains(definition.Capabilities.Features.Modalities.Input, "image") {
        continue
    }

    offerings, err := catalog.ProviderOfferings("openai")
    if err != nil {
        return err
    }
    for _, offering := range offerings {
        if offering.DefinitionID == definition.ID {
            fmt.Printf("%s is offered by %s\n", definition.Name, offering.ProviderID)
        }
    }
}
```

## Data Sources

Starmap combines data from multiple sources:

- **Provider APIs**: Real-time model availability (OpenAI, Anthropic, Google, Alibaba Cloud, Fireworks AI, DeepInfra, etc.)
- **models.dev**: Community-verified pricing and metadata ([models.dev](https://models.dev))
- **Embedded Catalog**: Baseline data shipped with starmap
- **Local Files**: Human authored-model and provider-serving YAML plus manual
  fallback data; generated `endpoints.yaml` is not an authority

For detailed source hierarchy, authority rules, and how sources work together, see **[ARCHITECTURE.md § Data Sources](docs/ARCHITECTURE.md#data-sources)**.

## Model Catalog

Starmap includes 500+ models from 10+ providers (OpenAI, Anthropic, Google, Groq, DeepSeek, Cerebras, Alibaba Cloud, Fireworks AI, DeepInfra, and more). Each package includes comprehensive documentation in its README.

Catalog reads keep natural Go scalar fields. When an algorithm needs to
distinguish an unreported value from an explicit zero, use the presence API:

```go
if model.Features != nil {
    supported, presence := model.Features.Support(catalogs.ModelFeatureToolCalls)
    switch presence {
    case catalogs.ValueKnown:
        fmt.Printf("tool calls reported: %t\n", supported)
    case catalogs.ValueUnknown:
        fmt.Println("tool call support explicitly unknown")
    case catalogs.ValueMissing:
        fmt.Println("source did not report tool call support")
    }
}
```

For source/plugin authors, non-zero literals need no extra ceremony. Use
`SetSupport`, `ModelLimits.Set`, `SetDescription`, or `SetOpenWeights` when a
reported `false`, `0`, or `""` must remain distinct from an omitted field.
Human YAML uses the same intuitive contract: omitted means missing, `null`
means unknown, and a zero-valued scalar is explicit.

## HTTP Server

Start a production-ready REST API server for programmatic catalog access:

```bash
# Start on default port 8080
starmap serve

# Custom configuration
starmap serve --port 3000 --cors --auth --rate-limit 100

# With specific CORS origins
starmap serve --cors-origins "https://example.com,https://app.example.com"
```

Go programs can embed the same server directly. Construction starts no listener
or background goroutine; `Serve` owns serving on the caller-provided listener,
and `Shutdown` drains HTTP before stopping server services:

```go
sm, err := starmap.New()
if err != nil {
    return err
}
srv, err := server.New(sm, server.DefaultConfig())
if err != nil {
    return err
}
listener, err := net.Listen("tcp", "127.0.0.1:8080")
if err != nil {
    return err
}
go func() {
    if err := srv.Serve(listener); err != nil {
        log.Printf("starmap server: %v", err)
    }
}()
defer srv.Shutdown(shutdownCtx)
```

The public server is read-only by default and does not import provider clients
or acquisition implementations. To expose `POST /api/v1/update`, explicitly
compose an `acquisition.Syncer` and pass `server.WithSyncer(syncer)`. This keeps
ordinary server embedding independent from provider credentials and cloud SDKs.

**Features:**
- **RESTful API**: Models, providers, search endpoints with filtering
- **OpenRouter catalog compatibility**: Exact model-by-author/slug and
  model-endpoints discovery routes over the same immutable catalog
- **Reactive Updates**: SSE (`/api/v1/updates/stream`) emits heartbeat comments and one post-commit `catalog.published` hint containing generation ID and sequence
- **Performance**: Generation-scoped in-memory caching, deterministic query sorting, rate limiting (per-IP)
- **Security**: Optional API key authentication, CORS support
- **Monitoring**: Health checks (`/health`, `/api/v1/ready`), operational
  catalog/publication/stream health (`/api/v1/stats` and `srv.Health()`), and
  metrics endpoint
- **Publication identity**: Catalog responses and real-time publication events carry the durable generation identity
- **Documentation**: OpenAPI 3.1 specs at `/api/v1/openapi.json`

**API Endpoints:**
```bash
# Models
GET  /api/v1/models              # List with filtering
GET  /api/v1/models/{id}         # Get specific model
POST /api/v1/models/search       # Advanced search

# OpenRouter-compatible catalog discovery
GET  /api/v1/model/{author}/{slug}
GET  /api/v1/models/{author}/{slug}/endpoints

# Providers
GET  /api/v1/providers           # List providers
GET  /api/v1/providers/{id}      # Get specific provider
GET  /api/v1/providers/{id}/models  # Get provider's models

# Remote generation consumption
GET  /api/v1/catalog/manifest
GET  /api/v1/catalog/generations/{generation_id}/manifest
GET  /api/v1/catalog/generations/{generation_id}/payload
GET  /api/v1/updates/stream      # Heartbeat-enabled publication hints

# Admin
POST /api/v1/update              # Trigger catalog sync
GET  /api/v1/stats               # Catalog statistics

# Health
GET  /health                     # Liveness probe
GET  /api/v1/ready               # Readiness check
```

The OpenRouter-compatible adapter implements OpenRouter's documented
[model-by-slug](https://openrouter.ai/docs/api/api-reference/models/get-a-model-by-its-slug)
and
[model-endpoints](https://openrouter.ai/docs/api/api-reference/endpoints/list-all-endpoints-for-a-model)
contracts. It resolves canonical author IDs, author aliases, known catalog model
aliases, and explicitly configured mode suffixes such as `:free`. It returns
authored identity and intrinsic facts from the model definition, then joins
every eligible provider offering at response time.
Endpoint rows therefore retain the serving provider and its exact opaque model
ID even when that provider serves models from unrelated labs. The model summary
uses the least expensive eligible USD provider price deterministically; each
endpoint retains its own provider price and limits in OpenRouter's documented
string units.

The adapter does not read generated `endpoints.yaml` and does not create another
catalog authority. That file remains the digest-bound, human-inspectable
projection of the same definition/offering join. Runtime latency, throughput,
and uptime fields are omitted because Starmap does not currently compose a
provider-performance telemetry producer; catalog freshness and SSE health are
not substitutes for provider performance. Optional server authentication still
governs these routes, with OpenRouter-shaped numeric `401` error envelopes.
Model detail links honor the server's configured path prefix.

Go consumers can opt into reactive remote catalogs without adding network
behavior to the root package:

```go
subscriber, err := remote.New(remote.Config{
    BaseURL: "https://starmap.example.com/api/v1",
})
if err != nil {
    return err
}
if err := subscriber.Start(ctx); err != nil {
    return err
}
defer subscriber.Close()

catalog := subscriber.Catalog()
model, err := catalog.FindModel("gpt-4o")

health := subscriber.Health()
log.Printf(
    "stream=%s generation=%s age=%ds retries=%d",
    health.StreamState,
    health.ActiveGenerationID,
    health.CatalogAgeSeconds,
    health.Retries,
)
```

The initial generation is verified before `Start` succeeds. SSE events are
generation hints, not catalog payloads; reconnect always performs verified
current-state catch-up, so dropped or replayed events cannot permanently stale
or partially mutate the catalog. Comment heartbeats reset the stream-liveness
deadline without triggering a fetch. The caller context owns initial fetch,
streaming, retry, and activation; `Close` cancels and joins that lifecycle
within a bounded timeout.

Polling is disabled by default. Deployments that must tolerate an unavailable
SSE route may opt into a bounded last-resort policy:

```go
PollingFallback: &remote.PollingFallbackPolicy{
    AfterFailures: 3,
    Interval:      30 * time.Second,
},
```

The subscriber sends conditional current-manifest requests only after that
stream-failure threshold. It never polls beside a healthy stream, consumes at
a rate bounded by the configured interval, and exposes the current mode and
cumulative counters through `PollingFallbackStatus()`. Authentication failures
(HTTP 401 or 403) are terminal for the active lifecycle: they do not retry or
enter polling fallback. Construct a new subscriber after credentials or access
policy have been corrected.

`subscriber.Health()` reports stream state, last heartbeat, last publication
event, last successful catch-up, active generation age, retry count, fallback
status, and a structured secret-free last error. Heartbeats establish transport
liveness only; they never change the active generation timestamp or catalog
age. The publisher exposes the matching server-side view through
`srv.Health()` and `/api/v1/stats`, including callback coalescing and SSE
backpressure/write termination counters.

`BaseURL` is the trusted publisher origin. Non-loopback servers require HTTPS
with a verified certificate chain, and redirects cannot change origin. Plain
HTTP is accepted only for local loopback embedding and tests.

**Configuration Flags:**
- `--port`: Server port (default: 8080)
- `--host`: Bind address (default: localhost)
- `--cors`: Enable CORS for all origins
- `--cors-origins`: Specific CORS origins (comma-separated)
- `--auth`: Enable API key authentication
- `--rate-limit`: Requests per minute per IP (default: 100)
- `--cache-ttl`: Cache TTL in seconds (default: 300)
- `--sse-heartbeat-interval`: Flushed comment heartbeat interval (default: 20s)
- `--sse-write-timeout`: Per-frame SSE write and flush deadline (default: 10s)

**Environment Variables:**
```bash
HTTP_PORT=8080
HTTP_HOST=0.0.0.0
API_KEY=changeme  # If --auth enabled
```

For the embeddable API, see [server/README.md](server/README.md).

## Configuration

### Environment Variables

Provider YAML declares the conventional environment names for each credential
field. Starmap checks those names in their declared order. If no conventional
name contains a value, Starmap checks the derived
`STARMAP_<PROVIDER_ID>_<FIELD_ID>` name. For example:

```bash
export OPENAI_API_KEY=sk-...

# Used only when OPENAI_API_KEY is not set.
export STARMAP_OPENAI_API_KEY=sk-...

# Starmap logging
export LOG_LEVEL=info
export LOG_FORMAT=auto
export LOG_OUTPUT=stderr

# Optional readiness budgets while the embedded offline bootstrap is active
export EMBEDDED_BOOTSTRAP_MAX_AGE=168h
export EMBEDDED_BOOTSTRAP_MAX_SIZE_BYTES=16777216
```

Starmap selects the first nonempty environment value. Resolution fails if that
value does not satisfy the catalog field contract. Starmap does not try a later
name. The resolver can use a Google, Azure, or AWS default credential chain
after catalog-declared ambient fields are absent.

Select a non-default configuration file with
`starmap --config /path/to/config.yaml <command>`. Set `catalog_path` in that
file or use `CATALOG_PATH` to select the human-editable provider YAML
workspace.

### Authentication Management

These commands check catalog-acquisition authentication. Starmap uses these
credentials only to contact providers and build catalog observations. It does
not publish the credentials in a catalog generation and it does not define how
an inference gateway stores or applies inference credentials.

Check and verify your acquisition authentication setup:

```bash
# Check authentication status for all providers
starmap providers

# Test credentials by making test API calls
starmap providers --test

# Test specific provider
starmap providers openai --test

# JSON output for automation
starmap providers --output json

# Manage Google Cloud authentication
starmap auth gcloud
```

The `providers` command shows:

- Configured providers
- Acquisition authentication method (API key or cloud chain)
- Missing credentials with setup instructions
- Provider details (name, ID, location, type, models count)

Provider records separately contain inference service facts, such as base URLs,
operation paths, offering capabilities, and status-page metadata. A consumer
such as Starport owns gateway and provider inference authentication. It must not
reuse Starmap's catalog-acquisition credentials for inference.

### Configuration File

Local storage uses separate lifecycle roots:

```text
~/.starmap/
├── catalog/          # one human-editable provider-YAML workspace
├── state/catalog/    # machine-owned immutable generation store
│   ├── current
│   └── generations/
├── cache/
├── logs/
├── sources/
└── config.yaml
```

The machine state directory is passive until the first commit. The catalog
workspace is the only human model representation and is both the local
observation and the post-commit YAML projection. Starmap rejects overlapping
workspace/state roots and rejects models.dev cache or checkout roots that
contain, equal, or sit beneath the workspace before reading or writing it.
Every authored and provider model YAML exposes the same complete Boolean
capability checklist: unobserved capabilities are displayed as `false`,
explicit uncertainty remains `null`, and missing numeric limits are not
invented as zero. This keeps hand editing discoverable while provenance and
source authority still allow a later provider or upstream observation to
replace untouched projection defaults.
Machine-store reads and commits reject symbolic-link substitutions for the
store root and its owned lock, current pointer, generation, manifest, and
payload entries. Deployments must protect the parent data path from a hostile
same-UID actor.
Atomic projection uses hidden sibling staging and a hidden sibling
generation/digest marker; neither is loaded as provider configuration, and
normal completion removes all staging. A sibling advisory writer lock
serializes projection and repair across processes; contention returns a typed
conflict while readers continue to observe one complete old or new tree. The
lock file carries no catalog data and an exited process cannot leave it held. A
pre-plan generation-store layout found at `~/.starmap/catalog` fails with a
typed migration error before mutation. Migrate that layout explicitly:

```bash
starmap migrate catalog
```

The command locks and validates the complete old store, including every
retained generation and its schema compatibility, before moving anything. It
then relocates the machine store to `~/.starmap/state/catalog` and projects the
current generation back to `~/.starmap/catalog` as provider YAML. Stop every
older Starmap process that uses this path before migration and do not restart
it afterward; older binaries do not understand the path's new meaning. A
normal failure restores the original layout. If another actor recreates the
vacated path, rollback preserves both it and the relocated store and returns a
typed conflict rather than deleting either. If the process exits after the
atomic store move, the next startup reads the exact relocated current
generation and repairs the missing YAML projection without publishing a new
generation.

Read-only construction uses the verified embedded catalog entirely in memory
and creates no workspace. The first explicit update observes that catalog as
`embedded_catalog`, commits one immutable generation even when its facts are
unchanged, and then atomically creates the complete provider-YAML workspace.
An absent path is never reported as `local_catalog`; local evidence begins only
after a real human workspace exists.

Later explicit updates load the human workspace and the running binary's
verified embedded revision as separate observations. Unchanged generated fields
can advance with the embedded revision, embedded data can fill missing fields,
and semantic human edits remain local evidence. Provider acquisition uses a
derived configuration view that keeps human connection settings while adding
providers introduced by the new embedded revision.

Starmap never watches the workspace implicitly. A running client retains its
current immutable catalog until the caller invokes
`Sync(ctx, sync.WithSources(sources.LocalCatalogID))`; the CLI equivalent is
`starmap update --source local`. One semantic change publishes one generation.
An unchanged or formatting-only reload publishes none.

Rollback reactivates a retained immutable generation through the same
generation-store compare-and-swap used by normal publication, then atomically
reproduces that generation's provider YAML and provenance:

```go
result, err := sm.Rollback(ctx, generationID)
if err != nil {
    return err
}
if result.Projection != nil && result.Projection.Status == sync.ProjectionStatusPendingRepair {
    // The generation is already active; preserve a concurrent human edit and
    // surface that the workspace still needs repair.
}
```

The generation payload digest and workspace semantic digest are intentionally
separate: derived read views live only in the immutable catalog. Repeating a
rollback to the current durable generation does not emit another publication.

```yaml
# ~/.starmap/config.yaml
catalog_path: ~/.starmap/catalog
embedded_bootstrap_max_age: 168h
embedded_bootstrap_max_size_bytes: 16777216

credential_sources:
  openai:
    api-key:
      reference: file:/run/secrets/openai-api-key
      fallback_ambient: false
```

`credential_sources` keys must be canonical provider IDs and
catalog-declared field IDs. An explicit reference runs before ambient
discovery. `fallback_ambient: true` permits ambient discovery only when the
explicit source reports `not_configured`. Invalid, denied, unavailable,
timeout, and cancellation failures are terminal.

Core references are `env:NAME` and `file:/absolute/path`. The complete grammar
is `backend:resource?version=VERSION#field`. The environment and file sources
reject `version` and `field`.

Starmap also includes these direct read sources:

| Backend | Reference resource | Authentication |
|---|---|---|
| `gcp-secret-manager` | `projects/PROJECT/secrets/SECRET` or the regional form | Application Default Credentials |
| `azure-key-vault` | `https://VAULT_HOST/secrets/SECRET` | `DefaultAzureCredential` |
| `aws-secrets-manager` | A secret name or ARN | The AWS default credential chain |
| `vault` | `MOUNT/PATH` for a KV v2 secret | The Vault client environment |
| `openbao` | `MOUNT/PATH` for a KV v2 secret | The OpenBao client environment |

The optional `version` selects a provider version. Each source selects the
latest version when `version` is absent. For Google Cloud, Azure, and AWS,
`field` selects one exact top-level JSON string. Without `field`, these sources
preserve the complete scalar payload. For Vault and OpenBao, `field` selects
one exact string value. A reference without `field` requires exactly one
string value.

Backend authentication values do not belong in a reference. Google Cloud uses
Application Default Credentials. Azure uses `DefaultAzureCredential`. AWS uses
its default configuration and credential chain. Vault uses its standard client
environment, including `VAULT_ADDR`, `VAULT_TOKEN`, and `VAULT_NAMESPACE`.
OpenBao uses its standard client environment, including `BAO_ADDR`,
`BAO_TOKEN`, and `BAO_NAMESPACE`.

Each direct source creates its official client on resolution and closes its
owned network resources after the read.

Use a regular, absolute credential file that is nonempty and no larger than
1 MiB. Starmap preserves every byte. It does not trim a trailing newline. If
the provider rejects a newline, create the file without one.

The process-owned resolver reads static files on each resolution and detects
in-place writes, atomic replacement, symlink target swaps, projected-volume
replacement, and agent rerender. Detection does not depend on modification
time alone. The resolver caches renewable cloud material until its refresh
time. Concurrent refreshes share one source operation. The resolver exposes
opaque material versions that contain no secret digest or source path.

Provider connection and credential metadata live with each human-readable
provider record. Credential values remain deployment state in environment
variables, explicit references, or cloud default chains. Embedding programs
can inject a deployment-owned `sources.ProviderCredentialResolver` with
`acquisition.WithCredentialResolver`.
Acquisition source selection and approval are operation inputs (`starmap
update` flags or `sync.Option` values), not long-lived configuration that can
silently start work.
Logging uses the explicit `LOG_LEVEL`, `LOG_FORMAT`, and `LOG_OUTPUT`
environment variables or the corresponding CLI flags where exposed.

## Development

To contribute or develop locally:

```bash
git clone https://github.com/agentstation/starmap.git
cd starmap
make all
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for complete development setup, testing guidelines, and contribution process.

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for:

- Development setup and workflow
- How to add new providers
- Testing requirements
- Pull request process
- Code guidelines

Quick links:
- [Report Bug](https://github.com/agentstation/starmap/issues)
- [Request Feature](https://github.com/agentstation/starmap/issues)
- [Contributing Guide](CONTRIBUTING.md)

## License

This project is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0).

The AGPL ensures that:
- Source code remains open for any network use
- Modifications must be shared with users
- The community benefits from all improvements

See [LICENSE](LICENSE) file for full details.

---

<div align="center">
Built with ❤️ by the Starmap Community

[Report Bug](https://github.com/agentstation/starmap/issues) • [Request Feature](https://github.com/agentstation/starmap/issues) • [Join Discord](https://discord.gg/starmap)
</div>

---

## API Reference

For complete API documentation including all types, interfaces, and functions, see **[API.md](docs/API.md)**.

Quick links:
- [Client Interface](docs/API.md#client)
- [Catalog Operations](docs/API.md#catalog)
- [Sync and Updates](docs/API.md#updater)
- [Event Hooks](docs/API.md#hooks)
- [Configuration Options](docs/API.md#option)
