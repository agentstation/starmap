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
	baseline, err := starmap.New()
	if err != nil {
		t.Fatal(err)
	}
	builder := catalogs.NewEmpty()
	for _, payload := range payloads {
		fixture, err := catalogs.DecodeCatalogPayload(payload)
		if err != nil {
			t.Fatal(err)
		}
		for _, provider := range fixture.Providers().List() {
			if declared, found := baseline.Catalog().Providers().Get(provider.ID); found {
				declared.Models = nil
				if declared.Catalog != nil && declared.Catalog.Endpoint.AuthorMapping != nil {
					for _, id := range declared.Catalog.Endpoint.AuthorMapping.Normalized {
						author, found := baseline.Catalog().Authors().Get(id)
						if !found {
							t.Fatalf("provider author %q is absent", id)
						}
						if err := builder.SetAuthor(*author); err != nil {
							t.Fatal(err)
						}
					}
				}
				if err := builder.SetProvider(*declared); err != nil {
					t.Fatal(err)
				}
			}
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
	candidate, err := starmap.NewCandidate(catalog, starmap.CandidateEvidence{}, starmap.WithCandidateGenerationID("reviewed-fixture"))
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	generation, err := candidate.Generation("reviewed-fixture", at)
	if err != nil {
		t.Fatal(err)
	}
	return reviewedRuntimeFixtureSource{read: runtime.SourceRead{Changed: true, Health: runtime.HealthOK,
		PublishedAt: at, ChannelUpdatedAt: at, Generation: generation}}
}
