package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	catalogerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestProviderRetentionRefusesTimeRegression(t *testing.T) {
	for _, durable := range []bool{false, true} {
		for _, conflict := range []bool{false, true} {
			t.Run(fmt.Sprintf("durable=%t/conflict=%t", durable, conflict), func(t *testing.T) {
				r := providerOrderRuntime(t, durable)
				at := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
				current := testProviderLayer(t, "provider", "current", "Current", at)
				if err := r.retainProviders(t.Context(), []ProviderLayer{current}); err != nil {
					t.Fatal(err)
				}
				if durable {
					loaded, err := r.store.loadProviders()
					if err != nil {
						t.Fatal(err)
					}
					r = &Runtime{store: r.store, layers: layerSet{providers: loaded}}
				}
				when := at.Add(-time.Second)
				if conflict {
					when = at.In(time.FixedZone("equivalent", 3600))
				}
				rejected := testProviderLayer(t, "provider", "other", "Other", when)
				first := testProviderLayer(t, "another", "valid", "Valid", at.Add(time.Second))
				err := r.retainProviders(t.Context(), []ProviderLayer{first, rejected})
				var target *catalogerrors.ConflictError
				if !errors.As(err, &target) {
					t.Fatalf("retention error = %v, want typed conflict", err)
				}
				assertRetainedProvider(t, r, current)
			})
		}
	}
}

func TestProviderRetentionRejectsAmbiguousBatch(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprintf("reverse=%t", reverse), func(t *testing.T) {
			r := providerOrderRuntime(t, true)
			at := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
			batch := []ProviderLayer{
				testProviderLayer(t, "provider", "first", "First", at),
				testProviderLayer(t, "provider", "second", "Second", at),
			}
			if reverse {
				slices.Reverse(batch)
			}
			var target *catalogerrors.ConflictError
			if err := r.retainProviders(t.Context(), batch); !errors.As(err, &target) {
				t.Fatalf("retention error = %v, want typed conflict", err)
			}
			if len(r.layers.providers) != 0 {
				t.Fatal("ambiguous batch changed memory")
			}
			retained, err := r.store.loadProviders()
			if err != nil || len(retained) != 0 {
				t.Fatalf("ambiguous batch changed durable state: %v", err)
			}
		})
	}
}

func TestProviderRetentionSelectsNewestBatchOrder(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprintf("reverse=%t", reverse), func(t *testing.T) {
			r := providerOrderRuntime(t, true)
			at := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
			older := testProviderLayer(t, "provider", "older", "Older", at)
			newer := testProviderLayer(t, "provider", "newer", "Newer", at.Add(time.Second))
			batch := []ProviderLayer{older, older, newer}
			if reverse {
				slices.Reverse(batch)
			}
			if err := r.retainProviders(t.Context(), batch); err != nil {
				t.Fatal(err)
			}
			assertRetainedProvider(t, r, newer)
			path := filepath.Join(r.store.root, providerLayerDirectoryName, "provider.json")
			before, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := r.retainProviders(t.Context(), []ProviderLayer{newer, newer}); err != nil {
				t.Fatal(err)
			}
			after, err := os.Stat(path)
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("identical observation rewrote durable evidence: %v", err)
			}
		})
	}
}

func TestProviderRetentionConcurrentUpdatesKeepNewest(t *testing.T) {
	r := providerOrderRuntime(t, true)
	at := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	layers := make([]ProviderLayer, 16)
	for index := range layers {
		layers[index] = testProviderLayer(t, "provider", fmt.Sprint(index), "Model", at.Add(time.Duration(index)*time.Second))
	}
	start := make(chan struct{})
	results := make(chan error, len(layers))
	var wg sync.WaitGroup
	for _, layer := range layers {
		wg.Go(func() {
			<-start
			results <- r.retainProviders(t.Context(), []ProviderLayer{layer})
		})
	}
	close(start)
	wg.Wait()
	close(results)
	for err := range results {
		var conflict *catalogerrors.ConflictError
		if err != nil && !errors.As(err, &conflict) {
			t.Errorf("unexpected retention failure: %v", err)
		}
	}
	assertRetainedProvider(t, r, layers[len(layers)-1])
}

func providerOrderRuntime(t *testing.T, durable bool) *Runtime {
	t.Helper()
	path := ""
	if durable {
		path = t.TempDir()
	}
	store, err := newLayerStore(path)
	if err != nil {
		t.Fatal(err)
	}
	return &Runtime{store: store}
}

func assertRetainedProvider(t *testing.T, r *Runtime, want ProviderLayer) {
	t.Helper()
	check := func(layers map[providerEvidenceKey]ProviderLayer) {
		got := layers[want.evidenceKey()]
		if len(layers) != 1 || got.Digest != want.Digest || !got.ObservedAt.Equal(want.ObservedAt) || !bytes.Equal(got.Payload, want.Payload) {
			t.Error("retained evidence differs from the expected newest observation")
		}
	}
	check(r.layers.providers)
	if r.store.durable() {
		loaded, err := r.store.loadProviders()
		if err != nil {
			t.Fatal(err)
		}
		check(loaded)
	}
}

func TestProviderRetentionDoesNotHideRetainedConflict(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprintf("reverse=%t", reverse), func(t *testing.T) {
			r := providerOrderRuntime(t, true)
			at := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
			current := testProviderLayer(t, "provider", "current", "Current", at)
			if err := r.retainProviders(t.Context(), []ProviderLayer{current}); err != nil {
				t.Fatal(err)
			}
			batch := []ProviderLayer{
				testProviderLayer(t, "provider", "conflict", "Conflict", at),
				testProviderLayer(t, "provider", "newer", "Newer", at.Add(time.Second)),
			}
			if reverse {
				slices.Reverse(batch)
			}
			var target *catalogerrors.ConflictError
			if err := r.retainProviders(t.Context(), batch); !errors.As(err, &target) {
				t.Fatalf("retention error = %v, want typed conflict", err)
			}
			assertRetainedProvider(t, r, current)
		})
	}
}

func TestProviderRetentionCancellationAfterPublicationCheck(t *testing.T) {
	r := providerOrderRuntime(t, true)
	layer := testProviderLayer(t, "provider", "model", "Model", time.Now().UTC())
	parent, cancel := context.WithCancel(t.Context())
	defer cancel()
	ctx := &retentionCheckContext{Context: parent, checked: make(chan struct{}), resume: make(chan struct{})}
	done := make(chan error, 1)
	go func() {
		_, err := r.publishProviders(ctx, []ProviderLayer{layer}, 0)
		done <- err
	}()
	<-ctx.checked
	cancel()
	close(ctx.resume)
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("publication error = %v, want cancellation", err)
	}
	if len(r.layers.providers) != 0 {
		t.Error("canceled publication retained provider evidence in memory")
	}
	loaded, err := r.store.loadProviders()
	if err != nil || len(loaded) != 0 {
		t.Errorf("canceled publication retained provider evidence on disk: %v", err)
	}
}

type retentionCheckContext struct {
	context.Context
	once    sync.Once
	checked chan struct{}
	resume  chan struct{}
}

func (c *retentionCheckContext) Err() error {
	err := c.Context.Err()
	c.once.Do(func() {
		close(c.checked)
		<-c.resume
	})
	return err
}
