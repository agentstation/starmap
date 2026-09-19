package catalogs_test

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// Helper function to add a model to a catalog through a provider.
func addModelToProvider(catalog *catalogs.Builder, providerID string, model catalogs.Model) error {
	// First verify the provider exists
	_, err := catalog.Provider(catalogs.ProviderID(providerID))
	if err != nil {
		// Create provider if it does not exist
		provider := catalogs.Provider{
			ID:     catalogs.ProviderID(providerID),
			Name:   providerID,
			Models: make(map[string]*catalogs.Model),
		}
		if err := catalog.SetProvider(provider); err != nil {
			return err
		}
	}

	return catalog.SetProviderModel(catalogs.ProviderID(providerID), model)
}

// Helper function to delete a model from a provider.
func deleteModelFromProvider(catalog *catalogs.Builder, providerID string, modelID string) error {
	return catalog.DeleteProviderModel(catalogs.ProviderID(providerID), modelID)
}

func builderProviderModels(catalog *catalogs.Builder, providerID catalogs.ProviderID) []catalogs.Model {
	models, err := catalog.ProviderModels(providerID)
	if err != nil {
		return []catalogs.Model{}
	}
	return models.List()
}

func allBuilderProviderModels(catalog *catalogs.Builder) []catalogs.Model {
	models := make([]catalogs.Model, 0)
	for _, provider := range catalog.Providers().List() {
		models = append(models, builderProviderModels(catalog, provider.ID)...)
	}
	return models
}

