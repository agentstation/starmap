package runtime

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestGenerationPinRetainsSelectionAcrossRebuildAndUnpin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog-store")
	store, err := storage.NewFilesystem(path)
	if err != nil {
		t.Fatal(err)
	}
	source := newStubSource("pin-source")
	initial := aliasGeneration(t, "pin-source-old", activeAlias("author/old"))
	source.replies = []SourceRead{aliasRead(initial)}
	options := []Option{WithStateDirectory(privateRuntimeDirectory(t)), WithSource(source),
		WithSourceRefreshMode("manual"), WithClientOptions(starmap.WithCatalogStore(store))}
	r := openTestRuntime(t, options...)
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	target := r.State()
	source.mu.Lock()
	source.replies = []SourceRead{aliasRead(aliasGeneration(t, "pin-source-new", activeAlias("author/old"), activeAlias("author/new")))}
	source.mu.Unlock()
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	latest := r.State()
	if latest.GenerationID == target.GenerationID {
		t.Fatal("fixture did not publish a replacement")
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		reopened, err := storage.NewFilesystem(path)
		if err != nil {
			t.Fatal(err)
		}
		pinned := openTestRuntime(t, append(options, WithGenerationPin(target.GenerationID), WithClientOptions(starmap.WithCatalogStore(reopened)))...)
		if pinned.State().GenerationID != target.GenerationID || pinned.Status().GenerationPin != target.GenerationID {
			t.Fatal("startup rebuilt a different generation from retained inputs")
		}
		assertAliasTarget(t, pinned.Catalog(), "author/old")
		if err := pinned.Close(); err != nil {
			t.Fatal(err)
		}
	}
	unpinned := openTestRuntime(t, append(options, WithGenerationPin(""))...)
	if unpinned.State().GenerationID != latest.GenerationID || unpinned.Client().CurrentGenerationID() != latest.GenerationID {
		t.Fatalf("unpin failed to restore retained inputs consistently: runtime=%s client=%s want=%s", unpinned.State().GenerationID, unpinned.Client().CurrentGenerationID(), latest.GenerationID)
	}
	assertAliasTarget(t, unpinned.Catalog(), "author/new")
}

func TestGenerationPinBlocksEveryCatalogMutation(t *testing.T) {
	store := storage.NewMemory()
	generation := aliasGeneration(t, "pinned")
	if err := store.Commit(t.Context(), generation, ""); err != nil {
		t.Fatal(err)
	}
	r := openTestRuntime(t, WithCatalogSource("embedded"), WithGenerationPin("pinned"), WithClientOptions(starmap.WithCatalogStore(store)))
	changed := false
	for _, operation := range []string{"source", "refresh", "sync", "prepare", "preview", "import", "remove", "update", "activate", "reload", "rollback"} {
		t.Run(operation, func(t *testing.T) {
			var err error
			switch operation {
			case "source":
				_, err = r.RefreshSource(t.Context())
			case "refresh":
				_, err = r.Refresh(t.Context())
			case "sync":
				_, err = r.Sync(t.Context())
			case "prepare", "preview":
				callback := func(context.Context, ObservationInputs) (ObservationUpdate, error) {
					changed = true
					return ObservationUpdate{}, nil
				}
				if operation == "prepare" {
					_, err = r.UpdateAcquisition(t.Context(), callback, sources.LocalCatalogID)
				} else {
					_, err = r.PreviewAcquisition(t.Context(), callback, sources.LocalCatalogID)
				}
			case "import":
				_, err = r.PublishObservations(t.Context(), manualTestObservation(t, "local", generation.Manifest.GeneratedAt, false))
			case "remove":
				_, err = r.ReplaceRemovalTargets(t.Context(), r.State())
			case "update":
				_, err = r.Client().Update(t.Context(), func(context.Context, *catalogs.Catalog) (*starmap.Candidate, error) { changed = true; return nil, nil })
			case "activate":
				_, err = r.Client().Activate(t.Context(), generation)
			case "reload":
				_, err = r.Client().Reload(t.Context(), func(catalogs.Generation) error { changed = true; return nil })
			case "rollback":
				_, err = r.Client().Rollback(t.Context(), generation.Manifest.GenerationID)
			}
			if !errors.IsConflict(err) || changed {
				t.Fatalf("pinned %s: error=%v callback=%v", operation, err, changed)
			}
			if r.State().GenerationID != generation.Manifest.GenerationID {
				t.Fatal("pin changed")
			}
		})
	}
}

