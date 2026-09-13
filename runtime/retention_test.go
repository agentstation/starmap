package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestAutomaticRetentionPreservesCatalogAcrossUpdateModes(t *testing.T) {
	for _, mode := range []string{"manual", "offline", "pinned"} {
		t.Run(mode, func(t *testing.T) {
			store, err := storage.NewFilesystem(filepath.Join(t.TempDir(), "catalog"))
			if err != nil {
				t.Fatal(err)
			}
			old, current := aliasGeneration(t, "retention-old"), aliasGeneration(t, "retention-current")
			if err := store.Commit(t.Context(), old, ""); err != nil {
				t.Fatal(err)
			}
			if err := store.Commit(t.Context(), current, old.Manifest.GenerationID); err != nil {
				t.Fatal(err)
			}
			directory := privateRuntimeDirectory(t)
			options := []Option{WithStateDirectory(directory), WithCatalogSource("embedded"), WithSourceRefreshMode("manual"),
				WithClientOptions(starmap.WithCatalogStore(store))}
			seed := openTestRuntime(t, options...)
			orphan := stageCollectionInput(t, seed.store, collectionObservation(t, 1))
			unknown := stageCollectionInput(t, seed.store, map[string]any{"version": 99, "operator": "keep"})
			if err := seed.Close(); err != nil {
				t.Fatal(err)
			}
			policy := DefaultRetentionPolicy()
			policy.MaxGenerations, policy.MaxBytes, policy.Interval = 1, 1, time.Hour
			options = append(options, WithRetentionPolicy(policy))
			if mode == "offline" {
				options = append(options, WithCatalogNetworkMode("offline"))
			}
			if mode == "pinned" {
				options = append(options, WithGenerationPin(current.Manifest.GenerationID))
			}
			r := openTestRuntime(t, options...)
			deadline := time.Now().Add(5 * time.Second)
			for r.RetentionSnapshot().AttemptedAt.IsZero() && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			report := r.RetentionSnapshot()
			if report.AttemptedAt.IsZero() || report.Reason != "required_content_exceeds_limit" || !report.OverLimit {
				t.Fatalf("automatic collection did not report protected capacity: %+v", report)
			}
			if report.RemovedGenerations != 1 || report.RemovedInputs != 1 {
				t.Fatalf("collection incomplete: %+v", report)
			}
			if _, err := store.Get(t.Context(), old.Manifest.GenerationID); !errors.IsNotFound(err) {
				t.Fatalf("old generation remains: %v", err)
			}
			if _, err := store.Get(t.Context(), current.Manifest.GenerationID); err != nil {
				t.Fatal(err)
			}
			inputs, err := r.store.directory.ExistingChild(inputPublicationDirectory)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := inputs.ReadFile(orphan, maxLayerBytes); !os.IsNotExist(err) {
				t.Fatalf("orphan remains: %v", err)
			}
			if _, err := inputs.ReadFile(unknown, maxLayerBytes); err != nil {
				t.Fatalf("unknown input changed: %v", err)
			}
			if r.State().GenerationID != current.Manifest.GenerationID {
				t.Fatal("cleanup replaced the served catalog")
			}
			if err := r.Close(); err != nil {
				t.Fatal(err)
			}
			reopened := openTestRuntime(t, append(options, WithRetentionEnabled(false))...)
			if reopened.State().GenerationID != current.Manifest.GenerationID {
				t.Fatal("restart lost the required catalog")
			}
		})
	}
}

