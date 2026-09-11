package starmap

import (
	"bytes"
	"context"
	stderrors "errors"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestPrepareGenerationPreservesExactBytesForActivation(t *testing.T) {
	store := storage.NewMemory()
	client, err := New(WithCatalogStore(store))
	if err != nil {
		t.Fatal(err)
	}
	before := client.CurrentCatalogState()
	candidate, err := NewCandidate(prepareEmptyCatalog(t), CandidateEvidence{}, WithCandidateGenerationID("prepared"))
	if err != nil {
		t.Fatal(err)
	}
	generation, err := client.PrepareGeneration(t.Context(), candidate)
	if err != nil {
		t.Fatal(err)
	}
	if client.CurrentCatalogState() != before {
		t.Fatal("preparation changed active state")
	}
	if _, err := store.Current(t.Context()); !errors.IsNotFound(err) {
		t.Fatalf("preparation wrote storage: %v", err)
	}
	if _, err := client.Activate(t.Context(), generation); err != nil {
		t.Fatal(err)
	}
	accepted, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Manifest.GenerationID != "prepared" || accepted.Manifest.SyncRunID != generation.Manifest.SyncRunID || !accepted.Manifest.GeneratedAt.Equal(generation.Manifest.GeneratedAt) || !bytes.Equal(accepted.Payload, generation.Payload) {
		t.Fatal("activation changed prepared identity or bytes")
	}
}

func TestPrepareGenerationRejectsInvalidAndCanceledCalls(t *testing.T) {
	client, err := New()
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := NewCandidate(prepareEmptyCatalog(t), CandidateEvidence{})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		client    *Client
		ctx       context.Context
		candidate *Candidate
	}{
		{"nil client", nil, t.Context(), candidate},
		{"nil context", client, nil, candidate},
		{"nil candidate", client, t.Context(), nil},
		{"empty candidate", client, t.Context(), &Candidate{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var validation *errors.ValidationError
			if _, err := tc.client.PrepareGeneration(tc.ctx, tc.candidate); !stderrors.As(err, &validation) {
				t.Fatalf("invalid preparation: %v", err)
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := client.PrepareGeneration(ctx, candidate); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("canceled preparation: %v", err)
	}
}

func prepareEmptyCatalog(t *testing.T) *catalogs.Catalog {
	t.Helper()
	catalog, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}
