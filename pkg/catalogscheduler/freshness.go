package catalogscheduler

import (
	"fmt"
	"maps"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

// SourceFreshnessSLA defines warning and readiness budgets for one source.
type SourceFreshnessSLA struct {
	Source        catalogmeta.SourceID
	DegradedAfter time.Duration
	UnreadyAfter  time.Duration
	Required      bool
}

// Validate verifies a useful two-threshold source policy.
func (s SourceFreshnessSLA) Validate() error {
	if strings.TrimSpace(s.Source.String()) == "" {
		return &errors.ValidationError{Field: "catalog_scheduler.freshness.source", Message: "is required"}
	}
	if s.DegradedAfter <= 0 || s.UnreadyAfter <= s.DegradedAfter {
		return &errors.ValidationError{
			Field: "catalog_scheduler.freshness.thresholds", Value: s,
			Message: "degraded threshold must be positive and unready threshold must be greater",
		}
	}
	return nil
}

// FreshnessPolicy is the explicit set of sources a deployment monitors.
type FreshnessPolicy struct {
	Sources []SourceFreshnessSLA
}

// FreshnessState is one source's current SLA disposition.
type FreshnessState string

const (
	// FreshnessStateFresh is within budget with a complete successful observation.
	FreshnessStateFresh FreshnessState = "fresh"
	// FreshnessStateDegraded remains ready but requires operator attention.
	FreshnessStateDegraded FreshnessState = "degraded"
	// FreshnessStateUnready exceeded a critical SLA or has a future timestamp.
	FreshnessStateUnready FreshnessState = "unready"
	// FreshnessStateMissing has not observed the configured source.
	FreshnessStateMissing FreshnessState = "missing"
)

// AlertSeverity is the operational priority of a freshness alert.
type AlertSeverity string

const (
	// AlertSeverityWarning identifies ready-but-degraded state.
	AlertSeverityWarning AlertSeverity = "warning"
	// AlertSeverityCritical identifies state that fails readiness.
	AlertSeverityCritical AlertSeverity = "critical"
)

// Stable freshness alert codes.
const (
	FreshnessAlertSourceMissing  = "source_freshness_missing"
	FreshnessAlertSourceFuture   = "source_freshness_future"
	FreshnessAlertSourceStale    = "source_freshness_stale"
	FreshnessAlertSourceDegraded = "source_observation_degraded"
)

// FreshnessAlert is one machine-readable operator signal.
type FreshnessAlert struct {
	Code     string               `json:"code"`
	Severity AlertSeverity        `json:"severity"`
	Source   catalogmeta.SourceID `json:"source"`
	Message  string               `json:"message"`
}

// SourceFreshness is one source's evaluated observation and SLA state.
type SourceFreshness struct {
	Source               catalogmeta.SourceID                `json:"source"`
	Required             bool                                `json:"required"`
	ObservationID        string                              `json:"observation_id,omitempty"`
	ObservedAt           time.Time                           `json:"observed_at"`
	Age                  time.Duration                       `json:"-"`
	AgeSeconds           int64                               `json:"age_seconds"`
	DegradedAfter        time.Duration                       `json:"-"`
	DegradedAfterSeconds int64                               `json:"degraded_after_seconds"`
	UnreadyAfter         time.Duration                       `json:"-"`
	UnreadyAfterSeconds  int64                               `json:"unready_after_seconds"`
	ObservationStatus    catalogmeta.ObservationStatus       `json:"observation_status,omitempty"`
	Completeness         catalogmeta.ObservationCompleteness `json:"completeness,omitempty"`
	Records              catalogmeta.ObservationRecordCounts `json:"records"`
	ProviderCoverage     catalogmeta.ProviderCoverage        `json:"provider_coverage"`
	PricingObservedAt    *time.Time                          `json:"pricing_observed_at,omitempty"`
	PricingAgeSeconds    int64                               `json:"pricing_age_seconds,omitempty"`
	State                FreshnessState                      `json:"state"`
}

// FreshnessReport is the deterministic readiness/degradation decision for all
// configured sources.
type FreshnessReport struct {
	EvaluatedAt time.Time         `json:"evaluated_at"`
	Ready       bool              `json:"ready"`
	Degraded    bool              `json:"degraded"`
	Sources     []SourceFreshness `json:"sources"`
	Alerts      []FreshnessAlert  `json:"alerts,omitempty"`
}

// Copy returns a report with caller-owned collection state.
func (r FreshnessReport) Copy() FreshnessReport {
	r.Sources = append([]SourceFreshness(nil), r.Sources...)
	for index := range r.Sources {
		if r.Sources[index].PricingObservedAt != nil {
			observedAt := *r.Sources[index].PricingObservedAt
			r.Sources[index].PricingObservedAt = &observedAt
		}
	}
	r.Alerts = append([]FreshnessAlert(nil), r.Alerts...)
	return r
}

// FreshnessMonitor retains only the newest validated observation per configured
// source and evaluates it against explicit deployment SLAs.
type FreshnessMonitor struct {
	mu           sync.RWMutex
	policy       []SourceFreshnessSLA
	observations map[catalogmeta.SourceID]catalogs.SourceObservationLink
}

// NewFreshnessMonitor creates an empty fail-closed monitor.
func NewFreshnessMonitor(policy FreshnessPolicy) (*FreshnessMonitor, error) {
	if len(policy.Sources) == 0 {
		return nil, &errors.ValidationError{Field: "catalog_scheduler.freshness.sources", Message: "at least one source SLA is required"}
	}
	rules := append([]SourceFreshnessSLA(nil), policy.Sources...)
	sort.Slice(rules, func(i, j int) bool { return rules[i].Source < rules[j].Source })
	for index, rule := range rules {
		if err := rule.Validate(); err != nil {
			return nil, err
		}
		if index > 0 && rules[index-1].Source == rule.Source {
			return nil, &errors.ValidationError{Field: "catalog_scheduler.freshness.sources", Value: rule.Source, Message: "source SLA is duplicated"}
		}
	}
	return &FreshnessMonitor{
		policy: rules, observations: make(map[catalogmeta.SourceID]catalogs.SourceObservationLink, len(rules)),
	}, nil
}

// RecordResult advances source freshness from a completed Sync result. It does
// not require catalog changes or a newly published generation.
func (m *FreshnessMonitor) RecordResult(result *pkgsync.Result) error {
	if result == nil {
		return &errors.ValidationError{Field: "catalog_scheduler.freshness.sync_result", Message: "is required"}
	}
	return m.Record(result.SourceObservations)
}

// Record advances configured sources without allowing out-of-order completion
// to regress their latest observation.
func (m *FreshnessMonitor) Record(observations []catalogs.SourceObservationLink) error {
	if m == nil {
		return &errors.ValidationError{Field: "catalog_scheduler.freshness_monitor", Message: "is required"}
	}
	validated := make([]catalogs.SourceObservationLink, len(observations))
	copy(validated, observations)
	for _, observation := range validated {
		if err := observation.Validate(); err != nil {
			return err
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	next := make(map[catalogmeta.SourceID]catalogs.SourceObservationLink, len(m.observations)+len(validated))
	maps.Copy(next, m.observations)
	for _, observation := range validated {
		if !m.monitors(observation.Source) {
			continue
		}
		existing, found := next[observation.Source]
		if found && observation.ObservedAt.Before(existing.ObservedAt) {
			continue
		}
		if found && observation.ObservedAt.Equal(existing.ObservedAt) && observation.ObservationID != existing.ObservationID {
			return &errors.ConflictError{
				Resource: "catalog scheduler source observation", Expected: existing.ObservationID,
				Actual: observation.ObservationID, Message: "same source timestamp is bound to different observation identities",
			}
		}
		next[observation.Source] = observation
	}
	m.observations = next
	return nil
}

// RecordManifest seeds freshness from a validated catalog generation.
func (m *FreshnessMonitor) RecordManifest(manifest catalogs.GenerationManifest) error {
	if err := manifest.Validate(); err != nil {
		return err
	}
	return m.Record(manifest.SourceObservations)
}

// RecordRuns restores no-change and published source observations from durable
// run history. Input order does not matter because older observations cannot
// replace newer state.
func (m *FreshnessMonitor) RecordRuns(records []RunRecord) error {
	observations := make([]catalogs.SourceObservationLink, 0)
	for _, record := range records {
		observations = append(observations, record.SourceObservations...)
	}
	return m.Record(observations)
}

// Report evaluates source ages at an explicit UTC instant. Missing required
// sources and future observations fail readiness; optional missing sources and
// warning-threshold/degraded observations preserve readiness but degrade it.
func (m *FreshnessMonitor) Report(at time.Time) (FreshnessReport, error) {
	if m == nil {
		return FreshnessReport{}, &errors.ValidationError{Field: "catalog_scheduler.freshness_monitor", Message: "is required"}
	}
	if at.IsZero() {
		return FreshnessReport{}, &errors.ValidationError{Field: "catalog_scheduler.freshness.evaluated_at", Message: "is required"}
	}
	at = at.UTC()
	m.mu.RLock()
	policy := append([]SourceFreshnessSLA(nil), m.policy...)
	observations := make(map[catalogmeta.SourceID]catalogs.SourceObservationLink, len(m.observations))
	maps.Copy(observations, m.observations)
	m.mu.RUnlock()

	report := FreshnessReport{EvaluatedAt: at, Ready: true, Sources: make([]SourceFreshness, 0, len(policy))}
	for _, rule := range policy {
		state, alert := evaluateSourceFreshness(at, rule, observations[rule.Source])
		report.Sources = append(report.Sources, state)
		if state.State != FreshnessStateFresh {
			report.Degraded = true
		}
		if state.State == FreshnessStateUnready || state.State == FreshnessStateMissing && rule.Required {
			report.Ready = false
		}
		if alert.Code != "" {
			report.Alerts = append(report.Alerts, alert)
		}
	}
	return report, nil
}

func (m *FreshnessMonitor) monitors(source catalogmeta.SourceID) bool {
	index := sort.Search(len(m.policy), func(index int) bool { return m.policy[index].Source >= source })
	return index < len(m.policy) && m.policy[index].Source == source
}

func evaluateSourceFreshness(at time.Time, rule SourceFreshnessSLA, observation catalogs.SourceObservationLink) (SourceFreshness, FreshnessAlert) {
	state := SourceFreshness{
		Source: rule.Source, Required: rule.Required, DegradedAfter: rule.DegradedAfter,
		DegradedAfterSeconds: int64(rule.DegradedAfter / time.Second),
		UnreadyAfter:         rule.UnreadyAfter, UnreadyAfterSeconds: int64(rule.UnreadyAfter / time.Second),
		State: FreshnessStateFresh,
	}
	if observation.ObservationID == "" {
		state.State = FreshnessStateMissing
		severity := AlertSeverityWarning
		if rule.Required {
			severity = AlertSeverityCritical
		}
		return state, freshnessAlert(FreshnessAlertSourceMissing, severity, rule.Source, "source has no successful observation")
	}
	state.ObservationID = observation.ObservationID
	state.ObservedAt = observation.ObservedAt
	state.Age = at.Sub(observation.ObservedAt)
	state.AgeSeconds = int64(state.Age / time.Second)
	state.ObservationStatus = observation.Status
	state.Completeness = observation.Completeness
	state.Records = observation.Metrics.Records
	state.ProviderCoverage = observation.Metrics.ProviderCoverage
	if observation.Metrics.PricingObservedAt != nil {
		pricingObservedAt := *observation.Metrics.PricingObservedAt
		state.PricingObservedAt = &pricingObservedAt
		state.PricingAgeSeconds = int64(at.Sub(pricingObservedAt) / time.Second)
	}
	if state.Age < 0 {
		state.State = FreshnessStateUnready
		return state, freshnessAlert(FreshnessAlertSourceFuture, AlertSeverityCritical, rule.Source, "source observation timestamp is in the future")
	}
	if state.Age > rule.UnreadyAfter {
		state.State = FreshnessStateUnready
		return state, freshnessAlert(FreshnessAlertSourceStale, AlertSeverityCritical, rule.Source,
			fmt.Sprintf("source age %s exceeds unready SLA %s", state.Age.Round(time.Second), rule.UnreadyAfter))
	}
	if state.Age > rule.DegradedAfter {
		state.State = FreshnessStateDegraded
		return state, freshnessAlert(FreshnessAlertSourceStale, AlertSeverityWarning, rule.Source,
			fmt.Sprintf("source age %s exceeds degraded SLA %s", state.Age.Round(time.Second), rule.DegradedAfter))
	}
	if observation.Status == catalogmeta.ObservationStatusDegraded || observation.Completeness == catalogmeta.ObservationCompletenessPartial {
		state.State = FreshnessStateDegraded
		return state, freshnessAlert(FreshnessAlertSourceDegraded, AlertSeverityWarning, rule.Source, "latest source observation is degraded or partial")
	}
	return state, FreshnessAlert{}
}

func freshnessAlert(code string, severity AlertSeverity, source catalogmeta.SourceID, message string) FreshnessAlert {
	return FreshnessAlert{Code: code, Severity: severity, Source: source, Message: message}
}
