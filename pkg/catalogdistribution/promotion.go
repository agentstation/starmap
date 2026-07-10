package catalogdistribution

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// DefaultMaxGenerationAge is the default stable-promotion freshness SLO.
	DefaultMaxGenerationAge = 7 * 24 * time.Hour
	// DefaultMaxProbeAge is the maximum age of hosted canary evidence used for
	// stable promotion.
	DefaultMaxProbeAge = 5 * time.Minute
	// DefaultMaxProbeLatency is the default hosted canary response SLO.
	DefaultMaxProbeLatency = 2 * time.Second
)

// Channel is a mutable hosted pointer with an explicit promotion role.
type Channel string

const (
	// ChannelDev receives a published generation first.
	ChannelDev Channel = "dev"
	// ChannelCanary receives only the generation currently selected in dev.
	ChannelCanary Channel = "canary"
	// ChannelStable receives only a canary generation with passing hosted SLO evidence.
	ChannelStable Channel = "stable"
)

// String returns the channel wire value.
func (c Channel) String() string { return string(c) }

// Validate verifies a supported promotion channel.
func (c Channel) Validate() error {
	switch c {
	case ChannelDev, ChannelCanary, ChannelStable:
		return nil
	default:
		return &errors.ValidationError{Field: "catalog_distribution.channel", Value: c, Message: "must be dev, canary, or stable"}
	}
}

// ParseChannel parses a channel query value. An empty value defaults to stable
// for backward-compatible consumer behavior.
func ParseChannel(value string) (Channel, error) {
	if strings.TrimSpace(value) == "" {
		return ChannelStable, nil
	}
	channel := Channel(strings.ToLower(strings.TrimSpace(value)))
	return channel, channel.Validate()
}

// PromotionPolicy defines freshness and availability evidence required before
// a canary generation may become stable.
type PromotionPolicy struct {
	MaxGenerationAge time.Duration
	MaxProbeAge      time.Duration
	MaxProbeLatency  time.Duration
}

// DefaultPromotionPolicy returns the production default hosted SLO policy.
func DefaultPromotionPolicy() PromotionPolicy {
	return PromotionPolicy{
		MaxGenerationAge: DefaultMaxGenerationAge,
		MaxProbeAge:      DefaultMaxProbeAge,
		MaxProbeLatency:  DefaultMaxProbeLatency,
	}
}

// Validate verifies positive promotion SLO budgets.
func (p PromotionPolicy) Validate() error {
	if p.MaxGenerationAge <= 0 || p.MaxProbeAge <= 0 || p.MaxProbeLatency <= 0 {
		return &errors.ValidationError{Field: "catalog_distribution.promotion_policy", Value: p, Message: "all SLO budgets must be positive"}
	}
	return nil
}

// PromotionProbe is hosted availability, freshness, latency, and identity
// evidence for one channel generation.
type PromotionProbe struct {
	Channel          Channel
	GenerationID     string
	ArtifactChecksum string
	ObservedAt       time.Time
	Latency          time.Duration
	Available        bool
	Fresh            bool
	Failure          string
}

// PromotionAction describes a pointer-control operation.
type PromotionAction string

const (
	// PromotionActionPromote advances a generation through a channel.
	PromotionActionPromote PromotionAction = "promote"
	// PromotionActionRollback returns a channel to a prior generation.
	PromotionActionRollback PromotionAction = "rollback"
)

// PromotionEvent is immutable queryable pointer-control telemetry.
type PromotionEvent struct {
	Sequence   uint64
	Action     PromotionAction
	Channel    Channel
	From       string
	To         string
	ObservedAt time.Time
	Success    bool
	Reason     string
}

// NewMemoryRepositoryWithPolicy creates an empty repository with explicit
// stable-promotion SLO budgets.
func NewMemoryRepositoryWithPolicy(policy PromotionPolicy) (*MemoryRepository, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return newMemoryRepository(policy), nil
}

func newMemoryRepository(policy PromotionPolicy) *MemoryRepository {
	return &MemoryRepository{
		items:    make(map[string]PublishedGeneration),
		channels: make(map[Channel]string),
		history: map[Channel]map[string]struct{}{
			ChannelDev: {}, ChannelCanary: {}, ChannelStable: {},
		},
		policy: policy,
		now:    time.Now,
	}
}

// Promote atomically advances a published generation through dev, canary, or
// stable. Canary requires the same dev selection. Stable additionally requires
// recent passing hosted canary evidence bound to the archive checksum.
func (r *MemoryRepository) Promote(channel Channel, generationID string, probe *PromotionProbe) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now().UTC()
	from := r.channels[channel]
	fail := func(err error) error {
		r.recordPromotionLocked(PromotionActionPromote, channel, from, generationID, now, false, err.Error())
		return err
	}
	if err := channel.Validate(); err != nil {
		return fail(err)
	}
	published, found := r.items[generationID]
	if !found {
		return fail(&errors.NotFoundError{Resource: "hosted catalog generation", ID: generationID})
	}
	if from == generationID {
		return nil
	}
	switch channel {
	case ChannelDev:
	case ChannelCanary:
		if r.channels[ChannelDev] != generationID {
			return fail(promotionOrderError(channel, generationID, ChannelDev))
		}
	case ChannelStable:
		if r.channels[ChannelCanary] != generationID {
			return fail(promotionOrderError(channel, generationID, ChannelCanary))
		}
		if err := r.validateStableProbeLocked(published, probe, now); err != nil {
			return fail(err)
		}
	}
	r.channels[channel] = generationID
	r.history[channel][generationID] = struct{}{}
	r.recordPromotionLocked(PromotionActionPromote, channel, from, generationID, now, true, "")
	return nil
}

