package app

import (
	"context"
	"sync"
	"testing"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// TestApp_New verifies app initialization.
func TestApp_New(t *testing.T) {
	app, err := New("1.0.0", "abc123", "2024-01-01", "test")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if app.Version() != "1.0.0" {
		t.Errorf("Version() = %s, want 1.0.0", app.Version())
	}
	if app.Commit() != "abc123" {
		t.Errorf("Commit() = %s, want abc123", app.Commit())
	}
	if app.Date() != "2024-01-01" {
		t.Errorf("Date() = %s, want 2024-01-01", app.Date())
	}
	if app.BuiltBy() != "test" {
		t.Errorf("BuiltBy() = %s, want test", app.BuiltBy())
	}
	if app.Logger() == nil {
		t.Error("Logger() returned nil")
	}
	if app.Config() == nil {
		t.Error("Config() returned nil")
	}
}

// TestApp_Starmap_Singleton verifies that Starmap() returns the same instance.
func TestApp_Starmap_Singleton(t *testing.T) {
	app, err := New("1.0.0", "test", "2024-01-01", "test")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Get starmap twice
	sm1, err := app.Starmap()
	if err != nil {
		t.Fatalf("Starmap() failed: %v", err)
	}

	sm2, err := app.Starmap()
	if err != nil {
		t.Fatalf("Starmap() failed on second call: %v", err)
	}

	// Verify it's the same instance (same pointer)
	if sm1 != sm2 {
		t.Error("Starmap() returned different instances, expected singleton")
	}
}

// TestApp_Starmap_ThreadSafe verifies concurrent Starmap() calls are safe.
func TestApp_Starmap_ThreadSafe(t *testing.T) {
	app, err := New("1.0.0", "test", "2024-01-01", "test")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	const goroutines = 100
	var wg sync.WaitGroup
	results := make([]starmap.Client, goroutines)
	errors := make([]error, goroutines)

	// Launch many goroutines to test concurrent access
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sm, err := app.Starmap()
			results[idx] = sm
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	// Verify all calls succeeded
	for i, err := range errors {
		if err != nil {
			t.Errorf("Goroutine %d: Starmap() failed: %v", i, err)
		}
	}

	// Verify all got the same instance
	first := results[0]
	for i, sm := range results[1:] {
		if sm != first {
			t.Errorf("Goroutine %d got different starmap instance", i+1)
		}
	}
}

// TestApp_Catalog_ReturnsDeepCopy verifies the CRITICAL thread safety rule.
// Per ARCHITECTURE.md § Thread Safety: "ALWAYS return deep copy" to prevent data races.
func TestApp_Catalog_ReturnsDeepCopy(t *testing.T) {
	app, err := New("1.0.0", "test", "2024-01-01", "test")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Get catalog twice
	cat1, err := app.Catalog()
	if err != nil {
		t.Fatalf("Catalog() failed: %v", err)
	}

	cat2, err := app.Catalog()
	if err != nil {
		t.Fatalf("Catalog() failed on second call: %v", err)
	}

	// CRITICAL: Verify these are different instances (deep copies)
	// We can't directly compare pointers since Catalog is an interface,
	// but we can verify that modifying one doesn't affect the other

	// Add a test provider to cat1
	testProvider := &catalogs.Provider{
		ID:   "test-provider",
		Name: "Test Provider",
	}

	if err := cat1.Providers().Set(testProvider.ID, testProvider); err != nil {
		t.Fatalf("Failed to set test provider: %v", err)
	}

	// Verify cat2 doesn't have the test provider (proving it's a copy)
	_, exists := cat2.Providers().Get("test-provider")
	if exists {
		t.Error("Catalog() did not return deep copy - mutation affected other instance!")
		t.Error("This is a CRITICAL thread safety violation per ARCHITECTURE.md § Thread Safety")
	}
}

// TestApp_Catalog_ThreadSafe verifies concurrent Catalog() calls are safe.
func TestApp_Catalog_ThreadSafe(t *testing.T) {
	app, err := New("1.0.0", "test", "2024-01-01", "test")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	const goroutines = 50
	var wg sync.WaitGroup
	errors := make([]error, goroutines)

	// Launch many goroutines that all get and mutate catalogs
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			cat, err := app.Catalog()
			if err != nil {
				errors[idx] = err
				return
			}

			// Try to mutate the catalog (should be safe since it's a copy)
			providerID := catalogs.ProviderID("test-" + string(rune(idx)))
			testProvider := &catalogs.Provider{
				ID:   providerID,
				Name: "Test",
			}
			if err := cat.Providers().Set(providerID, testProvider); err != nil {
				errors[idx] = err
			}
		}(i)
	}

	wg.Wait()

	// Verify all calls succeeded
	for i, err := range errors {
		if err != nil {
			t.Errorf("Goroutine %d: Catalog() or mutation failed: %v", i, err)
		}
	}
}

