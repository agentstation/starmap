package starmap

import (
	"time"

	"github.com/agentstation/starmap/pkg/sources"
)

// Fallback reasons. They say why the runtime still serves the verified
// embedded catalog instead of an upstream generation.
const (
	// FallbackNone means an upstream generation is active.
	FallbackNone = ""

	// FallbackAwaitingSource means no upstream reply arrived yet.
	FallbackAwaitingSource = "awaiting_source"

	// FallbackSourceUnavailable means every source read failed so far.
	FallbackSourceUnavailable = "source_unavailable"
)

// statusState holds the observations that Status turns into an operator-facing
// report. Every field comes from runtime-owned work, so Status reaches no
// external system.
type statusState struct {
	startedAt time.Time

	sourceCheckedAt time.Time
	sourceChangedAt time.Time
	sourceHealth    Health
	sourceReason    string
	upstreamHealth  Health
	upstreamChain   []SourceHop

	acquisitionStartedAt   time.Time
	acquisitionSucceededAt time.Time
	acquisitionHealth      Health
	attempts               []sources.ProviderAttempt

	lastRunID string
}

// RuntimeStatus is the operator-facing state of one connected runtime. It keeps
// usability, freshness, fallback, direct source health, and upstream-reported
// health as five independent values, so a warning on one never hides another.
type RuntimeStatus struct {
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

// Status returns the current runtime status. It reads retained state only, so
// it reaches no external system and never blocks on the source.
func (r *Runtime) Status() RuntimeStatus {
	if r == nil {
		return RuntimeStatus{}
	}
	now := r.config.now()

	r.mu.RLock()
	state := r.report
	effective := r.effective
	hasSource := r.layers.source != nil
	var channelUpdatedAt time.Time
	if hasSource {
		channelUpdatedAt = r.layers.source.ChannelUpdatedAt
	}
	r.mu.RUnlock()

	status := RuntimeStatus{
		Usable:            effective.Catalog != nil,
		GenerationID:      effective.GenerationID,
		PayloadChecksum:   effective.PayloadChecksum,
		SourceHealth:      orUnknown(state.sourceHealth),
		SourceReason:      state.sourceReason,
		UpstreamHealth:    orUnknown(state.upstreamHealth),
		AcquisitionHealth: orUnknown(state.acquisitionHealth),
		SourceKind:        r.config.source.Kind,
		ChannelUpdatedAt:  channelUpdatedAt,
		InstanceIdentity:  r.schedule.identity.Instance,
		Chain:             append([]SourceHop(nil), state.upstreamChain...),
		Lease:             string(r.lease.status()),
		LastRunID:         state.lastRunID,
		Providers:         append([]sources.ProviderAttempt(nil), state.attempts...),
		StartedAt:         state.startedAt,
		ObservedAt:        now,
	}
	if r.source != nil {
		status.SourceIdentity = r.source.Identity()
	}

	policy := r.config.freshness
	status.CatalogAge, status.Freshness = gradeAge(
		now, effective.GeneratedAt, policy.ChannelWarnAge, policy.ChannelCriticalAge)
	// A hop grades the propagated origin time, so a stall above it degrades it
	// too. Without a source layer the served catalog is the local baseline, and
	// its own generation time is the only channel time.
	channelTime := channelUpdatedAt
	if channelTime.IsZero() {
		channelTime = effective.GeneratedAt
	}
	status.ChannelUpdatedAt = channelTime
	status.ChannelAge, status.ChannelFreshness = gradeAge(
		now, channelTime, policy.ChannelWarnAge, policy.ChannelCriticalAge)
	status.SourceCheckAge, status.SourceCheckFreshness = gradeAge(
		now, state.sourceCheckedAt, policy.SourceCheckWarnAge, policy.SourceCheckCriticalAge)
	status.AcquisitionAge, status.AcquisitionFreshness = gradeAge(
		now, state.acquisitionSucceededAt, policy.AcquisitionWarnAge, policy.AcquisitionCriticalAge)

	// Fallback stays independent of health and freshness. A runtime can serve a
	// fresh embedded catalog while its source is unavailable, and it can serve a
	// stale upstream catalog while every read succeeds.
	switch {
	case hasSource:
		status.Fallback = false
		status.FallbackReason = FallbackNone
	case state.sourceCheckedAt.IsZero():
		status.Fallback = true
		status.FallbackReason = FallbackAwaitingSource
	default:
		status.Fallback = true
		status.FallbackReason = FallbackSourceUnavailable
	}
	return status
}

// gradeAge returns the age of one observation and its freshness grade. A zero
// timestamp reports an unknown grade, never a critical one, because no
// observation supports a judgment yet.
func gradeAge(now, observed time.Time, warn, critical time.Duration) (time.Duration, Freshness) {
	if observed.IsZero() {
		return 0, FreshnessUnknown
	}
	age := now.Sub(observed)
	if age < 0 {
		age = 0
	}
	return age, evaluateFreshness(age, warn, critical)
}

// orUnknown returns the reported health, or unknown when nothing reported yet.
func orUnknown(health Health) Health {
	if health == "" {
		return HealthUnknown
	}
	return health
}
