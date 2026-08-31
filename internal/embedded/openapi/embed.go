// Package openapi embeds the generated OpenAPI specifications.
package openapi

import _ "embed"

// SpecJSON contains the OpenAPI 3.0 specification in JSON format.
// Served at: GET /api/v1/openapi.json
//
//go:embed openapi.json
var SpecJSON []byte

// SpecYAML contains the OpenAPI 3.0 specification in YAML format.
// Served at: GET /api/v1/openapi.yaml
//
//go:embed openapi.yaml
var SpecYAML []byte
