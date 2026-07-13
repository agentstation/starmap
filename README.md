# Starmap ⭐🗺️

> An auto-updating AI Model Catalog available as a Golang Package, CLI Tool, or Server (RESTful, WebSockets, SSE).

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
- **Automatic Synchronization**: Keep your catalog current with scheduled updates
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
✅ **Real-time Synchronization**: Automatic updates from provider APIs
✅ **Flexible Architecture**: Simple merging or complex reconciliation
✅ **Multiple Interfaces**: CLI, Go package, and HTTP Server (REST + WebSocket + SSE)
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

```bash
# Add to your project
go get github.com/agentstation/starmap
```

### Docker

Starmap provides production-ready container images built with [ko](https://ko.build) using Google's secure Chainguard base images (~2MB, zero CVEs).

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
- `ghcr.io/agentstation/starmap:v0.0.17` - Specific version
- `ghcr.io/agentstation/starmap:0.0.17` - Specific version (no v prefix)

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
starmap models list --format json > models.json
```

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

	// Provider price and service facts live on an offering.
	offering, err := catalog.Offering("openai", "gpt-4o")
	if err == nil && offering.Pricing != nil {
		fmt.Printf("OpenAI pricing: %#v\n", offering.Pricing)
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

- **User Interfaces**: CLI, Go package, and HTTP Server (REST + WebSocket + SSE)
- **Core System**: Catalog management, reconciliation engine, and event hooks
- **Data Sources**: Provider APIs, models.dev, embedded catalog, and local files
- **Generation Stores**: Memory, filesystem, SQLite, or conditional object storage

For detailed architecture diagrams, design principles, and implementation details, see **[ARCHITECTURE.md](docs/ARCHITECTURE.md)**.

## Core Concepts

Starmap's core abstractions provide a clean separation of concerns:

### 1. Catalog
The concrete immutable product for model data access. Advanced producers use a separate builder; ordinary consumers retain and share the catalog safely. See [Catalog Package Documentation](pkg/catalogs/README.md).

### 2. CatalogStore
A generation-oriented commit/read/CAS boundary. The same conformance contract covers memory, filesystem, SQLite, and conditional object-storage adapters while retaining old immutable generations.
When a client starts with a configured store, it validates and publishes that
store's current generation before returning from `starmap.New`; an empty store
uses the verified embedded/local baseline until its first successful commit.

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
Intelligent multi-source data merging with field-level authority, provenance tracking, and conflict resolution. See [Reconciliation Package Documentation](pkg/reconciler/README.md).

### 6. Model Definition

The canonical provider-independent model record: authorship, lineage,
weights/architecture, release metadata, and intrinsic capabilities. Provider
pricing, limits, availability, regions, lifecycle, modes, endpoints, and
request behavior belong to provider offerings. See
[pkg/catalogs/README.md](pkg/catalogs/README.md) for the schema reference.

For detailed component design and interaction patterns, see **[ARCHITECTURE.md § System Components](docs/ARCHITECTURE.md#system-components)**.

## Project Structure

Starmap follows Go best practices with clear package separation:

- **`pkg/`** - Public API packages ([catalogs](pkg/catalogs/), [catalogstore](pkg/catalogstore/), [reconciler](pkg/reconciler/), [sources](pkg/sources/), [errors](pkg/errors/), etc.)
- **`internal/`** - Internal implementations (providers, embedded data, transport)
- **`cmd/starmap/`** - CLI application

See [CONTRIBUTING.md § Project Structure](CONTRIBUTING.md#project-structure) for detailed directory layout and dependency rules.

## Choosing Your Approach

Starmap provides two levels of data management complexity:

**Use [Catalog Package](pkg/catalogs/README.md) (Simple) When:**
- ✅ Merging embedded catalog with local overrides
- ✅ Combining two provider responses
- ✅ Testing with mock data
- ✅ Building simple tools

**Use [Reconciliation Package](pkg/reconciler/README.md) (Complex) When:**
- ✅ Syncing with multiple provider APIs
- ✅ Integrating models.dev for pricing
- ✅ Different sources own different fields
- ✅ Need audit trail of data sources
- ✅ Building production systems

For architecture details and reconciliation strategies, see **[ARCHITECTURE.md § Reconciliation System](docs/ARCHITECTURE.md#reconciliation-system)**.

## CLI Usage

### Core Commands

```bash
# Discovery
starmap models list              # List all models
starmap providers                # List all providers
starmap authors                  # List all authors

# Model field history
starmap models history gpt-4o                    # View field provenance
starmap models history gpt-4o --fields=Name      # Filter to specific field
starmap models history gpt-4o --fields=Name,ID   # Multiple fields

# Update catalog
starmap update                  # Update all providers
starmap update openai           # Update specific provider
starmap update --dry            # Preview changes

# Development
starmap validate                # Validate configurations
starmap deps check              # Check dependency status
starmap completion bash         # Generate shell completion
```

### Advanced Update Workflows

```bash
# Development: Use file-based catalog
starmap update groq --input-dir ./catalog --dry

# Production: Fresh update with auto-approval
starmap update --force -y

# Custom directories
starmap update --input ./dev --output ./prod

# Specific sources only
starmap update --source models.dev

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

# Strict mode - Fail if dependencies missing
starmap update --require-all-sources --skip-dep-prompts

# Auto-install - Install dependencies automatically
starmap update --auto-install-deps
```

The `starmap update` command owns the interactive prompt adapter. Go library,
server, scheduler, and other non-CLI sync calls never read stdin: they skip an
optional source with missing dependencies and return a typed error for a required
source unless an explicit noninteractive dependency policy is configured.

**Available Flags:**
- `--auto-install-deps` - Automatically install missing dependencies
- `--skip-dep-prompts` - Skip sources with missing dependencies without prompting
- `--require-all-sources` - Fail if any dependencies are missing (CI/CD mode)

**Common Scenario:** The `models_dev_git` source requires `bun` for building. If missing, Starmap offers to install it or falls back to `models_dev_http` which provides the same data without dependencies.

#### Checking Dependencies

Use `starmap deps check` to verify dependency status before running updates:

```bash
# Check all dependencies
starmap deps check

# JSON output for tooling
starmap deps check --format json

# YAML output
starmap deps check --format yaml
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
    "github.com/agentstation/starmap/pkg/reconciler"
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

// Query exact provider service facts separately.
offering, err := catalog.Offering("openai", "gpt-4o")
if err == nil {
    fmt.Printf("Provider model: %s\n", offering.ProviderModelID)
}
```

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

stats := sm.HookStats() // failures, panics, drops, and callback latency
```

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

#### Syncing with Provider APIs
```go
// Non-dry mutation requires an explicit writable generation store.
store, err := catalogstore.NewFilesystem("./catalog")
if err != nil {
    return err
}
sm, err := starmap.New(starmap.WithCatalogStore(store))
if err != nil {
    return err
}

// Sync a selected provider API.
result, err := sm.Sync(ctx,
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

### Advanced Patterns

#### Explicit Updates with Custom Logic
```go
updateFunc := func(ctx context.Context, current *catalogs.Builder) (*catalogs.Builder, error) {
    // Custom sync logic
    // Honor ctx while calling providers or merging data.
    return updatedCatalog, nil
}

sm, _ := starmap.New(
    starmap.WithCatalogStore(store),
    starmap.WithUpdateFunc(updateFunc),
)

// The deployment or Starport scheduler invokes this idempotent operation.
if err := sm.Update(ctx); err != nil {
    return err
}
```

#### Filtering and Querying
```go
// Find vision-capable models under $10/M tokens
models := catalog.Models()
models.ForEach(func(id string, model *catalogs.Model) bool {
    if model.Features.Vision && model.Pricing.Input < 10 {
        fmt.Printf("Vision model: %s ($%.2f/M)\n", 
            model.ID, model.Pricing.Input)
    }
    return true
})
```

## Data Sources

Starmap combines data from multiple sources:

- **Provider APIs**: Real-time model availability (OpenAI, Anthropic, Google, Alibaba Cloud, Fireworks AI, DeepInfra, etc.)
- **models.dev**: Community-verified pricing and metadata ([models.dev](https://models.dev))
- **Embedded Catalog**: Baseline data shipped with starmap
- **Local Files**: User customizations and overrides

For detailed source hierarchy, authority rules, and how sources work together, see **[ARCHITECTURE.md § Data Sources](docs/ARCHITECTURE.md#data-sources)**.

## Model Catalog

Starmap includes 500+ models from 10+ providers (OpenAI, Anthropic, Google, Groq, DeepSeek, Cerebras, Alibaba Cloud, Fireworks AI, DeepInfra, and more). Each package includes comprehensive documentation in its README.

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

**Features:**
- **RESTful API**: Models, providers, search endpoints with filtering
- **Real-time Updates**: WebSocket (`/api/v1/updates/ws`) and SSE (`/api/v1/updates/stream`) carry the same post-commit generation/sync-run identity
- **Performance**: Generation-scoped in-memory caching, deterministic query sorting, rate limiting (per-IP)
- **Security**: Optional API key authentication, CORS support
- **Monitoring**: Health checks (`/health`, `/api/v1/ready`), metrics endpoint
- **Publication identity**: Catalog responses and real-time publication events carry the durable generation identity
- **Documentation**: OpenAPI 3.1 specs at `/api/v1/openapi.json`

**API Endpoints:**
```bash
# Models
GET  /api/v1/models              # List with filtering
GET  /api/v1/models/{id}         # Get specific model
POST /api/v1/models/search       # Advanced search

# Providers
GET  /api/v1/providers           # List providers
GET  /api/v1/providers/{id}      # Get specific provider
GET  /api/v1/providers/{id}/models  # Get provider's models

# Remote generation consumption
GET  /api/v1/catalog/manifest
GET  /api/v1/catalog/generations/{generation_id}/snapshot

# Admin
POST /api/v1/update              # Trigger catalog sync
GET  /api/v1/stats               # Catalog statistics
GET  /api/v1/operations          # Generation, freshness, last sync, scheduler state

# Health
GET  /health                     # Liveness probe
GET  /api/v1/ready               # Readiness check
```

**Configuration Flags:**
- `--port`: Server port (default: 8080)
- `--host`: Bind address (default: localhost)
- `--cors`: Enable CORS for all origins
- `--cors-origins`: Specific CORS origins (comma-separated)
- `--auth`: Enable API key authentication
- `--rate-limit`: Requests per minute per IP (default: 100)
- `--cache-ttl`: Cache TTL in seconds (default: 300)

**Environment Variables:**
```bash
HTTP_PORT=8080
HTTP_HOST=0.0.0.0
STARMAP_API_KEY=your-api-key  # If --auth enabled
```

For full server documentation, see [internal/server/README.md](internal/server/README.md).

## Configuration

### Environment Variables

```bash
# Provider API Keys
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
GOOGLE_API_KEY=...
GROQ_API_KEY=...
DEEPSEEK_API_KEY=...
CEREBRAS_API_KEY=...
DASHSCOPE_API_KEY=...
FIREWORKS_API_KEY=...

# Optional for DeepInfra catalog fetch; required for inference calls
DEEPINFRA_TOKEN=...

# Alibaba Cloud Model Studio workspace domain override (optional)
ALIBABA_MODEL_STUDIO_BASE_URL=https://{WorkspaceId}.ap-southeast-1.maas.aliyuncs.com/compatible-mode/v1

# Google Vertex (optional)
GOOGLE_VERTEX_PROJECT=my-project
GOOGLE_VERTEX_LOCATION=us-central1
GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json

# Starmap Configuration
STARMAP_CONFIG=/path/to/config.yaml
STARMAP_CACHE_DIR=/var/cache/starmap
STARMAP_LOG_LEVEL=info

# Optional readiness budgets while the embedded offline bootstrap is active
EMBEDDED_BOOTSTRAP_MAX_AGE=168h
EMBEDDED_BOOTSTRAP_MAX_SIZE_BYTES=16777216
```

### Authentication Management

Check and verify your authentication setup:

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
- Which providers have configured credentials
- Authentication method (API key, ADC, OAuth)
- Credential source (environment variable, config file, application default)
- Missing credentials with setup instructions
- Provider details (name, ID, location, type, models count)

### Configuration File

Local storage uses separate lifecycle roots:

```text
~/.starmap/
├── catalog/          # canonical immutable generation database
│   ├── current
│   └── generations/
├── exports/catalog/  # optional editable/portable YAML tree
├── cache/
├── logs/
├── sources/
└── config.yaml
```

The canonical database is passive until the first commit. YAML exports are
never used as the durable publication database, and Starmap rejects configured
database/export paths that contain one another. Because this layout predates
the first public launch, draft path names and configuration aliases are not
carried forward as compatibility surface.

```yaml
# ~/.starmap/config.yaml
catalog_path: ~/.starmap/catalog
catalog_export_path: ~/.starmap/exports/catalog
embedded_bootstrap_max_age: 168h
embedded_bootstrap_max_size_bytes: 16777216

providers:
  openai:
    api_key: ${OPENAI_API_KEY}
    rate_limit: 100
  
catalog:
  type: embedded
  
sync:
  sources:
    - Provider APIs
    - models.dev (git)
  auto_approve: false
  
logging:
  level: info
  format: json
```

## Development

To contribute or develop locally:

```bash
git clone https://github.com/agentstation/starmap.git
cd starmap
make all
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for complete development setup, testing guidelines, and contribution process. Provider work follows the normative [Adding a Provider](docs/ADDING_PROVIDERS.md) contract.

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
