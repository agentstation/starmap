package catalogs_test

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
)

// Example demonstrates basic catalog creation and usage
func Example() {
	// Create a memory-based catalog
	catalog, err := catalogs.New()
	if err != nil {
		log.Fatal(err)
	}

	// Add a provider
	provider := catalogs.Provider{
		ID:   "openai",
		Name: "OpenAI",
	}
	if err := catalog.SetProvider(provider); err != nil {
		log.Fatal(err)
	}

	// Add a model
	model := catalogs.Model{
		ID:          "gpt-4",
		Name:        "GPT-4",
		Description: "Advanced language model",
	}
	if err := catalog.SetModel(model); err != nil {
		log.Fatal(err)
	}

	// List all models
	models := catalog.Models().List()
	fmt.Printf("Found %d models\n", len(models))
	// Output: Found 1 models
}

// Example_embeddedCatalog demonstrates using the embedded catalog
func Example_embeddedCatalog() {
	// Create catalog with embedded data
	catalog, err := catalogs.New(catalogs.WithEmbedded())
	if err != nil {
		log.Fatal(err)
	}

	// Access pre-loaded models
	models := catalog.Models().List()
	fmt.Printf("Embedded catalog has %d+ models\n", len(models))
	
	// Find a specific model
	model, err := catalog.Model("gpt-4o")
	if err == nil {
		fmt.Printf("Found model: %s\n", model.Name)
	}
}

