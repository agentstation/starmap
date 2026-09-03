// Package operations tracks the asynchronous administrative operations of the
// HTTP server. A caller starts one operation, reads its status later, and may
// cancel it. Every status field comes from a closed set or from the caller's
// own bounded detail, so a status is safe to log and to serialize.
package operations

import (
	"slices"
	"time"

	"github.com/agentstation/starmap/pkg/sources"
)

// Kind names one administrative operation class. The closed set bounds every
// metric label that carries a kind.
type Kind string

// KindCatalogUpdate is one catalog acquisition run.
const KindCatalogUpdate Kind = "catalog_update"

var kinds = []Kind{KindCatalogUpdate}

// Valid reports whether the kind belongs to the closed set.
func (k Kind) Valid() bool { return slices.Contains(kinds, k) }

// String returns the kind label.
func (k Kind) String() string { return string(k) }

// Kinds returns the closed operation kind set.
func Kinds() []Kind { return slices.Clone(kinds) }

// State names where one operation sits in its lifecycle. The closed set bounds
// every metric label that carries a state.
type State string

const (
	// StateAccepted means the server holds the operation and has not run it.
	StateAccepted State = "accepted"

	// StateRunning means the operation runs now.
	StateRunning State = "running"

	// StateSucceeded means the operation finished without an error.
	StateSucceeded State = "succeeded"

	// StateFailed means the operation stopped on an error.
	StateFailed State = "failed"

	// StateCanceled means a caller canceled the operation.
	StateCanceled State = "canceled"
)

var states = []State{
	StateAccepted,
	StateRunning,
	StateSucceeded,
	StateFailed,
	StateCanceled,
}

// Valid reports whether the state belongs to the closed set.
func (s State) Valid() bool { return slices.Contains(states, s) }

// String returns the state label.
func (s State) String() string { return string(s) }

// Terminal reports whether the state ends the operation lifecycle.
func (s State) Terminal() bool {
	switch s {
	case StateSucceeded, StateFailed, StateCanceled:
		return true
	case StateAccepted, StateRunning:
		return false
	}
	return false
}

// States returns the closed operation state set.
func States() []State { return slices.Clone(states) }

// Status is one operation snapshot. It carries a bounded reason code instead of
// provider message text, so a caller may log it and serialize it.
type Status struct {
	// ID is the opaque operation identity that a caller reads back.
	ID string `json:"id"`

	// Kind names the operation class.
	Kind Kind `json:"kind"`

	// State names the current lifecycle position.
	State State `json:"state"`

	// Reason holds the bounded failure cause. A successful operation and a
	// canceled operation both leave it empty.
	Reason sources.ProviderReason `json:"reason,omitempty"`

	// AcceptedAt records when the server accepted the operation.
	AcceptedAt time.Time `json:"accepted_at"`

	// StartedAt records when the operation began to run.
	StartedAt time.Time `json:"started_at,omitzero"`

	// CompletedAt records when the operation reached a terminal state.
	CompletedAt time.Time `json:"completed_at,omitzero"`

	// Detail holds the bounded result summary that the operation produced.
	Detail map[string]any `json:"detail,omitempty"`
}

// Copy returns a caller-owned snapshot. The caller may then read the detail map
// while the registry records a later transition.
func (s Status) Copy() Status {
	if s.Detail == nil {
		return s
	}
	detail := make(map[string]any, len(s.Detail))
	for key, value := range s.Detail {
		detail[key] = value
	}
	s.Detail = detail
	return s
}

// Sample is one bounded metric row. The kind, the state, and the reason all
// come from closed sets, so the metric cardinality stays bounded.
type Sample struct {
	// Kind names the operation class.
	Kind Kind

	// State names the lifecycle position that the total counts.
	State State

	// Reason holds the bounded failure cause of a failed row.
	Reason sources.ProviderReason

	// Total is the monotonic count of entries into the state.
	Total int
}
