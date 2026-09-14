package publication

import (
	"bytes"
	"strconv"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
	catalogruntime "github.com/agentstation/starmap/runtime"
)

func TestPublicationCompactionPreservesChangesAbsenceReturnAndScopeRemoval(t *testing.T) {
	baseline, profile := producerPipelineFixture(t)
	profile.Scopes = append(profile.Scopes, ScopePolicy{Scope: Scope{Source: sources.ModelsDevHTTPID}, Enabled: true, Required: true, DisabledAction: Preserve})
	original := publicationBaselineFixture(t, baseline)
	state, err := NewState(original, "publisher")
	if err != nil {
		t.Fatal(err)
	}
	var fullHistory []sources.Observation
	start := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	for step := range 13 {
		at := start.Add(time.Duration(step) * 4 * time.Hour)
		if step == 12 {
			profile.Scopes[1].Enabled = false
			profile.Scopes[1].DisabledAction = Remove
			var remaining []sources.Observation
			for _, observation := range fullHistory {
				if observation.ProviderBinding == nil || observation.ProviderBinding.ID != "two" {
					remaining = append(remaining, observation)
				}
			}
			fullHistory = remaining
		}
		run := Run{StartedAt: at, CompletedAt: at}
		var bindings []sources.ProviderAcquisitionBinding
		for _, policy := range profile.Scopes {
			attempt := Attempt{Scope: policy.Scope, Outcome: Disabled}
			if policy.Enabled {
				observation := unresolvedPublicationObservation(t, at)
				if policy.Scope.Binding != nil {
					bindings = append(bindings, *policy.Scope.Binding)
					observation = publicationInventoryAt(t, baseline, policy.Scope.Binding, step, at)
				}
				attempt.Outcome, attempt.Observation = Succeeded, &observation
				fullHistory = append(fullHistory, observation)
			}
			run.Attempts = append(run.Attempts, attempt)
		}
		prepared, err := preparePublication(t.Context(), state, profile, run, "history-"+strconv.Itoa(step))
		if err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		// Compare against all original observations, including those absent from the checkpoint.
		reference, err := catalogruntime.ReplayAcquisition(t.Context(), original, "publisher", bindings, fullHistory)
		if err != nil {
			t.Fatal(err)
		}
		expected, err := reference.Generation("reference", at)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(expected.Payload, prepared.Next.Generation().Payload) {
			t.Fatalf("step %d changed facts, provenance, or membership after compaction", step)
		}
		checkpoint, err := EncodeState(prepared.Next)
		if err != nil {
			t.Fatal(err)
		}
		state, err = RestoreState(t.Context(), checkpoint.Data, checkpoint.Checksum)
		if err != nil {
			t.Fatalf("step %d cannot restore: %v", step, err)
		}
		catalog, err := catalogs.DecodeCatalogGeneration(state.Generation())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := catalog.Offering("provider", "two"); err != nil {
			t.Fatalf("step %d hid the baseline offering: %v", step, err)
		}
		scopes := catalog.MembershipScopes()
		if step == 12 {
			if len(scopes) != 1 || scopes[0].BindingID != "one" {
				t.Fatal("explicit removal changed another account or retained the removed scope")
			}
		} else {
			for _, scope := range scopes {
				present, known := scope.Membership(scope.BindingID)
				want := scope.BindingID == "one" || step < 4 || step >= 9
				if !known || present != want {
					t.Fatalf("step %d scope %s: present=%t known=%t", step, scope.BindingID, present, known)
				}
			}
		}
		if step == 3 && len(state.history) > 6 {
			t.Fatalf("stable interleaved sources retained %d observations", len(state.history))
		}
	}
}

func publicationInventoryAt(t *testing.T, baseline *catalogs.Catalog, binding *sources.ProviderAcquisitionBinding, step int, at time.Time) sources.Observation {
	t.Helper()
	builder, err := catalogs.NewBuilderFrom(baseline)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := builder.Provider(binding.ProviderID)
	if err != nil {
		t.Fatal(err)
	}
	model := provider.Models[binding.ID]
	model.Limits = &catalogs.ModelLimits{ContextWindow: 100}
	if step >= 6 {
		model.Limits.ContextWindow = 200
	}
	provider.Models = map[string]*catalogs.Model{binding.ID: model}
	if binding.ID == "two" && step >= 4 && step < 9 {
		provider.Models = map[string]*catalogs.Model{}
	}
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	observation, err := sources.NewObservation(sources.ProvidersID, catalog, sources.ObservationMetadata{
		ProviderBinding: binding, ObservedAt: at, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded})
	if err != nil {
		t.Fatal(err)
	}
	return observation
}
