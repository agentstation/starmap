# Starmap

A comprehensive AI model catalog system that provides centralized information about AI models, their capabilities, pricing, and providers. Use starmap as a CLI tool, HTTP server, or Go package for discovering and comparing AI models across different providers.

## Table of Contents

- [Features](#features)
- [CLI Installation](#cli-installation)
- [Usage as a CLI Tool](#usage-as-a-cli-tool)
  - [Basic Commands](#basic-commands)
  - [Syncing with Provider APIs](#syncing-with-provider-apis)
  - [Development Workflow](#development-workflow)
- [Usage as an HTTP Server](#usage-as-an-http-server)
  - [Coming Soon](#coming-soon)
- [Usage as a Go Package](#usage-as-a-go-package)
  - [Go Package Installation](#go-package-installation)
  - [Basic Usage](#basic-usage)
  - [Using Different Catalog Sources](#using-different-catalog-sources)
  - [Event Hooks for Model Changes](#event-hooks-for-model-changes)
  - [Automatic Updates (Default Behavior)](#automatic-updates-default-behavior)
  - [Automatic Updates with Custom Logic](#automatic-updates-with-custom-logic)
  - [Manual Updates](#manual-updates)
  - [Future: Remote Server Mode](#future-remote-server-mode)
  - [Working with Models](#working-with-models)
  - [Error Handling](#error-handling)
- [Configuration](#configuration)
- [Thread Safety](#thread-safety)
- [Contributing](#contributing)
- [License](#license)

## Features

- **Model Discovery**: Find and compare AI models across different providers
- **Capability Assessment**: Understand model features, limits, and pricing
- **Catalog Maintenance**: Keep model information synchronized with provider APIs
- **Integration Planning**: Access structured data for building AI applications
- **Event-Driven Updates**: React to model changes with custom hooks
- **Automatic Synchronization**: Configurable update intervals
- **Remote Server Support**: Future support for centralized starmap servers

## CLI Installation

```bash
go install github.com/agentstation/starmap/cmd/starmap@latest
```

## Usage as a CLI Tool

### Basic Commands

```bash
# List all available models
starmap list models

# List all providers
starmap list providers

# List all authors
starmap list authors

# Sync catalog with provider APIs
starmap sync

# Sync specific provider
starmap sync --provider openai

# Export catalog data
starmap export
```

### Syncing with Provider APIs

```bash
# Preview changes without applying
starmap sync --dry-run

# Sync specific provider with auto-approval
starmap sync --provider anthropic --auto-approve

# Fresh sync (replaces all models)
starmap sync --provider groq --fresh

# Use custom input/output directories
starmap sync --input ./dev-catalog --output ./updated-catalog
```

### Development Workflow

```bash
# Use file-based catalog for development
starmap sync --input ./internal/embedded/catalog --provider groq --dry-run

# Update testdata for providers
starmap testdata --update

# Validate provider configurations
starmap validate
```

For detailed CLI usage and development instructions, see [CLAUDE.md](./CLAUDE.md).

## Usage as an HTTP Server

### Coming Soon

HTTP server functionality is planned for future releases. This will enable:

- **Centralized Catalog Service**: Run starmap as a web service
- **REST API**: Access model data via HTTP endpoints
- **Real-time Updates**: Subscribe to model changes via webhooks
- **Multi-tenant Support**: Serve multiple clients with API keys
- **Caching Layer**: Improved performance for high-traffic scenarios

Planned API endpoints:

```bash
# Get all models
GET /api/v1/models

# Get specific model
GET /api/v1/models/{id}

# Get models by provider
GET /api/v1/providers/{id}/models

# Subscribe to updates
POST /api/v1/webhooks
```

## Usage as a Go Package

### Go Package Installation

```bash
go get github.com/agentstation/starmap
```

### Basic Usage

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/agentstation/starmap"
    "github.com/agentstation/starmap/pkg/catalogs"
)

func main() {
    // Create a new starmap instance with default embedded catalog
    sm, err := starmap.New()
    if err != nil {
        log.Fatal(err)
    }
    
    // Get the catalog (always returns a copy)
    catalog, err := sm.Catalog()
    if err != nil {
        log.Fatal(err)
    }
    
    // List all providers
    providers := catalog.Providers()
    fmt.Printf("Found %d providers\n", providers.Len())
    
    // Get a specific model
    model, err := catalog.Model("gpt-4o")
    if err != nil {
        fmt.Printf("Model not found: %v\n", err)
    } else {
        fmt.Printf("Model: %s by %s\n", model.Name, model.AuthorID)
    }
}
```

### Using Different Catalog Sources

```go
import (
    "github.com/agentstation/starmap"
    "github.com/agentstation/starmap/pkg/catalogs/files"
    "github.com/agentstation/starmap/pkg/catalogs/memory"
)

// Use a file-based catalog for development
filesCatalog, err := files.New("./catalog-data")
if err != nil {
    log.Fatal(err)
}

sm, err := starmap.New(
    starmap.WithInitialCatalog(filesCatalog),
)

// Use an in-memory catalog for testing
memoryCatalog, err := memory.New()
if err != nil {
    log.Fatal(err)
}

sm, err := starmap.New(
    starmap.WithInitialCatalog(memoryCatalog),
)
```

### Event Hooks for Model Changes

```go
sm, err := starmap.New()
if err != nil {
    log.Fatal(err)
}

// Register event handlers
sm.OnModelAdded(func(model catalogs.Model) {
    fmt.Printf("🆕 New model added: %s\n", model.ID)
})

sm.OnModelUpdated(func(old, new catalogs.Model) {
    fmt.Printf("📝 Model updated: %s\n", new.ID)
    if old.Pricing.Input != new.Pricing.Input {
        fmt.Printf("  Price changed: %f -> %f\n", old.Pricing.Input, new.Pricing.Input)
    }
})

sm.OnModelRemoved(func(model catalogs.Model) {
    fmt.Printf("🗑️  Model removed: %s\n", model.ID)
})
```

### Automatic Updates (Default Behavior)

By default, starmap automatically starts background updates when you create a new instance. Updates run every hour and will attempt to sync with provider APIs if configured.

```go
// Auto-updates are enabled by default
sm, err := starmap.New()
if err != nil {
    log.Fatal(err)
}
// Background updates are already running!

// You can customize the update interval
sm, err := starmap.New(
    starmap.WithAutoUpdateInterval(30 * time.Minute),
)

// Or disable auto-updates entirely
sm, err := starmap.New(
    starmap.WithAutoUpdates(false),
)

// You can also control auto-updates at runtime
sm.AutoUpdatesOff() // Stop background updates
sm.AutoUpdatesOn()  // Restart background updates
```

### Automatic Updates with Custom Logic

```go
// Custom update function that syncs with provider APIs
updateFunc := func(currentCatalog catalogs.Catalog) (catalogs.Catalog, error) {
    // Your custom sync logic here
    // This could call provider APIs, merge data, etc.
    return currentCatalog, nil
}

sm, err := starmap.New(
    starmap.WithAutoUpdateInterval(30 * time.Minute),
    starmap.WithUpdateFunc(updateFunc),
)
if err != nil {
    log.Fatal(err)
}
// Auto-updates are already running with your custom logic!

// Your application continues to run...
// The catalog will be updated automatically every 30 minutes

// Cleanup when your application shuts down
defer sm.AutoUpdatesOff()
```

### Manual Updates

```go
// Disable auto-updates if you prefer manual control
sm, err := starmap.New(
    starmap.WithAutoUpdates(false),
)
if err != nil {
    log.Fatal(err)
}

// Trigger a manual update
if err := sm.Update(); err != nil {
    log.Printf("Update failed: %v", err)
} else {
    fmt.Println("Catalog updated successfully")
}
```

### Future: Remote Server Mode

#### HTTP Server Integration

Once HTTP server support is available, you'll be able to configure starmap to use a remote server:

```go
// This will be supported in future versions
apiKey := "sk-your-api-key"
sm, err := starmap.New(
    starmap.WithRemoteServer("https://api.starmap.ai", &apiKey),
    starmap.WithRemoteServerOnly(true), // Don't hit provider APIs directly
    starmap.WithAutoUpdateInterval(5 * time.Minute),
)
```

### Working with Models

```go
catalog, err := sm.Catalog()
if err != nil {
    log.Fatal(err)
}

// Get all models from a specific provider
models := catalog.Models()
models.ForEach(func(id string, model *catalogs.Model) bool {
    if model.ProviderID == "openai" {
        fmt.Printf("OpenAI Model: %s - %s\n", model.ID, model.Name)
    }
    return true // Continue iteration
})

// Find models by capability
models.ForEach(func(id string, model *catalogs.Model) bool {
    if model.Capabilities.Vision {
        fmt.Printf("Vision-capable model: %s\n", model.ID)
    }
    return true
})

// Get pricing information
if model, err := catalog.Model("gpt-4o"); err == nil {
    fmt.Printf("Input price: $%f per 1M tokens\n", model.Pricing.Input)
    fmt.Printf("Output price: $%f per 1M tokens\n", model.Pricing.Output)
    fmt.Printf("Context window: %d tokens\n", model.ContextWindow)
}
```

### Error Handling

```go
catalog, err := sm.Catalog()
if err != nil {
    log.Fatal("Failed to get catalog:", err)
}

// Handle model not found
model, err := catalog.Model("non-existent-model")
if err != nil {
    fmt.Printf("Model lookup failed: %v\n", err)
    // Continue with fallback logic
}

// Handle provider not found
provider, err := catalog.Provider("unknown-provider")
if err != nil {
    fmt.Printf("Provider lookup failed: %v\n", err)
}
```

## Configuration

The starmap package supports various configuration options through the functional options pattern:

- `WithInitialCatalog()` - Specify a custom catalog implementation
- `WithAutoUpdateInterval()` - Set automatic update frequency
- `WithUpdateFunc()` - Provide custom update logic
- `WithRemoteServer()` - Configure remote server URL and API key (future)
- `WithRemoteServerOnly()` - Disable direct provider API calls (future)

## Thread Safety

The starmap interface is fully thread-safe:

- Multiple goroutines can safely call `Catalog()` concurrently
- Event hooks are called sequentially but won't block other operations
- All catalog operations return copies, preventing data races

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

[License information]
