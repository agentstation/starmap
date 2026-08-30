package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
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

func TestRootCommandHasOneCanonicalPublicSpelling(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	application, err := New("1.0.0", "abc123", "2024-01-01", "test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	root := application.createRootCommand()
	if root.PersistentFlags().Lookup("output") == nil {
		t.Fatal("root command has no canonical --output flag")
	}
	for _, removed := range []string{"fmt", "format"} {
		if root.PersistentFlags().Lookup(removed) != nil {
			t.Fatalf("root command exposes prelaunch alias --%s", removed)
		}
	}
	for _, command := range root.Commands() {
		if len(command.Aliases) != 0 {
			t.Errorf("command %q exposes prelaunch aliases %v", command.Name(), command.Aliases)
		}
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

	// Verify it is the same instance (same pointer)
	if sm1 != sm2 {
		t.Error("Starmap() returned different instances, expected singleton")
	}
}

func TestApp_StarmapConfiguresPassiveFilesystemCatalogStore(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "catalog")
	app, err := New("1.0.0", "test", "2024-01-01", "test", WithConfig(&Config{
		CatalogPath: storePath,
	}))
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if _, err := app.Starmap(); err != nil {
		t.Fatalf("Starmap() failed: %v", err)
	}
	if _, err := os.Stat(storePath); !os.IsNotExist(err) {
		t.Fatalf("read-only client construction created store path: %v", err)
	}
}

func TestEmbeddedBudgetConfigurationPropagatesToReadiness(t *testing.T) {
	app, err := New("1.0.0", "test", "2026-07-10", "test", WithConfig(&Config{
		CatalogPath:                   filepath.Join(t.TempDir(), "catalog"),
		EmbeddedBootstrapMaxSizeBytes: 1,
	}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	readiness, err := app.Readiness()
	if err != nil {
		t.Fatalf("Readiness: %v", err)
	}
	if readiness.Ready || len(readiness.Issues) != 1 ||
		readiness.Issues[0].Code != starmap.ReadinessIssueEmbeddedBootstrapOversize {
		t.Fatalf("readiness = %#v", readiness)
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
	results := make([]*starmap.Client, goroutines)
	errors := make([]error, goroutines)

	// Launch many goroutines to test concurrent access
	for i := range goroutines {
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

// TestApp_Catalog_ReturnsImmutableSnapshot verifies collection reads cannot
// mutate the published generation.
func TestApp_Catalog_ReturnsImmutableCatalog(t *testing.T) {
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

	providers := cat1.Providers().List()
	if len(providers) == 0 {
		t.Fatal("Expected embedded providers")
	}
	originalName := providers[0].Name
	providers[0].Name = "mutated caller value"
	provider, ok := cat2.Providers().Get(providers[0].ID)
	if !ok {
		t.Fatalf("Provider %s missing from second catalog", providers[0].ID)
	}
	if provider.Name != originalName {
		t.Error("Catalog() returned shared provider state")
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

	// Launch many goroutines that all get and read catalog snapshots.
	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			cat, err := app.Catalog()
			if err != nil {
				errors[idx] = err
				return
			}

			if cat.Providers().Len() == 0 {
				errors[idx] = fmt.Errorf("catalog has no providers")
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

	// Create two starmaps with custom options.
	sm1, err := app.Starmap(starmap.WithEmbeddedBootstrapMaxAge(time.Hour))
	if err != nil {
		t.Fatalf("Starmap(opts...) failed: %v", err)
	}

	sm2, err := app.Starmap(starmap.WithEmbeddedBootstrapMaxAge(time.Hour))
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

	if app.Config() != customConfig {
		t.Error("WithConfig() option not applied")
	}
	if app.Logger() != &customLogger {
		t.Error("WithLogger() option not applied")
	}
}

func TestAppCredentialResolverUsesCatalogValidatedExplicitReference(t *testing.T) {
	const environment = "STARMAP_TEST_EXPLICIT_OPENAI_KEY"
	t.Setenv(environment, "exact-secret-value\n")
	application, err := New("test", "test", "test", "test", WithConfig(&Config{
		CredentialSources: map[string]map[string]CredentialSourceConfig{
			"openai": {
				"api-key": {Reference: "env:" + environment},
			},
		},
	}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	resolver, err := application.CredentialResolver()
	if err != nil {
		t.Fatalf("CredentialResolver: %v", err)
	}
	second, err := application.CredentialResolver()
	if err != nil {
		t.Fatalf("CredentialResolver second call: %v", err)
	}
	if resolver != second {
		t.Fatal("CredentialResolver did not return the process-owned resolver")
	}
	catalog, err := application.Catalog()
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	provider, err := catalog.Provider("openai")
	if err != nil {
		t.Fatalf("Provider: %v", err)
	}
	material, err := resolver.ResolveCatalog(context.Background(), &provider)
	if err != nil {
		t.Fatalf("ResolveCatalog: %v", err)
	}
	if value, exists := material.Value("api-key"); !exists || value != "exact-secret-value\n" {
		t.Fatalf("resolved exact value exists = %v, length = %d", exists, len(value))
	}
}

func TestAppCredentialResolverRejectsNonCatalogConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		field    string
	}{
		{name: "unknown provider", provider: "yaml-only-missing", field: "api-key"},
		{name: "unknown field", provider: "openai", field: "missing-field"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			application, err := New("test", "test", "test", "test", WithConfig(&Config{
				CredentialSources: map[string]map[string]CredentialSourceConfig{
					test.provider: {
						test.field: {Reference: "env:SHOULD_NOT_BE_READ"},
					},
				},
			}))
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if _, err := application.CredentialResolver(); err == nil {
				t.Fatal("CredentialResolver accepted non-catalog configuration")
			}
		})
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

// BenchmarkApp_Starmap measures starmap singleton access performance.
func BenchmarkApp_Starmap(b *testing.B) {
	app, err := New("1.0.0", "test", "2024-01-01", "test")
	if err != nil {
		b.Fatalf("New() failed: %v", err)
	}

	b.ResetTimer()
	for b.Loop() {
		_, err := app.Starmap()
		if err != nil {
			b.Fatalf("Starmap() failed: %v", err)
		}
	}
}
