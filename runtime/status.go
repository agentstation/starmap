package runtime

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

// Status returns the current runtime status. It reads retained state only, so
// it reaches no external system and never blocks on the source.
func (r *Runtime) Status() Status {
	if r == nil {
		return Status{}
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

	report := Status{
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
		report.SourceIdentity = r.source.Identity()
	}

	policy := r.config.freshness
	report.CatalogAge, report.Freshness = gradeAge(
		now, effective.GeneratedAt, policy.ChannelWarnAge, policy.ChannelCriticalAge)
	// A hop grades the propagated origin time, so a stall above it degrades it
	// too. Without a source layer the served catalog is the local baseline, and
	// its own generation time is the only channel time.
	channelTime := channelUpdatedAt
	if channelTime.IsZero() {
		channelTime = effective.GeneratedAt
	}
	report.ChannelUpdatedAt = channelTime
	report.ChannelAge, report.ChannelFreshness = gradeAge(
		now, channelTime, policy.ChannelWarnAge, policy.ChannelCriticalAge)
	report.SourceCheckAge, report.SourceCheckFreshness = gradeAge(
		now, state.sourceCheckedAt, policy.SourceCheckWarnAge, policy.SourceCheckCriticalAge)
	report.AcquisitionAge, report.AcquisitionFreshness = gradeAge(
		now, state.acquisitionSucceededAt, policy.AcquisitionWarnAge, policy.AcquisitionCriticalAge)

	// Fallback stays independent of health and freshness. A runtime can serve a
	// fresh embedded catalog while its source is unavailable, and it can serve a
	// stale upstream catalog while every read succeeds.
	switch {
	case hasSource:
		report.Fallback = false
		report.FallbackReason = FallbackNone
	case state.sourceCheckedAt.IsZero():
		report.Fallback = true
		report.FallbackReason = FallbackAwaitingSource
	default:
		report.Fallback = true
		report.FallbackReason = FallbackSourceUnavailable
	}
	return report
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
