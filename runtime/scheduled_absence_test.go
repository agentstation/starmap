package runtime

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestAcquiredOfferingSurvivesCompleteScopedAbsence(t *testing.T) {
	for _, scheduled := range []bool{false, true} {
		name := "manual"
		if scheduled {
			name = "scheduled"
		}
		t.Run(name, func(t *testing.T) {
			binding := sources.ProviderAcquisitionBinding{
				SchemaVersion: sources.ProviderAcquisitionBindingSchemaVersion,
				ID:            "inventory", Revision: "1", ProviderID: "provider",
				Public: true, Region: "global", APISurface: "models.list",
				MembershipAuthority: sources.ProviderMembershipProvider,
				CredentialRole:      sources.ProviderBindingCatalogAcquisition, CredentialProfileID: "public",
			}
			connected, options, at := providerResetRuntime(t, storage.NewMemory(), WithProviderBindings(binding))
			baseline := connected.Catalog()
			builder, err := catalogs.NewBuilderFrom(baseline)
			if err != nil {
				t.Fatal(err)
			}
			provider, err := builder.Provider("provider")
			if err != nil {
				t.Fatal(err)
			}
			model := catalogs.DeepCopyModel(*provider.Models["model"])
			model.ID = "acquired-only"
			provider.Models[model.ID] = &model
			if err := builder.SetProvider(provider); err != nil {
				t.Fatal(err)
			}
			acquired, err := catalogs.NewObservationCatalog(builder)
			if err != nil {
				t.Fatal(err)
			}
			publish := func(observation sources.Observation) {
				t.Helper()
				if scheduled {
					layer, err := NewProviderLayer("provider", observation)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := connected.publishProviders(t.Context(), []ProviderLayer{layer}, connected.lease.epoch()); err != nil {
						t.Fatal(err)
					}
				} else if _, err := connected.PublishObservations(t.Context(), observation); err != nil {
					t.Fatal(err)
				}
			}
			publish(providerResetObservation(t, sources.ProvidersID, acquired, at, &binding))
			if _, err := connected.Catalog().Offering("provider", catalogs.ProviderModelID(model.ID)); err != nil {
				t.Fatalf("fixture did not publish acquired offering: %v", err)
			}
			publish(providerResetObservation(t, sources.ProvidersID, baseline, at.Add(time.Minute), &binding))
			check := func(runtime *Runtime) {
				t.Helper()
				if _, err := runtime.Catalog().Offering("provider", catalogs.ProviderModelID(model.ID)); err != nil {
					t.Error("complete inventory deleted the acquired offering")
				}
				key := catalogs.MembershipScopeKey{PublisherID: runtime.layers.publisherID, BindingID: binding.ID, BindingRevision: binding.Revision}
				present, known := runtime.Catalog().ScopeMembership(key, "provider", model.ID)
				if present || !known {
					t.Errorf("membership = %t/%t, want absent/known", present, known)
				}
			}
			check(connected)
			generation := connected.State().GenerationID
			if err := connected.Close(); err != nil {
				t.Fatal(err)
			}
			restarted := openTestRuntime(t, options...)
			check(restarted)
			if restarted.State().GenerationID != generation {
				t.Error("restart changed accepted generation")
			}
		})
	}
}
