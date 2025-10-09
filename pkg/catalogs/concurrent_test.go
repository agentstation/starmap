package catalogs_test

import (
	"context"
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
func addModelToProvider(catalog catalogs.Catalog, providerID string, model catalogs.Model) error {
	// First ensure the provider exists
	_, err := catalog.Provider(catalogs.ProviderID(providerID))
	if err != nil {
		// Create provider if it doesn't exist
		provider := catalogs.Provider{
			ID:     catalogs.ProviderID(providerID),
			Name:   providerID,
			Models: make(map[string]*catalogs.Model),
		}
		if err := catalog.SetProvider(provider); err != nil {
			return err
		}
	}

	// Use the thread-safe SetModel method on providers
	return catalog.Providers().SetModel(catalogs.ProviderID(providerID), model)
}

// Helper function to delete a model from a provider.
func deleteModelFromProvider(catalog catalogs.Catalog, providerID string, modelID string) error {
	// Use the thread-safe DeleteModel method on providers
	return catalog.Providers().DeleteModel(catalogs.ProviderID(providerID), modelID)
}

// TestConcurrentCatalogAccess tests thread safety with multiple readers and writers.
func TestConcurrentCatalogAccess(t *testing.T) {
	t.Run("concurrent_reads_and_writes", func(t *testing.T) {
		catalog := catalogs.Empty()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var wg sync.WaitGroup
		errors := make(chan error, 1000)

		// Track operations
		var reads, writes atomic.Int64

		// 50 concurrent readers
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					default:
						// Random read operations
						switch id % 4 {
						case 0:
							_ = catalog.Providers().List()
						case 1:
							_ = catalog.Authors().List()
						case 2:
							_ = catalog.Endpoints().List()
						}
						reads.Add(1)
						time.Sleep(time.Millisecond) // Small delay
					}
				}
			}(i)
		}

		// 10 concurrent writers
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					select {
					case <-ctx.Done():
						return
					default:
						// Create unique models
						model := catalogs.Model{
							ID:   fmt.Sprintf("model-%d-%d", id, j),
							Name: fmt.Sprintf("Model %d-%d", id, j),
						}
						if err := addModelToProvider(catalog, "test-provider", model); err != nil {
							errors <- err
						}
						writes.Add(1)
						time.Sleep(5 * time.Millisecond) // Writers are slower
					}
				}
			}(i)
		}

		// Wait for completion
		wg.Wait()
		close(errors)

		// Check for errors
		for err := range errors {
			t.Errorf("Concurrent access error: %v", err)
		}

		// Verify operations completed
		t.Logf("Completed %d reads and %d writes", reads.Load(), writes.Load())
		assert.Greater(t, reads.Load(), int64(100))
		assert.Greater(t, writes.Load(), int64(100))
	})

	t.Run("concurrent_merge_operations", func(t *testing.T) {
		base := catalogs.Empty()
		var wg sync.WaitGroup
		errors := make(chan error, 100)

		// Add initial data
		for i := 0; i < 10; i++ {
			err := addModelToProvider(base, "test-provider", catalogs.Model{
				ID:   fmt.Sprintf("model-%d", i),
				Name: fmt.Sprintf("Model %d", i),
			})
			require.NoError(t, err)
		}

		// Multiple concurrent mergers
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				// Create update catalog
				updates := catalogs.Empty()
				for j := 0; j < 5; j++ {
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
		catalog := catalogs.Empty()
		numProviders := 20
		numUpdates := 50

		var wg sync.WaitGroup
		errors := make(chan error, numProviders*numUpdates)

		// Each goroutine updates its own provider repeatedly
		for i := 0; i < numProviders; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				providerID := catalogs.ProviderID(fmt.Sprintf("provider-%d", id))

				for j := 0; j < numUpdates; j++ {
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
		catalog := catalogs.Empty()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var wg sync.WaitGroup
		readErrors := make(chan error, 100)

		// Start continuous readers
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					default:
						// Should never panic or error
						models := catalog.Models().List()
						if models == nil {
							readErrors <- fmt.Errorf("got nil models list")
						}
					}
				}
			}()
		}

		// Perform bulk write
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				model := catalogs.Model{
					ID:   fmt.Sprintf("bulk-model-%d", i),
					Name: fmt.Sprintf("Bulk Model %d", i),
				}
				err := addModelToProvider(catalog, "test-provider", model)
				assert.NoError(t, err)
			}
		}()

		// Let it run for a bit
		time.Sleep(2 * time.Second)
		cancel()
		wg.Wait()
		close(readErrors)

		// Check for read errors
		for err := range readErrors {
			t.Errorf("Read error during bulk write: %v", err)
		}

		// Verify bulk write succeeded
		models := catalog.Models().List()
		assert.GreaterOrEqual(t, len(models), 1000)
	})

	t.Run("concurrent_copy_operations", func(t *testing.T) {
		source := catalogs.Empty()

		// Add test data
		for i := 0; i < 100; i++ {
			err := addModelToProvider(source, "test-provider", catalogs.Model{
				ID:   fmt.Sprintf("model-%d", i),
				Name: fmt.Sprintf("Model %d", i),
			})
			require.NoError(t, err)
		}

		var wg sync.WaitGroup
		copies := make([]catalogs.Catalog, 10)

		// Multiple concurrent copies
		for i := 0; i < 10; i++ {
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
			models := copy.Models().List()
			assert.Len(t, models, 100, "Copy %d has wrong number of models", i)

			// Modify copy shouldn't affect others
			err := addModelToProvider(copy, "test-provider", catalogs.Model{
				ID:   fmt.Sprintf("copy-%d-exclusive", i),
				Name: fmt.Sprintf("Copy %d Exclusive", i),
			})
			assert.NoError(t, err)
		}

		// Verify copies are independent
		for i, copy := range copies {
			models := copy.Models().List()
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
		if testing.Short() {
			t.Skip("Skipping race detection in short mode")
		}

		catalog := catalogs.Empty()
		modelID := "race-model"

		var wg sync.WaitGroup
		updates := 100

		// Two goroutines racing to update the same model
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(writer int) {
				defer wg.Done()
				for j := 0; j < updates; j++ {
					model := catalogs.Model{
						ID:          modelID,
						Name:        fmt.Sprintf("Model by writer %d iteration %d", writer, j),
						Description: fmt.Sprintf("Updated at %v by writer %d", time.Now(), writer),
					}
					_ = addModelToProvider(catalog, "test-provider", model) // Ignore errors, we're testing races
				}
			}(i)
		}

		wg.Wait()

		// The model should exist with data from one of the writers
		model, err := catalog.FindModel(modelID)
		assert.NoError(t, err)
		assert.NotEmpty(t, model.Name)
		assert.NotNil(t, model.Description)
	})

	t.Run("deadlock_prevention", func(t *testing.T) {
		catalog1 := catalogs.Empty()
		catalog2 := catalogs.Empty()

		// Setup initial data
		for i := 0; i < 10; i++ {
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
			for i := 0; i < 100; i++ {
				models := catalog1.Models().List()
				for _, model := range models {
					_ = addModelToProvider(catalog2, "test-provider", model)
				}
			}
			done <- true
		}()

		// Goroutine 2: Copy from catalog2 to catalog1
		go func() {
			for i := 0; i < 100; i++ {
				models := catalog2.Models().List()
				for _, model := range models {
					_ = addModelToProvider(catalog1, "test-provider", model)
				}
			}
			done <- true
		}()

		// Wait with timeout to detect deadlock
		timeout := time.After(5 * time.Second)
		for i := 0; i < 2; i++ {
			select {
			case <-done:
				// Success
			case <-timeout:
				t.Fatal("Deadlock detected - operations did not complete in time")
			}
		}
	})

	t.Run("concurrent_delete_operations", func(t *testing.T) {
		catalog := catalogs.Empty()
		numWorkers := 10
		modelsPerWorker := 100

		// Add models to different providers (one provider per worker)
		for worker := 0; worker < numWorkers; worker++ {
			providerID := fmt.Sprintf("provider-%d", worker)
			for i := 0; i < modelsPerWorker; i++ {
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
		for worker := 0; worker < numWorkers; worker++ {
			wg.Add(1)
			go func(w int) {
				defer wg.Done()
				providerID := fmt.Sprintf("provider-%d", w)

				for i := 0; i < modelsPerWorker; i++ {
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

		// All models should be deleted
		models := catalog.Models().List()
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
		catalog := catalogs.Empty()
		// Pre-populate
		for i := 0; i < 1000; i++ {
			addModelToProvider(catalog, "test-provider", catalogs.Model{
				ID:   fmt.Sprintf("model-%d", i),
				Name: fmt.Sprintf("Model %d", i),
			})
		}

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = catalog.Models().List()
			}
		})
	})

	b.Run("concurrent_writes", func(b *testing.B) {
		catalog := catalogs.Empty()
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
		catalog := catalogs.Empty()
		counter := atomic.Int64{}

		// Pre-populate
		for i := 0; i < 100; i++ {
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
					_ = catalog.Models().List()
				}
				i++
			}
		})
	})
}
