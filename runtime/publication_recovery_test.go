package runtime

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

type retentionAmbiguousStore struct {
	*storage.Memory
	ambiguous atomic.Bool
}

func (s *retentionAmbiguousStore) Commit(ctx context.Context, generation catalogs.Generation, expected string) error {
	if err := s.Memory.Commit(ctx, generation, expected); err != nil {
		return err
	}
	if s.ambiguous.Swap(false) {
		return fs.ErrPermission
	}
	return nil
}

func TestRetentionRecoversCatalogCommitWithLostReply(t *testing.T) {
	layer := scopedProviderLayer(t, "binding", "1", time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))
	store := &retentionAmbiguousStore{Memory: storage.NewMemory()}
	options := []Option{WithStateDirectory(privateRuntimeDirectory(t)), WithCatalogSource("embedded"), WithProviderBindings(*layer.Receipt.ProviderBinding), WithClientOptions(starmap.WithCatalogStore(store))}
	connected := openTestRuntime(t, options...)
	store.ambiguous.Store(true)
	if _, err := connected.publishProviders(t.Context(), []ProviderLayer{layer}, connected.lease.epoch()); err == nil {
		t.Fatal("lost commit reply did not return an error")
	}
	accepted, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if connected.State().GenerationID == accepted.Manifest.GenerationID {
		t.Fatal("fixture did not isolate an ambiguous commit")
	}
	if _, err := connected.rebuild(t.Context(), connected.lease.epoch()); !errors.IsConflict(err) {
		t.Fatalf("unresolved publication allowed another rebuild: %v", err)
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	recovered := openTestRuntime(t, options...)
	if recovered.State().GenerationID != accepted.Manifest.GenerationID {
		t.Fatal("recovery did not select the accepted generation")
	}
	retained, err := recovered.store.loadProviders()
	if err != nil {
		t.Fatal(err)
	}
	if len(retained) != 1 || retained[layer.evidenceKey()].Receipt.Link.ObservationID != layer.Receipt.Link.ObservationID {
		t.Fatal("recovery lost the accepted provider receipt")
	}
	if pending, err := recovered.store.loadInputPublication(); err != nil || pending != nil {
		t.Fatalf("recovery remains pending: %v", err)
	}
}