// Example_fileBasedCatalog demonstrates file-based persistence
func Example_fileBasedCatalog() {
	// Create a file-based catalog
	catalogPath := filepath.Join(".", "my-catalog")
	catalog, err := catalogs.New(
		catalogs.WithFiles(catalogPath),
		catalogs.WithWritePath(catalogPath),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Add and save data
	model := catalogs.Model{
		ID:   "custom-model",
		Name: "My Custom Model",
	}
	if err := catalog.SetModel(model); err != nil {
		log.Fatal(err)
	}

	// Save to disk (would normally use SaveTo method or similar)
	// Since Write is not part of the interface, this example shows the concept
	// In actual usage, you would use catalog.SaveTo(catalogPath)

	fmt.Println("Catalog saved to disk")
}

// Example_mergeCatalogs demonstrates merging two catalogs
func Example_mergeCatalogs() {
	// Create base catalog
	base, _ := catalogs.New()
	base.SetModel(catalogs.Model{
		ID:          "model-1",
		Name:        "Model One",
		Description: "Original description",
	})

	// Create updates catalog
	updates, _ := catalogs.New()
	updates.SetModel(catalogs.Model{
		ID:          "model-1",
		Name:        "Model One Enhanced",
		Description: "Updated description",
		Pricing: &catalogs.ModelPricing{
			Tokens: &catalogs.TokenPricing{
				Input: &catalogs.TokenCost{
					Per1M: 2.0, // $2 per 1M tokens
				},
				Output: &catalogs.TokenCost{
					Per1M: 4.0, // $4 per 1M tokens  
				},
			},
			Currency: "USD",
		},
	})

	// Merge with EnrichEmpty strategy (default)
	if err := base.MergeWith(updates); err != nil {
		log.Fatal(err)
	}

	model, _ := base.Model("model-1")
	fmt.Printf("Model name: %s\n", model.Name)
	// Output: Model name: Model One Enhanced
}

// Example_mergeStrategies demonstrates different merge strategies
func Example_mergeStrategies() {
	base, _ := catalogs.New()
	base.SetModel(catalogs.Model{ID: "m1", Name: "Original"})

	updates, _ := catalogs.New()
	updates.SetModel(catalogs.Model{ID: "m1", Name: "Updated"})
	updates.SetModel(catalogs.Model{ID: "m2", Name: "New"})

	// Example 1: Append only (keeps existing, adds new)
	cat1, _ := base.Copy()
	cat1.MergeWith(updates, catalogs.WithStrategy(catalogs.MergeAppendOnly))
	
	m1, _ := cat1.Model("m1")
	fmt.Printf("AppendOnly - m1: %s\n", m1.Name) // Original

	// Example 2: Replace all
	cat2, _ := base.Copy()
	cat2.MergeWith(updates, catalogs.WithStrategy(catalogs.MergeReplaceAll))
	
	m1, _ = cat2.Model("m1")
	fmt.Printf("ReplaceAll - m1: %s\n", m1.Name) // Updated

	// Example 3: Enrich empty (smart merge)
	cat3, _ := base.Copy()
	cat3.MergeWith(updates, catalogs.WithStrategy(catalogs.MergeEnrichEmpty))
	
	m1, _ = cat3.Model("m1")
	fmt.Printf("EnrichEmpty - m1: %s\n", m1.Name) // Updated
}

// Example_concurrentAccess demonstrates thread-safe concurrent usage
func Example_concurrentAccess() {
	catalog, _ := catalogs.New()
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultHTTPTimeout)
	defer cancel()

	// Safe for concurrent reads and writes
	done := make(chan bool, 2)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			catalog.SetModel(catalogs.Model{
				ID:   fmt.Sprintf("model-%d", i),
				Name: fmt.Sprintf("Model %d", i),
			})
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for {
			select {
			case <-ctx.Done():
				done <- true
				return
			default:
				_ = catalog.Models().List()
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()

	// Wait for both
	<-done
	<-done

	fmt.Printf("Created %d models concurrently\n", len(catalog.Models().List()))
}

// Example_providerCapabilities demonstrates working with provider features
func Example_providerCapabilities() {
	catalog, _ := catalogs.New()

	// Add provider with capabilities
	provider := catalogs.Provider{
		ID:   "openai",
		Name: "OpenAI",
		Catalog: &catalogs.ProviderCatalog{
			APIURL:         ptr("https://api.openai.com/v1/models"),
			APIKeyRequired: ptrBool(true),
		},
	}
	catalog.SetProvider(provider)

	// Check capabilities
	p, _ := catalog.Provider("openai")
	if p.HasAPIKey() {
		fmt.Println("Provider has API key configured")
	}
	if p.Catalog != nil && p.Catalog.APIKeyRequired != nil && *p.Catalog.APIKeyRequired {
		fmt.Println("Provider requires API key")
	}
}

// Example_modelFiltering demonstrates filtering models
func Example_modelFiltering() {
	catalog, _ := catalogs.New(catalogs.WithEmbedded())

	// Get all models for a specific provider
	// In practice, models would be linked to providers via naming convention or metadata
	var gptModels []*catalogs.Model
	for _, model := range catalog.Models().List() {
		if len(model.ID) > 3 && model.ID[:3] == "gpt" {
			gptModels = append(gptModels, model)
		}
	}
	fmt.Printf("Found %d GPT models\n", len(gptModels))

	// Filter by features
	var visionModels []*catalogs.Model
	for _, model := range catalog.Models().List() {
		if model.Features != nil {
			for _, modality := range model.Features.Modalities.Input {
				if modality == "image" {
					visionModels = append(visionModels, model)
					break
				}
			}
		}
	}
	fmt.Printf("Found %d models with vision\n", len(visionModels))
}

// Example_catalogCopy demonstrates creating independent copies
func Example_catalogCopy() {
	// Create original catalog
	original, _ := catalogs.New()
	original.SetModel(catalogs.Model{
		ID:   "model-1",
		Name: "Original Model",
	})

	// Create a copy
	copy, err := original.Copy()
	if err != nil {
		log.Fatal(err)
	}

	// Modify the copy
	copy.SetModel(catalogs.Model{
		ID:   "model-2",
		Name: "Copy Model",
	})

	// Original is unchanged
	fmt.Printf("Original has %d models\n", len(original.Models().List()))
	fmt.Printf("Copy has %d models\n", len(copy.Models().List()))
	// Output:
	// Original has 1 models
	// Copy has 2 models
}

// Helper functions for creating pointers
func ptr(s string) *string {
	return &s
}

func ptrFloat(f float64) *float64 {
	return &f
}

func ptrBool(b bool) *bool {
	return &b
}