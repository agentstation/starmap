package handlers

import (
	"fmt"
	"net/http"
	"time"

	ws "github.com/agentstation/starmap/internal/server/websocket"
)

// HandleWebSocket handles WebSocket connections at /api/v1/updates/ws.
// @Summary WebSocket updates
// @Description WebSocket connection for real-time catalog updates
// @Tags updates
// @Success 101 "Switching Protocols"
// @Router /api/v1/updates/ws [get].
func (h *Handlers) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error().Err(err).Msg("WebSocket upgrade failed")
		return
	}

	// Create and register client
	clientID := fmt.Sprintf("%s-%d", r.RemoteAddr, time.Now().Unix())
	client := ws.NewClient(clientID, h.wsHub, conn)

	// Register client with hub (this connects it to the event stream)
	h.wsHub.Register(client)

	// Start client pumps (read and write must run concurrently)
	go client.WritePump()
	go client.ReadPump()
}

// HandleSSE handles Server-Sent Events at /api/v1/updates/stream.
// @Summary SSE updates stream
// @Description Server-Sent Events stream for catalog change notifications
// @Tags updates
// @Produce text/event-stream
// @Success 200 "Event stream"
// @Router /api/v1/updates/stream [get].
func (h *Handlers) HandleSSE(w http.ResponseWriter, r *http.Request) {
	h.sseBroadcaster.ServeHTTP(w, r)
}
