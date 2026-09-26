package acquisition

import (
	"context"
	"errors"
	"testing"

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
