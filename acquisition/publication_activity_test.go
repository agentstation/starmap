package acquisition

import (
	"context"
	stderrors "errors"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
	"github.com/agentstation/starmap/runtime"
)

type activityCommitStore struct {
	*storage.Memory
	failure error
}

func (s *activityCommitStore) Commit(ctx context.Context, generation catalogs.Generation, expected string) error {
	if s.failure != nil {
		return s.failure
	}
	return s.Memory.Commit(ctx, generation, expected)
}

func TestManualPublicationFailureRetainsSourceActivity(t *testing.T) {
	for _, connectedMode := range []bool{false, true} {
		t.Run(map[bool]string{false: "standalone", true: "connected"}[connectedMode], func(t *testing.T) {
			store := &activityCommitStore{Memory: storage.NewMemory()}
			var client *starmap.Client
			var connected *runtime.Runtime
			var err error
			if connectedMode {
				connected, err = runtime.Open(t.Context(), runtime.WithCatalogSource("embedded"), runtime.WithAcquisitionEnabled(false), runtime.WithSourcePollInterval(0), runtime.WithClientOptions(starmap.WithCatalogStore(store)))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = connected.Close() })
				client = connected.Client()
			} else {
				client, err = starmap.New(starmap.WithCatalogStore(store))
				if err != nil {
					t.Fatal(err)
				}
			}
			options := []Option{
				WithProviderClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) {
					return batchClientFunc(func(context.Context, sources.ProviderCredentialMaterial) ([]catalogs.Model, error) {
						return []catalogs.Model{{ID: "gpt-4o-mini", Name: "GPT-4o mini", Limits: &catalogs.ModelLimits{ContextWindow: 9999}}}, nil
					}), nil
				}),
				WithCredentialResolver(sources.ProviderCredentialResolverFunc(func(_ context.Context, p *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
					return sources.NewProviderCredentialMaterial(p.Credentials.Profiles[0], nil, sources.ProviderCredentialMetadata{}), nil
				})),
			}
			var syncer *Syncer
			if connectedMode {
				syncer, err = NewForRuntime(connected, options...)
			} else {
				syncer, err = New(client, options...)
			}
			if err != nil {
				t.Fatal(err)
			}
			before := client.Catalog()
			beforeID := client.CurrentGenerationID()
			injected := stderrors.New("injected catalog commit failure")
			store.failure = injected
			result, err := syncer.Sync(t.Context(), pkgsync.WithSources(sources.ProvidersID), pkgsync.WithProvider("openai"))
			if result != nil || !stderrors.Is(err, injected) {
				t.Fatalf("publication failure=%+v, %v", result, err)
			}
			if client.Catalog() != before || client.CurrentGenerationID() != beforeID {
				t.Fatal("failed publication changed accepted catalog")
			}
			if connected != nil && (connected.State().Catalog != before || len(connected.Status().AcceptedAcquisitionSources) != 0) {
				t.Fatal("failed publication changed runtime catalog or accepted input")
			}

			accepted := sources.AcceptedSourcesFromError(err)
			if connectedMode {
				if accepted == nil || accepted.GenerationID != connected.State().GenerationID || len(accepted.Sources) != 0 {
					t.Fatalf("failure accepted snapshot=%+v", accepted)
				}
			} else if accepted != nil {
				t.Fatal("standalone composition invented retained input ownership")
			}

			activity := sources.ActivityFromError(err)
			attempts := sources.ProviderAttemptsFromError(err)
			if len(activity) != 4 || len(attempts) != 1 || !attempts[0].Requested {
				t.Fatalf("publication failure lost acquisition report: %+v, %+v", activity, attempts)
			}
			var observed bool
			for _, row := range activity {
				if row.Source == sources.ProvidersID {
					observed = row.Attempted && row.Eligibility == sources.EligibilityEligible
				}
			}
			if !observed {
				t.Fatalf("provider activity=%+v", activity)
			}
		})
	}
}
