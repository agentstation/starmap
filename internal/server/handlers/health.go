package handlers

import (
	"net/http"

	"github.com/agentstation/starmap/internal/server/response"
)

// HandleHealth handles GET /api/v1/health.
// @Summary Health check
// @Description Health check endpoint (liveness probe)
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=object}
// @Router /api/v1/health [get].
func (h *Handlers) HandleHealth(w http.ResponseWriter, _ *http.Request) {
	response.OK(w, map[string]any{
		"status":  "healthy",
		"service": "starmap-api",
		"version": "v1",
	})
}

// HandleReady handles GET /api/v1/ready.
// @Summary Readiness check
// @Description Readiness check including cache and data source status
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=object}
// @Failure 503 {object} response.Response{error=response.Error}
// @Router /api/v1/ready [get].
func (h *Handlers) HandleReady(w http.ResponseWriter, _ *http.Request) {
	// Check catalog availability
	_, err := h.app.Catalog()
	if err != nil {
		response.ServiceUnavailable(w, "Catalog not available")
		return
	}

	response.OK(w, map[string]any{
		"status": "ready",
		"cache": map[string]any{
			"items": h.cache.ItemCount(),
		},
		"websocket_clients": h.wsHub.ClientCount(),
		"sse_clients":       h.sseBroadcaster.ClientCount(),
	})
}
