# Starmap API Documentation

> REST API documentation for the Starmap HTTP server

**Version:** v1
**Base URL:** `http://localhost:8080/api/v1`
**Last Updated:** 2025-10-14

## Table of Contents

- [Overview](#overview)
- [Getting Started](#getting-started)
- [Authentication](#authentication)
- [Response Format](#response-format)
- [Error Handling](#error-handling)
- [Endpoints](#endpoints)
  - [Models](#models)
  - [Providers](#providers)
  - [Administration](#administration)
  - [Health & Metrics](#health--metrics)
  - [Real-time Updates](#real-time-updates)
- [Filtering & Search](#filtering--search)
- [Rate Limiting](#rate-limiting)
- [CORS](#cors)
- [Examples](#examples)

## Overview

The Starmap HTTP API provides programmatic access to the unified AI model catalog. It offers:

- **RESTful endpoints** for querying models and providers
- **Advanced filtering** with multiple criteria
- **Real-time updates** via WebSocket and Server-Sent Events
- **In-memory caching** for performance
- **Rate limiting** to prevent abuse
- **Optional authentication** with API keys

## Getting Started

### Starting the Server

```bash
# Start with default settings (port 8080, no auth)
starmap serve

# Start with custom port
starmap serve --port 3000

# Enable authentication
export API_KEY="your-secret-key"
starmap serve --auth

# Enable CORS for specific origins
starmap serve --cors-origins "https://example.com,https://app.example.com"

# Full configuration
starmap serve \
  --port 8080 \
  --host localhost \
  --cors \
  --auth \
  --rate-limit 100 \
  --cache-ttl 300
```

### Configuration Options

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--port` | `HTTP_PORT` | `8080` | Server port |
| `--host` | `HTTP_HOST` | `localhost` | Bind address |
| `--cors` | - | `false` | Enable CORS for all origins |
| `--cors-origins` | `CORS_ORIGINS` | - | Allowed CORS origins (comma-separated) |
| `--auth` | `ENABLE_AUTH` | `false` | Enable API key authentication |
| `--auth-header` | - | `X-API-Key` | Authentication header name |
| `--rate-limit` | `RATE_LIMIT_RPM` | `100` | Requests per minute per IP |
| `--cache-ttl` | `CACHE_TTL` | `300` | Cache TTL in seconds |
| `--read-timeout` | `READ_TIMEOUT` | `10s` | HTTP read timeout |
| `--write-timeout` | `WRITE_TIMEOUT` | `10s` | HTTP write timeout |
| `--idle-timeout` | `IDLE_TIMEOUT` | `120s` | HTTP idle timeout |

## Authentication

When authentication is enabled, all requests (except health endpoints) require an API key.

### API Key Header

```http
X-API-Key: your-secret-key
```

Or using the Authorization header:

```http
Authorization: Bearer your-secret-key
```

### Public Endpoints

The following endpoints are always publicly accessible:

- `GET /health`
- `GET /api/v1/health`
- `GET /api/v1/ready`

### Example

```bash
# With X-API-Key header
curl -H "X-API-Key: your-secret-key" \
  http://localhost:8080/api/v1/models

# With Authorization header
curl -H "Authorization: Bearer your-secret-key" \
  http://localhost:8080/api/v1/models
```

## Response Format

All API responses follow a consistent format:

### Success Response

```json
{
  "data": {
    // Response data here
  },
  "error": null
}
```

### Error Response

```json
{
  "data": null,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message",
    "details": "Additional error details"
  }
}
```

## Error Handling

### Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `BAD_REQUEST` | 400 | Invalid request format or parameters |
| `UNAUTHORIZED` | 401 | Invalid or missing API key |
| `NOT_FOUND` | 404 | Resource not found |
| `METHOD_NOT_ALLOWED` | 405 | HTTP method not supported |
| `RATE_LIMITED` | 429 | Rate limit exceeded |
| `INTERNAL_ERROR` | 500 | Internal server error |
| `SERVICE_UNAVAILABLE` | 503 | Service temporarily unavailable |

### Example Error Response

```json
{
  "data": null,
  "error": {
    "code": "NOT_FOUND",
    "message": "Model not found",
    "details": "No model with ID 'gpt-5' exists"
  }
}
```

## Endpoints

### Models

#### List Models

```http
GET /api/v1/models
```

List all models with optional filtering.

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Filter by exact model ID |
| `name` | string | Filter by exact model name (case-insensitive) |
| `name_contains` | string | Filter by partial model name match |
| `provider` | string | Filter by provider ID |
| `modality_input` | string | Filter by input modality (comma-separated) |
| `modality_output` | string | Filter by output modality (comma-separated) |
| `feature` | string | Filter by feature (streaming, tool_calls, etc.) |
| `tag` | string | Filter by tag (comma-separated) |
| `open_weights` | boolean | Filter by open weights status |
| `min_context` | integer | Minimum context window size |
| `max_context` | integer | Maximum context window size |
| `sort` | string | Sort field (id, name, release_date, context_window) |
| `order` | string | Sort order (asc, desc) |
| `limit` | integer | Maximum results (default: 100, max: 1000) |
| `offset` | integer | Result offset for pagination |

**Example Request:**

```bash
curl "http://localhost:8080/api/v1/models?provider=openai&feature=tool_calls&limit=10"
```

**Example Response:**

```json
{
  "data": {
    "models": [
      {
        "id": "gpt-4",
        "name": "GPT-4",
        "description": "Large multimodal model",
        "features": {
          "modalities": {
            "input": ["text", "image"],
            "output": ["text"]
          },
          "tool_calls": true,
          "streaming": true
        },
        "limits": {
          "context_window": 128000,
          "output_tokens": 16384
        }
      }
    ],
    "pagination": {
      "total": 1,
      "limit": 10,
      "offset": 0,
      "count": 1
    }
  },
  "error": null
}
```

#### Get Model by ID

```http
GET /api/v1/models/{id}
```

Retrieve detailed information about a specific model.

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Model ID |

**Example Request:**

```bash
curl http://localhost:8080/api/v1/models/gpt-4
```

**Example Response:**

```json
{
  "data": {
    "id": "gpt-4",
    "name": "GPT-4",
    "authors": [
      {
        "name": "OpenAI",
        "url": "https://openai.com"
      }
    ],
    "description": "Large multimodal model with advanced reasoning",
    "metadata": {
      "release_date": "2023-03-14T00:00:00Z",
      "open_weights": false,
      "tags": ["chat", "vision"]
    },
    "features": {
      "modalities": {
        "input": ["text", "image"],
        "output": ["text"]
      },
      "tool_calls": true,
      "tools": true,
      "tool_choice": true,
      "streaming": true
    },
    "limits": {
      "context_window": 128000,
      "output_tokens": 16384
    },
    "pricing": {
      "tokens": {
        "input": {
          "per_1m": 30.0
        },
        "output": {
          "per_1m": 60.0
        }
      }
    }
  },
  "error": null
}
```

#### Advanced Model Search

```http
POST /api/v1/models/search
```

Perform advanced search with multiple criteria.

**Request Body:**

```json
{
  "ids": ["gpt-4", "claude-3-opus"],
  "name_contains": "gpt",
  "provider": "openai",
  "modalities": {
    "input": ["text", "image"],
    "output": ["text"]
  },
  "features": {
    "streaming": true,
    "tool_calls": true
  },
  "tags": ["chat", "vision"],
  "open_weights": false,
  "context_window": {
    "min": 32000,
    "max": 200000
  },
  "output_tokens": {
    "min": 4000,
    "max": 16000
  },
  "release_date": {
    "after": "2024-01-01",
    "before": "2025-01-01"
  },
  "sort": "release_date",
  "order": "desc",
  "max_results": 100
}
```

**Example Request:**

```bash
curl -X POST http://localhost:8080/api/v1/models/search \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "openai",
    "features": {"tool_calls": true},
    "context_window": {"min": 32000}
  }'
```

**Example Response:**

```json
{
  "data": {
    "models": [...],
    "count": 5
  },
  "error": null
}
```

### Providers

#### List Providers

```http
GET /api/v1/providers
```

List all providers.

**Example Request:**

```bash
curl http://localhost:8080/api/v1/providers
```

**Example Response:**

```json
{
  "data": {
    "providers": [
      {
        "id": "openai",
        "name": "OpenAI",
        "model_count": 42,
        "headquarters": "San Francisco, CA",
        "docs_url": "https://platform.openai.com/docs"
      }
    ],
    "count": 1
  },
  "error": null
}
```

#### Get Provider by ID

```http
GET /api/v1/providers/{id}
```

Retrieve detailed information about a specific provider.

**Example Request:**

```bash
curl http://localhost:8080/api/v1/providers/openai
```

#### Get Provider Models

```http
GET /api/v1/providers/{id}/models
```

List all models for a specific provider.

**Example Request:**

```bash
curl http://localhost:8080/api/v1/providers/openai/models
```

**Example Response:**

```json
{
  "data": {
    "provider": {
      "id": "openai",
      "name": "OpenAI"
    },
    "models": [...],
    "count": 42
  },
  "error": null
}
```

### Administration

#### Trigger Catalog Update

```http
POST /api/v1/update
```

Manually trigger catalog synchronization.

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `provider` | string | Update specific provider only |

**Example Request:**

```bash
# Update all providers
curl -X POST http://localhost:8080/api/v1/update

# Update specific provider
curl -X POST "http://localhost:8080/api/v1/update?provider=openai"
```

**Example Response:**

```json
{
  "data": {
    "status": "completed",
    "total_changes": 5,
    "providers_changed": 1,
    "dry_run": false
  },
  "error": null
}
```

#### Get Catalog Statistics

```http
GET /api/v1/stats
```

Get catalog statistics.

**Example Response:**

```json
{
  "data": {
    "models": {
      "total": 250
    },
    "providers": {
      "total": 8
    },
    "cache": {
      "item_count": 42
    },
    "realtime": {
      "websocket_clients": 3,
      "sse_clients": 1
    }
  },
  "error": null
}
```

### Health & Metrics

#### Health Check

```http
GET /api/v1/health
GET /health
```

Health check endpoint (liveness probe).

**Example Response:**

```json
{
  "data": {
    "status": "healthy",
    "service": "starmap-api",
    "version": "v1"
  },
  "error": null
}
```

#### Readiness Check

```http
GET /api/v1/ready
```

Readiness check including cache and data source status.

**Example Response:**

```json
{
  "data": {
    "status": "ready",
    "cache": {
      "items": 42
    },
    "websocket_clients": 3,
    "sse_clients": 1
  },
  "error": null
}
```

#### Metrics

```http
GET /metrics
```

Prometheus-compatible metrics endpoint.

### Real-time Updates

#### WebSocket

```http
WS /api/v1/updates/ws
```

WebSocket connection for real-time catalog updates.

**Message Format:**

```json
{
  "type": "sync.completed",
  "timestamp": "2025-10-14T12:00:00Z",
  "data": {
    "total_changes": 5,
    "providers_changed": 1
  }
}
```

**Event Types:**

- `client.connected` - Client connected to stream
- `sync.started` - Catalog sync initiated
- `sync.completed` - Catalog sync finished
- `model.created` - New model added
- `model.updated` - Model modified
- `model.deleted` - Model removed

**Example (JavaScript):**

```javascript
const ws = new WebSocket('ws://localhost:8080/api/v1/updates/ws');

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  console.log('Event:', message.type, message.data);
};
```

#### Server-Sent Events (SSE)

```http
GET /api/v1/updates/stream
```

Server-Sent Events stream for catalog change notifications.

**Example (JavaScript):**

```javascript
const eventSource = new EventSource('http://localhost:8080/api/v1/updates/stream');

eventSource.addEventListener('sync.completed', (event) => {
  const data = JSON.parse(event.data);
  console.log('Sync completed:', data);
});

eventSource.addEventListener('connected', (event) => {
  console.log('Connected to updates stream');
});
```

## Filtering & Search

### Simple Filtering (GET)

Use query parameters for simple filtering:

```bash
# Filter by provider
curl "http://localhost:8080/api/v1/models?provider=openai"

# Multiple filters
curl "http://localhost:8080/api/v1/models?provider=openai&feature=tool_calls&min_context=32000"

# Modality filtering
curl "http://localhost:8080/api/v1/models?modality_input=text,image&modality_output=text"

# Tag filtering
curl "http://localhost:8080/api/v1/models?tag=chat,vision"
```

### Advanced Search (POST)

Use the search endpoint for complex queries:

```bash
curl -X POST http://localhost:8080/api/v1/models/search \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "openai",
    "features": {
      "tool_calls": true,
      "streaming": true
    },
    "context_window": {
      "min": 32000
    },
    "tags": ["chat"],
    "sort": "release_date",
    "order": "desc"
  }'
```

## Rate Limiting

The API enforces rate limiting per IP address.

**Default:** 100 requests per minute
**Header:** Rate limit info in response headers (future)

When rate limited, you'll receive a `429` response:

```json
{
  "data": null,
  "error": {
    "code": "RATE_LIMITED",
    "message": "Rate limit exceeded",
    "details": "Too many requests. Please try again later."
  }
}
```

## CORS

CORS can be configured via command-line flags:

```bash
# Enable CORS for all origins
starmap serve --cors

# Enable CORS for specific origins
starmap serve --cors-origins "https://example.com,https://app.example.com"
```

## Examples

### Complete Workflow

```bash
# 1. Start server
starmap serve --port 8080

# 2. Check health
curl http://localhost:8080/health

# 3. List all models
curl http://localhost:8080/api/v1/models

# 4. Search for specific models
curl -X POST http://localhost:8080/api/v1/models/search \
  -H "Content-Type: application/json" \
  -d '{"provider": "openai", "features": {"tool_calls": true}}'

# 5. Get specific model
curl http://localhost:8080/api/v1/models/gpt-4

# 6. Get provider info
curl http://localhost:8080/api/v1/providers/openai

# 7. Trigger catalog update
curl -X POST http://localhost:8080/api/v1/update

# 8. Check statistics
curl http://localhost:8080/api/v1/stats
```

### With Authentication

```bash
export API_KEY="your-secret-key"

# Start server with auth
starmap serve --auth

# Make authenticated request
curl -H "X-API-Key: $API_KEY" \
  http://localhost:8080/api/v1/models
```

### Real-time Updates

```javascript
// WebSocket example
const ws = new WebSocket('ws://localhost:8080/api/v1/updates/ws');

ws.onopen = () => console.log('Connected');
ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  if (msg.type === 'sync.completed') {
    console.log('Catalog updated:', msg.data.total_changes, 'changes');
  }
};

// SSE example
const eventSource = new EventSource('http://localhost:8080/api/v1/updates/stream');
eventSource.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Update:', data);
};
```

## Best Practices

1. **Use Caching**: Results are cached by default (5 min TTL)
2. **Filter Early**: Use query parameters to reduce response size
3. **Paginate**: Use `limit` and `offset` for large result sets
4. **Handle Errors**: Always check the `error` field in responses
5. **Rate Limits**: Implement client-side rate limiting
6. **Real-time**: Use WebSocket/SSE for live updates instead of polling
7. **Authentication**: Keep API keys secure, never commit to version control

## Support

For issues, questions, or feature requests, please visit:
- GitHub: https://github.com/agentstation/starmap
- Documentation: https://docs.starmap.dev (future)
