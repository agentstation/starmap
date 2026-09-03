package handlers

import (
	stderrors "errors"
	"net/http"
	"strings"
	"time"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/internal/server/response"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/remote"
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
	generation catalogs.Generation,
) {
	data, err := remote.MarshalManifest(generation.Manifest)
	if err != nil {
		response.InternalError(writer, h.logger, err)
		return
	}
	writer.Header().Set("Content-Type", remote.ManifestMediaType)
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("X-Starmap-Generation-ID", generation.Manifest.GenerationID)
	etag := remote.ManifestETag(generation.Manifest.GenerationID)
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
	h.streamCatalogPayload(writer, generation.Payload)
}

// streamCatalogPayload writes one payload in bounded chunks. It resets the
// write deadline before every chunk, so the deadline bounds one chunk and never
// bounds the transfer. A slow reader then keeps its download, and a stalled
// reader still releases the connection.
func (h *Handlers) streamCatalogPayload(writer http.ResponseWriter, payload []byte) {
	controller := http.NewResponseController(writer)
	for offset := 0; offset < len(payload); offset += constants.CatalogPayloadChunkBytes {
		end := min(offset+constants.CatalogPayloadChunkBytes, len(payload))
		err := controller.SetWriteDeadline(
			time.Now().Add(constants.CatalogPayloadChunkTimeout),
		)
		if err != nil && !stderrors.Is(err, http.ErrNotSupported) {
			h.log().Warn().Err(err).Msg("Catalog payload write deadline was refused")
			return
		}
		if _, err := writer.Write(payload[offset:end]); err != nil {
			return
		}
	}
}
