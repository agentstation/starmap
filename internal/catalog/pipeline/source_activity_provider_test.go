package pipeline

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestSourceActivityUsesScopedProviderPreflight(t *testing.T) {
	for _, mode := range []string{"present", "mixed", "missing", "strict-missing"} {
		t.Run(mode, func(t *testing.T) {
			builder, bindings := manualBindingFixture(t)
			var failed atomic.Bool
			var calls, resolutions atomic.Int32
			runner := NewBoundAcquisition(func(*catalogs.Provider) (sources.ProviderClient, error) {
				return manualBindingClient{failed: &failed, calls: &calls}, nil
			}, sources.ProviderCredentialResolverFunc(func(_ context.Context, p *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
				resolutions.Add(1)
				if len(p.Credentials.CatalogAcquisition.Alternatives) != 1 {
					t.Error("provider scope escaped credential selection")
				}
				id := p.Credentials.CatalogAcquisition.Alternatives[0]
				if mode == "missing" || mode == "strict-missing" || (mode == "mixed" && id == "two") {
					return sources.ProviderCredentialMaterial{}, &pkgerrors.AuthenticationError{Provider: "provider", Message: "fixture absent"}
				}
				for _, profile := range p.Credentials.Profiles {
					if profile.ID == id {
						return sources.NewProviderCredentialMaterial(profile, map[catalogs.ProviderCredentialFieldID]string{"api-key": "fixture"}, sources.ProviderCredentialMetadata{}), nil
					}
				}
				return sources.ProviderCredentialMaterial{}, &pkgerrors.AuthenticationError{Provider: "provider", Message: "fixture profile absent"}
			}), bindings)
			runner.loadEmbedded = func() (*catalogs.Catalog, error) { return buildCatalog(t, builder), nil }
			prepared, err := runner.Prepare(t.Context(), buildCatalog(t, builder), pkgsync.WithSources(sources.ProvidersID), pkgsync.WithDryRun(true), pkgsync.WithCatalogPath(t.TempDir()), pkgsync.WithRequireAllSources(mode == "strict-missing"))
			if mode == "strict-missing" {
				if err == nil {
					t.Fatal("strict acquisition accepted missing credentials")
				}
				activities, attempts := sources.ActivityFromError(err), sources.ProviderAttemptsFromError(err)
				if len(activities) != 4 || len(attempts) != 2 {
					t.Fatal("strict failure lost source or provider outcomes")
				}
				for _, state := range activities {
					if state.Source == sources.ProvidersID && state.Eligibility != sources.EligibilityIneligible {
						t.Fatalf("strict source state=%+v", state)
					}
				}
				for _, attempt := range attempts {
					if attempt.Requested {
						t.Fatal("missing credential caused a request")
					}
				}
				attempts[0].BindingID = "caller-change"
				if sources.ProviderAttemptsFromError(err)[0].BindingID == "caller-change" {
					t.Fatal("failure outcomes alias caller state")
				}
				if calls.Load() != 0 || resolutions.Load() != 2 {
					t.Fatalf("strict request/resolution counts=%d/%d", calls.Load(), resolutions.Load())
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(prepared.Result.SourceActivities) != 4 || len(prepared.Result.ProviderAttempts) != 2 {
				t.Fatal("source or provider report is incomplete")
			}
			for i, attempt := range prepared.Result.ProviderAttempts {
				if attempt.BindingID != bindings[i].ID || attempt.BindingRevision != bindings[i].Revision {
					t.Fatalf("provider attempt escaped binding: %+v", attempt)
				}
				requested := mode != "missing" && (mode != "mixed" || attempt.BindingID == "one")
				if attempt.Requested != requested {
					t.Fatalf("provider request state=%+v", attempt)
				}
			}
			expected := sources.EligibilityEligible
			expectedCalls := int32(2)
			if mode == "missing" {
				expected = sources.EligibilityIneligible
				expectedCalls = 0
			}
			if mode == "mixed" {
				expectedCalls = 1
			}
			for _, state := range prepared.Result.SourceActivities {
				if state.Source == sources.ProvidersID && (state.Eligibility != expected || !state.Attempted) {
					t.Fatalf("provider state=%+v", state)
				}
			}
			if calls.Load() != expectedCalls || resolutions.Load() != 2 {
				t.Fatalf("requests/resolutions=%d/%d", calls.Load(), resolutions.Load())
			}
		})
	}
}
