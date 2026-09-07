package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestSourceReportPreservesPublicationWhenRetentionFails(t *testing.T) {
	store := storage.NewMemory()
	source := newStubSource("pending-source")
	source.replies = []SourceRead{testSourceRead(t, "source-generation", testCatalogPayload(t, "provider", "model", "Model"), time.Now().UTC())}
	connected := openTestRuntime(t, WithSource(source), WithClientOptions(starmap.WithCatalogStore(store)))
	obstruction := filepath.Join(connected.store.root, sourceLayerFileName)
	if err := os.Mkdir(obstruction, privatefiles.DirectoryMode); err != nil {
		t.Fatal(err)
	}
	report, err := connected.RefreshSource(t.Context())
	if err == nil {
		t.Fatal("retention obstruction did not fail")
	}
	if !report.Published || report.GenerationID != connected.State().GenerationID || report.Reason != "retention_pending" {
		t.Fatalf("report lost the accepted publication: %+v", report)
	}
	if report.Health != HealthDegraded {
		t.Fatal("retention failure did not degrade health")
	}
}

func TestAcquisitionReportPreservesPublicationWhenRetentionFails(t *testing.T) {
	for _, windowed := range []bool{false, true} {
		name := "final"
		if windowed {
			name = "window"
		}
		t.Run(name, func(t *testing.T) {
			layer := testProviderLayer(t, "provider", "model", "Model", time.Now().UTC())
			acquirer := &publicationReportAcquirer{layer: layer, windowed: windowed}
			connected := openTestRuntime(t, WithAcquirer(acquirer), WithClientOptions(starmap.WithCatalogStore(storage.NewMemory())))
			obstruction := filepath.Join(connected.store.root, providerLayerDirectoryName, layer.evidenceKey().filename())
			if err := os.Mkdir(obstruction, privatefiles.DirectoryMode); err != nil {
				t.Fatal(err)
			}
			report, err := connected.Sync(t.Context())
			if err == nil {
				t.Fatal("retention obstruction did not fail")
			}
			if !report.Published || report.GenerationID != connected.State().GenerationID {
				t.Fatalf("report lost the accepted publication: %+v", report)
			}
		})
	}
}

type publicationReportAcquirer struct {
	layer    ProviderLayer
	windowed bool
}

func (a *publicationReportAcquirer) AcquireProviders(ctx context.Context, request AcquisitionRequest) (AcquisitionResult, error) {
	result := AcquisitionResult{Eligible: 1, Layers: []ProviderLayer{a.layer}, Attempts: []sources.ProviderAttempt{testAttempt("provider", sources.ProviderOutcomeSucceeded, "")}}
	if a.windowed {
		return result, request.Publish(ctx, result.Layers)
	}
	return result, nil
}
