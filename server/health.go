package server

import "time"

// State is the embeddable server lifecycle state.
type State string

const (
	// StateIdle means construction succeeded but Start or Serve has not run.
	StateIdle State = "idle"
	// StateServing means server-owned services are active.
	StateServing State = "serving"
	// StateStopped means Shutdown completed and streaming is unavailable.
	StateStopped State = "stopped"
)

// StreamState is the server-side SSE publication stream state.
type StreamState string

const (
	// StreamStateIdle means the broadcaster accepts streams but has no clients.
	StreamStateIdle StreamState = "idle"
	// StreamStateStreaming means at least one SSE client is connected.
	StreamStateStreaming StreamState = "streaming"
	// StreamStateStopped means the broadcaster rejects new streams.
	StreamStateStopped StreamState = "stopped"
)

// Health is an immutable snapshot of publisher catalog, callback, and stream
// delivery health. Catalog freshness is derived only from the active
// generation timestamp; heartbeat activity cannot refresh it.
type Health struct {
	State              State             `json:"state"`
	ActiveGenerationID string            `json:"active_generation_id,omitempty"`
	CatalogGeneratedAt time.Time         `json:"catalog_generated_at"`
	CatalogAgeSeconds  int64             `json:"catalog_age_seconds"`
	Publication        PublicationHealth `json:"publication"`
	Stream             StreamHealth      `json:"stream"`
}

// PublicationHealth reports post-commit callback delivery, including every
// pending generation coalesced by the bounded callback dispatcher.
type PublicationHealth struct {
	Completed   uint64        `json:"completed"`
	Failures    uint64        `json:"failures"`
	Panics      uint64        `json:"panics"`
	Coalesced   uint64        `json:"coalesced"`
	LastLatency time.Duration `json:"last_latency"`
	MaxLatency  time.Duration `json:"max_latency"`
}

// StreamHealth reports SSE liveness and delivery. BackpressureTerminated and
// Failed make every forced connection recovery observable.
type StreamHealth struct {
	State                  StreamState `json:"state"`
	Clients                int         `json:"clients"`
	LastHeartbeatAt        time.Time   `json:"last_heartbeat_at"`
	LastEventAt            time.Time   `json:"last_event_at"`
	LastGenerationID       string      `json:"last_generation_id,omitempty"`
	LastSequence           uint64      `json:"last_sequence"`
	LastErrorKind          string      `json:"last_error_kind,omitempty"`
	LastErrorAt            time.Time   `json:"last_error_at"`
	Published              uint64      `json:"published"`
	Sent                   uint64      `json:"sent"`
	Heartbeats             uint64      `json:"heartbeats"`
	Disconnected           uint64      `json:"disconnected"`
	BackpressureTerminated uint64      `json:"backpressure_terminated"`
	Failed                 uint64      `json:"failed"`
}

// Health returns current server health without performing I/O.
func (s *Server) Health() Health {
	if s == nil || s.implementation == nil {
		return Health{State: StateStopped}
	}
	health := s.implementation.OperationalHealth()
	return Health{
		State:              State(health.State),
		ActiveGenerationID: health.ActiveGenerationID,
		CatalogGeneratedAt: health.CatalogGeneratedAt,
		CatalogAgeSeconds:  health.CatalogAgeSeconds,
		Publication: PublicationHealth{
			Completed:   health.Publication.Completed,
			Failures:    health.Publication.Failures,
			Panics:      health.Publication.Panics,
			Coalesced:   health.Publication.Coalesced,
			LastLatency: health.Publication.LastLatency,
			MaxLatency:  health.Publication.MaxLatency,
		},
		Stream: StreamHealth{
			State:                  StreamState(health.Stream.State),
			Clients:                health.Stream.Clients,
			LastHeartbeatAt:        health.Stream.LastHeartbeatAt,
			LastEventAt:            health.Stream.LastEventAt,
			LastGenerationID:       health.Stream.LastGenerationID,
			LastSequence:           health.Stream.LastSequence,
			LastErrorKind:          health.Stream.LastErrorKind,
			LastErrorAt:            health.Stream.LastErrorAt,
			Published:              health.Stream.Published,
			Sent:                   health.Stream.Sent,
			Heartbeats:             health.Stream.Heartbeats,
			Disconnected:           health.Stream.Disconnected,
			BackpressureTerminated: health.Stream.BackpressureTerminated,
			Failed:                 health.Stream.Failed,
		},
	}
}
