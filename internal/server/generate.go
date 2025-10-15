// Package server provides HTTP server implementation for the Starmap API.
//
// The server package implements a clean, layered architecture following Go best practices:
//
//   - Server: Core server struct with lifecycle management
//   - Config: Server configuration with sensible defaults
//   - Router: Route registration and middleware chain
//   - Handlers: HTTP request handlers organized by domain
//
// The architecture follows the pattern: CLI → App → Server → Router → Handlers
//
// Usage:
//
//	cfg := server.DefaultConfig()
//	cfg.Port = 8080
//
//	srv, err := server.New(app, cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	srv.Start() // Start background services
//	http.ListenAndServe(":8080", srv.Handler())
package server

//go:generate gomarkdoc --output README.md .