// TestConcurrentCatalogAccess tests thread safety with multiple readers and writers.
func TestConcurrentCatalogAccess(t *testing.T) {
	t.Run("concurrent_reads_and_writes", func(t *testing.T) {
		catalog := catalogs.NewEmpty()
		require.NoError(t, catalog.SetProvider(catalogs.Provider{ID: "test-provider", Name: "Test"}))
		start := make(chan struct{})
		var workers sync.WaitGroup
		var reads, writes atomic.Int64

		for id := range 50 {
			workers.Go(func() {
				<-start
				for range 100 {
					switch id % 3 {
					case 0:
						_ = catalog.Providers().List()
					case 1:
						_ = catalog.Authors().List()
					case 2:
						_ = catalog.AuthoredModels()
					}
					reads.Add(1)
				}
			})
		}
		for id := range 10 {
			workers.Go(func() {
				<-start
				for iteration := range 100 {
					model := catalogs.Model{
						ID:   fmt.Sprintf("model-%d-%d", id, iteration),
						Name: fmt.Sprintf("Model %d-%d", id, iteration),
					}
					if err := catalog.SetProviderModel("test-provider", model); err != nil {
						t.Errorf("SetProviderModel: %v", err)
						return
					}
					writes.Add(1)
				}
			})
		}
		close(start)
		workers.Wait()
		assert.Equal(t, int64(5000), reads.Load())
		assert.Equal(t, int64(1000), writes.Load())
		models, err := catalog.ProviderModels("test-provider")
		require.NoError(t, err)
		assert.Len(t, models.List(), 1000)
	})

	t.Run("concurrent_merge_operations", func(t *testing.T) {
		base := catalogs.NewEmpty()
		var wg sync.WaitGroup
		errors := make(chan error, 100)

		// Add initial data
		for i := range 10 {
			err := addModelToProvider(base, "test-provider", catalogs.Model{
				ID:   fmt.Sprintf("model-%d", i),
				Name: fmt.Sprintf("Model %d", i),
			})
			require.NoError(t, err)
		}

		// Multiple concurrent mergers
		for i := range 5 {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				// Create update catalog
				updates := catalogs.NewEmpty()
				for j := range 5 {
					model := catalogs.Model{
						ID:          fmt.Sprintf("model-%d", j),
						Name:        fmt.Sprintf("Updated Model %d by merger %d", j, id),
						Description: fmt.Sprintf("Updated by merger %d", id),
					}
					if err := addModelToProvider(updates, "test-provider", model); err != nil {
						errors <- err
						return
					}
				}

				// Merge with base
				if err := base.MergeWith(updates); err != nil {
					errors <- err
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for errors
		for err := range errors {
			t.Errorf("Merge error: %v", err)
		}
	})

	t.Run("concurrent_provider_updates", func(t *testing.T) {
		catalog := catalogs.NewEmpty()
		numProviders := 20
		numUpdates := 50

		var wg sync.WaitGroup
		errors := make(chan error, numProviders*numUpdates)

		// Each goroutine updates its own provider repeatedly
		for i := range numProviders {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				providerID := catalogs.ProviderID(fmt.Sprintf("provider-%d", id))

				for j := range numUpdates {
					provider := catalogs.Provider{
						ID:   providerID,
						Name: fmt.Sprintf("Provider %d v%d", id, j),
					}
					if err := catalog.SetProvider(provider); err != nil {
						errors <- err
					}
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for errors
		for err := range errors {
			t.Errorf("Provider update error: %v", err)
		}

		// Verify all providers exist
		providers := catalog.Providers().List()
		assert.Len(t, providers, numProviders)
	})

	t.Run("readers_during_bulk_write", func(t *testing.T) {
		catalog := catalogs.NewEmpty()
		require.NoError(t, catalog.SetProvider(catalogs.Provider{ID: "test-provider", Name: "Test"}))
		var ready, readers sync.WaitGroup
		ready.Add(10)
		done := make(chan struct{})
		stop := sync.OnceFunc(func() { close(done) })
		defer func() { stop(); readers.Wait() }()
		for range 10 {
			readers.Go(func() {
				first := true
				for {
					models, err := catalog.ProviderModels("test-provider")
					if err != nil {
						t.Errorf("ProviderModels: %v", err)
					} else if models == nil {
						t.Error("ProviderModels returned nil")
					}
					if first {
						ready.Done()
						first = false
					}
					select {
					case <-done:
						return
					default:
					}
				}
			})
		}
		ready.Wait()
		for i := range 1000 {
			require.NoError(t, catalog.SetProviderModel("test-provider", catalogs.Model{
				ID: fmt.Sprintf("bulk-model-%d", i), Name: fmt.Sprintf("Bulk Model %d", i),
			}))
		}
		stop()
		readers.Wait()
		models, err := catalog.ProviderModels("test-provider")
		require.NoError(t, err)
		assert.Len(t, models.List(), 1000)
	})

	t.Run("concurrent_copy_operations", func(t *testing.T) {
		source := catalogs.NewEmpty()

		// Add test data
		for i := range 100 {
			err := addModelToProvider(source, "test-provider", catalogs.Model{
				ID:   fmt.Sprintf("model-%d", i),
				Name: fmt.Sprintf("Model %d", i),
			})
			require.NoError(t, err)
		}

		var wg sync.WaitGroup
		copies := make([]*catalogs.Builder, 10)

		// Multiple concurrent copies
		for i := range 10 {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				copy, err := source.Copy()
				assert.NoError(t, err)
				copies[idx] = copy
			}(i)
		}

		wg.Wait()

		// Verify all copies are independent and complete
		for i, copy := range copies {
			assert.NotNil(t, copy, "Copy %d is nil", i)
			models := builderProviderModels(copy, "test-provider")
			assert.Len(t, models, 100, "Copy %d has wrong number of models", i)

			// Modify copy should not affect others
			err := addModelToProvider(copy, "test-provider", catalogs.Model{
				ID:   fmt.Sprintf("copy-%d-exclusive", i),
				Name: fmt.Sprintf("Copy %d Exclusive", i),
			})
			assert.NoError(t, err)
		}

		// Verify copies are independent
		for i, copy := range copies {
			models := builderProviderModels(copy, "test-provider")
			exclusiveCount := 0
			for _, model := range models {
				if model.ID == fmt.Sprintf("copy-%d-exclusive", i) {
					exclusiveCount++
				}
			}
			assert.Equal(t, 1, exclusiveCount, "Copy %d should have exactly one exclusive model", i)
		}
	})

	t.Run("race_condition_detection", func(t *testing.T) {
		catalog := catalogs.NewEmpty()
		require.NoError(t, catalog.SetProvider(catalogs.Provider{ID: "test-provider", Name: "Test"}))
		modelID := "race-model"

		var wg sync.WaitGroup
		updates := 100

		// Two goroutines racing to update the same model
		for i := range 2 {
			wg.Add(1)
			go func(writer int) {
				defer wg.Done()
				for j := range updates {
					model := catalogs.Model{
						ID:          modelID,
						Name:        fmt.Sprintf("Model by writer %d iteration %d", writer, j),
						Description: fmt.Sprintf("Writer %d iteration %d", writer, j),
					}
					if err := catalog.SetProviderModel("test-provider", model); err != nil {
						t.Errorf("SetProviderModel: %v", err)
						return
					}
				}
			}(i)
		}

		wg.Wait()

		// The model should exist with data from one of the writers
		model, err := catalog.ProviderModel("test-provider", modelID)
		require.NoError(t, err)
		for writer := range 2 {
			if model.Name == fmt.Sprintf("Model by writer %d iteration %d", writer, updates-1) {
				assert.Equal(t, fmt.Sprintf("Writer %d iteration %d", writer, updates-1), model.Description)
				return
			}
		}
		t.Fatalf("concurrent writers left an incomplete or mixed model: %+v", model)
	})

	t.Run("deadlock_prevention", func(t *testing.T) {
		catalog1 := catalogs.NewEmpty()
		catalog2 := catalogs.NewEmpty()

		// Setup initial data
		for i := range 10 {
			model := catalogs.Model{
				ID:   fmt.Sprintf("model-%d", i),
				Name: fmt.Sprintf("Model %d", i),
			}
			assert.NoError(t, addModelToProvider(catalog1, "test-provider", model))
			assert.NoError(t, addModelToProvider(catalog2, "test-provider", model))
		}

		done := make(chan bool, 2)

		// Goroutine 1: Copy from catalog1 to catalog2
		go func() {
			for range 100 {
				models := builderProviderModels(catalog1, "test-provider")
				for _, model := range models {
					_ = addModelToProvider(catalog2, "test-provider", model)
				}
			}
			done <- true
		}()

		// Goroutine 2: Copy from catalog2 to catalog1
		go func() {
			for range 100 {
				models := builderProviderModels(catalog2, "test-provider")
				for _, model := range models {
					_ = addModelToProvider(catalog1, "test-provider", model)
				}
			}
			done <- true
		}()

		// Wait with timeout to detect deadlock
		timeout := time.After(5 * time.Second)
		for range 2 {
			select {
			case <-done:
				// Success
			case <-timeout:
				t.Fatal("Deadlock detected - operations did not complete in time")
			}
		}
	})

	t.Run("concurrent_delete_operations", func(t *testing.T) {
		catalog := catalogs.NewEmpty()
		numWorkers := 10
		modelsPerWorker := 100

		// Add models to different providers (one provider per worker)
		for worker := range numWorkers {
			providerID := fmt.Sprintf("provider-%d", worker)
			for i := range modelsPerWorker {
				err := addModelToProvider(catalog, providerID, catalogs.Model{
					ID:   fmt.Sprintf("model-%d-%d", worker, i),
					Name: fmt.Sprintf("Model %d-%d", worker, i),
				})
				require.NoError(t, err)
			}
		}

		var wg sync.WaitGroup
		errors := make(chan error, numWorkers*modelsPerWorker)

		// Concurrent deletions - each worker deletes from their own provider
		for worker := range numWorkers {
			wg.Add(1)
			go func(w int) {
				defer wg.Done()
				providerID := fmt.Sprintf("provider-%d", w)

				for i := range modelsPerWorker {
					modelID := fmt.Sprintf("model-%d-%d", w, i)
					if err := deleteModelFromProvider(catalog, providerID, modelID); err != nil {
						// Should not fail since each worker has their own provider
						errors <- fmt.Errorf("worker %d failed to delete %s: %v", w, modelID, err)
					}
				}
			}(worker)
		}

		wg.Wait()
		close(errors)

		// Check for unexpected errors
		for err := range errors {
			t.Errorf("Delete error: %v", err)
		}

		models := allBuilderProviderModels(catalog)
		if len(models) > 0 {
			t.Errorf("Expected 0 models, got %d", len(models))
			for _, m := range models {
				t.Logf("Remaining model: %s", m.ID)
			}
		}
		assert.Empty(t, models)
	})
}

// BenchmarkConcurrentAccess benchmarks concurrent operations.
func BenchmarkConcurrentAccess(b *testing.B) {
	b.Run("concurrent_reads", func(b *testing.B) {
		catalog := catalogs.NewEmpty()
		// Pre-populate
		for i := range 1000 {
			addModelToProvider(catalog, "test-provider", catalogs.Model{
				ID:   fmt.Sprintf("model-%d", i),
				Name: fmt.Sprintf("Model %d", i),
			})
		}

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = builderProviderModels(catalog, "test-provider")
			}
		})
	})

	b.Run("concurrent_writes", func(b *testing.B) {
		catalog := catalogs.NewEmpty()
		counter := atomic.Int64{}

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				id := counter.Add(1)
				addModelToProvider(catalog, "test-provider", catalogs.Model{
					ID:   fmt.Sprintf("model-%d", id),
					Name: fmt.Sprintf("Model %d", id),
				})
			}
		})
	})

	b.Run("concurrent_mixed", func(b *testing.B) {
		catalog := catalogs.NewEmpty()
		counter := atomic.Int64{}

		// Pre-populate
		for i := range 100 {
			addModelToProvider(catalog, "test-provider", catalogs.Model{
				ID:   fmt.Sprintf("model-%d", i),
				Name: fmt.Sprintf("Model %d", i),
			})
		}

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				if i%10 == 0 {
					// Write operation (10%)
					id := counter.Add(1)
					addModelToProvider(catalog, "test-provider", catalogs.Model{
						ID:   fmt.Sprintf("bench-model-%d", id),
						Name: fmt.Sprintf("Bench Model %d", id),
					})
				} else {
					// Read operation (90%)
					_ = builderProviderModels(catalog, "test-provider")
				}
				i++
			}
		})
	})
}
