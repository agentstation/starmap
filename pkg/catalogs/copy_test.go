package catalogs

import (
	"testing"
	"time"

	"github.com/agentstation/utc"
)

func TestDeepCopyProviderModels(t *testing.T) {
	// Test nil input
	t.Run("nil input", func(t *testing.T) {
		result := DeepCopyProviderModels(nil)
		if result != nil {
			t.Error("Expected nil result for nil input")
		}
	})

	// Test empty map
	t.Run("empty map", func(t *testing.T) {
		input := make(map[string]*Model)
		result := DeepCopyProviderModels(input)
		if result == nil {
			t.Fatal("Expected non-nil result for empty map")
		}
		if len(result) != 0 {
			t.Error("Expected empty result map")
		}
		// Verify it's a different map instance
		if &input == &result {
			t.Error("Expected different map instances")
		}
	})

	// Test map with models
	t.Run("map with models", func(t *testing.T) {
		model1 := &Model{
			ID:        "model-1",
			Name:      "Test Model 1",
			CreatedAt: utc.Time{Time: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
		}
		model2 := &Model{
			ID:        "model-2",
			Name:      "Test Model 2",
			CreatedAt: utc.Time{Time: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)},
		}

		input := map[string]*Model{
			"model-1": model1,
			"model-2": model2,
		}

		result := DeepCopyProviderModels(input)

		// Check length
		if len(result) != len(input) {
			t.Errorf("Expected length %d, got %d", len(input), len(result))
		}

		// Check that models are copied, not shared
		for k, v := range input {
			resultModel := result[k]
			if resultModel == nil {
				t.Errorf("Missing model for key %s", k)
				continue
			}

			// Different pointers (deep copy)
			if v == resultModel {
				t.Errorf("Expected different pointer for model %s", k)
			}

			// Same content
			if v.ID != resultModel.ID || v.Name != resultModel.Name {
				t.Errorf("Model content mismatch for key %s", k)
			}
		}

		// Verify mutation independence
		result["model-1"].Name = "Modified"
		if input["model-1"].Name == "Modified" {
			t.Error("Original model should not be affected by copy mutation")
		}
	})

	// Test map with nil model pointer
	t.Run("map with nil model", func(t *testing.T) {
		input := map[string]*Model{
			"nil-model": nil,
		}

		result := DeepCopyProviderModels(input)
		if len(result) != 1 {
			t.Error("Expected one entry in result")
		}
		if result["nil-model"] != nil {
			t.Error("Expected nil model to remain nil")
		}
	})
}

func TestDeepCopyAuthorModels(t *testing.T) {
	// Test nil input
	t.Run("nil input", func(t *testing.T) {
		result := DeepCopyAuthorModels(nil)
		if result != nil {
			t.Error("Expected nil result for nil input")
		}
	})

	// Test basic functionality (same logic as provider models)
	t.Run("basic functionality", func(t *testing.T) {
		model := &Model{
			ID:        "author-model",
			Name:      "Author Test Model",
			CreatedAt: utc.Time{Time: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
		}

		input := map[string]*Model{
			"author-model": model,
		}

		result := DeepCopyAuthorModels(input)

		if len(result) != 1 {
			t.Error("Expected one model in result")
		}

		resultModel := result["author-model"]
		if resultModel == model {
			t.Error("Expected different pointer (deep copy)")
		}

		if resultModel.ID != model.ID {
			t.Error("Expected same content after copy")
		}
	})
}

func TestDeepCopyProvider(t *testing.T) {
	t.Run("provider with models", func(t *testing.T) {
		model := &Model{
			ID:   "test-model",
			Name: "Test Model",
		}

		original := Provider{
			ID:   "test-provider",
			Name: "Test Provider",
			Models: map[string]*Model{
				"test-model": model,
			},
		}

		copy := DeepCopyProvider(original)

		// Basic field copy
		if copy.ID != original.ID || copy.Name != original.Name {
			t.Error("Provider fields not copied correctly")
		}

		// Models map should be deep copied
		if &copy.Models == &original.Models {
			t.Error("Models map should be different instance")
		}

		if copy.Models["test-model"] == original.Models["test-model"] {
			t.Error("Model pointers should be different (deep copy)")
		}

		if copy.Models["test-model"].ID != original.Models["test-model"].ID {
			t.Error("Model content should be the same")
		}

		// Test mutation independence
		copy.Models["test-model"].Name = "Modified"
		if original.Models["test-model"].Name == "Modified" {
			t.Error("Original should not be affected by copy mutation")
		}
	})

	t.Run("provider without models", func(t *testing.T) {
		original := Provider{
			ID:     "test-provider",
			Name:   "Test Provider",
			Models: nil,
		}

		copy := DeepCopyProvider(original)

		if copy.Models != nil {
			t.Error("Models should remain nil")
		}
		if copy.ID != original.ID {
			t.Error("Provider fields should be copied")
		}
	})
}

func TestDeepCopyAuthor(t *testing.T) {
	t.Run("author with models", func(t *testing.T) {
		model := &Model{
			ID:   "test-model",
			Name: "Test Model",
		}

		original := Author{
			ID:          "test-author",
			Name:        "Test Author",
			Description: stringPtr("Test description"),
			Models: map[string]*Model{
				"test-model": model,
			},
		}

		copy := DeepCopyAuthor(original)

		// Basic field copy
		if copy.ID != original.ID || copy.Name != original.Name {
			t.Error("Author fields not copied correctly")
		}

		// Models map should be deep copied
		if &copy.Models == &original.Models {
			t.Error("Models map should be different instance")
		}

		if copy.Models["test-model"] == original.Models["test-model"] {
			t.Error("Model pointers should be different (deep copy)")
		}

		if copy.Models["test-model"].ID != original.Models["test-model"].ID {
			t.Error("Model content should be the same")
		}

		// Test mutation independence
		copy.Models["test-model"].Name = "Modified"
		if original.Models["test-model"].Name == "Modified" {
			t.Error("Original should not be affected by copy mutation")
		}
	})
}

func TestShallowCopyProviderModels(t *testing.T) {
	t.Run("shallow copy behavior", func(t *testing.T) {
		model := &Model{
			ID:   "test-model",
			Name: "Test Model",
		}

		input := map[string]*Model{
			"test-model": model,
		}

		result := ShallowCopyProviderModels(input)

		// Different map instances
		if &input == &result {
			t.Error("Expected different map instances")
		}

		// Same model pointers (shallow copy)
		if result["test-model"] != input["test-model"] {
			t.Error("Expected same model pointers (shallow copy)")
		}

		// Verify shared mutation
		result["test-model"].Name = "Modified"
		if input["test-model"].Name != "Modified" {
			t.Error("Both maps should share model instances")
		}
	})
}

func TestShallowCopyAuthorModels(t *testing.T) {
	t.Run("shallow copy behavior", func(t *testing.T) {
		model := &Model{
			ID:   "test-model",
			Name: "Test Model",
		}

		input := map[string]*Model{
			"test-model": model,
		}

		result := ShallowCopyAuthorModels(input)

		// Different map instances
		if &input == &result {
			t.Error("Expected different map instances")
		}

		// Same model pointers (shallow copy)
		if result["test-model"] != input["test-model"] {
			t.Error("Expected same model pointers (shallow copy)")
		}
	})
}
