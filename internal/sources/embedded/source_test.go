package embedded

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestSourceReturnsCompleteContentAddressedEmbeddedObservation(t *testing.T) {
	builder := catalogs.NewEmpty()
	author := catalogs.Author{ID: "test-author", Name: "Test Author"}
	if err := builder.SetAuthor(author); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	for _, model := range []catalogs.Model{
		{ID: "embedded-one--model", Name: "Embedded Model", Authors: []catalogs.Author{author}},
		{ID: "embedded-two--model", Name: "Provider-Scoped Embedded Model", Authors: []catalogs.Author{author}},
	} {
		if err := builder.SetAuthorModel(author.ID, model); err != nil {
			t.Fatalf("SetAuthorModel(%s): %v", model.ID, err)
		}
	}
	if err := builder.SetProvider(catalogs.Provider{
		ID: "embedded-one",
		Models: map[string]*catalogs.Model{
			"model": {ID: "model", ModelRef: "test-author/embedded-one--model", Name: "Embedded Model"},
		},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	if err := builder.SetProvider(catalogs.Provider{
		ID: "embedded-two",
		Models: map[string]*catalogs.Model{
			"model": {ID: "model", ModelRef: "test-author/embedded-two--model", Name: "Provider-Scoped Embedded Model"},
		},
	}); err != nil {
		t.Fatalf("SetProvider second provider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	observedAt := time.Date(2026, time.July, 28, 9, 0, 0, 0, time.UTC)
	source := New(catalog)
	source.now = func() time.Time { return observedAt }

	observation, err := source.Observe(context.Background())
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if observation.SourceID != sources.EmbeddedCatalogID ||
		observation.Status != sources.ObservationStatusSucceeded ||
		observation.Completeness != sources.ObservationCompletenessComplete ||
		observation.Records.Accepted != 2 {
		t.Fatalf("observation = %#v", observation)
	}
	if observation.ObservedAt != observedAt ||
		observation.Revision.Kind != sources.RevisionKindContentDigest ||
		observation.Revision.Value != observation.EvidenceChecksum {
		t.Fatalf("observation evidence = %#v", observation)
	}
	if err := observation.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestSourceHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := New(nil).Observe(ctx)
	if !stderrors.Is(err, context.Canceled) {
		t.Fatalf("Observe error = %v, want context canceled", err)
	}
}
