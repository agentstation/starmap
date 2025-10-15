// Package sse provides Server-Sent Events support for real-time updates.
package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Broadcaster manages Server-Sent Events connections.
type Broadcaster struct {
	clients    map[chan Event]bool
	newClients chan chan Event
	closed     chan chan Event
	events     chan Event
	mu         sync.RWMutex
	logger     *zerolog.Logger
}

// NewBroadcaster creates a new SSE broadcaster.
func NewBroadcaster(logger *zerolog.Logger) *Broadcaster {
	return &Broadcaster{
		clients:    make(map[chan Event]bool),
		newClients: make(chan chan Event, 10), // Buffered to prevent blocking when clients connect before Run() starts
		closed:     make(chan chan Event, 10), // Buffered to prevent blocking during client cleanup
		events:     make(chan Event, 256),
		logger:     logger,
	}
}

// Run starts the broadcaster's main loop. Should be called in a goroutine.
// The broadcaster will run until the context is cancelled.
func (b *Broadcaster) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			// Graceful shutdown: close all client connections
			b.mu.Lock()
			for client := range b.clients {
				close(client)
			}
			b.clients = make(map[chan Event]bool)
			b.mu.Unlock()
			b.logger.Info().Msg("SSE broadcaster shut down")
			return

		case client := <-b.newClients:
			b.mu.Lock()
			b.clients[client] = true
			b.mu.Unlock()
			b.logger.Info().
				Int("total_clients", len(b.clients)).
				Msg("SSE client connected")

		case client := <-b.closed:
			b.mu.Lock()
			delete(b.clients, client)
			close(client)
			b.mu.Unlock()
			b.logger.Info().
				Int("total_clients", len(b.clients)).
				Msg("SSE client disconnected")

		case event := <-b.events:
			b.mu.RLock()
			for client := range b.clients {
				select {
				case client <- event:
				default:
					// Client buffer full, skip this event for this client
					b.logger.Warn().Msg("SSE client buffer full, event skipped")
				}
			}
			b.mu.RUnlock()
		}
	}
}

// Broadcast sends an event to all connected SSE clients.
func (b *Broadcaster) Broadcast(event Event) {
	select {
	case b.events <- event:
	default:
		b.logger.Warn().Msg("SSE broadcast channel full, event dropped")
	}
}

// ClientCount returns the number of connected SSE clients.
func (b *Broadcaster) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}

// ServeHTTP handles SSE connections.
func (b *Broadcaster) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create client channel
	client := make(chan Event, 256)

	// Register client
	b.newClients <- client

	// Ensure cleanup
	defer func() {
		b.closed <- client
	}()

	// Get flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send initial connection event
	initialEvent := Event{
		Event: "connected",
		Data: map[string]any{
			"message":   "Connected to Starmap updates stream",
			"timestamp": time.Now(),
		},
	}
	b.writeEvent(w, flusher, initialEvent)

	// Stream events
	for {
		select {
		case event := <-client:
			b.writeEvent(w, flusher, event)

		case <-r.Context().Done():
			return
		}
	}
}

// writeEvent writes an SSE event to the response writer.
func (b *Broadcaster) writeEvent(w http.ResponseWriter, flusher http.Flusher, event Event) {
	// Write event type if specified
	if event.Event != "" {
		_, _ = fmt.Fprintf(w, "event: %s\n", event.Event)
	}

	// Write event ID if specified
	if event.ID != "" {
		_, _ = fmt.Fprintf(w, "id: %s\n", event.ID)
	}

	// Write data as JSON
	data, err := json.Marshal(event.Data)
	if err != nil {
		b.logger.Error().Err(err).Msg("Failed to marshal SSE event data")
		return
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)

	// Flush the response
	flusher.Flush()
}

// Event represents an SSE event.
type Event struct {
	Event string `json:"event,omitempty"` // Event type (optional)
	ID    string `json:"id,omitempty"`    // Event ID (optional)
	Data  any    `json:"data"`            // Event data
}
