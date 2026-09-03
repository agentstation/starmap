package runtime

import "github.com/agentstation/starmap/runtime/status"

// The observable runtime vocabulary lives in the leaf package
// github.com/agentstation/starmap/runtime/status. A server renders that
// vocabulary without the attested source machinery of this package. These
// aliases keep every name valid under the runtime import path, so a caller
// selects the import that matches its own dependency budget.

// Status is the operator-facing state of one connected runtime.
type Status = status.Status

// SourceHop is one sanitized entry in an upstream source chain.
type SourceHop = status.SourceHop

// SourceKind names one supported upstream catalog source.
type SourceKind = status.SourceKind

// Freshness is the evaluated age of one observed timestamp.
type Freshness = status.Freshness

// Health is the operator-facing state of one runtime component.
type Health = status.Health

// Source kinds. Selection is terminal, so a deployment that names a custom
// source never falls back to the public GitHub channel.
const (
	// SourcePublic reads the attested public GitHub catalog channel.
	SourcePublic = status.SourcePublic

	// SourceGitHub reads an attested channel from a named repository.
	SourceGitHub = status.SourceGitHub

	// SourceStarmap reads from another Starmap runtime.
	SourceStarmap = status.SourceStarmap

	// SourceFile reads one immutable generation from a local path.
	SourceFile = status.SourceFile

	// SourceEmbedded pins the runtime to the verified embedded bootstrap.
	SourceEmbedded = status.SourceEmbedded
)

// Freshness grades of one observed timestamp.
const (
	// FreshnessUnknown means no observation supports an evaluation yet.
	FreshnessUnknown = status.FreshnessUnknown

	// FreshnessCurrent means the age is inside every threshold.
	FreshnessCurrent = status.FreshnessCurrent

	// FreshnessWarn means the age passed the warning threshold.
	FreshnessWarn = status.FreshnessWarn

	// FreshnessCritical means the age passed the critical threshold.
	FreshnessCritical = status.FreshnessCritical
)

// Health states of one runtime component.
const (
	// HealthUnknown means the component has not reported yet.
	HealthUnknown = status.HealthUnknown

	// HealthOK means the component reached its last objective.
	HealthOK = status.HealthOK

	// HealthDegraded means the component works with reduced evidence.
	HealthDegraded = status.HealthDegraded

	// HealthUnavailable means the component cannot reach its dependency.
	HealthUnavailable = status.HealthUnavailable
)

// SourceKinds returns a caller-owned copy of every accepted source name.
func SourceKinds() []SourceKind { return status.SourceKinds() }
