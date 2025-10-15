// Package events provides a unified event system for real-time catalog updates.
//
// This package implements a broker pattern that connects Starmap's hooks system
// to multiple transport mechanisms (WebSocket, SSE, etc.) through a common event
// pipeline. This eliminates code duplication and provides a single point for
// event distribution.
package events

import "time"

// EventType represents the type of catalog event.
type EventType string

// Event types for catalog changes.
const (
	// Model events (from Starmap hooks).
	ModelAdded   EventType = "model.added"
	ModelUpdated EventType = "model.updated"
	ModelDeleted EventType = "model.deleted"

	// Sync events (from sync operations).
	SyncStarted   EventType = "sync.started"
	SyncCompleted EventType = "sync.completed"

	// Client events (from transport layers).
	ClientConnected EventType = "client.connected"
)

// Event represents a catalog event with type, timestamp, and data.
type Event struct {
	Type      EventType `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data"`
}