func TestRetentionRecoversPartialWritesAfterCatalogAcceptance(t *testing.T) {
	at := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	first := scopedProviderLayer(t, "first", "1", at)
	second := scopedProviderLayer(t, "second", "1", at)
	store := storage.NewMemory()
	options := []Option{WithStateDirectory(privateRuntimeDirectory(t)), WithCatalogSource("embedded"), WithProviderBindings(*first.Receipt.ProviderBinding, *second.Receipt.ProviderBinding), WithClientOptions(starmap.WithCatalogStore(store))}
	connected := openTestRuntime(t, options...)
	obstruction := filepath.Join(connected.store.root, providerLayerDirectoryName, bindingLayerDirectoryName, second.evidenceKey().filename())
	if err := os.Mkdir(obstruction, privatefiles.DirectoryMode); err != nil {
		t.Fatal(err)
	}
	state, err := connected.publishProviders(t.Context(), []ProviderLayer{first, second}, connected.lease.epoch())
	if err == nil {
		t.Fatal("retention obstruction did not fail")
	}
	accepted, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if state.GenerationID != accepted.Manifest.GenerationID || connected.State().GenerationID != state.GenerationID {
		t.Fatal("accepted catalog differs from the active runtime")
	}
	record, err := connected.store.loadInputPublication()
	if err != nil {
		t.Fatal(err)
	}
	if record == nil || record.Phase != inputPublicationCommitted {
		t.Fatal("partial retention lost its committed recovery record")
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(obstruction); err != nil {
		t.Fatal(err)
	}
	recovered := openTestRuntime(t, options...)
	retained, err := recovered.store.loadProviders()
	if err != nil {
		t.Fatal(err)
	}
	if len(retained) != 2 {
		t.Fatalf("recovered %d inputs, want both", len(retained))
	}
	if recovered.State().GenerationID != accepted.Manifest.GenerationID {
		t.Fatal("recovery changed accepted identity")
	}
}

func TestRetentionRecoveryValidatesEveryInputBeforeWriting(t *testing.T) {
	store, err := newLayerStore(privateRuntimeDirectory(t))
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	first := scopedProviderLayer(t, "first", "1", at)
	second := scopedProviderLayer(t, "second", "1", at)
	one, err := store.stageInput(t.Context(), first)
	if err != nil {
		t.Fatal(err)
	}
	two, err := store.stageInput(t.Context(), second)
	if err != nil {
		t.Fatal(err)
	}
	record := inputPublication{Version: inputPublicationVersion, Phase: inputPublicationCommitted, GenerationID: "target", PayloadChecksum: "target-checksum", Providers: []string{one, two}}
	if err := store.writeInputPublication(t.Context(), record); err != nil {
		t.Fatal(err)
	}
	directory, err := store.directory.ExistingChild(inputPublicationDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if err := directory.WriteFile(two, []byte("{}"), ".test-"); err != nil {
		t.Fatal(err)
	}
	if err := store.recoverInputPublication(t.Context(), starmap.CatalogState{}); !errors.IsConflict(err) {
		t.Fatalf("corrupt input recovery = %v", err)
	}
	retained, err := store.loadProviders()
	if err != nil {
		t.Fatal(err)
	}
	if len(retained) != 0 {
		t.Fatal("recovery wrote a valid prefix before detecting corruption")
	}
	if pending, err := store.loadInputPublication(); err != nil || pending == nil {
		t.Fatalf("corrupt recovery lost its record: %v", err)
	}
}

func TestRetentionRecoveryRefusesUnresolvedHead(t *testing.T) {
	store, err := newLayerStore(privateRuntimeDirectory(t))
	if err != nil {
		t.Fatal(err)
	}
	layer := scopedProviderLayer(t, "binding", "1", time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))
	name, err := store.stageInput(t.Context(), layer)
	if err != nil {
		t.Fatal(err)
	}
	record := inputPublication{Version: inputPublicationVersion, Phase: inputPublicationPrepared, ExpectedID: "before", ExpectedChecksum: "before-checksum", GenerationID: "target", PayloadChecksum: "target-checksum", Providers: []string{name}}
	if err := store.writeInputPublication(t.Context(), record); err != nil {
		t.Fatal(err)
	}
	current := starmap.CatalogState{GenerationID: "unrelated", PayloadChecksum: "other-checksum"}
	if err := store.recoverInputPublication(t.Context(), current); !errors.IsConflict(err) {
		t.Fatalf("unresolved recovery = %v", err)
	}
	retained, err := store.loadProviders()
	if err != nil {
		t.Fatal(err)
	}
	if len(retained) != 0 {
		t.Fatal("unresolved recovery retained inputs")
	}
	if err := validateMigrationCatalog(t.Context(), filepath.Dir(store.root), "test-runtime"); !errors.IsConflict(err) {
		t.Fatalf("migration accepted pending publication: %v", err)
	}
}

func TestRetentionRecoveryRejectsInvalidRecords(t *testing.T) {
	for name, record := range map[string]string{
		"version":       `{"version":3,"phase":"idle"}`,
		"phase":         `{"version":1,"phase":"unknown"}`,
		"trailing":      `{"version":1,"phase":"idle"} {}`,
		"unknown-field": `{"version":1,"phase":"idle","unrecognized":true}`,
		"idle-inputs":   `{"version":1,"phase":"idle","generation_id":"other"}`,
	} {
		t.Run(name, func(t *testing.T) {
			store, err := newLayerStore(privateRuntimeDirectory(t))
			if err != nil {
				t.Fatal(err)
			}
			if err := store.directory.WriteFile(inputPublicationName, []byte(record), ".test-"); err != nil {
				t.Fatal(err)
			}
			if err := store.recoverInputPublication(t.Context(), starmap.CatalogState{}); err == nil {
				t.Fatal("invalid recovery record succeeded")
			}
			after, err := readLayerFile(store.directory, inputPublicationName)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != record {
				t.Fatal("recovery changed an unrecognized record")
			}
		})
	}
}

func TestRetentionRecoveryDoesNotInferCommitFromUnchangedCatalog(t *testing.T) {
	store, err := newLayerStore(privateRuntimeDirectory(t))
	if err != nil {
		t.Fatal(err)
	}
	layer := scopedProviderLayer(t, "binding", "1", time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))
	name, err := store.stageInput(t.Context(), layer)
	if err != nil {
		t.Fatal(err)
	}
	record := inputPublication{Version: inputPublicationVersion, Phase: inputPublicationPrepared, ExpectedID: "unchanged", ExpectedChecksum: "same-checksum", GenerationID: "unchanged", PayloadChecksum: "same-checksum", Providers: []string{name}}
	if err := store.writeInputPublication(t.Context(), record); err != nil {
		t.Fatal(err)
	}
	current := starmap.CatalogState{GenerationID: "unchanged", PayloadChecksum: "same-checksum"}
	if err := store.recoverInputPublication(t.Context(), current); err != nil {
		t.Fatal(err)
	}
	retained, err := store.loadProviders()
	if err != nil {
		t.Fatal(err)
	}
	if len(retained) != 0 {
		t.Fatal("unchanged catalog implicitly accepted pending inputs")
	}
}
