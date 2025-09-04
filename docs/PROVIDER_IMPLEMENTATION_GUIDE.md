---
title: "PROVIDER_IMPLEMENTATION_GUIDE"
weight: 10
---
# Provider Implementation Guide

This guide walks through adding a new provider to Starmap, using concrete examples from existing implementations.

## Table of Contents

- [Overview](#overview)
- [Step 1: Provider Configuration](#step-1-provider-configuration)
- [Step 2: Client Implementation](#step-2-client-implementation)
- [Step 3: Registration](#step-3-registration)
- [Step 4: Testing](#step-4-testing)
- [Step 5: Documentation](#step-5-documentation)

## Overview

Adding a new provider requires:
1. Provider metadata in YAML
2. Client implementation following the interface
3. Registration in the provider switch
4. Test data and test cases
5. Documentation updates

## Step 1: Provider Configuration

### Add Provider to `internal/embedded/catalog/providers.yaml`

```yaml
- id: newprovider
  name: New Provider
  description: AI models from New Provider
  website: https://newprovider.com
  api_docs: https://newprovider.com/api/docs
  base_url: https://api.newprovider.com
  api_key_env: NEWPROVIDER_API_KEY
  endpoints:
    - id: models
      path: /v1/models
      method: GET
  rate_limit:
    requests_per_minute: 60
  metadata:
    type: proprietary
    foundation: false
```

### Key Fields Explained

- **id**: Unique identifier (lowercase, no spaces)
- **api_key_env**: Environment variable for API key
- **base_url**: API base URL
- **endpoints**: API endpoints for fetching models
- **rate_limit**: Provider-specific rate limits

## Step 2: Client Implementation

### Create `internal/sources/providers/newprovider/client.go`

```go
package newprovider

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    
    "github.com/agentstation/starmap/pkg/catalogs"
    "github.com/agentstation/starmap/pkg/constants"
    "github.com/agentstation/starmap/pkg/errors"
    "github.com/agentstation/starmap/pkg/logging"
    "github.com/agentstation/starmap/internal/transport"
)

// Client implements the provider client for NewProvider
type Client struct {
    provider   *catalogs.Provider
    httpClient *http.Client
    apiKey     string
}

// NewClient creates a new NewProvider client
func NewClient(provider *catalogs.Provider) (*Client, error) {
    apiKey := provider.GetAPIKey()
    if apiKey == "" {
        return nil, &errors.ConfigurationError{
            Provider: string(provider.ID),
            Field:    "api_key",
            Message:  "API key not configured",
        }
    }
    
    return &Client{
        provider:   provider,
        httpClient: transport.NewHTTPClient(constants.DefaultHTTPTimeout),
        apiKey:     apiKey,
    }, nil
}

// FetchModels retrieves all available models from NewProvider
func (c *Client) FetchModels(ctx context.Context) ([]*catalogs.Model, error) {
    endpoint := c.provider.GetEndpoint("models")
    if endpoint == nil {
        return nil, &errors.NotFoundError{
            Resource: "endpoint",
            ID:       "models",
        }
    }
    
    url := fmt.Sprintf("%s%s", c.provider.BaseURL, endpoint.Path)
    
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return nil, fmt.Errorf("creating request: %w", err)
    }
    
    // Add authentication
    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
    req.Header.Set("Content-Type", "application/json")
    
    logging.Debug().
        Str("provider", string(c.provider.ID)).
        Str("url", url).
        Msg("Fetching models from API")
    
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, &errors.APIError{
            Provider: string(c.provider.ID),
            Err:      err,
            Message:  "request failed",
        }
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return nil, &errors.APIError{
            Provider:   string(c.provider.ID),
            StatusCode: resp.StatusCode,
            Message:    fmt.Sprintf("unexpected status code: %d", resp.StatusCode),
        }
    }
    
    var response ModelsResponse
    if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
        return nil, fmt.Errorf("decoding response: %w", err)
    }
    
    return c.convertModels(response.Models), nil
}

// convertModels converts provider-specific models to catalog models
func (c *Client) convertModels(apiModels []APIModel) []*catalogs.Model {
    models := make([]*catalogs.Model, 0, len(apiModels))
    
    for _, apiModel := range apiModels {
        model := c.convertModel(apiModel)
        if model != nil {
            models = append(models, model)
        }
    }
    
    logging.Info().
        Str("provider", string(c.provider.ID)).
        Int("count", len(models)).
        Msg("Converted models from API")
    
    return models
}

// convertModel converts a single API model to catalog format
func (c *Client) convertModel(apiModel APIModel) *catalogs.Model {
    model := &catalogs.Model{
        ID:         catalogs.ModelID(apiModel.ID),
        Name:       apiModel.Name,
        ProviderID: c.provider.ID,
        AuthorID:   inferAuthorID(apiModel.ID),
        
        Metadata: &catalogs.ModelMetadata{
            Created:     apiModel.Created,
            Description: apiModel.Description,
        },
        
        Limits: &catalogs.ModelLimits{
            MaxContextLength: apiModel.ContextLength,
        },
    }
    
    // Add capabilities based on model type
    if apiModel.Type == "chat" {
        model.Features = &catalogs.ModelFeatures{
            Chat: true,
        }
    }
    
    return model
}

// inferAuthorID attempts to determine the author from model ID
func inferAuthorID(modelID string) catalogs.AuthorID {
    // Implement provider-specific logic
    // Example: "gpt-4" -> "openai"
    return catalogs.AuthorID("newprovider")
}

// APIModel represents a model in the provider's API response
type APIModel struct {
    ID            string `json:"id"`
    Name          string `json:"name"`
    Type          string `json:"type"`
    Created       int64  `json:"created"`
    Description   string `json:"description"`
    ContextLength int    `json:"context_length"`
}

// ModelsResponse represents the API response for listing models
type ModelsResponse struct {
    Models []APIModel `json:"models"`
}
```

## Step 3: Registration

### Update `internal/sources/providers/providers.go`

Add your provider to the switch statement:

```go
func getClient(provider *catalogs.Provider) (Client, error) {
    switch provider.ID {
    case "openai":
        return openai.NewClient(provider)
    case "anthropic":
        return anthropic.NewClient(provider)
    case "newprovider":  // Add your provider here
        return newprovider.NewClient(provider)
    default:
        return nil, fmt.Errorf("unsupported provider: %s", provider.ID)
    }
}
```

## Step 4: Testing

### Create Test Data

1. Capture real API response:
```bash
# Set your API key
export NEWPROVIDER_API_KEY=your-key-here

# Update testdata
starmap testdata --provider newprovider --update
```

2. Create `internal/sources/providers/newprovider/testdata/models_list.json`:
```json
{
  "models": [
    {
      "id": "model-1",
      "name": "Model One",
      "type": "chat",
      "created": 1234567890,
      "description": "A chat model",
      "context_length": 8192
    }
  ]
}
```

### Create `internal/sources/providers/newprovider/client_test.go`

```go
package newprovider

import (
    "context"
    "encoding/json"
    "os"
    "path/filepath"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    "github.com/agentstation/starmap/pkg/catalogs"
)

func TestClient_FetchModels(t *testing.T) {
    // Load test provider
    provider := &catalogs.Provider{
        ID:      "newprovider",
        Name:    "New Provider",
        BaseURL: "https://api.newprovider.com",
    }
    
    // Test with missing API key
    t.Run("missing_api_key", func(t *testing.T) {
        _, err := NewClient(provider)
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "API key not configured")
    })
    
    // Test model conversion
    t.Run("convert_models", func(t *testing.T) {
        // Load testdata
        data, err := os.ReadFile(filepath.Join("testdata", "models_list.json"))
        require.NoError(t, err)
        
        var response ModelsResponse
        err = json.Unmarshal(data, &response)
        require.NoError(t, err)
        
        // Create client with test API key
        os.Setenv("NEWPROVIDER_API_KEY", "test-key")
        defer os.Unsetenv("NEWPROVIDER_API_KEY")
        
        client, err := NewClient(provider)
        require.NoError(t, err)
        
        // Convert models
        models := client.convertModels(response.Models)
        
        assert.Len(t, models, 1)
        assert.Equal(t, "model-1", string(models[0].ID))
        assert.Equal(t, "Model One", models[0].Name)
        assert.Equal(t, 8192, models[0].Limits.MaxContextLength)
    })
}
```

## Step 5: Documentation

### Update README Files

1. **Add to main README.md** provider list
2. **Update CLAUDE.md** with provider-specific notes
3. **Create provider README** at `docs/catalog/providers/newprovider/README.md`:

```markdown
# NewProvider

## Overview
NewProvider offers AI models for [specific use cases].

## Authentication
Set the `NEWPROVIDER_API_KEY` environment variable with your API key.

## Supported Models
- Model One: Chat completions with 8K context
- Model Two: Advanced reasoning with 32K context

## Rate Limits
- 60 requests per minute
- Burst of 10 requests

## Known Issues
- Pricing not available via API
- Model metadata may be incomplete
```

## Best Practices

### Error Handling
- Use custom error types from `pkg/errors`
- Wrap errors with context
- Check for specific error types

### Logging
- Use structured logging with `pkg/logging`
- Include provider ID in all log messages
- Log at appropriate levels (Debug for API calls, Info for results)

### Constants
- Use constants from `pkg/constants` for timeouts, limits
- No magic numbers in code

### Testing
- Always provide testdata files
- Test error conditions
- Mock HTTP responses for unit tests
- Use `-update` flag to refresh testdata

## Checklist

Before submitting your provider implementation:

- [ ] Provider added to `providers.yaml`
- [ ] Client implements all required methods
- [ ] Provider registered in switch statement
- [ ] Testdata files created
- [ ] Unit tests passing
- [ ] Documentation updated
- [ ] No hardcoded values
- [ ] Proper error handling
- [ ] Structured logging used
- [ ] Rate limiting respected

## Common Patterns

### Authentication Headers

Different providers use different auth patterns:

```go
// Bearer token
req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

// API key header
req.Header.Set("X-API-Key", apiKey)

// Basic auth
req.SetBasicAuth("api", apiKey)
```

### Response Parsing

Handle both single and batch responses:

```go
// Single object response
var model APIModel
json.Unmarshal(data, &model)

// Array response
var models []APIModel
json.Unmarshal(data, &models)

// Wrapped response
var response struct {
    Data []APIModel `json:"data"`
}
json.Unmarshal(data, &response)
```

### Model ID Inference

Determine author from model naming patterns:

```go
func inferAuthorID(modelID string) catalogs.AuthorID {
    switch {
    case strings.HasPrefix(modelID, "gpt"):
        return "openai"
    case strings.HasPrefix(modelID, "claude"):
        return "anthropic"
    case strings.Contains(modelID, "llama"):
        return "meta"
    default:
        return catalogs.AuthorID(providerID)
    }
}
```

## Support

For questions about provider implementation:
1. Check existing provider implementations for examples
2. Review test cases for patterns
3. Open an issue with the `provider-implementation` label