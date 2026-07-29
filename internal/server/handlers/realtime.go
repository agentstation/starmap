package handlers

import (
	"net/http"
)

// HandleSSE handles Server-Sent Events at /api/v1/updates/stream.
// @Summary SSE updates stream
// @Description Heartbeat-enabled stream of post-commit catalog publication hints
// @Tags updates
// @Produce text/event-stream
// @Success 200 "Event stream"
// @Router /api/v1/updates/stream [get].
func (h *Handlers) HandleSSE(w http.ResponseWriter, r *http.Request) {
	h.sseBroadcaster.ServeHTTP(w, r)
}