// TestApp_StarmapWithOptions tests that Starmap with options creates new instances each time.
func TestApp_StarmapWithOptions(t *testing.T) {
	app, err := New("1.0.0", "test", "2024-01-01", "test")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Create two starmaps with custom options (using embedded catalog as option)
	sm1, err := app.Starmap(starmap.WithEmbeddedCatalog())
	if err != nil {
		t.Fatalf("Starmap(opts...) failed: %v", err)
	}

	sm2, err := app.Starmap(starmap.WithEmbeddedCatalog())
	if err != nil {
		t.Fatalf("Starmap(opts...) failed on second call: %v", err)
	}

	// These should be DIFFERENT instances (not singleton) when options provided
	if sm1 == sm2 {
		t.Error("Starmap(opts...) returned same instance, expected new instance each time")
	}

	// And they should be different from the default singleton
	smDefault, err := app.Starmap()
	if err != nil {
		t.Fatalf("Starmap() failed: %v", err)
	}

	if sm1 == smDefault {
		t.Error("Starmap(opts...) returned default singleton, expected new instance")
	}
}

// TestApp_WithOptions tests functional options pattern.
func TestApp_WithOptions(t *testing.T) {
	// Create custom config
	customConfig := &Config{
		Verbose: true,
		Quiet:   false,
		Output:  "json",
	}

	// Create custom logger
	customLogger := zerolog.Nop() // No-op logger for testing

	// Create app with options
	app, err := New("1.0.0", "test", "2024-01-01", "test",
		WithConfig(customConfig),
		WithLogger(&customLogger),
	)
	if err != nil {
		t.Fatalf("New() with options failed: %v", err)
	}

	// Verify options were applied
	if app.Config() != customConfig {
		t.Error("WithConfig() option not applied")
	}
	if app.Logger() != &customLogger {
		t.Error("WithLogger() option not applied")
	}
}

// TestApp_Shutdown verifies graceful shutdown.
func TestApp_Shutdown(t *testing.T) {
	app, err := New("1.0.0", "test", "2024-01-01", "test")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Initialize starmap (lazy initialization)
	_, err = app.Starmap()
	if err != nil {
		t.Fatalf("Starmap() failed: %v", err)
	}

	// Shutdown should not error
	ctx := context.Background()
	if err := app.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() failed: %v", err)
	}
}

// TestApp_ShutdownWithoutStarmap verifies shutdown works even if starmap never initialized.
func TestApp_ShutdownWithoutStarmap(t *testing.T) {
	app, err := New("1.0.0", "test", "2024-01-01", "test")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Shutdown without ever calling Starmap()
	ctx := context.Background()
	if err := app.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() failed: %v", err)
	}
}

// BenchmarkApp_Catalog measures catalog retrieval performance.
func BenchmarkApp_Catalog(b *testing.B) {
	app, err := New("1.0.0", "test", "2024-01-01", "test")
	if err != nil {
		b.Fatalf("New() failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := app.Catalog()
		if err != nil {
			b.Fatalf("Catalog() failed: %v", err)
		}
	}
}

// BenchmarkApp_Starmap measures starmap singleton access performance.
func BenchmarkApp_Starmap(b *testing.B) {
	app, err := New("1.0.0", "test", "2024-01-01", "test")
	if err != nil {
		b.Fatalf("New() failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := app.Starmap()
		if err != nil {
			b.Fatalf("Starmap() failed: %v", err)
		}
	}
}