func TestRetentionRefusesUnresolvedPublicationAndCancellation(t *testing.T) {
	store := storage.NewMemory()
	old, current := aliasGeneration(t, "retention-old"), aliasGeneration(t, "retention-current")
	if err := store.Commit(t.Context(), old, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(t.Context(), current, old.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	r := openTestRuntime(t, WithCatalogSource("embedded"), WithClientOptions(starmap.WithCatalogStore(store)),
		WithRetentionMaxGenerations(1), WithRetentionMaxBytes(1))
	pending := inputPublication{Version: inputPublicationVersion, Phase: inputPublicationPrepared,
		GenerationID: "pending", PayloadChecksum: current.Manifest.Payload.Checksum,
		ExpectedID: current.Manifest.GenerationID, ExpectedChecksum: current.Manifest.Payload.Checksum}
	pending.Source = stageCollectionInput(t, r.store, sourceLayer{Identity: "pending-source", GenerationID: old.Manifest.GenerationID,
		Checksum: old.Manifest.Payload.Checksum, Payload: old.Payload})
	if err := r.store.writeInputPublication(t.Context(), pending); err != nil {
		t.Fatal(err)
	}
	if err := r.collectRetainedState(t.Context()); err == nil {
		t.Fatal("pending publication permitted generation collection")
	}
	if report := r.RetentionSnapshot(); report.Reason != "publication_recovery_pending" {
		t.Fatalf("wrong failure: %+v", report)
	}
	if _, err := store.Get(t.Context(), old.Manifest.GenerationID); err != nil {
		t.Fatal("pending recovery lost a generation", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := r.collectRetainedState(ctx); err != context.Canceled {
		t.Fatalf("cancellation ignored: %v", err)
	}
	if _, err := store.Get(t.Context(), old.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
}

func TestRetentionPolicyRejectsInvalidLimits(t *testing.T) {
	for _, change := range []func(*RetentionPolicy){
		func(p *RetentionPolicy) { p.Interval = 0 },
		func(p *RetentionPolicy) { p.MaxGenerations = 0 },
		func(p *RetentionPolicy) { p.MaxBytes = 0 },
		func(p *RetentionPolicy) { p.InputMaxBytes = 0 },
		func(p *RetentionPolicy) { p.ScanEntries = storage.MaxRetentionScanEntries + 1 },
	} {
		policy := DefaultRetentionPolicy()
		change(&policy)
		policy.Enabled = false
		if policy.Validate() == nil {
			t.Fatal("disabled scheduling accepted invalid limits")
		}
	}
}

func TestAutomaticRetentionRepeatsAndStopsAtShutdown(t *testing.T) {
	timers := make(chan chan time.Time, 4)
	r := openTestRuntime(t, WithCatalogSource("embedded"), WithClientOptions(starmap.WithCatalogStore(storage.NewMemory())),
		WithRetentionEnabled(true), func(config *options) error {
			config.scheduleTimer = func(wait time.Duration) <-chan time.Time {
				fire := make(chan time.Time, 1)
				timers <- fire
				return fire
			}
			return nil
		})
	next := func() chan time.Time {
		t.Helper()
		select {
		case fire := <-timers:
			return fire
		case <-time.After(5 * time.Second):
			t.Fatal("retention did not schedule its next pass")
			return nil
		}
	}
	first := next()
	orphan := stageCollectionInput(t, r.store, collectionObservation(t, 3))
	first <- time.Now()
	second := next()
	if r.RetentionSnapshot().RemovedInputs != 1 {
		t.Fatal("periodic pass did not remove new orphan input")
	}
	inputs, err := r.store.directory.ExistingChild(inputPublicationDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inputs.ReadFile(orphan, maxLayerBytes); !os.IsNotExist(err) {
		t.Fatal("periodic orphan remains", err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	before := r.RetentionSnapshot()
	second <- time.Now()
	if r.RetentionSnapshot() != before {
		t.Fatal("shutdown changed the last retention report")
	}
}

func TestRetentionReportsSharedCoordinationWithoutDeletingGenerations(t *testing.T) {
	store := storage.NewMemory()
	old, current := aliasGeneration(t, "shared-old"), aliasGeneration(t, "shared-current")
	if err := store.Commit(t.Context(), old, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(t.Context(), current, old.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	r := openTestRuntime(t, WithCatalogSource("embedded"), WithClientOptions(starmap.WithCatalogStore(store)),
		WithLeaseStore(&stubLeaseStore{}), WithRetentionMaxGenerations(1), WithRetentionMaxBytes(1))
	orphan := stageCollectionInput(t, r.store, collectionObservation(t, 4))
	if err := r.collectRetainedState(t.Context()); err != nil {
		t.Fatal(err)
	}
	report := r.RetentionSnapshot()
	if report.GenerationCollection != "shared_coordination_required" || report.Health != HealthDegraded || report.RemovedInputs != 1 {
		t.Fatalf("shared limitation or local progress missing: %+v", report)
	}
	if _, err := store.Get(t.Context(), old.Manifest.GenerationID); err != nil {
		t.Fatal("uncoordinated shared deletion", err)
	}
	inputs, err := r.store.directory.ExistingChild(inputPublicationDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inputs.ReadFile(orphan, maxLayerBytes); !os.IsNotExist(err) {
		t.Fatal("local collection did not proceed", err)
	}
}

func TestAutomaticRetentionKeepsOriginalPinAfterOriginRestart(t *testing.T) {
	store := storage.NewMemory()
	source := newStubSource("retained-pin")
	source.replies = []SourceRead{aliasRead(aliasGeneration(t, "pin-first"))}
	options := []Option{WithStateDirectory(privateRuntimeDirectory(t)), WithCatalogSource("embedded"),
		WithSource(source), WithSourceRefreshMode("manual"), WithAuthorityOrigin(store, originTestConfig())}
	r := openTestRuntime(t, options...)
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	selected := r.State().GenerationID
	source.mu.Lock()
	source.replies = []SourceRead{aliasRead(aliasGeneration(t, "pin-next"))}
	source.mu.Unlock()
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		pinned := openTestRuntime(t, append(options, WithGenerationPin(selected), WithRetentionEnabled(true),
			WithRetentionMaxGenerations(1), WithRetentionMaxBytes(1))...)
		deadline := time.Now().Add(5 * time.Second)
		for pinned.RetentionSnapshot().AttemptedAt.IsZero() && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		report := pinned.RetentionSnapshot()
		if report.AttemptedAt.IsZero() || report.GenerationCollection != "supported" || !report.OverLimit {
			t.Fatalf("origin did not complete guarded collection: %+v", report)
		}
		if _, err := store.Get(t.Context(), selected); err != nil {
			t.Fatal("original pin missing", err)
		}
		if _, err := store.Get(t.Context(), pinned.State().GenerationID); err != nil {
			t.Fatal("served pin missing", err)
		}
		if err := pinned.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
