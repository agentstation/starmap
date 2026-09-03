package server

import (
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/server/sse"
	"github.com/agentstation/starmap/runtime/status"
)

// OperationalHealth is the internal server's immutable production health.
type OperationalHealth struct {
	State              string
	ActiveGenerationID string
	CatalogGeneratedAt time.Time
	CatalogAgeSeconds  int64
	Publication        PublicationHealth
	Stream             StreamHealth
	Source             SourceHealth
}

// SourceHealth reports the runtime source of this node. Direct health and
// upstream-reported health stay independent values. A healthy transfer of a
// degraded upstream catalog is still a degraded catalog. A stalled transfer of
// a healthy upstream catalog is still a stalled node.
type SourceHealth struct {
	// Identity is the stable identity of this node inside a fleet.
	Identity string

	// SourceIdentity is the safe identity of the selected source.
	SourceIdentity string

	// Kind names the selected source.
	Kind string

	// Direct is what this node observed while it read its own source.
	Direct string

	// Upstream is the health the upstream reported about itself.
	Upstream string

	// Reason is the safe reason code of the last source failure.
	Reason string

	// ChannelUpdatedAt is the origin publication time the chain propagated.
	ChannelUpdatedAt time.Time

	// ChannelAgeSeconds is the age of that propagated publication time.
	ChannelAgeSeconds int64

	// ChannelFreshness grades the propagated publication time.
	ChannelFreshness string

	// Hops counts the upstream nodes above this node.
	Hops int

	// Fallback reports whether this node serves its local baseline.
	Fallback bool
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
	MaxClients             int
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
	Refused                uint64
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
		Source:             sourceHealth(s.app.RuntimeStatus()),
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

// sourceHealth projects the runtime source status onto the server health
// report. It copies bounded codes and ages only, so the report carries no
// message text and no endpoint.
func sourceHealth(status status.Status) SourceHealth {
	return SourceHealth{
		Identity:          status.InstanceIdentity,
		SourceIdentity:    status.SourceIdentity,
		Kind:              string(status.SourceKind),
		Direct:            string(status.SourceHealth),
		Upstream:          string(status.UpstreamHealth),
		Reason:            status.SourceReason,
		ChannelUpdatedAt:  status.ChannelUpdatedAt,
		ChannelAgeSeconds: int64(status.ChannelAge / time.Second),
		ChannelFreshness:  string(status.ChannelFreshness),
		Hops:              len(status.Chain),
		Fallback:          status.Fallback,
	}
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
		MaxClients:             health.MaxClients,
		LastHeartbeatAt:        health.LastHeartbeatAt,
		LastEventAt:            health.LastEventAt,
		LastGenerationID:       health.LastGenerationID,
		LastSequence:           health.LastSequence,
		Published:              health.Delivery.Published,
		Sent:                   health.Delivery.Sent,
		Heartbeats:             health.Delivery.Heartbeats,
		Disconnected:           health.Delivery.Disconnected,
		Refused:                health.Delivery.Refused,
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
