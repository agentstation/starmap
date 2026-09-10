package runtime

import (
	"context"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestCanonicalAliasClientRejectsHistoryLoss(t *testing.T) {
	for _, operation := range []string{"activate", "update"} {
		t.Run(operation, func(t *testing.T) {
			client, err := starmap.New(starmap.WithCatalogStore(storage.NewMemory()))
			if err != nil {
				t.Fatal(err)
			}
			initial := aliasGeneration(t, "rename", activeAlias("author/old"))
			if _, err := client.Activate(t.Context(), initial); err != nil {
				t.Fatal(err)
			}
			next := aliasGeneration(t, "missing-history")
			if operation == "activate" {
				_, err = client.Activate(t.Context(), next)
			} else {
				catalog, decodeErr := catalogs.DecodeCatalogGeneration(next)
				if decodeErr != nil {
					t.Fatal(decodeErr)
				}
				_, err = client.Update(t.Context(), func(context.Context, *catalogs.Catalog) (*starmap.Candidate, error) {
					return starmap.NewCandidate(catalog, starmap.CandidateEvidence{})
				})
			}
			if err == nil || client.CurrentCatalogState().GenerationID != initial.Manifest.GenerationID {
				t.Fatalf("%s replaced accepted history: %v", operation, err)
			}
			assertAliasTarget(t, client.Catalog(), "author/old")
		})
	}
}

func TestCanonicalAliasStartupRejectsRetainedHistoryLoss(t *testing.T) {
	initial := aliasGeneration(t, "rename", activeAlias("author/old"))
	source := newStubSource("canonical-baseline")
	source.replies = []SourceRead{aliasRead(initial)}
	options := []Option{WithSource(source), WithStateDirectory(privateRuntimeDirectory(t)), WithClientOptions(starmap.WithCatalogStore(storage.NewMemory()))}
	connected := openTestRuntime(t, options...)
	if _, err := connected.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	changed := aliasGeneration(t, "missing-history")
	if err := connected.store.saveSource(t.Context(), sourceLayer{
		Manifest: &changed.Manifest, Identity: source.Identity(), GenerationID: changed.Manifest.GenerationID,
		Checksum: changed.Manifest.Payload.Checksum, Payload: changed.Payload, PublishedAt: changed.Manifest.GeneratedAt,
	}); err != nil {
		t.Fatal(err)
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(t.Context(), options...)
	if err == nil {
		_ = reopened.Close()
		t.Fatal("restart silently replaced accepted alias history")
	}
}

func TestCanonicalAliasFailedPublicationPreservesOldID(t *testing.T) {
	old := activeAlias("author/old")
	initial := aliasGeneration(t, "rename", old)
	source := newStubSource("canonical-baseline")
	source.replies = []SourceRead{aliasRead(initial)}
	store := &retentionRejectingStore{Memory: storage.NewMemory()}
	options := []Option{WithSource(source), WithStateDirectory(privateRuntimeDirectory(t)), WithClientOptions(starmap.WithCatalogStore(store))}
	connected := openTestRuntime(t, options...)
	if _, err := connected.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	before := connected.State()
	old.State = catalogs.CanonicalAliasRemoved
	source.replies = []SourceRead{aliasRead(aliasGeneration(t, "removed", old))}
	store.reject.Store(true)
	if _, err := connected.RefreshSource(t.Context()); err == nil {
		t.Fatal("rejected alias publication succeeded")
	}
	if connected.State().GenerationID != before.GenerationID {
		t.Fatal("failed publication changed the active generation")
	}
	assertAliasTarget(t, connected.Catalog(), old.ID)
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	store.reject.Store(false)
	restarted := openTestRuntime(t, options...)
	assertAliasTarget(t, restarted.Catalog(), old.ID)
	if restarted.State().GenerationID != before.GenerationID {
		t.Fatal("restart published the failed alias replacement")
	}
}
