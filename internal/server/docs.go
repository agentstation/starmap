// Package server provides HTTP server implementation for the Starmap API.
//
// This file contains general API documentation annotations for Swag/OpenAPI generation.
// These annotations describe the overall API (title, version, security, etc.)
// while individual endpoint annotations live in the handler files.
package server

// @title Starmap API
// @version 1.0
// @description REST API for the Starmap AI model catalog with real-time updates via WebSocket and SSE.
// @description
// @description Features:
// @description - Comprehensive model and provider queries
// @description - Advanced filtering and search
// @description - Real-time updates via WebSocket and Server-Sent Events
// @description - In-memory caching for performance
// @description - Rate limiting and authentication support
//
// @contact.name Starmap Project
// @contact.url https://github.com/agentstation/starmap
//
// @license.name MIT
// @license.url https://github.com/agentstation/starmap/blob/master/LICENSE
//
// @host localhost:8080
// @BasePath /api/v1
//
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
// @description API key for authentication (optional, configurable)
