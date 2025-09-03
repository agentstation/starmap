package modelsdev

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
)

// EnhanceModelsWithModelsDevData enhances API models with models.dev data BEFORE comparison
func EnhanceModelsWithModelsDevData(apiModels []catalogs.Model, provider *catalogs.Provider, api *ModelsDevAPI) ([]catalogs.Model, int) {
	if api == nil || provider == nil {
		return apiModels, 0
	}

	// Try to find models.dev provider data using primary ID first
	modelsDevProvider, exists := api.GetProvider(provider.ID)

	// If not found, try aliases
	if !exists && len(provider.Aliases) > 0 {
		for _, alias := range provider.Aliases {
			modelsDevProvider, exists = api.GetProvider(alias)
			if exists {
				logging.Debug().
					Str("provider_id", string(provider.ID)).
					Str("alias", string(alias)).
					Msg("Found models.dev data using alias")
				break
			}
		}
	}

	if !exists {
		logging.Debug().
			Str("provider_id", string(provider.ID)).
			Msg("Provider not found in models.dev data")
		return apiModels, 0
	}

	// Enhance all API models with models.dev data
	enhanced := make([]catalogs.Model, len(apiModels))
	successes := 0
	for i, model := range apiModels {
		var ok bool
		enhanced[i], ok = enhanceModelWithModelsDevData(model, modelsDevProvider)
		if ok {
			successes++
		}
	}

	return enhanced, successes
}

// enhanceModelWithModelsDevData enhances a single model with models.dev data
func enhanceModelWithModelsDevData(apiModel catalogs.Model, modelsDevProvider *ModelsDevProvider) (catalogs.Model, bool) {
	// Look for the model in models.dev data
	modelsDevModel, exists := modelsDevProvider.Model(apiModel.ID)
	if !exists {
		// Try alternate ID patterns (some providers use different naming)
		alternateIDs := generateAlternateIDs(apiModel.ID)
		for _, altID := range alternateIDs {
			if altModel, altExists := modelsDevProvider.Model(altID); altExists {
				modelsDevModel = altModel
				exists = true
				break
			}
		}
	}

	if !exists {
		return apiModel, false
	}

	// Convert models.dev model to starmap model
	modelsDevStarmap, err := modelsDevModel.ToStarmapModel()
	if err != nil {
		logging.Debug().
			Err(err).
			Str("model_id", apiModel.ID).
			Msg("Error converting models.dev model")
		return apiModel, false
	}

	// Use smart three-way merge with models.dev priority for limits
	enhanced := smartMergeThreeWay(apiModel, *modelsDevStarmap, catalogs.Model{})

	return enhanced, true
}

// smartMergeThreeWay performs a three-way merge with smart priority
func smartMergeThreeWay(api, modelsdev, existing catalogs.Model) catalogs.Model {
	result := existing // Start with existing as base

	// First, merge API data (for basic model info and availability)
	result = catalogs.MergeModels(result, api)

	// Then, merge models.dev data with priority for limits and detailed specs
	// This gives models.dev higher priority for things like context_window, output_tokens, pricing
	if modelsdev.Limits != nil {
		if result.Limits == nil {
			result.Limits = &catalogs.ModelLimits{}
		}
		// models.dev has more accurate limit data
		if modelsdev.Limits.ContextWindow > 0 {
			result.Limits.ContextWindow = modelsdev.Limits.ContextWindow
		}
		if modelsdev.Limits.OutputTokens > 0 {
			result.Limits.OutputTokens = modelsdev.Limits.OutputTokens
		}
	}

	// Merge other models.dev data that should have priority
	if modelsdev.Pricing != nil {
		result.Pricing = modelsdev.Pricing
	}

	// Merge metadata from models.dev (release date, knowledge cutoff, open weights)
	if modelsdev.Metadata != nil {
		if result.Metadata == nil {
			result.Metadata = &catalogs.ModelMetadata{}
		}
		// Copy metadata fields from models.dev
		if !modelsdev.Metadata.ReleaseDate.IsZero() {
			result.Metadata.ReleaseDate = modelsdev.Metadata.ReleaseDate
		}
		if modelsdev.Metadata.KnowledgeCutoff != nil && !modelsdev.Metadata.KnowledgeCutoff.IsZero() {
			result.Metadata.KnowledgeCutoff = modelsdev.Metadata.KnowledgeCutoff
		}
		// Copy open weights flag (models.dev data is typically more accurate)
		result.Metadata.OpenWeights = modelsdev.Metadata.OpenWeights
	}

	return result
}

// generateAlternateIDs generates possible alternate IDs for model lookup
func generateAlternateIDs(modelID string) []string {
	var alternates []string

	// Common transformations between providers
	// Example: "gpt-4" might be "openai/gpt-4" in models.dev

	// Remove provider prefix if it exists
	if idx := findLastSlash(modelID); idx != -1 {
		withoutPrefix := modelID[idx+1:]
		alternates = append(alternates, withoutPrefix)
	}

	// Add common provider prefixes
	commonPrefixes := []string{"openai", "anthropic", "google", "meta", "mistral"}
	for _, prefix := range commonPrefixes {
		alternates = append(alternates, fmt.Sprintf("%s/%s", prefix, modelID))
	}

	// Try with dashes replaced by underscores and vice versa
	dashToUnderscore := replaceDashUnderscore(modelID, "_")
	underscoreToDash := replaceDashUnderscore(modelID, "-")
	if dashToUnderscore != modelID {
		alternates = append(alternates, dashToUnderscore)
	}
	if underscoreToDash != modelID {
		alternates = append(alternates, underscoreToDash)
	}

	return alternates
}

// findLastSlash finds the last occurrence of '/' in a string
func findLastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}

// replaceDashUnderscore replaces dashes with underscores or vice versa
func replaceDashUnderscore(s, replacement string) string {
	if replacement == "_" {
		result := ""
		for _, char := range s {
			if char == '-' {
				result += "_"
			} else {
				result += string(char)
			}
		}
		return result
	} else {
		result := ""
		for _, char := range s {
			if char == '_' {
				result += "-"
			} else {
				result += string(char)
			}
		}
		return result
	}
}

// CopyProviderLogos copies provider logos from models.dev to output directory
func CopyProviderLogos(client *Client, outputDir string, providerIDs []catalogs.ProviderID) error {
	providersPath := client.GetProvidersPath()

	for _, providerID := range providerIDs {
		// Source logo path in models.dev
		sourceLogo := fmt.Sprintf("%s/%s/logo.svg", providersPath, providerID)

		// Destination logo path in output
		destLogo := fmt.Sprintf("%s/%s/logo.svg", outputDir, providerID)

		// Copy if source exists
		if err := copyFile(sourceLogo, destLogo); err != nil {
			logging.Warn().
				Err(err).
				Str("provider_id", string(providerID)).
				Msg("Could not copy logo for provider")
			// Don't fail the entire operation for missing logos
		}
	}

	return nil
}

// copyFile copies a file from source to destination
func copyFile(src, dst string) error {
	// Check if source file exists
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return &errors.NotFoundError{
			Resource: "file",
			ID:       src,
		}
	}

	// Create destination directory if it doesn't exist
	destDir := filepath.Dir(dst)
	if err := os.MkdirAll(destDir, constants.DirPermissions); err != nil {
		return errors.WrapIO("create", "destination directory", err)
	}

	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return errors.WrapIO("open", src, err)
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := os.Create(dst)
	if err != nil {
		return errors.WrapIO("create", dst, err)
	}
	defer dstFile.Close()

	// Copy file contents
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return errors.WrapIO("copy", "file contents", err)
	}

	return nil
}
