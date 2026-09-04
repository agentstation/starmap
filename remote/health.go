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
	// StreamStateStreaming means the subscriber receives publication events.
	StreamStateStreaming StreamState = "streaming"
	// StreamStateRetrying means the subscriber waits before another connection attempt.
	StreamStateRetrying StreamState = "retrying"
	// StreamStatePolling means explicit conditional fallback polling is active.
	StreamStatePolling StreamState = "polling"
	// StreamStateWaitingForCredentials means the publisher rejected the
	// credential and the subscriber waits for a replacement.
	StreamStateWaitingForCredentials StreamState = "waiting_for_credentials"
	// StreamStateStopped means the subscriber lifecycle ended.
	StreamStateStopped StreamState = "stopped"
)

// UpstreamReport is what the upstream disclosed about itself. It stays
// separate from the subscriber's own transport health, so a healthy transfer
// of a degraded upstream catalog still reports the degradation.
type UpstreamReport struct {
	// Identity is the safe identity of the serving upstream node.
	Identity string `json:"identity"`

	// Health is what the upstream observed while it read its own source.
	Health string `json:"health"`

	// UpstreamHealth is what the upstream's own upstream reported.
	UpstreamHealth string `json:"upstream_health"`

	// GenerationID identifies the generation the upstream serves.
	GenerationID string `json:"generation_id,omitempty"`

	// ChannelUpdatedAt is the propagated origin channel time.
	ChannelUpdatedAt time.Time `json:"channel_updated_at"`

	// Hops counts the upstream nodes above the serving node.
	Hops int `json:"hops"`

	// ObservedAt is when the subscriber read the disclosure.
	ObservedAt time.Time `json:"observed_at"`
}

// HealthError describes the latest subscriber error without secrets. It
// excludes endpoint URLs, response bodies, and wrapped error text. Those values
// can contain credentials or publisher details.
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
	RetryNotBefore          time.Time             `json:"retry_not_before,omitempty"`
	Upstream                *UpstreamReport       `json:"upstream,omitempty"`
}

// Health returns the current subscriber health without performing I/O.
func (s *Subscriber) Health() Health {
	if s == nil {
		return Health{StreamState: StreamStateStopped}
	}
	now := s.currentTime()
	state := s.State()
	s.mu.Lock()
	health := Health{
		StreamState:             s.streamState,
		ActiveGenerationID:      state.GenerationID,
		CatalogGeneratedAt:      state.GeneratedAt,
		LastHeartbeatAt:         s.lastHeartbeatAt,
		LastEventAt:             s.lastEventAt,
		LastSuccessfulCatchUpAt: s.lastCatchUpAt,
		Retries:                 s.retries,
		PollingFallback:         s.fallback,
		RetryNotBefore:          s.retryNotBefore,
	}
	if s.lastError != nil {
		lastError := *s.lastError
		health.LastError = &lastError
	}
	if s.upstream != nil {
		upstream := *s.upstream
		health.Upstream = &upstream
	}
	s.mu.Unlock()
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
	// A declared credential-change signal makes an authentication failure
	// recoverable, so health reports it as nonterminal.
	healthError.Terminal = healthError.Terminal && s.config.CredentialChanges == nil
	s.mu.Lock()
	s.lastError = &healthError
	s.mu.Unlock()
}

// publishRetryBoundary records the hard not-before boundary that a refusal
// declared, so health explains why the subscriber waits.
func (s *Subscriber) publishRetryBoundary(boundary time.Time) {
	s.mu.Lock()
	s.retryNotBefore = boundary
	s.mu.Unlock()
}

// recordUpstream stores the disclosure the upstream served about itself.
func (s *Subscriber) recordUpstream(report UpstreamReport) {
	s.mu.Lock()
	s.upstream = &report
	s.mu.Unlock()
}

// lastErrorWasAuthentication reports whether the publisher rejected the
// credential of the last failed request.
func (s *Subscriber) lastErrorWasAuthentication() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastError != nil && s.lastError.Kind == "http" &&
		(s.lastError.StatusCode == http.StatusUnauthorized ||
			s.lastError.StatusCode == http.StatusForbidden)
}

// clearAuthenticationFailure forgets an authentication failure after the
// caller supplied a new credential, so the next failure decides again.
func (s *Subscriber) clearAuthenticationFailure() {
	s.mu.Lock()
	if s.lastError != nil && s.lastError.Kind == "http" &&
		(s.lastError.StatusCode == http.StatusUnauthorized ||
			s.lastError.StatusCode == http.StatusForbidden) {
		s.lastError = nil
	}
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
