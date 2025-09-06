package modelsdev

import (
	"github.com/agentstation/utc"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// ConvertToStarmapModel converts a models.dev model to a starmap model.
// This is shared between GitSource and HTTPSource to avoid duplication.
func ConvertToStarmapModel(mdModel Model) catalogs.Model {
	model := catalogs.Model{
		ID:   mdModel.ID,
		Name: mdModel.Name,
	}

	// Add metadata if available
	if mdModel.ReleaseDate != "" || (mdModel.Knowledge != nil && *mdModel.Knowledge != "") {
		model.Metadata = &catalogs.ModelMetadata{}

		// Parse release date
		if mdModel.ReleaseDate != "" {
			if releaseDate, err := parseDate(mdModel.ReleaseDate); err == nil {
				model.Metadata.ReleaseDate = utc.Time{Time: *releaseDate}
			}
		}

		// Parse knowledge cutoff
		if mdModel.Knowledge != nil && *mdModel.Knowledge != "" {
			if knowledgeDate, err := parseDate(*mdModel.Knowledge); err == nil {
				knowledgeCutoff := utc.Time{Time: *knowledgeDate}
				model.Metadata.KnowledgeCutoff = &knowledgeCutoff
			}
		}

		// Set open weights flag
		model.Metadata.OpenWeights = mdModel.OpenWeights
	}

	// Add pricing if available
	if mdModel.Cost != nil && (mdModel.Cost.Input != nil || mdModel.Cost.Output != nil) {
		model.Pricing = &catalogs.ModelPricing{
			Currency: "USD", // models.dev uses USD
			Tokens:   &catalogs.ModelTokenPricing{},
		}

		// Map input cost (models.dev uses cost per 1M tokens)
		if mdModel.Cost.Input != nil && *mdModel.Cost.Input > 0 {
			model.Pricing.Tokens.Input = &catalogs.ModelTokenCost{
				Per1M: *mdModel.Cost.Input,
			}
		}

		// Map output cost
		if mdModel.Cost.Output != nil && *mdModel.Cost.Output > 0 {
			model.Pricing.Tokens.Output = &catalogs.ModelTokenCost{
				Per1M: *mdModel.Cost.Output,
			}
		}
	}

	// Add limits if available
	if mdModel.Limit.Context > 0 || mdModel.Limit.Output > 0 {
		model.Limits = &catalogs.ModelLimits{}

		if mdModel.Limit.Context > 0 {
			model.Limits.ContextWindow = int64(mdModel.Limit.Context)
		}

		if mdModel.Limit.Output > 0 {
			model.Limits.OutputTokens = int64(mdModel.Limit.Output)
		}
	}

	return model
}
