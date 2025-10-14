# Starmap ⭐🗺️

> A unified AI model catalog system providing accurate, up-to-date information about AI models, their capabilities, pricing, and availability across providers

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

[Installation](#installation) • [Quick Start](#quick-start) • [API Reference](API.md) • [Contributing](CONTRIBUTING.md)

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
- [HTTP Server](#http-server-coming-soon)
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

For detailed architecture diagrams, design principles, and implementation details, see **[ARCHITECTURE.md](ARCHITECTURE.md)**.

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

For detailed component design and interaction patterns, see **[ARCHITECTURE.md § System Components](ARCHITECTURE.md#system-components)**.

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

For architecture details and reconciliation strategies, see **[ARCHITECTURE.md § Reconciliation System](ARCHITECTURE.md#reconciliation-system)**.

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

For detailed source hierarchy, authority rules, and how sources work together, see **[ARCHITECTURE.md § Data Sources](ARCHITECTURE.md#data-sources)**.

## Model Catalog

Starmap includes 500+ models from 10+ providers (OpenAI, Anthropic, Google, Groq, DeepSeek, Cerebras, and more). Each package includes comprehensive documentation in its README.

## HTTP Server (Coming Soon)

Future HTTP server with REST API, GraphQL, WebSocket, and webhooks for centralized catalog service with multi-tenant support.

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

For complete API documentation including all types, interfaces, and functions, see **[API.md](API.md)**.

Quick links:
- [Client Interface](API.md#client)
- [Catalog Operations](API.md#catalog)
- [Sync and Updates](API.md#updater)
- [Event Hooks](API.md#hooks)
- [Configuration Options](API.md#option)
