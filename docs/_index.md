---
title: "Starmap Documentation"
weight: 1
---

# Starmap - AI Model Catalog

Welcome to Starmap, a unified AI model catalog system providing accurate, up-to-date information about AI models, their capabilities, pricing, and availability across providers.

## Quick Links

### 📚 Browse the Catalog
- [**Model Catalog**](catalog/readme/) - Complete listing of all AI models
- [**Providers**](catalog/providers/readme/) - Browse models by provider
- [**Authors**](catalog/authors/readme/) - Browse models by creator/author

### 🚀 Getting Started
- [**Provider Implementation Guide**](provider_implementation_guide/) - Add new provider support
- [**Release Process**](release_process/) - How to create and publish releases
- [**Deployment Guide**](deployment/) - Deploy documentation to GitHub Pages

## What is Starmap?

Starmap solves the problem of fragmented AI model information by combining data from multiple sources into a single, authoritative catalog. It addresses key challenges:

- **Missing Pricing**: Provider APIs often lack pricing information
- **Incomplete Metadata**: Basic specs without detailed capabilities  
- **Rapid Changes**: Models launch weekly, requiring constant updates
- **Multiple Sources**: Combines provider APIs, community data, and manual fixes

## Features

- 🔄 **Multi-source reconciliation** - Combines data from provider APIs, models.dev, and embedded catalogs
- 📊 **Field-level authority** - Different sources can be authoritative for different fields
- 🔍 **Provenance tracking** - Know where each piece of data came from
- 💾 **Multiple storage backends** - Memory, filesystem, embedded, or custom storage
- 🧵 **Thread-safe operations** - Safe for concurrent access
- 📦 **Go embeddings** - Ship with baseline data in the binary

## Architecture

Starmap uses a two-tier architecture:

1. **Simple Catalog Operations** (`pkg/catalogs/`)
   - Two-catalog merging with strategies
   - Multiple storage backends
   - Use for: testing, simple tools, local overrides

2. **Complex Reconciliation** (`pkg/reconcile/`)
   - Multi-source reconciliation with field-level authority
   - Provenance tracking and audit trails
   - Use for: production sync, combining 3+ sources

## Installation

```bash
# Homebrew (macOS/Linux)
brew install agentstation/tap/starmap

# Or install from source
go install github.com/agentstation/starmap@latest

# Or download pre-built binaries
# See Releases page for downloads
```

## Usage

```bash
# Update local catalog
starmap update

# Sync specific provider
starmap sync -p openai

# Generate documentation
starmap docs generate

# View help
starmap --help
```

## Contributing

We welcome contributions! Please see our [Provider Implementation Guide](provider_implementation_guide/) for adding new provider support.

## License

Starmap is open source software licensed under the MIT License.