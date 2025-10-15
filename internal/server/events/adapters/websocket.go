// Package adapters provides transport-specific implementations of the Subscriber interface.
package adapters

import (
	"github.com/agentstation/starmap/internal/server/events"
	ws "github.com/agentstation/starmap/internal/server/websocket"
)

// WebSocketSubscriber adapts the WebSocket hub to the Subscriber interface.
type WebSocketSubscriber struct {
	hub *ws.Hub
}

// NewWebSocketSubscriber creates a new WebSocket subscriber.
func NewWebSocketSubscriber(hub *ws.Hub) *WebSocketSubscriber {
	return &WebSocketSubscriber{hub: hub}
}

// Send delivers an event to all WebSocket clients.
func (w *WebSocketSubscriber) Send(event events.Event) error {
	w.hub.Broadcast(ws.Message{
		Type:      string(event.Type),
		Timestamp: event.Timestamp,
		Data:      event.Data,
	})
	return nil
}

// Close is a no-op for WebSocket (hub manages its own lifecycle).
func (w *WebSocketSubscriber) Close() error {
	return nil
}