func TestGenerationPinKeepsPermissionWithdrawalEffective(t *testing.T) {
	source, options := authorityRuntimeFixture(t)
	options = append(options, WithStateDirectory(privateRuntimeDirectory(t)), WithSourceRefreshMode("manual"))
	r := openTestRuntime(t, options...)
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	target := r.State()
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	options = append(options, WithGenerationPin(target.GenerationID))
	pinned := openTestRuntime(t, options...)
	if !pinned.AllowsNewAttempt() {
		t.Fatal("pin lost valid retained permission")
	}
	if err := pinned.RefreshPermission(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !pinned.AllowsNewAttempt() {
		t.Fatal("pin blocked permission renewal")
	}
	source.permissionMu.Lock()
	source.receipt.Head.Sequence++
	source.receipt.Head.GenerationID = "withdrawn-after-pin"
	source.receipt.Head.RequiredPermissionRevision = "sha256:" + strings.Repeat("c", 64)
	source.permissionMu.Unlock()
	if err := pinned.RefreshPermission(t.Context()); err != nil {
		t.Fatal(err)
	}
	if pinned.AllowsNewAttempt() || !pinned.Status().CatalogAvailable || pinned.State().GenerationID != target.GenerationID {
		t.Fatal("pin bypassed a permission withdrawal or discarded metadata")
	}
	if err := pinned.Close(); err != nil {
		t.Fatal(err)
	}
	source.permissionMu.Lock()
	source.permissionErr = fs.ErrNotExist
	source.permissionMu.Unlock()
	restarted := openTestRuntime(t, options...)
	if restarted.AllowsNewAttempt() || restarted.State().GenerationID != target.GenerationID {
		t.Fatal("restart lost pin or withdrawal")
	}
}

func TestGenerationPinBindsPermissionToSelectedArtifact(t *testing.T) {
	source, options := authorityRuntimeFixture(t)
	options = append(options, WithStateDirectory(privateRuntimeDirectory(t)), WithSourceRefreshMode("manual"))
	r := openTestRuntime(t, options...)
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	target := r.State()
	source.permissionMu.Lock()
	source.receipt.Head.Sequence++
	source.receipt.Head.GenerationID = "metadata-after-pin-target"
	next := source.receipt
	source.permissionMu.Unlock()
	source.mu.Lock()
	generation := source.replies[0].Generation.Copy()
	generation.Manifest.GenerationID = next.Head.GenerationID
	generation.Manifest.AuthorityHead = next.Head
	source.replies = []SourceRead{aliasRead(generation)}
	source.mu.Unlock()
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	pinned := openTestRuntime(t, append(options, WithGenerationPin(target.GenerationID))...)
	if err := pinned.RefreshPermission(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !pinned.AllowsNewAttempt() {
		t.Fatal("compatible metadata revision blocked a valid pin")
	}
	pinned.mu.RLock()
	enforced := pinned.permissions.enforced
	pinned.mu.RUnlock()
	if enforced != target.AuthorityHead || pinned.State().AuthorityHead != target.AuthorityHead {
		t.Fatal("permission state was bound to the latest layer instead of the pinned artifact")
	}
	if err := pinned.Close(); err != nil {
		t.Fatal(err)
	}
	incompatible := append(options, WithGenerationPin(target.GenerationID), WithSourceAuthority("replacement", "production"))
	if unexpected, err := Open(t.Context(), incompatible...); err == nil {
		_ = unexpected.Close()
		t.Fatal("an incompatible authority accepted the pin")
	}
}

type generationPinReadStore struct {
	*storage.Memory
	change func(catalogs.Generation) catalogs.Generation
}

func (s *generationPinReadStore) Get(ctx context.Context, id string) (catalogs.Generation, error) {
	generation, err := s.Memory.Get(ctx, id)
	if err == nil {
		generation = s.change(generation)
	}
	return generation, err
}

func TestGenerationPinRejectsInvalidRetainedArtifacts(t *testing.T) {
	for _, scenario := range []string{"missing", "identity", "payload", "schema"} {
		t.Run(scenario, func(t *testing.T) {
			base := storage.NewMemory()
			generation := aliasGeneration(t, "pin-target")
			if err := base.Commit(t.Context(), generation, ""); err != nil {
				t.Fatal(err)
			}
			store := &generationPinReadStore{Memory: base, change: func(g catalogs.Generation) catalogs.Generation {
				switch scenario {
				case "identity":
					g.Manifest.GenerationID = "other"
				case "payload":
					g.Payload[0] ^= 1
				case "schema":
					g.Manifest.SchemaVersion = catalogs.CurrentCatalogSchemaVersion + 1
				}
				return g
			}}
			id := generation.Manifest.GenerationID
			if scenario == "missing" {
				id = "missing"
			}
			directory := privateRuntimeDirectory(t)
			options := []Option{WithCatalogSource("embedded"), WithAcquisitionEnabled(false), WithSourcePollInterval(0),
				WithStateDirectory(directory), WithClientOptions(starmap.WithCatalogStore(store)), WithGenerationPin(id)}
			if unexpected, err := Open(t.Context(), options...); err == nil {
				_ = unexpected.Close()
				t.Fatal("invalid pin reached readiness")
			}
			retained, err := base.Current(t.Context())
			if err != nil || retained.Manifest.GenerationID != generation.Manifest.GenerationID {
				t.Fatal("refused pin changed storage")
			}
			recovered := openTestRuntime(t, WithCatalogSource("embedded"), WithStateDirectory(directory), WithClientOptions(starmap.WithCatalogStore(base)))
			if recovered.State().GenerationID != generation.Manifest.GenerationID {
				t.Fatal("failed pin changed accepted state")
			}
		})
	}
}
