// Package handlers provides HTTP request handlers for the Starmap API.
//
// Handlers are organized by domain for maintainability:
//
//   - models.go: Model listing, retrieval, and search
//   - providers.go: Provider listing, retrieval, and models
//   - admin.go: Administrative operations (update, stats)
//   - health.go: Health and readiness checks
//   - realtime.go: WebSocket and SSE real-time updates
//   - openapi.go: OpenAPI 3.1 specification endpoints
//
// All handlers follow a consistent pattern:
//
//  1. Validate input
//  2. Check cache (if applicable)
//  3. Query catalog/data source
//  4. Transform data
//  5. Cache result (if applicable)
//  6. Return response
//
// Handlers use dependency injection for testability and receive all
// dependencies through the Handlers struct.
package handlers

//go:generate gomarkdoc --output README.md .
