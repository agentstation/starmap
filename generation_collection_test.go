package starmap

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

type clientGenerationCollector interface {
	CanCollectGenerations() bool
	CollectGenerations(context.Context, storage.RetentionRequest) (storage.RetentionReport, error)
}

func TestClientGenerationCollectionBeforeFirstPublication(t *testing.T) {
	store := storage.NewMemory()
	client, err := NewContext(t.Context(), WithCatalogStore(store))
	if err != nil {
		t.Fatal(err)
	}
	state := client.CurrentCatalogState()
	if _, err := store.Current(t.Context()); !errors.IsNotFound(err) {
		t.Fatal("constructor wrote storage", err)
	}
	request := storage.RetentionRequest{ExpectedGenerationID: state.GenerationID, MaxGenerations: 1, MaxBytes: 1}
	if report, err := client.CollectGenerations(t.Context(), request); err != nil || report.After.Generations != 0 {
		t.Fatalf("empty store collection failed: %+v, %v", report, err)
	}
	if client.CurrentCatalogState() != state {
		t.Fatal("empty collection changed embedded state")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := client.CollectGenerations(ctx, request); err != context.Canceled {
		t.Fatal("cancellation ignored", err)
	}
}

func TestClientGenerationCollectionProtectsServedAndRequiredGenerations(t *testing.T) {
	for _, kind := range []string{"memory", "filesystem"} {
		t.Run(kind, func(t *testing.T) {
			var store storage.RetainingStore = storage.NewMemory()
			if kind == "filesystem" {
				filesystem, err := storage.NewFilesystem(filepath.Join(t.TempDir(), "catalog"))
				if err != nil {
					t.Fatal(err)
				}
				store = filesystem
			}
			var generations []catalogs.Generation
			previous := ""
			for _, id := range []string{"old", "served", "required", "current"} {
				generation := rootRemoteGeneration(t)
				generation.Manifest.GenerationID = id
				generations = append(generations, generation)
			}
			for _, generation := range generations[:2] {
				if err := store.Commit(t.Context(), generation, previous); err != nil {
					t.Fatal(err)
				}
				previous = generation.Manifest.GenerationID
			}
			client, err := NewContext(t.Context(), WithCatalogStore(store))
			if err != nil {
				t.Fatal(err)
			}
			collector, ok := any(client).(clientGenerationCollector)
			if !ok || !collector.CanCollectGenerations() {
				t.Fatal("client has no generation collection capability")
			}
			for _, generation := range generations[2:] {
				if err := store.Commit(t.Context(), generation, previous); err != nil {
					t.Fatal(err)
				}
				previous = generation.Manifest.GenerationID
			}
			state := client.CurrentCatalogState()
			request := storage.RetentionRequest{ExpectedGenerationID: "current", RequiredGenerationIDs: []string{"required"}, MaxGenerations: 1, MaxBytes: 1}
			report, err := collector.CollectGenerations(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(report.Removed, []string{"old"}) || !report.OverLimit || report.Protected.Generations != 3 {
				t.Fatalf("required content or capacity report changed: %+v", report)
			}
			if client.CurrentCatalogState() != state {
				t.Fatal("collection changed the served generation")
			}
			if !slices.Equal(request.RequiredGenerationIDs, []string{"required"}) {
				t.Fatal("collection changed caller-owned requirements")
			}
			for _, id := range []string{"served", "required", "current"} {
				if _, err := store.Get(t.Context(), id); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
