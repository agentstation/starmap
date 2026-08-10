package modelsdev

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// processFetch handles the common logic for fetching models from models.dev API.
func processFetch(
	catalog *catalogs.Builder,
	api *API,
	providers catalogs.ProvidersReader,
	opts ...sources.Option,
) (int, int, []sources.ObservationIssue, error) {
	options := sources.Defaults().Apply(opts...)
	candidateCount := modelsDevCandidateCount(api, providers, options.ProviderID)

	catalog.SetMergeStrategy(catalogs.MergeEnrichEmpty)

	added := 0
	issues := make([]sources.ObservationIssue, 0)
	providerKeys := make([]string, 0, len(*api))
	for providerKey := range *api {
		providerKeys = append(providerKeys, providerKey)
	}
	sort.Strings(providerKeys)
	for _, providerKey := range providerKeys {
		mdProvider := (*api)[providerKey]
		providerAdded, providerIssues, limitExceeded, err := processModelsDevProvider(
			catalog,
			&mdProvider,
			providers,
			options.ProviderID,
			constants.MaxCatalogModels-added,
		)
		added += providerAdded
		issues = append(issues, providerIssues...)
		if err != nil {
			return added, candidateCount - added, issues, err
		}
		if limitExceeded {
			break
		}
	}

	return added, candidateCount - added, issues, nil
}

func processModelsDevProvider(
	catalog *catalogs.Builder,
	mdProvider *Provider,
	providers catalogs.ProvidersReader,
	providerFilter *catalogs.ProviderID,
	remaining int,
) (int, []sources.ObservationIssue, bool, error) {
	providerID := canonicalProviderID(providers, catalogs.ProviderID(mdProvider.ID))
	if providerFilter != nil && providerID != *providerFilter {
		return 0, nil, false, nil
	}
	issues := modelsDevProviderIssues(providerID, mdProvider)
	if mdProvider.Models == nil {
		return 0, append(issues, sources.ObservationIssue{
			Scope: sources.ObservationIssueScopeProvider, Code: sources.ObservationIssueCodeSchemaDrift,
			Subject: string(providerID), Message: "required models object is missing or null",
		}), false, nil
	}

	provider, err := catalog.Provider(providerID)
	if err != nil {
		provider = catalogs.Provider{ID: providerID, Name: mdProvider.ID}
	}
	mergeModelsDevProviderMetadata(&provider, mdProvider.toStarmapProviderMetadata())
	if provider.Models == nil {
		provider.Models = make(map[string]*catalogs.Model)
	}

	added, modelIssues, limitExceeded := addModelsDevModels(&provider, mdProvider, remaining)
	issues = append(issues, modelIssues...)
	if len(provider.Models) > 0 {
		if err := catalog.SetProvider(provider); err != nil {
			return added, issues, limitExceeded, errors.WrapResource(
				"set", "provider", string(provider.ID), err,
			)
		}
	}
	return added, issues, limitExceeded, nil
}

func modelsDevProviderIssues(
	providerID catalogs.ProviderID,
	provider *Provider,
) []sources.ObservationIssue {
	issues := make([]sources.ObservationIssue, 0, len(provider.RecordReport.Issues)+1)
	for _, issue := range provider.RecordReport.Issues {
		issues = append(issues, modelsDevRecordIssue(providerID, issue.Subject, issue.Err))
	}
	if provider.RecordReport.Truncated {
		issues = append(issues, sources.ObservationIssue{
			Scope: sources.ObservationIssueScopeProvider, Code: sources.ObservationIssueCodePayloadLimit,
			Subject: string(providerID), Message: "models.dev model count exceeds maximum; excess records quarantined",
		})
	}
	return issues
}

