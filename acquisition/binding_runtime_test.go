package acquisition_test

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/acquisition"
	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/runtime"
)

type scopedRuntimeObserver struct {
	mu     sync.Mutex
	layers map[string]runtime.ProviderLayer
	failed string
}

func (o *scopedRuntimeObserver) ObserveProvider(context.Context, *catalogs.Catalog, catalogs.ProviderID) (acquisition.ProviderObservation, error) {
	return acquisition.ProviderObservation{}, stderrors.New("unexpected unscoped acquisition")
}

func (o *scopedRuntimeObserver) ObserveProviderBinding(_ context.Context, _ *catalogs.Catalog, b sources.ProviderAcquisitionBinding) (acquisition.ProviderObservation, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.failed == b.ID {
		return acquisition.ProviderObservation{}, stderrors.New("scripted provider failure")
	}
	return acquisition.ProviderObservation{Layer: o.layers[b.ID], Attempt: sources.ProviderAttempt{ProviderID: b.ProviderID, Outcome: sources.ProviderOutcomeSucceeded, Requested: true}}, nil
}

func runtimeBoundLayer(t *testing.T, binding sources.ProviderAcquisitionBinding, name string, at time.Time) runtime.ProviderLayer {
	t.Helper()
	payload := providerPayload(t, binding.ProviderID, "binding-model-"+binding.ID, name)
	catalog, err := catalogs.DecodeSourceObservationPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	observation, err := sources.NewObservation(sources.ProvidersID, catalog, sources.ObservationMetadata{
		ProviderBinding: &binding, ObservedAt: at, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
	})
	if err != nil {
		t.Fatal(err)
	}
	layer, err := runtime.NewProviderLayer(binding.ProviderID, observation)
	if err != nil {
		t.Fatal(err)
	}
	return layer
}

func TestBoundAcquirerRuntimeRetainsIndependentScopesAfterFailureAndRestart(t *testing.T) {
	baseline, err := starmap.New()
	if err != nil {
		t.Fatal(err)
	}
	provider, found := baseline.Catalog().Providers().Get("openai")
	if !found || provider.Credentials == nil || len(provider.Credentials.CatalogAcquisition.Alternatives) == 0 {
		t.Fatal("embedded fixture lacks an acquisition profile")
	}
	first := sources.ProviderAcquisitionBinding{
		SchemaVersion: 1, ID: "first", Revision: "1", ProviderID: provider.ID, AccountID: "first-account", Region: "global", APISurface: "models",
		CredentialRole: sources.ProviderBindingCatalogAcquisition, CredentialProfileID: provider.Credentials.CatalogAcquisition.Alternatives[0],
	}
	second := first
	second.ID, second.AccountID = "second", "second-account"
	at := time.Now().UTC()
	observer := &scopedRuntimeObserver{layers: map[string]runtime.ProviderLayer{
		first.ID: runtimeBoundLayer(t, first, "First One", at), second.ID: runtimeBoundLayer(t, second, "Second One", at),
	}}
	acquirer, err := acquisition.NewAcquirer(acquisition.WithProviderObserver(observer))
	if err != nil {
		t.Fatal(err)
	}
	reviewed := reviewedRuntimeSource(t, observer.layers[first.ID].Payload, observer.layers[second.ID].Payload)
	directory := filepath.Join(t.TempDir(), "runtime")
	open := func() *runtime.Runtime {
		t.Helper()
		connected, err := runtime.Open(t.Context(), runtime.WithStateDirectory(directory), runtime.WithSource(reviewed), runtime.WithSourcePollInterval(0), runtime.WithAcquisitionEnabled(false), runtime.WithAcquirer(acquirer), runtime.WithProviderBindings(first, second))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := connected.Close(); err != nil {
				t.Error(err)
			}
		})
		if _, err := connected.RefreshSource(t.Context()); err != nil {
			t.Fatal(err)
		}
		return connected
	}
	connected := open()
	report, err := connected.Sync(t.Context(), provider.ID)
	if err != nil || report.Eligible != 2 || report.Succeeded != 2 || !report.Published {
		t.Fatalf("first bound sync: %+v, %v", report, err)
	}
	for i, id := range []string{"first", "second"} {
		if report.Attempts[i].BindingID != id || report.Attempts[i].BindingRevision != "1" {
			t.Fatal("report collapsed binding attempts")
		}
	}
	status := connected.Status()
	if len(status.Providers) != 2 || status.Providers[0].BindingID != "first" || status.Providers[1].BindingID != "second" {
		t.Fatal("status lost the separate scopes")
	}
	status.Providers[0].BindingID = "caller-change"
	if connected.Status().Providers[0].BindingID != "first" {
		t.Fatal("status attempt aliases caller state")
	}
	updated := runtimeBoundLayer(t, first, "First Two", at.Add(time.Second))
	observer.mu.Lock()
	observer.failed = second.ID
	observer.layers[first.ID] = updated
	observer.mu.Unlock()
	report, err = connected.Sync(t.Context(), provider.ID)
	if err != nil || report.Succeeded != 1 || report.Failed != 1 || len(report.Retained) != 1 || report.Retained[0] != provider.ID {
		t.Fatalf("partial bound sync: %+v, %v", report, err)
	}
	if modelName(t, connected.Catalog(), provider.ID, "binding-model-first") != "First Two" || modelName(t, connected.Catalog(), provider.ID, "binding-model-second") != "Second One" {
		t.Fatal("one scope replaced its failed sibling")
	}
	records, err := os.ReadDir(filepath.Join(directory, "catalog-runtime", "providers", "bindings"))
	if err != nil {
		t.Fatal(err)
	}
	bindingRecords := 0
	for _, entry := range records {
		if entry.Name() == privatefiles.PublicationDirectoryName && entry.IsDir() {
			continue
		}
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".json") {
			t.Fatalf("unexpected binding entry: %s", entry.Name())
		}
		bindingRecords++
	}
	if bindingRecords != 2 {
		t.Fatalf("retained binding records: %d, %v", bindingRecords, records)
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := open()
	if modelName(t, reopened.Catalog(), provider.ID, "binding-model-first") != "First Two" || modelName(t, reopened.Catalog(), provider.ID, "binding-model-second") != "Second One" {
		t.Fatal("restart lost independent binding evidence")
	}
}
