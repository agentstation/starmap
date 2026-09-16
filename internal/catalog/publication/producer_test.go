package publication

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/catalog/pipeline"
	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestProducerRetainsRequiredEvidenceAfterCredentialFailure(t *testing.T) {
	profile, run := admissionFixture(t)
	retained := *run.Attempts[0].Observation
	start := retained.ObservedAt.Add(time.Minute)
	producer, err := NewProducer(profile, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	producer.collect = func(context.Context, *catalogs.Catalog, ...pkgsync.Option) (*pipeline.Collected, error) {
		binding := profile.Scopes[0].Scope.Binding
		return &pipeline.Collected{
			StartedAt: start, CompletedAt: start.Add(time.Second),
			ProviderAttempts: []sources.ProviderAttempt{{ProviderID: binding.ProviderID, BindingID: binding.ID, BindingRevision: binding.Revision,
				Outcome: sources.ProviderOutcomeSkippedNotConfigured, Reason: sources.ProviderReasonCredentialUnavailable}},
		}, nil
	}
	for _, includeRetained := range []bool{false, true} {
		var inputs []sources.Observation
		if includeRetained {
			inputs = []sources.Observation{retained}
		}
		collected, err := producer.Collect(t.Context(), retained.Catalog, inputs)
		if err != nil {
			t.Fatal(err)
		}
		if collected.Decision.Allowed != includeRetained || collected.Decision.FreshAcquisition {
			t.Fatalf("required source decision=%+v", collected.Decision)
		}
		attempt := collected.Run.Attempts[0]
		if attempt.Outcome != MissingCredentials || attempt.Observation != nil {
			t.Fatalf("missing credentials became evidence: %+v", attempt)
		}
		if includeRetained && collected.Decision.Inputs[0].ID != retained.ID {
			t.Fatal("retained observation identity changed")
		}
	}
}

func TestProducerProfileOwnsSourceSelection(t *testing.T) {
	profile, run := admissionFixture(t)
	profile.Scopes[0].Enabled = false
	profile.Scopes[0].DisabledAction = Remove
	producer, err := NewProducer(profile, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	producer.collect = func(context.Context, *catalogs.Catalog, ...pkgsync.Option) (*pipeline.Collected, error) {
		t.Fatal("disabled profile started source work")
		return nil, nil
	}
	collected, err := producer.Collect(t.Context(), run.Attempts[0].Observation.Catalog, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !collected.Decision.Allowed || len(collected.Decision.Removals) != 1 || len(collected.Decision.Inputs) != 0 {
		t.Fatalf("disabled scope policy=%+v", collected.Decision)
	}
	for _, option := range []pkgsync.Option{pkgsync.WithSources(sources.ModelsDevHTTPID), pkgsync.WithProvider("other"), pkgsync.WithFresh(true), pkgsync.WithRequireAllSources(true)} {
		if _, err := producer.Collect(t.Context(), run.Attempts[0].Observation.Catalog, nil, option); err == nil {
			t.Fatal("acquisition options overrode the publication profile")
		}
	}
}

func TestProducerKeepsPartialAndFailedAttemptsDistinct(t *testing.T) {
	profile, run := admissionFixture(t)
	complete := *run.Attempts[0].Observation
	partial, err := sources.NewObservation(complete.SourceID, complete.Catalog, sources.ObservationMetadata{
		ProviderBinding: complete.ProviderBinding, ObservedAt: complete.ObservedAt, Revision: complete.Revision,
		Completeness: sources.ObservationCompletenessPartial, Status: sources.ObservationStatusDegraded,
		Issues: []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeProvider, Subject: string(complete.ProviderBinding.ProviderID), Code: sources.ObservationIssueCodeInvalidRecord, Message: "fixture partial reply"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, outcome := range []sources.ProviderOutcome{sources.ProviderOutcomeSucceeded, sources.ProviderOutcomeFailed} {
		producer, err := NewProducer(profile, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		producer.collect = func(context.Context, *catalogs.Catalog, ...pkgsync.Option) (*pipeline.Collected, error) {
			binding := profile.Scopes[0].Scope.Binding
			attempt := sources.ProviderAttempt{ProviderID: binding.ProviderID, BindingID: binding.ID, BindingRevision: binding.Revision, Outcome: outcome, Requested: true}
			if outcome == sources.ProviderOutcomeFailed {
				attempt.Reason = sources.ProviderReasonResponseInvalid
			}
			return &pipeline.Collected{StartedAt: run.StartedAt, CompletedAt: run.CompletedAt,
				Observations: []sources.Observation{partial}, ProviderAttempts: []sources.ProviderAttempt{attempt}}, nil
		}
		collected, err := producer.Collect(t.Context(), complete.Catalog, nil)
		if err != nil {
			t.Fatal(err)
		}
		want := Partial
		if outcome == sources.ProviderOutcomeFailed {
			want = Failed
		}
		if collected.Decision.Allowed || collected.Run.Attempts[0].Outcome != want {
			t.Fatalf("partial/failed admission=%+v", collected)
		}
	}
}

func TestProducerCancellationDiscardsCollectedInputs(t *testing.T) {
	profile, run := admissionFixture(t)
	producer, err := NewProducer(profile, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	producer.collect = func(context.Context, *catalogs.Catalog, ...pkgsync.Option) (*pipeline.Collected, error) {
		cancel()
		return &pipeline.Collected{StartedAt: run.StartedAt, CompletedAt: run.CompletedAt, Observations: []sources.Observation{*run.Attempts[0].Observation}}, nil
	}
	if collected, err := producer.Collect(ctx, run.Attempts[0].Observation.Catalog, nil); !stderrors.Is(err, context.Canceled) || collected.Decision.Allowed {
		t.Fatalf("canceled producer returned admission: %+v, %v", collected, err)
	}
}

func TestProducerRejectsCompleteEvidenceReturnedWithSourceFailure(t *testing.T) {
	scope := Scope{Source: sources.ModelsDevHTTPID}
	observation := admissionObservation(t, scope, time.Now().UTC())
	profile := Profile{Version: "v1", Scopes: []ScopePolicy{{Scope: scope, Enabled: true, Required: true, DisabledAction: Preserve}}}
	producer, err := NewProducer(profile, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	producer.collect = func(context.Context, *catalogs.Catalog, ...pkgsync.Option) (*pipeline.Collected, error) {
		return &pipeline.Collected{StartedAt: observation.ObservedAt, CompletedAt: observation.ObservedAt,
			Observations:   []sources.Observation{observation},
			SourceFailures: []error{pkgerrors.WrapResource("observe", "source", string(scope.Source), &pkgerrors.ConfigError{Message: "source reply failed"})}}, nil
	}
	result, err := producer.Collect(t.Context(), observation.Catalog, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision.Allowed || result.Run.Attempts[0].Outcome != Failed || result.Run.Attempts[0].Observation != nil {
		t.Fatalf("source failure became fresh publication evidence: %+v", result)
	}
}

func TestProducerAdmitsRetainedEvidenceAfterDependencyFailure(t *testing.T) {
	scope := Scope{Source: sources.ModelsDevGitID}
	observation := admissionObservation(t, scope, time.Now().UTC().Add(-time.Minute))
	profile := Profile{Version: "v1", Scopes: []ScopePolicy{{Scope: scope, Enabled: true, Required: true, MaxRetainedAge: time.Hour, DisabledAction: Preserve}}}
	producer, err := NewProducer(profile, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	dependency := &pkgerrors.DependencyError{Source: string(scope.Source), Dependency: "git", Message: "dependency unavailable"}
	producer.collect = func(context.Context, *catalogs.Catalog, ...pkgsync.Option) (*pipeline.Collected, error) {
		return nil, &sources.ActivityError{Err: &pkgerrors.ConfigError{Component: "sync sources", Message: "no acquisition source available", Err: stderrors.Join(dependency)}}
	}
	for _, retained := range [][]sources.Observation{nil, {observation}} {
		result, err := producer.Collect(t.Context(), observation.Catalog, retained)
		if err != nil {
			t.Fatal(err)
		}
		if result.Decision.Allowed != (len(retained) != 0) || result.Decision.FreshAcquisition || result.Run.Attempts[0].Outcome != Failed {
			t.Fatalf("dependency admission=%+v", result)
		}
		if len(retained) != 0 && result.Decision.Inputs[0].ID != observation.ID {
			t.Fatal("retained identity changed")
		}
	}
}

func TestProducerCannotConvertUnscopedErrorsToRetainedSuccess(t *testing.T) {
	profile, run := admissionFixture(t)
	observation := *run.Attempts[0].Observation
	observation = admissionObservation(t, profile.Scopes[0].Scope, time.Now().UTC().Add(-time.Minute))
	for _, failure := range []error{
		&pkgerrors.ConfigError{Component: "catalog paths", Message: "invalid root"},
		stderrors.Join(&pkgerrors.DependencyError{Source: "providers", Dependency: "fixture", Message: "missing"}, &pkgerrors.ConfigError{Component: "catalog", Message: "read failed"}),
	} {
		producer, err := NewProducer(profile, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		producer.collect = func(context.Context, *catalogs.Catalog, ...pkgsync.Option) (*pipeline.Collected, error) {
			return nil, failure
		}
		result, err := producer.Collect(t.Context(), observation.Catalog, []sources.Observation{observation})
		if !stderrors.Is(err, failure) || result.Decision.Allowed {
			t.Fatalf("unscoped error admitted: %+v, %v", result, err)
		}
	}
}

func TestProducerRejectsUnscopedCollectionFailures(t *testing.T) {
	profile, run := admissionFixture(t)
	for _, failure := range []error{
		&pkgerrors.ConfigError{Message: "unclassified failure"},
		pkgerrors.WrapResource("observe", "source", string(sources.ModelsDevHTTPID), &pkgerrors.ConfigError{Message: "undeclared source failed"}),
	} {
		producer, err := NewProducer(profile, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		producer.collect = func(context.Context, *catalogs.Catalog, ...pkgsync.Option) (*pipeline.Collected, error) {
			return &pipeline.Collected{StartedAt: run.StartedAt, CompletedAt: run.CompletedAt, SourceFailures: []error{failure}}, nil
		}
		result, err := producer.Collect(t.Context(), run.Attempts[0].Observation.Catalog, nil)
		if err == nil || result.Decision.Allowed {
			t.Fatalf("unscoped collection failure accepted: %+v, %v", result, err)
		}
	}
}

func TestProducerRequiresExactProviderAttemptIdentity(t *testing.T) {
	profile, run := admissionFixture(t)
	observation := *run.Attempts[0].Observation
	binding := *profile.Scopes[0].Scope.Binding
	for _, mismatch := range []string{"missing", "revision", "provider", "duplicate"} {
		t.Run(mismatch, func(t *testing.T) {
			producer, err := NewProducer(profile, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			attempt := sources.ProviderAttempt{ProviderID: binding.ProviderID, BindingID: binding.ID, BindingRevision: binding.Revision, Outcome: sources.ProviderOutcomeSucceeded, Requested: true}
			attempts := []sources.ProviderAttempt{attempt}
			switch mismatch {
			case "missing":
				attempts = nil
			case "revision":
				attempts[0].BindingRevision = "another"
			case "provider":
				attempts[0].ProviderID = "another"
			case "duplicate":
				attempts = append(attempts, attempt)
			}
			producer.collect = func(context.Context, *catalogs.Catalog, ...pkgsync.Option) (*pipeline.Collected, error) {
				return &pipeline.Collected{StartedAt: run.StartedAt, CompletedAt: run.CompletedAt, Observations: []sources.Observation{observation}, ProviderAttempts: attempts}, nil
			}
			result, err := producer.Collect(t.Context(), observation.Catalog, nil)
			if mismatch == "missing" {
				if err != nil || result.Decision.Allowed || result.Run.Attempts[0].Outcome != Failed {
					t.Fatalf("unproven provider attempt=%+v, %v", result, err)
				}
			} else if err == nil || result.Decision.Allowed {
				t.Fatalf("mismatched attempt accepted: %+v, %v", result, err)
			}
		})
	}
}
