package acquisition_test

import (
	"context"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/runtime"
)

type reviewedRuntimeFixtureSource struct{ read runtime.SourceRead }

func (s reviewedRuntimeFixtureSource) Identity() string { return "reviewed-fixture" }
func (s reviewedRuntimeFixtureSource) Read(context.Context) (runtime.SourceRead, error) {
	return s.read, nil
}

// reviewedRuntimeSource supplies authored definitions separately from provider observations.
func reviewedRuntimeSource(t *testing.T, payloads ...[]byte) runtime.Source {
	t.Helper()
	client, err := starmap.New()
	if err != nil {
		t.Fatal(err)
	}
	builder, err := catalogs.NewBuilderFrom(client.EmbeddedCatalogState().Catalog)
	if err != nil {
		t.Fatal(err)
	}
	for _, payload := range payloads {
		fixture, err := catalogs.DecodeCatalogPayload(payload)
		if err != nil {
			t.Fatal(err)
		}
		for _, author := range fixture.Authors().List() {
			if err := builder.SetAuthor(author); err != nil {
				t.Fatal(err)
			}
		}
		for _, record := range fixture.AuthoredModels() {
			if err := builder.SetAuthorModel(record.AuthorID, record.Model); err != nil {
				t.Fatal(err)
			}
		}
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	return reviewedRuntimeFixtureSource{read: runtime.SourceRead{Changed: true, Health: runtime.HealthOK, PublishedAt: at, ChannelUpdatedAt: at,
		Generation: catalogs.Generation{Manifest: catalogs.GenerationManifest{GenerationID: "reviewed-fixture", GeneratedAt: at, Payload: catalogs.DescribeCatalogPayload(payload)}, Payload: payload}}}
}
