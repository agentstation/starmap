package handlers

import (
	"net/http"
	"strings"

	"github.com/agentstation/starmap/internal/server/response"
	"github.com/agentstation/starmap/pkg/catalogremote"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
)

// HandleCatalogManifest serves the current strict generation manifest.
func (h *Handlers) HandleCatalogManifest(writer http.ResponseWriter, request *http.Request) {
	client, err := h.app.Starmap()
	if err != nil {
		response.InternalError(writer, h.logger, err)
		return
	}
	generation, err := client.CurrentGeneration(request.Context())
	if err != nil {
		response.InternalError(writer, h.logger, err)
		return
	}
	h.writeCatalogManifest(writer, request, generation)
}

// HandleCatalogGenerationManifest serves an immutable manifest by generation ID.
func (h *Handlers) HandleCatalogGenerationManifest(
	writer http.ResponseWriter,
	request *http.Request,
	generationID string,
) {
	client, err := h.app.Starmap()
	if err != nil {
		response.InternalError(writer, h.logger, err)
		return
	}
	generation, err := client.Generation(request.Context(), generationID)
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	h.writeCatalogManifest(writer, request, generation)
}

func (h *Handlers) writeCatalogManifest(
	writer http.ResponseWriter,
	request *http.Request,
	generation catalogstore.Generation,
) {
	data, err := catalogremote.MarshalManifest(generation.Manifest)
	if err != nil {
		response.InternalError(writer, h.logger, err)
		return
	}
	writer.Header().Set("Content-Type", catalogremote.ManifestMediaType)
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("X-Starmap-Generation-ID", generation.Manifest.GenerationID)
	etag := catalogremote.ManifestETag(generation.Manifest.GenerationID)
	writer.Header().Set("ETag", etag)
	if headerETagMatches(request.Header.Get("If-None-Match"), etag) {
		writer.WriteHeader(http.StatusNotModified)
		return
	}
	_, _ = writer.Write(data)
}

func headerETagMatches(value, etag string) bool {
	for candidate := range strings.SplitSeq(value, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || candidate == etag ||
			strings.TrimPrefix(candidate, "W/") == etag {
			return true
		}
	}
	return false
}

// HandleCatalogPayload serves an immutable canonical payload by generation ID.
func (h *Handlers) HandleCatalogPayload(writer http.ResponseWriter, request *http.Request, generationID string) {
	client, err := h.app.Starmap()
	if err != nil {
		response.InternalError(writer, h.logger, err)
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
