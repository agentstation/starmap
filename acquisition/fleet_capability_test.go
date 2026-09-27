package acquisition

import (
	"context"
	"errors"
	"testing"

	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/internal/sources/providers"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/runtime"
)

func TestFleetCapabilityResolvesAcquisitionWithoutProviderCalls(t *testing.T) {
	current, binding := acquisitionBindingFixture(t)
	for _, mode := range []string{"available", "missing", "wrong-profile"} {
		t.Run(mode, func(t *testing.T) {
			resolver := sources.ProviderCredentialResolverFunc(func(_ context.Context, p *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
				if len(p.Credentials.CatalogAcquisition.Alternatives) != 1 || p.Credentials.CatalogAcquisition.Alternatives[0] != binding.CredentialProfileID {
					t.Fatal("wrong acquisition profile selection")
				}
				if mode == "missing" {
					return sources.ProviderCredentialMaterial{}, errors.New("unavailable")
				}
				profile := p.Credentials.Profiles[0]
				if mode == "wrong-profile" {
					profile.ID = "another-profile"
				}
				return sources.NewProviderCredentialMaterial(profile, nil, sources.ProviderCredentialMetadata{}), nil
			})
			observer := newProviderSourceObserver(resolver)
			observer.options = append(observer.options, providers.WithClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) {
				t.Fatal("capability check constructed a provider client")
				return nil, nil
			}))
			acquirer, err := NewAcquirer(WithProviderObserver(observer))
			if err != nil {
				t.Fatal(err)
			}
			err = acquirer.CheckFleetAcquisition(t.Context(), runtime.FleetAcquisitionRequirements{Catalog: current, Bindings: []sources.ProviderAcquisitionBinding{binding}})
			if (err == nil) != (mode == "available") {
				t.Fatalf("check error: %v", err)
			}
		})
	}
}

func TestFleetCapabilityOptionalCandidates(t *testing.T) {
	for _, scenario := range []struct {
		name, value         string
		required, wantError bool
	}{
		{"optional-missing", "", false, false},
		{"optional-configured", "valid-key", false, false},
		{"optional-invalid", "invalid-key", false, true},
		{"required-missing", "", true, true},
		{"required-configured", "valid-key", true, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			provider := catalogs.Provider{ID: "candidate", Name: "Candidate", Credentials: &catalogs.ProviderCredentials{
				Fields:             []catalogs.ProviderCredentialField{{ID: "api-key", Kind: catalogs.ProviderCredentialFieldSecret, Required: true, Pattern: "^valid-key$"}},
				Profiles:           []catalogs.ProviderCredentialProfile{{ID: "api-key", Primitive: catalogs.ProviderAuthenticationAPIKey, Fields: []catalogs.ProviderCredentialFieldID{"api-key"}, Placements: []catalogs.ProviderCredentialPlacement{{Field: "api-key", Kind: catalogs.ProviderCredentialPlacementHeader, Name: "Authorization", Scheme: catalogs.ProviderCredentialSchemeBearer}}}},
				CatalogAcquisition: catalogs.ProviderCredentialPlane{Required: true, Alternatives: []catalogs.ProviderCredentialProfileID{"api-key"}},
			}}
			builder := catalogs.NewEmpty()
			if err := builder.SetProvider(provider); err != nil {
				t.Fatal(err)
			}
			catalog, err := builder.Build()
			if err != nil {
				t.Fatal(err)
			}
			resolver := auth.NewResolver(auth.WithEnvironmentLookup(func(string) (string, bool) { return scenario.value, scenario.value != "" }))
			observer := newProviderSourceObserver(resolver)
			observer.options = append(observer.options, providers.WithClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) {
				t.Fatal("capability check opened a provider client")
				return nil, nil
			}))
			acquirer, err := NewAcquirer(WithProviderObserver(observer))
			if err != nil {
				t.Fatal(err)
			}
			request := runtime.FleetAcquisitionRequirements{Catalog: catalog, Candidates: []catalogs.ProviderID{provider.ID}}
			if scenario.required {
				request.Providers, request.Candidates = request.Candidates, nil
			}
			err = acquirer.CheckFleetAcquisition(t.Context(), request)
			if (err != nil) != scenario.wantError {
				t.Fatalf("check error: %v", err)
			}
		})
	}
}
