package handlers

import (
	"net/http"

	"github.com/agentstation/starmap/internal/embedded/openapi"
)

// HandleOpenAPIJSON serves the embedded OpenAPI 3.1 specification in JSON format.
// @Summary Get OpenAPI specification (JSON)
// @Description Returns the OpenAPI 3.1 specification for this API in JSON format
// @Tags meta
// @Produce json
// @Success 200 {object} object "OpenAPI 3.1 specification"
// @Router /api/v1/openapi.json [get].
func (h *Handlers) HandleOpenAPIJSON(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
	_, _ = w.Write(openapi.SpecJSON)
}

// HandleOpenAPIYAML serves the embedded OpenAPI 3.1 specification in YAML format.
// @Summary Get OpenAPI specification (YAML)
// @Description Returns the OpenAPI 3.1 specification for this API in YAML format
// @Tags meta
// @Produce application/x-yaml
// @Success 200 {string} string "OpenAPI 3.1 specification"
// @Router /api/v1/openapi.yaml [get].
func (h *Handlers) HandleOpenAPIYAML(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/x-yaml")
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
	_, _ = w.Write(openapi.SpecYAML)
}
