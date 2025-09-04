---
title: "API Documentation"
weight: 20
menu:
  before:
    weight: 20
---

# API Documentation

## Overview

Starmap provides a comprehensive Go API for working with AI model catalogs. This documentation covers the core packages and interfaces.

## Core Packages

### Catalogs Package (`pkg/catalogs`)
The main interface for working with model catalogs. Provides reading, writing, and querying capabilities.

```go
import "github.com/agentstation/starmap/pkg/catalogs"
```

[View Go Package Documentation →](https://pkg.go.dev/github.com/agentstation/starmap/pkg/catalogs)

### Reconcile Package (`pkg/reconcile`)
Advanced reconciliation system for merging multiple data sources with field-level authority.

```go
import "github.com/agentstation/starmap/pkg/reconcile"
```

[View Go Package Documentation →](https://pkg.go.dev/github.com/agentstation/starmap/pkg/reconcile)

### Sources Package (`pkg/sources`)
Interfaces and implementations for various data sources (APIs, files, embedded data).

```go
import "github.com/agentstation/starmap/pkg/sources"
```

[View Go Package Documentation →](https://pkg.go.dev/github.com/agentstation/starmap/pkg/sources)

## Quick Start

### Reading a Catalog

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/agentstation/starmap/pkg/catalogs"
)

func main() {
    // Load catalog from embedded data
    catalog, err := catalogs.New(catalogs.WithEmbedded())
    if err != nil {
        log.Fatal(err)
    }
    
    // List all providers
    providers := catalog.Providers().List()
    for _, provider := range providers {
        fmt.Printf("Provider: %s (%d models)\n", provider.Name, len(provider.Models))
    }
    
    // Get a specific model
    model, err := catalog.Models().Get("gpt-4")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Model: %s\n", model.Name)
}
```

### Updating from Sources

```go
// Sync catalog from provider APIs
syncer := starmap.NewSyncer(catalog)
err := syncer.Sync(context.Background(), starmap.SyncOptions{
    Providers: []string{"openai", "anthropic"},
})
if err != nil {
    log.Fatal(err)
}
```

## Interfaces

### Reader Interface
For read-only access to catalogs:
- `Models()` - Access model data
- `Providers()` - Access provider data
- `Authors()` - Access author data

### Writer Interface
For modifying catalogs:
- `Set(item)` - Add or update items
- `Delete(id)` - Remove items
- `Save()` - Persist changes

### Syncer Interface
For synchronizing with external sources:
- `Sync()` - Update from sources
- `Reconcile()` - Merge multiple sources

## Error Handling

Starmap uses typed errors for better error handling:

```go
import "github.com/agentstation/starmap/pkg/errors"

if err != nil {
    if errors.IsNotFound(err) {
        // Handle 404
    } else if errors.IsRateLimit(err) {
        // Handle rate limiting
    }
}
```

## Environment Variables

Configure provider API keys:

```bash
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."
export GOOGLE_API_KEY="..."
```

## Examples

For more examples, see:
- [Basic Usage Examples](https://github.com/agentstation/starmap/tree/master/examples)
- [Provider Implementation Guide](../provider_implementation_guide/)
- [Source Code](https://github.com/agentstation/starmap)

## Package Documentation

Full Go package documentation is available at:
- [pkg.go.dev/github.com/agentstation/starmap](https://pkg.go.dev/github.com/agentstation/starmap)

## Support

For issues or questions:
- [GitHub Issues](https://github.com/agentstation/starmap/issues)
- [Discussions](https://github.com/agentstation/starmap/discussions)