func addModelsDevModels(
	provider *catalogs.Provider,
	mdProvider *Provider,
	remaining int,
) (int, []sources.ObservationIssue, bool) {
	added := 0
	issues := make([]sources.ObservationIssue, 0)
	modelKeys := make([]string, 0, len(mdProvider.Models))
	for modelKey := range mdProvider.Models {
		modelKeys = append(modelKeys, modelKey)
	}
	sort.Strings(modelKeys)
	for _, modelKey := range modelKeys {
		if added >= remaining {
			issues = append(issues, sources.ObservationIssue{
				Scope: sources.ObservationIssueScopeSource, Code: sources.ObservationIssueCodePayloadLimit,
				Message: "models.dev model count exceeds maximum; excess records quarantined",
			})
			return added, issues, true
		}
		mdModel := mdProvider.Models[modelKey]
		if !mdModel.hasCatalogData() {
			continue
		}
		if err := validateModelsDevModelIdentity(modelKey, &mdModel); err != nil {
			issues = append(issues, modelsDevRecordIssue(provider.ID, modelKey, err))
			continue
		}
		model, err := mdModel.ToStarmapModel()
		if err != nil {
			issues = append(issues, modelsDevRecordIssue(
				provider.ID,
				modelKey,
				errors.WrapResource("convert", "model", mdModel.ID, err),
			))
			continue
		}
		provider.Models[model.ID] = model
		added++
	}
	return added, issues, false
}

func modelsDevCandidateCount(
	api *API,
	providers catalogs.ProvidersReader,
	providerFilter *catalogs.ProviderID,
) int {
	if api == nil {
		return 0
	}
	count := 0
	for _, provider := range *api {
		providerID := canonicalProviderID(providers, catalogs.ProviderID(provider.ID))
		if providerFilter != nil && providerID != *providerFilter {
			continue
		}
		count += provider.RecordReport.Rejected
		for _, model := range provider.Models {
			if model.hasCatalogData() {
				count++
			}
		}
	}
	return count
}

func canonicalProviderID(providers catalogs.ProvidersReader, id catalogs.ProviderID) catalogs.ProviderID {
	if providers == nil {
		return id
	}
	provider, found := providers.Resolve(id)
	if !found || provider == nil {
		return id
	}
	return provider.ID
}

func validateModelsDevModelIdentity(mapKey string, model *Model) error {
	if model == nil {
		return &errors.ValidationError{Field: "model", Message: "is required"}
	}
	if strings.TrimSpace(model.ID) == "" {
		return &errors.ValidationError{Field: "model.id", Value: model.ID, Message: "is required"}
	}
	if model.ID != strings.TrimSpace(model.ID) {
		return &errors.ValidationError{Field: "model.id", Value: model.ID, Message: "must not contain leading or trailing whitespace"}
	}
	if strings.IndexFunc(model.ID, unicode.IsControl) >= 0 {
		return &errors.ValidationError{Field: "model.id", Value: model.ID, Message: "must not contain control characters"}
	}
	if model.ID != mapKey {
		return &errors.ValidationError{Field: "model.id", Value: model.ID, Message: fmt.Sprintf("must match map identity %q", mapKey)}
	}
	if strings.TrimSpace(model.Name) == "" {
		return &errors.ValidationError{Field: "model.name", Value: model.Name, Message: "is required"}
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
	mergeModelsDevCatalogMetadata(provider, metadata)
	mergeModelsDevExtensions(provider, metadata)
}

func mergeModelsDevCatalogMetadata(provider, metadata *catalogs.Provider) {
	if metadata.Catalog == nil {
		return
	}
	if provider.Catalog == nil {
		catalogCopy := *metadata.Catalog
		if metadata.Catalog.Docs != nil {
			docs := *metadata.Catalog.Docs
			catalogCopy.Docs = &docs
		}
		provider.Catalog = &catalogCopy
		return
	}
	if provider.Catalog.Docs == nil && metadata.Catalog.Docs != nil {
		docs := *metadata.Catalog.Docs
		provider.Catalog.Docs = &docs
	}
}

func mergeModelsDevExtensions(provider, metadata *catalogs.Provider) {
	if len(metadata.Extensions) == 0 {
		return
	}
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
			if _, found := existing.Fields[key]; !found {
				existing.Fields[key] = value
			}
		}
		provider.Extensions[source] = existing
	}
}
