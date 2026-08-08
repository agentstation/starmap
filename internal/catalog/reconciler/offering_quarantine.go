package reconciler

import (
	"slices"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/provenance"
)

const unresolvedModelReferenceMessage = "provider offering has no explicit canonical model reference and was quarantined"

func authoredModelIdentities(reader catalogs.Reader) map[catalogs.ModelDefinitionID]struct{} {
	records := reader.AuthoredModels()
	identities := make(map[catalogs.ModelDefinitionID]struct{}, len(records))
	for _, record := range records {
		identities[record.ID()] = struct{}{}
	}
	return identities
}

func quarantineUnresolvedProviderOfferings(
	catalog *catalogs.Builder,
	baseline *catalogs.Catalog,
) ([]catalogmeta.ReconciliationIssue, error) {
	authored := authoredModelIdentities(catalog)
	issues := make([]catalogmeta.ReconciliationIssue, 0)
	for _, provider := range catalog.Providers().List() {
		models := make([]*catalogs.Model, 0, len(provider.Models))
		for _, model := range provider.Models {
			models = append(models, model)
		}

		var baselineModels map[string]*catalogs.Model
		if baseline != nil {
			if baselineProvider, err := baseline.Provider(provider.ID); err == nil {
				baselineModels = baselineProvider.Models
			}
		}
		resolved, providerIssues, err := resolvableProviderModels(
			provider.ID,
			models,
			authored,
			baselineModels,
		)
		if err != nil {
			return nil, err
		}
		provider.Models = providerModelMap(resolved)
		if err := catalog.SetProvider(provider); err != nil {
			return nil, errors.WrapResource("set", "provider", string(provider.ID), err)
		}
		issues = append(issues, providerIssues...)
	}
	slices.SortFunc(issues, compareReconciliationIssue)
	return issues, nil
}

func resolvableProviderModels(
	providerID catalogs.ProviderID,
	models []*catalogs.Model,
	authored map[catalogs.ModelDefinitionID]struct{},
	baseline map[string]*catalogs.Model,
) ([]*catalogs.Model, []catalogmeta.ReconciliationIssue, error) {
	resolved := make([]*catalogs.Model, 0, len(models))
	issues := make([]catalogmeta.ReconciliationIssue, 0)
	for _, model := range models {
		if model == nil {
			continue
		}
		if model.ModelRef != "" {
			if _, _, err := catalogs.ParseModelDefinitionID(model.ModelRef); err != nil {
				return nil, nil, errors.WrapResource(
					"validate",
					"provider model reference",
					string(model.ModelRef),
					err,
				)
			}
			if _, found := authored[model.ModelRef]; found {
				resolved = append(resolved, model)
				continue
			}
		}
		if baselineModel := baseline[model.ID]; baselineModel != nil {
			if _, found := authored[baselineModel.ModelRef]; found {
				carried := *model
				carried.ModelRef = baselineModel.ModelRef
				resolved = append(resolved, &carried)
				continue
			}
		}
		issues = append(issues, catalogmeta.ReconciliationIssue{
			Code:            catalogmeta.ReconciliationIssueUnresolvedModelReference,
			ProviderID:      string(providerID),
			ProviderModelID: model.ID,
			Message:         unresolvedModelReferenceMessage,
		})
	}
	return resolved, issues, nil
}

func compareReconciliationIssue(left, right catalogmeta.ReconciliationIssue) int {
	if result := strings.Compare(left.ProviderID, right.ProviderID); result != 0 {
		return result
	}
	if result := strings.Compare(left.ProviderModelID, right.ProviderModelID); result != 0 {
		return result
	}
	return strings.Compare(string(left.Code), string(right.Code))
}

func providerModelMap(models []*catalogs.Model) map[string]*catalogs.Model {
	indexed := make(map[string]*catalogs.Model, len(models))
	for _, model := range models {
		if model != nil {
			indexed[model.ID] = model
		}
	}
	return indexed
}

func removeQuarantinedModelProvenance(
	entries provenance.Map,
	issues []catalogmeta.ReconciliationIssue,
) {
	for _, issue := range issues {
		if issue.Code != catalogmeta.ReconciliationIssueUnresolvedModelReference {
			continue
		}
		prefix := string(catalogmeta.ResourceTypeModel) + ":" + provenance.ModelResourceID(
			issue.ProviderID,
			issue.ProviderModelID,
		) + ":"
		for key := range entries {
			if strings.HasPrefix(key, prefix) {
				delete(entries, key)
			}
		}
	}
}
