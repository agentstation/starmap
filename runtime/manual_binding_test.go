package runtime

import (
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestManualHistoryRechecksBindingsOnRestart(t *testing.T) {
	for _, name := range []string{"retired", "new-revision", "changed-selectors"} {
		t.Run(name, func(t *testing.T) {
			at := time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)
			binding := *scopedProviderLayer(t, "binding", "1", at).Receipt.ProviderBinding
			observation, err := sources.NewObservation(sources.ProvidersID, manualProviderObservation(t, 200, at.Add(time.Minute)).Catalog, sources.ObservationMetadata{
				ObservedAt: at.Add(time.Minute), ProviderBinding: &binding, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
				Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
			})
			if err != nil {
				t.Fatal(err)
			}
			payload, err := catalogs.EncodeCatalogPayload(manualProviderObservation(t, 100, at).Catalog)
			if err != nil {
				t.Fatal(err)
			}
			source := newStubSource("binding-baseline")
			source.replies = []SourceRead{testSourceRead(t, "binding-baseline-generation", payload, at)}
			options := []Option{WithStateDirectory(privateRuntimeDirectory(t)), WithSource(source), WithClientOptions(starmap.WithCatalogStore(storage.NewMemory())), WithSourcePollInterval(0), WithAcquisitionEnabled(false)}
			connected := openTestRuntime(t, append(options, WithProviderBindings(binding))...)
			if _, err := connected.RefreshSource(t.Context()); err != nil {
				t.Fatal(err)
			}
			accepted, err := connected.PublishObservations(t.Context(), observation)
			if err != nil {
				t.Fatal(err)
			}
			provider, err := accepted.Catalog.Provider("provider")
			if err != nil || provider.Models["model"].Limits.ContextWindow != 200 {
				t.Fatalf("active manual binding lost its facts: %v", err)
			}
			if err := connected.Close(); err != nil {
				t.Fatal(err)
			}
			var active []sources.ProviderAcquisitionBinding
			switch name {
			case "new-revision":
				binding.Revision = "2"
				active = append(active, binding)
			case "changed-selectors":
				binding.AccountID = "other-account"
				active = append(active, binding)
			}
			reopened, err := Open(t.Context(), append(options, WithProviderBindings(active...))...)
			if name == "changed-selectors" {
				if reopened != nil {
					t.Cleanup(func() { _ = reopened.Close() })
				}
				if !errors.IsConflict(err) {
					t.Fatalf("selector reuse = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = reopened.Close() })
			provider, err = reopened.Catalog().Provider("provider")
			if err != nil || provider.Models["model"].Limits.ContextWindow != 100 {
				t.Fatal("inactive manual binding survived the policy change")
			}
			if _, err := reopened.PublishObservations(t.Context(), observation); !errors.IsConflict(err) {
				t.Fatalf("inactive manual publication = %v", err)
			}
		})
	}
}
