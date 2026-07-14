package modelsdev

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

const (
	gitCommitField            = "models_dev.git.commit"
	modelIDField              = "model.id"
	validationRequiredMessage = "is required"
)

// processFetch handles the common logic for fetching models from models.dev API.
func processFetch(catalog *catalogs.Builder, api *API, opts ...sources.Option) (int, int, []sources.ObservationIssue, error) {
	options := sources.Defaults().Apply(opts...)
	candidateCount := modelsDevCandidateCount(api, options.ProviderID)

	// Set the default merge strategy for models.dev catalog enrichment.
	catalog.SetMergeStrategy(catalogs.MergeEnrichEmpty)

	// Add providers with models that have mapped catalog data from models.dev.
	added := 0
	limitExceeded := false
	issues := make([]sources.ObservationIssue, 0)
	providerKeys := make([]string, 0, len(*api))
	for providerKey := range *api {
		providerKeys = append(providerKeys, providerKey)
	}
	sort.Strings(providerKeys)
	for _, providerKey := range providerKeys {
		mdProvider := (*api)[providerKey]
		// Convert provider ID from models.dev format
		providerID := catalogs.ProviderID(mdProvider.ID)
		if options.ProviderID != nil && providerID != *options.ProviderID {
			continue
		}
		if mdProvider.Models == nil {
			issues = append(issues, sources.ObservationIssue{
				Scope: sources.ObservationIssueScopeProvider, Code: sources.ObservationIssueCodeSchemaDrift,
				Subject: string(providerID), Message: "required models object is missing or null",
			})
			continue
		}

		// Get or create provider in catalog
		provider, err := catalog.Provider(providerID)
		if err != nil {
			// Provider doesn't exist, create a minimal one
			provider = catalogs.Provider{
				ID:   providerID,
				Name: mdProvider.ID, // Use ID as name for now
			}
		}
		mergeModelsDevProviderMetadata(&provider, mdProvider.toStarmapProviderMetadata())

		// Initialize models map if needed
		if provider.Models == nil {
			provider.Models = make(map[string]*catalogs.Model)
		}

		// Add models with pricing/limits data
		modelKeys := make([]string, 0, len(mdProvider.Models))
		for modelKey := range mdProvider.Models {
			modelKeys = append(modelKeys, modelKey)
		}
		sort.Strings(modelKeys)
		for _, modelKey := range modelKeys {
			if added >= constants.MaxCatalogModels {
				limitExceeded = true
				issues = append(issues, sources.ObservationIssue{
					Scope: sources.ObservationIssueScopeSource, Code: sources.ObservationIssueCodePayloadLimit,
					Message: "models.dev model count exceeds maximum; excess records quarantined",
				})
				break
			}
			mdModel := mdProvider.Models[modelKey]
			// Only include models that have data Starmap can map.
			if mdModel.hasCatalogData() {
				if err := validateModelsDevModelIdentity(modelKey, &mdModel); err != nil {
					issues = append(issues, modelsDevRecordIssue(providerID, modelKey, err))
					continue
				}
				model, err := mdModel.ToStarmapModel()
				if err != nil {
					issues = append(issues, modelsDevRecordIssue(providerID, modelKey, errors.WrapResource("convert", "model", mdModel.ID, err)))
					continue
				}
				provider.Models[model.ID] = model
				added++
			}
		}

		// Update provider in catalog if we added any models
		if len(provider.Models) > 0 {
			if err := catalog.SetProvider(provider); err != nil {
				return added, candidateCount - added, issues, errors.WrapResource("set", "provider", string(provider.ID), err)
			}
		}
		if limitExceeded {
			break
		}
	}

	return added, candidateCount - added, issues, nil
}

func modelsDevCandidateCount(api *API, providerFilter *catalogs.ProviderID) int {
	if api == nil {
		return 0
	}
	count := 0
	for _, provider := range *api {
		if providerFilter != nil && catalogs.ProviderID(provider.ID) != *providerFilter {
			continue
		}
		for _, model := range provider.Models {
			if model.hasCatalogData() {
				count++
			}
		}
	}
	return count
}

func validateModelsDevModelIdentity(mapKey string, model *Model) error {
	if model == nil {
		return &errors.ValidationError{Field: "model", Message: validationRequiredMessage}
	}
	if strings.TrimSpace(model.ID) == "" {
		return &errors.ValidationError{Field: modelIDField, Value: model.ID, Message: validationRequiredMessage}
	}
	if model.ID != strings.TrimSpace(model.ID) {
		return &errors.ValidationError{Field: modelIDField, Value: model.ID, Message: "must not contain leading or trailing whitespace"}
	}
	if strings.IndexFunc(model.ID, unicode.IsControl) >= 0 {
		return &errors.ValidationError{Field: modelIDField, Value: model.ID, Message: "must not contain control characters"}
	}
	if model.ID != mapKey {
		return &errors.ValidationError{Field: modelIDField, Value: model.ID, Message: fmt.Sprintf("must match map identity %q", mapKey)}
	}
	if strings.TrimSpace(model.Name) == "" {
		return &errors.ValidationError{Field: "model.name", Value: model.Name, Message: validationRequiredMessage}
	}
	if strings.IndexFunc(model.Name, unicode.IsControl) >= 0 {
		return &errors.ValidationError{Field: "model.name", Value: model.Name, Message: "must not contain control characters"}
	}
	return nil
}

func modelsDevRecordIssue(providerID catalogs.ProviderID, modelKey string, err error) sources.ObservationIssue {
	return sources.ObservationIssue{
		Scope: sources.ObservationIssueScopeRecord, Code: sources.ObservationIssueCodeInvalidRecord,
		Subject: string(providerID) + "/" + modelKey, Message: err.Error(),
	}
}

func mergeModelsDevProviderMetadata(provider *catalogs.Provider, metadata *catalogs.Provider) {
	if metadata == nil {
		return
	}
	if provider.Name == "" || provider.Name == string(provider.ID) {
		provider.Name = metadata.Name
	}
	if len(metadata.Advisories) > 0 {
		existingAdvisories := make(map[string]struct{}, len(provider.Advisories))
		for _, advisory := range provider.Advisories {
			existingAdvisories[advisory.Name] = struct{}{}
		}
		for _, advisory := range metadata.Advisories {
			if _, exists := existingAdvisories[advisory.Name]; exists {
				continue
			}
			provider.Advisories = append(provider.Advisories, advisory)
		}
	}
	if len(metadata.Extensions) > 0 {
		if provider.Extensions == nil {
			provider.Extensions = metadata.Extensions.Copy()
			return
		}
		for source, extension := range metadata.Extensions {
			existing := provider.Extensions[source]
			if existing.Fields == nil {
				existing.Fields = make(map[string]any)
			}
			for key, value := range extension.Copy().Fields {
				if _, exists := existing.Fields[key]; !exists {
					existing.Fields[key] = value
				}
			}
			provider.Extensions[source] = existing
		}
	}
}