// Rollback atomically moves a channel to a generation previously served by
// that same channel. The reason is mandatory operational telemetry.
func (r *MemoryRepository) Rollback(channel Channel, generationID, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now().UTC()
	from := r.channels[channel]
	fail := func(err error) error {
		r.recordPromotionLocked(PromotionActionRollback, channel, from, generationID, now, false, err.Error())
		return err
	}
	if err := channel.Validate(); err != nil {
		return fail(err)
	}
	if strings.TrimSpace(reason) == "" {
		return fail(&errors.ValidationError{Field: "catalog_distribution.rollback_reason", Message: "is required"})
	}
	if _, found := r.items[generationID]; !found {
		return fail(&errors.NotFoundError{Resource: "hosted catalog generation", ID: generationID})
	}
	if _, served := r.history[channel][generationID]; !served {
		return fail(&errors.ValidationError{Field: "catalog_distribution.rollback_generation", Value: generationID, Message: "was not previously served by this channel"})
	}
	if from == generationID {
		return nil
	}
	r.channels[channel] = generationID
	r.recordPromotionLocked(PromotionActionRollback, channel, from, generationID, now, true, strings.TrimSpace(reason))
	return nil
}

// PromotionEvents returns caller-owned promotion and rollback telemetry,
// including rejected attempts.
func (r *MemoryRepository) PromotionEvents() []PromotionEvent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]PromotionEvent(nil), r.events...)
}

// ProbeChannel fetches a hosted channel through the public HTTP protocol and
// returns evidence suitable for stable promotion under the supplied policy.
func (c *Client) ProbeChannel(ctx context.Context, channel Channel, policy PromotionPolicy, observedAt time.Time) PromotionProbe {
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	} else {
		observedAt = observedAt.UTC()
	}
	probe := PromotionProbe{Channel: channel, ObservedAt: observedAt}
	if err := policy.Validate(); err != nil {
		probe.Failure = err.Error()
		return probe
	}
	started := time.Now()
	generation, pointer, err := c.fetchChannel(ctx, channel)
	probe.Latency = time.Since(started)
	if err != nil {
		probe.Failure = err.Error()
		return probe
	}
	probe.Available = true
	probe.GenerationID = generation.Manifest.GenerationID
	probe.ArtifactChecksum = pointer.Artifact.Checksum
	age := observedAt.Sub(generation.Manifest.GeneratedAt)
	probe.Fresh = age >= 0 && age <= policy.MaxGenerationAge
	if !probe.Fresh {
		probe.Failure = fmt.Sprintf("generation age %s exceeds freshness policy %s or is in the future", age, policy.MaxGenerationAge)
	}
	if probe.Latency > policy.MaxProbeLatency {
		probe.Failure = fmt.Sprintf("probe latency %s exceeds policy %s", probe.Latency, policy.MaxProbeLatency)
	}
	return probe
}

func (r *MemoryRepository) validateStableProbeLocked(published PublishedGeneration, probe *PromotionProbe, now time.Time) error {
	if probe == nil {
		return &errors.ValidationError{Field: "catalog_distribution.stable_probe", Message: "passing canary evidence is required"}
	}
	if probe.Channel != ChannelCanary || probe.GenerationID != published.Generation.Manifest.GenerationID ||
		probe.ArtifactChecksum != published.Artifact.Checksum {
		return &errors.ValidationError{Field: "catalog_distribution.stable_probe", Value: probe.GenerationID, Message: "does not match the canary generation and archive"}
	}
	if !probe.Available || !probe.Fresh || probe.Failure != "" {
		return &errors.ValidationError{Field: "catalog_distribution.stable_probe", Value: probe.Failure, Message: "availability, freshness, and latency SLOs must pass"}
	}
	if probe.Latency < 0 || probe.Latency > r.policy.MaxProbeLatency {
		return &errors.ValidationError{Field: "catalog_distribution.probe_latency", Value: probe.Latency, Message: "exceeds stable promotion policy"}
	}
	age := now.Sub(probe.ObservedAt)
	if probe.ObservedAt.IsZero() || age < 0 || age > r.policy.MaxProbeAge {
		return &errors.ValidationError{Field: "catalog_distribution.probe_age", Value: age, Message: "probe is stale or from the future"}
	}
	generationAge := probe.ObservedAt.Sub(published.Generation.Manifest.GeneratedAt)
	if generationAge < 0 || generationAge > r.policy.MaxGenerationAge {
		return &errors.ValidationError{Field: "catalog_distribution.generation_age", Value: generationAge, Message: "exceeds stable promotion policy"}
	}
	return nil
}

func (r *MemoryRepository) recordPromotionLocked(action PromotionAction, channel Channel, from, to string, at time.Time, success bool, reason string) {
	r.sequence++
	r.events = append(r.events, PromotionEvent{
		Sequence: r.sequence, Action: action, Channel: channel, From: from, To: to,
		ObservedAt: at, Success: success, Reason: reason,
	})
}

func promotionOrderError(channel Channel, generationID string, prerequisite Channel) error {
	return &errors.ValidationError{
		Field: "catalog_distribution.promotion_order", Value: generationID,
		Message: fmt.Sprintf("%s promotion requires the same generation in %s", channel, prerequisite),
	}
}
