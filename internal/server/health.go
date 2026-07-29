package server

import (
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/server/sse"
)

// OperationalHealth is the internal server's immutable production health.
type OperationalHealth struct {
	State              string
	ActiveGenerationID string
	CatalogGeneratedAt time.Time
	CatalogAgeSeconds  int64
	Publication        PublicationHealth
	Stream             StreamHealth
}

// PublicationHealth reports post-commit callback delivery.
type PublicationHealth struct {
	Completed   uint64
	Failures    uint64
	Panics      uint64
	Coalesced   uint64
	LastLatency time.Duration
	MaxLatency  time.Duration
}

// StreamHealth reports server-side SSE delivery.
type StreamHealth struct {
	State                  string
	Clients                int
	LastHeartbeatAt        time.Time
	LastEventAt            time.Time
	LastGenerationID       string
	LastSequence           uint64
	LastErrorKind          string
	LastErrorAt            time.Time
	Published              uint64
	Sent                   uint64
	Heartbeats             uint64
	Disconnected           uint64
	BackpressureTerminated uint64
	Failed                 uint64
}

func (s *Server) operationalHealth() OperationalHealth {
	if s == nil || s.client == nil {
		return OperationalHealth{State: "stopped"}
	}
	state := s.client.CurrentCatalogState()
	now := s.currentTime()
	health := OperationalHealth{
		State:              "idle",
		ActiveGenerationID: state.GenerationID,
		CatalogGeneratedAt: state.GeneratedAt,
		Publication:        publicationHealth(s.client.HookStats()),
		Stream:             streamHealth(s.sseBroadcaster.Health()),
	}
	if s.started.Load() {
		health.State = "serving"
	}
	if s.stopped.Load() {
		health.State = "stopped"
	}
	if !state.GeneratedAt.IsZero() {
		health.CatalogAgeSeconds = int64(now.Sub(state.GeneratedAt) / time.Second)
	}
	return health
}

func publicationHealth(stats starmap.HookDeliveryStats) PublicationHealth {
	return PublicationHealth{
		Completed:   stats.Completed,
		Failures:    stats.Failures,
		Panics:      stats.Panics,
		Coalesced:   stats.Coalesced,
		LastLatency: stats.LastLatency,
		MaxLatency:  stats.MaxLatency,
	}
}

func streamHealth(health sse.Health) StreamHealth {
	result := StreamHealth{
		State:                  string(health.State),
		Clients:                health.Clients,
		LastHeartbeatAt:        health.LastHeartbeatAt,
		LastEventAt:            health.LastEventAt,
		LastGenerationID:       health.LastGenerationID,
		LastSequence:           health.LastSequence,
		Published:              health.Delivery.Published,
		Sent:                   health.Delivery.Sent,
		Heartbeats:             health.Delivery.Heartbeats,
		Disconnected:           health.Delivery.Disconnected,
		BackpressureTerminated: health.Delivery.BackpressureTerminated,
		Failed:                 health.Delivery.Failed,
	}
	if health.LastError != nil {
		result.LastErrorKind = health.LastError.Kind
		result.LastErrorAt = health.LastError.OccurredAt
	}
	return result
}

func (s *Server) currentTime() time.Time {
	if s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}
