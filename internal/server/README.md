# Starmap HTTP Server

> Production-ready HTTP server providing REST API, WebSocket, and Server-Sent Events (SSE) for real-time catalog access

## Overview

The Starmap HTTP server provides programmatic access to the AI model catalog through multiple interfaces:

- **REST API** - Standard HTTP endpoints for catalog operations
- **WebSocket** - Real-time bidirectional updates
- **Server-Sent Events (SSE)** - Server-to-client streaming updates
- **OpenAPI 3.1** - Complete API specification with interactive docs

## Quick Start

```bash
# Start server with defaults (port 8080)
starmap serve

# Custom configuration
starmap serve \
  --port 3000 \
  --cors \
  --auth \
  --rate-limit 100 \
  --cache-ttl 300
```

## Architecture

### Package Structure

```
internal/server/
├── server.go           # Server struct & lifecycle management
├── config.go           # Configuration management
├── router.go           # Route registration & middleware setup
├── docs.go             # OpenAPI documentation
├── generate.go         # Code generation directives
├── cache/              # In-memory caching layer
├── events/             # Unified event broker system
│   ├── broker.go       # Event distribution hub
│   └── adapters/       # SSE/WebSocket adapters
├── filter/             # Query filtering & pagination
├── handlers/           # HTTP request handlers
│   ├── models.go       # Model endpoints
│   ├── providers.go    # Provider endpoints
│   ├── admin.go        # Admin operations
│   ├── health.go       # Health checks
│   ├── realtime.go     # WebSocket/SSE handlers
│   └── openapi.go      # OpenAPI spec endpoints
├── middleware/         # HTTP middleware
│   ├── auth.go         # API key authentication
│   ├── cors.go         # CORS headers
│   ├── ratelimit.go    # Token bucket rate limiting
│   └── middleware.go   # Logging & recovery
├── response/           # Consistent response formatting
├── sse/                # Server-Sent Events broadcaster
└── websocket/          # WebSocket hub & client management
```

### Component Responsibilities

**Core Components:**
- `server.go` - HTTP server lifecycle (start, graceful shutdown)
- `config.go` - Server configuration with sensible defaults
- `router.go` - HTTP router setup with middleware chain

**Request Processing:**
- `handlers/` - Business logic for each endpoint
- `middleware/` - Request/response processing pipeline
- `response/` - Standardized API response format

**Real-time Updates:**
- `events/` - Event broker for catalog change notifications
- `events/adapters/` - Transport adapters (SSE, WebSocket)
- `sse/` - SSE broadcaster implementation
- `websocket/` - WebSocket hub and client management

**Supporting Features:**
- `cache/` - In-memory cache with TTL
- `filter/` - Query parsing and filtering

## Configuration

### Command-Line Flags

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

### Environment Variables

```bash
# Server configuration
HTTP_PORT=8080
HTTP_HOST=0.0.0.0
ENABLE_AUTH=true
STARMAP_API_KEY=your-secret-key

# CORS
CORS_ORIGINS="https://example.com,https://app.example.com"

# Rate limiting
RATE_LIMIT_RPM=100

# Cache
CACHE_TTL=300
```

## API Endpoints

### Models

```bash
GET  /api/v1/models              # List with filtering
GET  /api/v1/models/{id}         # Get specific model
POST /api/v1/models/search       # Advanced search
```

### Providers

```bash
GET  /api/v1/providers           # List providers
GET  /api/v1/providers/{id}      # Get specific provider
GET  /api/v1/providers/{id}/models  # Get provider's models
```

### Administration

```bash
POST /api/v1/update              # Trigger catalog sync
GET  /api/v1/stats               # Catalog statistics
```

### Health & Documentation

```bash
GET  /health                     # Liveness probe
GET  /api/v1/ready               # Readiness check
GET  /api/v1/openapi.json        # OpenAPI 3.1 spec (JSON)
GET  /api/v1/openapi.yaml        # OpenAPI 3.1 spec (YAML)
```

### Real-time Updates

```bash
WS   /api/v1/updates/ws          # WebSocket connection
GET  /api/v1/updates/stream      # Server-Sent Events
```

For complete API documentation, see [REST_API.md](../../REST_API.md).

## Features

### Authentication

Optional API key authentication with public/private path support:

```bash
# Enable authentication
export STARMAP_API_KEY=your-secret-key
starmap serve --auth

# Make authenticated request
curl -H "X-API-Key: your-secret-key" \
  http://localhost:8080/api/v1/models
```

**Public endpoints** (no auth required):
- `/health`
- `/api/v1/health`
- `/api/v1/ready`

### CORS

Flexible CORS configuration:

```bash
# Enable for all origins
starmap serve --cors

# Specific origins
starmap serve --cors-origins "https://example.com,https://app.example.com"
```

### Rate Limiting

Per-IP token bucket rate limiting:

- Default: 100 requests per minute
- Configurable via `--rate-limit` flag
- Returns `429 Too Many Requests` when exceeded
- Automatic cleanup of inactive visitors

### Caching

In-memory cache with TTL for improved performance:

- Default TTL: 5 minutes (300 seconds)
- Configurable via `--cache-ttl` flag
- Automatic expiration and cleanup
- Cache statistics in `/api/v1/stats`

### Real-time Updates

Two transport options for live catalog updates:

**WebSocket:**
```javascript
const ws = new WebSocket('ws://localhost:8080/api/v1/updates/ws');
ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  console.log('Event:', message.type, message.data);
};
```

**Server-Sent Events:**
```javascript
const eventSource = new EventSource('http://localhost:8080/api/v1/updates/stream');
eventSource.addEventListener('sync.completed', (event) => {
  const data = JSON.parse(event.data);
  console.log('Sync completed:', data);
});
```

**Event Types:**
- `client.connected` - Client connected
- `sync.started` - Catalog sync initiated
- `sync.completed` - Catalog sync finished
- `model.created` - New model added
- `model.updated` - Model modified
- `model.deleted` - Model removed

## Development

### Running Tests

```bash
# All server tests
go test ./internal/server/...

# With race detector
go test -race ./internal/server/...

# Specific package
go test -v ./internal/server/middleware
```

### Coverage

```bash
# Generate coverage report
go test -coverprofile=coverage.out ./internal/server/...
go tool cover -html=coverage.out
```

### Generating OpenAPI Docs

```bash
# Generate OpenAPI specs (requires swag v2)
make generate

# Or directly
swag init --dir internal/server --output internal/embedded/openapi --ot json,yaml
```

## Package Documentation

- [cache/](cache/) - In-memory caching
- [events/](events/) - Event broker system
- [events/adapters/](events/adapters/) - Event transport adapters
- [filter/](filter/) - Query filtering
- [handlers/](handlers/) - HTTP handlers
- [middleware/](middleware/) - HTTP middleware
- [response/](response/) - Response formatting
- [sse/](sse/) - Server-Sent Events
- [websocket/](websocket/) - WebSocket support

## References

- [REST_API.md](../../REST_API.md) - Complete REST API documentation
- [ARCHITECTURE.md](../../ARCHITECTURE.md) - Overall system architecture
- [OpenAPI Spec](internal/embedded/openapi/) - Interactive API documentation
