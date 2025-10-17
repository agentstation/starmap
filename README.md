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

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev)
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
✅ **Accurate Pricing**: Community-verified pricing data via models.dev  
✅ **Real-time Synchronization**: Automatic updates from provider APIs  
✅ **Flexible Architecture**: Simple merging or complex reconciliation  
✅ **Multiple Interfaces**: CLI, Go package, and future HTTP API  
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

```bash
# Add to your project
go get github.com/agentstation/starmap
```

### Docker (Coming Soon)

```bash
# Run as container
docker run -p 8080:8080 ghcr.io/agentstation/starmap:latest
```

## Quick Start

### CLI: List Available Models

```bash
# List all models
starmap list models

# Filter by provider
starmap list models --provider openai

# Search by capability
starmap list models --capability vision

# Export as JSON
starmap export --format json > models.json
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
    
    // Get the catalog
    catalog, err := sm.Catalog()
    if err != nil {
        log.Fatal(err)
    }
    
    // Find GPT-4 model
    model, err := catalog.Model("gpt-4o")
    if err == nil {
        fmt.Printf("Model: %s\n", model.Name)
        fmt.Printf("Context: %d tokens\n", model.ContextWindow)
        fmt.Printf("Input Price: $%.2f/1M tokens\n", model.Pricing.Input)
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
starmap update --provider openai --auto-approve
```

## Architecture

Starmap uses a layered architecture with clean separation of concerns:

- **User Interfaces**: CLI, Go package, and future HTTP API
- **Core System**: Catalog management, reconciliation engine, and event hooks
- **Data Sources**: Provider APIs, models.dev, embedded catalog, and local files
- **Storage Backends**: Memory, filesystem, embedded, or custom (S3, GCS, etc.)

For detailed architecture diagrams, design principles, and implementation details, see **[ARCHITECTURE.md](docs/ARCHITECTURE.md)**.

## Core Concepts

Starmap's core abstractions provide a clean separation of concerns:

### 1. Catalog
The fundamental abstraction for model data storage and access. Provides CRUD operations, multiple storage backends, and thread-safe collections. See [Catalog Package Documentation](pkg/catalogs/README.md).

### 2. Source
Abstraction for fetching data from external systems (provider APIs, models.dev, local files). Each implements a common interface for consistent data access.

### 3. Reconciliation
Intelligent multi-source data merging with field-level authority, provenance tracking, and conflict resolution. See [Reconciliation Package Documentation](pkg/reconcile/README.md).

### 4. Model
Comprehensive AI model specification including capabilities (chat, vision, audio), pricing (token costs), limits (context window, rate limits), and metadata. See [pkg/catalogs/README.md](pkg/catalogs/README.md) for the complete Model structure.

For detailed component design and interaction patterns, see **[ARCHITECTURE.md § System Components](docs/ARCHITECTURE.md#system-components)**.

## Project Structure

Starmap follows Go best practices with clear package separation:

- **`pkg/`** - Public API packages ([catalogs](pkg/catalogs/), [reconcile](pkg/reconcile/), [sources](pkg/sources/), [errors](pkg/errors/), etc.)
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

**Use [Reconciliation Package](pkg/reconcile/README.md) (Complex) When:**
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
starmap list models              # List all models
starmap list providers          # List all providers  
starmap list authors            # List all authors

# Update catalog
starmap update                  # Update all providers
starmap update -p openai        # Update specific provider
starmap update --dry-run        # Preview changes

# Development
starmap validate                # Validate configurations
starmap deps check              # Check dependency status
starmap generate completion bash # Generate shell completion
```

### Advanced Update Workflows

```bash
# Development: Use file-based catalog
starmap update --input ./catalog --provider groq --dry-run

# Production: Fresh update with auto-approval
starmap update --fresh --auto-approve

# Custom directories
starmap update --input ./dev --output ./prod

# Specific sources only
starmap update --sources "Provider APIs,models.dev (git)"
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
    "github.com/agentstation/starmap/pkg/reconcile"
)
```

### Basic Usage Patterns

#### Simple Catalog Access
```go
// Default embedded catalog with auto-updates
sm, _ := starmap.New()
catalog, _ := sm.Catalog()

// Query models
model, _ := catalog.Model("claude-3-opus")
fmt.Printf("Context: %d tokens\n", model.ContextWindow)
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
```

#### Custom Storage Backend
```go
// Use filesystem for development
catalog, _ := catalogs.New(
    catalogs.WithPath("./my-catalog"),
)

sm, _ := starmap.New(
    starmap.WithInitialCatalog(catalog),
)
```

#### Complex Reconciliation
```go
// Set up multi-source reconciliation
reconciler, _ := reconcile.New(
    reconcile.WithAuthorities(map[string]reconcile.SourceAuthority{
        "pricing": {Primary: "models.dev"},
        "limits":  {Primary: "models.dev"},
    }),
)

// Fetch from all sources
sources := []sources.Source{
    providers.New(),
    modelsdev.NewGitSource(),
}

// Reconcile and get unified catalog
result, _ := reconciler.ReconcileCatalogs(ctx, sources)
```

### Advanced Patterns

#### Automatic Updates with Custom Logic
```go
updateFunc := func(current catalogs.Catalog) (catalogs.Catalog, error) {
    // Custom sync logic
    // Could call provider APIs, merge data, etc.
    return updatedCatalog, nil
}

sm, _ := starmap.New(
    starmap.WithAutoUpdateInterval(30 * time.Minute),
    starmap.WithUpdateFunc(updateFunc),
)
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

- **Provider APIs**: Real-time model availability (OpenAI, Anthropic, Google, etc.)
- **models.dev**: Community-verified pricing and metadata ([models.dev](https://models.dev))
- **Embedded Catalog**: Baseline data shipped with starmap
- **Local Files**: User customizations and overrides

For detailed source hierarchy, authority rules, and how sources work together, see **[ARCHITECTURE.md § Data Sources](docs/ARCHITECTURE.md#data-sources)**.

## Model Catalog

Starmap includes 500+ models from 10+ providers (OpenAI, Anthropic, Google, Groq, DeepSeek, Cerebras, and more). Each package includes comprehensive documentation in its README.

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
- **Real-time Updates**: WebSocket (`/api/v1/updates/ws`) and SSE (`/api/v1/updates/stream`)
- **Performance**: In-memory caching, rate limiting (per-IP)
- **Security**: Optional API key authentication, CORS support
- **Monitoring**: Health checks (`/health`, `/api/v1/ready`), metrics endpoint
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

# Admin
POST /api/v1/update              # Trigger catalog sync
GET  /api/v1/stats               # Catalog statistics

# Health
GET  /health                     # Liveness probe
GET  /api/v1/ready               # Readiness check
```

**Configuration Flags:**
- `--port, -p`: Server port (default: 8080)
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

# Google Vertex (optional)
GOOGLE_VERTEX_PROJECT=my-project
GOOGLE_VERTEX_LOCATION=us-central1
GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json

# Starmap Configuration
STARMAP_CONFIG=/path/to/config.yaml
STARMAP_CACHE_DIR=/var/cache/starmap
STARMAP_LOG_LEVEL=info
```

### Configuration File

```yaml
# ~/.starmap.yaml
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
    rate_limit: 100
  
catalog:
  type: embedded
  auto_update: true
  update_interval: 1h
  
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
