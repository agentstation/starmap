// Package status owns the observable vocabulary of a connected catalog
// runtime. It holds the status report, the health and freshness grades, the
// source kinds, and the sanitized source chain hop.
//
// The package is a leaf. It reads no catalog source and opens no connection.
// A server that renders runtime status therefore depends on this vocabulary
// alone. It never depends on the attested source machinery that the runtime
// package carries.
package status

import (
	"slices"
	"time"

	"github.com/agentstation/starmap/pkg/sources"
)

// SourceKind names one supported upstream catalog source. Selection is
// terminal: a deployment that names a custom source never falls back to the
// public GitHub channel.
type SourceKind string

const (
	// SourcePublic reads the attested public GitHub catalog channel of the
	// Starmap project. It is the default.
	SourcePublic SourceKind = "public"

	// SourceGitHub reads an attested catalog channel from a named repository.
	SourceGitHub SourceKind = "github"

	// SourceStarmap reads from another Starmap runtime. The cascade client is
	// an injected composition, because the root package holds no HTTP client.
	SourceStarmap SourceKind = "starmap"

	// SourceFile reads one immutable generation from a local path.
	SourceFile SourceKind = "file"

	// SourceEmbedded pins the runtime to the verified embedded bootstrap and
	// contacts no external system.
	SourceEmbedded SourceKind = "embedded"
)

// sourceKinds lists every accepted source name in declaration order.
var sourceKinds = []SourceKind{
	SourcePublic,
	SourceGitHub,
	SourceStarmap,
	SourceFile,
	SourceEmbedded,
}

// SourceKinds returns a caller-owned copy of every accepted source name.
func SourceKinds() []SourceKind { return slices.Clone(sourceKinds) }

// Valid reports whether the kind is one of the accepted source names.
func (k SourceKind) Valid() bool { return slices.Contains(sourceKinds, k) }

// String returns the wire value of the source kind.
func (k SourceKind) String() string { return string(k) }

// Custom reports whether the kind names a deployment-owned source. A custom
// source never falls back to the public channel.
func (k SourceKind) Custom() bool { return k.Valid() && k != SourcePublic }

// Freshness is the evaluated age of one observed timestamp.
type Freshness string

const (
	// FreshnessUnknown means no observation supports an evaluation yet.
	FreshnessUnknown Freshness = "unknown"

	// FreshnessCurrent means the age is inside every threshold.
	FreshnessCurrent Freshness = "current"

	// FreshnessWarn means the age passed the warning threshold.
	FreshnessWarn Freshness = "warn"

	// FreshnessCritical means the age passed the critical threshold.
	FreshnessCritical Freshness = "critical"
)

// String returns the wire value of the freshness level.
func (f Freshness) String() string { return string(f) }

// Health is the operator-facing state of one runtime component.
type Health string

const (
	// HealthUnknown means the component has not reported yet.
	HealthUnknown Health = "unknown"

	// HealthOK means the component reached its last objective.
	HealthOK Health = "ok"

	// HealthDegraded means the component works with reduced evidence.
	HealthDegraded Health = "degraded"

	// HealthUnavailable means the component cannot reach its dependency.
	HealthUnavailable Health = "unavailable"
)

// String returns the wire value of the health state.
func (h Health) String() string { return string(h) }

// SourceHop is one sanitized entry in an upstream source chain. A hop names
// the reporting identity and its health, never an address.
type SourceHop struct {
	Identity    string
	Health      Health
	PublishedAt time.Time
	ObservedAt  time.Time
}

// Status is the operator-facing state of one connected runtime. It keeps
// usability, freshness, fallback, direct source health, and upstream-reported
// health as five independent values, so a warning on one never hides another.
type Status struct {
	// Usable reports whether the runtime serves a catalog now. A runtime that
	// serves the verified embedded catalog is usable.
	Usable bool

	// GenerationID identifies the served catalog generation.
	GenerationID string

	// PayloadChecksum is the digest of the served catalog payload.
	PayloadChecksum string

	// CatalogAge is the age of the served generation.
	CatalogAge time.Duration

	// Freshness grades the age of the served generation.
	Freshness Freshness

	// ChannelUpdatedAt is the origin publication time that the upstream chain
	// propagated. Every hop carries the same value, so a downstream grades the
	// age of the origin channel and not only its own check.
	ChannelUpdatedAt time.Time

	// ChannelAge is the age of the propagated origin publication time.
	ChannelAge time.Duration

	// ChannelFreshness grades the age of the propagated origin publication
	// time. A cascade that stalls at any hop degrades every hop below it.
	ChannelFreshness Freshness

	// SourceCheckAge is the age of the last upstream check.
	SourceCheckAge time.Duration

	// SourceCheckFreshness grades the age of the last upstream check.
	SourceCheckFreshness Freshness

	// AcquisitionAge is the age of the last acquisition success.
	AcquisitionAge time.Duration

	// AcquisitionFreshness grades the age of the last acquisition success.
	AcquisitionFreshness Freshness

	// Fallback reports whether the runtime serves the embedded catalog because
	// no upstream generation is active.
	Fallback bool

	// FallbackReason names why the runtime fell back.
	FallbackReason string

	// SourceHealth is what this runtime observed while it read its own source.
	SourceHealth Health

	// SourceReason is the safe reason code of the last source failure.
	SourceReason string

	// UpstreamHealth is the health the upstream reported about itself. It stays
	// independent of SourceHealth, so a healthy transfer of a degraded upstream
	// catalog still reports the degradation.
	UpstreamHealth Health

	// AcquisitionHealth is the state of the last provider acquisition run.
	AcquisitionHealth Health

	// InstanceIdentity is the stable identity of this runtime inside a fleet.
	// A downstream compares it against a served source chain, so a cascade
	// rejects a self reference that URL comparison cannot detect.
	InstanceIdentity string

	// SourceIdentity is the safe identity of the selected source.
	SourceIdentity string

	// SourceKind names the selected source.
	SourceKind SourceKind

	// Chain is the sanitized upstream source chain, nearest hop first.
	Chain []SourceHop

	// Lease reports the runtime lease state.
	Lease string

	// LastRunID identifies the last refresh run.
	LastRunID string

	// Providers holds one terminal attempt per provider of the last run.
	Providers []sources.ProviderAttempt

	// StartedAt is when the runtime opened.
	StartedAt time.Time

	// ObservedAt is when the runtime built this report.
	ObservedAt time.Time
}
