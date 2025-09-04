---
title: "Provider Guide"
weight: 60
menu:
  after:
    weight: 60
---

# Provider Implementation Guide

## Overview

This guide explains how to add new provider support to Starmap or enhance existing provider integrations.

## Quick Start

### Adding a New Provider

1. **Add Provider Definition**
   ```yaml
   # internal/embedded/catalog/providers.yaml
   - id: "new-provider"
     name: "New Provider"
     website: "https://newprovider.com"
     description: "Provider description"
   ```

2. **Create Provider Client**
   ```go
   // internal/sources/providers/newprovider/client.go
   package newprovider
   
   type Client struct {
       apiKey string
       baseURL string
   }
   ```

3. **Register Provider**
   ```go
   // internal/sources/providers/providers.go
   case "new-provider":
       return newprovider.NewClient(apiKey)
   ```

4. **Test Implementation**
   ```bash
   # Update testdata
   starmap testdata --provider new-provider --update
   
   # Test fetching
   starmap fetch models --provider new-provider
   ```

## Provider Client Interface

Each provider client must implement the `Provider` interface:

```go
type Provider interface {
    ID() string
    Name() string
    FetchModels(ctx context.Context) ([]*Model, error)
}
```

### Required Methods

- `ID()` - Returns the provider's unique identifier
- `Name()` - Returns the provider's display name
- `FetchModels()` - Fetches available models from the provider's API

## Authentication

Provider clients should support API key authentication via environment variables:

```bash
export NEWPROVIDER_API_KEY="your-api-key"
```

## Rate Limiting

Implement rate limiting to respect provider API limits:

```go
func (c *Client) FetchModels(ctx context.Context) ([]*Model, error) {
    // Add delay between requests
    time.Sleep(100 * time.Millisecond)
    // ...
}
```

## Error Handling

Use typed errors from `pkg/errors`:

```go
if resp.StatusCode == 404 {
    return nil, &errors.NotFoundError{
        Resource: "models",
        ID:       providerID,
    }
}
```

## Testing

### Unit Tests

Create unit tests for your provider client:

```go
// internal/sources/providers/newprovider/client_test.go
func TestFetchModels(t *testing.T) {
    // Test implementation
}
```

### Integration Tests

Test with real API (requires API key):

```bash
go test ./internal/sources/providers/newprovider -integration
```

### Testdata

Generate testdata for offline testing:

```bash
starmap testdata --provider new-provider --update
```

## Common Patterns

### Pagination

Handle paginated responses:

```go
var allModels []*Model
nextPage := ""

for {
    models, next, err := c.fetchPage(ctx, nextPage)
    if err != nil {
        return nil, err
    }
    
    allModels = append(allModels, models...)
    
    if next == "" {
        break
    }
    nextPage = next
}
```

### Model Mapping

Map provider-specific model format to Starmap format:

```go
func mapToStarmapModel(providerModel *APIModel) *catalogs.Model {
    return &catalogs.Model{
        ID:   catalogs.ModelID(providerModel.Name),
        Name: providerModel.DisplayName,
        // Map other fields...
    }
}
```

## Best Practices

1. **Use Constants** - Define API endpoints and other values as constants
2. **Handle Timeouts** - Set reasonable timeouts for API requests
3. **Log Errors** - Use structured logging for debugging
4. **Validate Input** - Check API keys and parameters before making requests
5. **Cache Results** - Consider caching responses to reduce API calls

## Example Implementation

See the OpenAI provider implementation as a reference:

- Client: `internal/sources/providers/openai/client.go`
- Tests: `internal/sources/providers/openai/client_test.go`
- Testdata: `internal/sources/providers/openai/testdata/`

## Troubleshooting

### API Key Issues

```bash
# Check if API key is set
echo $NEWPROVIDER_API_KEY

# Test with explicit key
NEWPROVIDER_API_KEY="key" starmap fetch models --provider new-provider
```

### Rate Limiting

If you encounter rate limit errors:
1. Add delays between requests
2. Implement exponential backoff
3. Respect `Retry-After` headers

### Debugging

Enable verbose logging:

```bash
starmap fetch models --provider new-provider --verbose
```

## Contributing

When submitting a new provider:

1. Include unit tests
2. Generate testdata
3. Update documentation
4. Add provider to README

See our [contribution guidelines](https://github.com/agentstation/starmap/blob/master/CONTRIBUTING.md) for more details.