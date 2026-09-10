package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestPipelineReusesVerifiedEmbeddedCatalog(t *testing.T) {
	embedded, _, err := bootstrap.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		inputs, err := New(nil).loadCatalogInputs(t.Context(), "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if inputs.embedded != embedded {
			t.Fatal("acquisition rebuilt the immutable embedded catalog")
		}
		if inputs.providerConfig == nil || inputs.providerConfig.Providers().Len() != embedded.Providers().Len() {
			t.Fatal("acquisition lost the embedded provider configuration")
		}
	}
}

func TestPipelineStopsInputLoadingAfterCancellation(t *testing.T) {
	for _, stage := range []string{"workspace", "embedded"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			pipeline := New(nil)
			pipeline.loadWorkspace = func(string) (*catalogs.Builder, error) {
				if stage == "workspace" {
					cancel()
				}
				return catalogs.NewEmpty(), nil
			}
			pipeline.loadEmbedded = func() (*catalogs.Catalog, error) {
				if stage == "workspace" {
					t.Fatal("embedded load started after workspace cancellation")
				}
				cancel()
				return catalogs.NewEmpty().Build()
			}
			inputs, err := pipeline.loadCatalogInputs(ctx, "", nil)
			if !errors.Is(err, context.Canceled) || inputs.providerConfig != nil {
				t.Fatalf("canceled inputs = %+v, %v", inputs, err)
			}
		})
	}
}

var benchmarkEmbeddedInput catalogs.Reader

func BenchmarkPipelineEmbeddedInput(b *testing.B) {
	pipeline := New(nil)
	if _, err := pipeline.loadEmbedded(); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		catalog, err := pipeline.loadEmbedded()
		if err != nil {
			b.Fatal(err)
		}
		benchmarkEmbeddedInput = catalog
	}
}
