package handlers

import (
	"net/http"

	"github.com/agentstation/starmap/internal/server/response"
	"github.com/agentstation/starmap/pkg/catalogremote"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// HandleCatalogManifest serves the current strict generation manifest.
func (h *Handlers) HandleCatalogManifest(writer http.ResponseWriter, request *http.Request) {
	client, err := h.app.Starmap()
	if err != nil {
		response.InternalError(writer, err)
		return
	}
	generation, err := client.CurrentGeneration(request.Context())
	if err != nil {
		response.InternalError(writer, err)
		return
	}
	data, err := catalogremote.MarshalManifest(generation.Manifest)
	if err != nil {
		response.InternalError(writer, err)
		return
	}
	writer.Header().Set("Content-Type", catalogremote.ManifestMediaType)
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("X-Starmap-Generation-ID", generation.Manifest.GenerationID)
	_, _ = writer.Write(data)
}

// HandleCatalogSnapshot serves an immutable canonical payload by generation ID.
func (h *Handlers) HandleCatalogSnapshot(writer http.ResponseWriter, request *http.Request, generationID string) {
	client, err := h.app.Starmap()
	if err != nil {
		response.InternalError(writer, err)
		return
	}
	generation, err := client.Generation(request.Context(), generationID)
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	writer.Header().Set("Content-Type", catalogs.CatalogPayloadMediaType)
	writer.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	writer.Header().Set("X-Starmap-Generation-ID", generation.Manifest.GenerationID)
	_, _ = writer.Write(generation.Payload)
}
