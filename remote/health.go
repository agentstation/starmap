package remote

import (
	"context"
	stderrors "errors"
	"net/http"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

// StreamState is the subscriber's current reactive transport state.
type StreamState string

const (
	// StreamStateIdle means Start has not established a lifecycle.
	StreamStateIdle StreamState = "idle"
	// StreamStateStarting means initial verification or stream setup is active.
	StreamStateStarting StreamState = "starting"
	// StreamStateStreaming means an SSE stream is established and caught up.
	StreamStateStreaming StreamState = "streaming"
	// StreamStateRetrying means the subscriber is recovering a failed stream.
	StreamStateRetrying StreamState = "retrying"
	// StreamStatePolling means explicit conditional fallback polling is active.
	StreamStatePolling StreamState = "polling"
	// StreamStateStopped means the one-shot lifecycle has ended.
	StreamStateStopped StreamState = "stopped"
)

// HealthError is a secret-free classification of the latest subscriber error.
// It deliberately excludes endpoint URLs, response bodies, and wrapped error
// text because those values can contain credentials or publisher details.
type HealthError struct {
	Operation  string    `json:"operation"`
	Kind       string    `json:"kind"`
	StatusCode int       `json:"status_code,omitempty"`
	Terminal   bool      `json:"terminal"`
	OccurredAt time.Time `json:"occurred_at"`
}

// Health is an immutable snapshot of subscriber transport and catalog health.
// Stream activity and catalog freshness are independent: heartbeats never
// change CatalogGeneratedAt or CatalogAgeSeconds.
type Health struct {
	StreamState             StreamState           `json:"stream_state"`
	ActiveGenerationID      string                `json:"active_generation_id,omitempty"`
	CatalogGeneratedAt      time.Time             `json:"catalog_generated_at"`
	CatalogAgeSeconds       int64                 `json:"catalog_age_seconds"`
	LastHeartbeatAt         time.Time             `json:"last_heartbeat_at"`
	LastEventAt             time.Time             `json:"last_event_at"`
	LastSuccessfulCatchUpAt time.Time             `json:"last_successful_catch_up_at"`
	Retries                 uint64                `json:"retries"`
	LastError               *HealthError          `json:"last_error,omitempty"`
	PollingFallback         PollingFallbackStatus `json:"polling_fallback"`
}

// Health returns the current subscriber health without performing I/O.
func (s *Subscriber) Health() Health {
	if s == nil {
		return Health{StreamState: StreamStateStopped}
	}
	now := s.currentTime()
	s.activationMu.Lock()
	s.mu.Lock()
	health := Health{
		StreamState:             s.streamState,
		ActiveGenerationID:      s.active.id,
		CatalogGeneratedAt:      s.active.generatedAt,
		LastHeartbeatAt:         s.lastHeartbeatAt,
		LastEventAt:             s.lastEventAt,
		LastSuccessfulCatchUpAt: s.lastCatchUpAt,
		Retries:                 s.retries,
		PollingFallback:         s.fallback,
	}
	if s.lastError != nil {
		lastError := *s.lastError
		health.LastError = &lastError
	}
	s.mu.Unlock()
	s.activationMu.Unlock()
	if !health.CatalogGeneratedAt.IsZero() {
		health.CatalogAgeSeconds = int64(now.Sub(health.CatalogGeneratedAt) / time.Second)
	}
	return health
}

func (s *Subscriber) currentTime() time.Time {
	if s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

func (s *Subscriber) setStreamState(state StreamState) {
	s.mu.Lock()
	s.streamState = state
	s.mu.Unlock()
}

func (s *Subscriber) recordHeartbeat() {
	s.mu.Lock()
	s.lastHeartbeatAt = s.currentTime()
	s.mu.Unlock()
}

func (s *Subscriber) recordPublicationEvent() {
	s.mu.Lock()
	s.lastEventAt = s.currentTime()
	s.mu.Unlock()
}

func (s *Subscriber) recordCatchUp() {
	s.mu.Lock()
	s.lastCatchUpAt = s.currentTime()
	s.mu.Unlock()
}

func (s *Subscriber) recordRetry() {
	s.mu.Lock()
	s.retries++
	s.mu.Unlock()
}

func (s *Subscriber) recordHealthError(operation string, err error) {
	if err == nil {
		return
	}
	healthError := classifyHealthError(operation, err, s.currentTime())
	s.mu.Lock()
	s.lastError = &healthError
	s.mu.Unlock()
}

func classifyHealthError(operation string, err error, occurredAt time.Time) HealthError {
	healthError := HealthError{
		Operation:  operation,
		Kind:       "transport",
		OccurredAt: occurredAt,
	}
	var apiErr *errors.APIError
	var validationErr *errors.ValidationError
	switch {
	case stderrors.As(err, &apiErr) && apiErr.StatusCode != 0:
		healthError.Kind = "http"
		healthError.StatusCode = apiErr.StatusCode
		healthError.Terminal = apiErr.StatusCode == http.StatusUnauthorized ||
			apiErr.StatusCode == http.StatusForbidden
	case stderrors.Is(err, context.Canceled):
		healthError.Kind = "canceled"
	case stderrors.Is(err, context.DeadlineExceeded):
		healthError.Kind = "timeout"
	case stderrors.As(err, &validationErr):
		healthError.Kind = "validation"
	case stderrors.Is(err, errors.ErrConflict):
		healthError.Kind = "conflict"
	case stderrors.As(err, &apiErr):
		healthError.Kind = "transport"
	}
	return healthError
}
