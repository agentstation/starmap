package docs

import (
	"github.com/agentstation/starmap/pkg/catalogs"
)

// hasModality checks if a model supports a specific modality in either input or output.
func hasModality(features *catalogs.ModelFeatures, modality catalogs.ModelModality) bool {
	if features == nil {
		return false
	}

	// Check input modalities
	for _, m := range features.Modalities.Input {
		if m == modality {
			return true
		}
	}

	// Check output modalities
	for _, m := range features.Modalities.Output {
		if m == modality {
			return true
		}
	}

	return false
}


// hasText checks if model supports text modality.
func hasText(features *catalogs.ModelFeatures) bool {
	return hasModality(features, catalogs.ModelModalityText)
}

// hasVision checks if model supports image/vision modality.
func hasVision(features *catalogs.ModelFeatures) bool {
	return hasModality(features, catalogs.ModelModalityImage)
}

// hasAudio checks if model supports audio modality.
func hasAudio(features *catalogs.ModelFeatures) bool {
	return hasModality(features, catalogs.ModelModalityAudio)
}

// hasVideo checks if model supports video modality.
func hasVideo(features *catalogs.ModelFeatures) bool {
	return hasModality(features, catalogs.ModelModalityVideo)
}

// hasPDF checks if model supports PDF modality.
func hasPDF(features *catalogs.ModelFeatures) bool {
	return hasModality(features, catalogs.ModelModalityPDF)
}


// hasToolSupport checks if model has any tool-related capabilities.
func hasToolSupport(features *catalogs.ModelFeatures) bool {
	if features == nil {
		return false
	}
	return features.Tools || features.ToolCalls || features.ToolChoice
}
