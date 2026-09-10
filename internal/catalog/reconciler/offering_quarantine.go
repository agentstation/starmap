package reconciler

import (
	"slices"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
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
	collector *collector,
) ([]evidence.ReviewCandidate, error) {
	authored := authoredModelIdentities(catalog)
	var aliases *catalogs.CanonicalAliasIndex
	if baseline != nil {
		aliases = baseline.CanonicalAliases()
	}
	issues := make([]evidence.ReviewCandidate, 0)
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
			aliases,
		)
		if err != nil {
			return nil, err
		}
		for index := range providerIssues {
			observation, found := collector.reviewCandidateObservation(
				&provider,
				providerIssues[index].ProviderModelID,
			)
			if !found {
				return nil, &errors.ValidationError{
					Field: "review_candidate.source", Value: providerIssues[index].ProviderModelID,
					Message: "no source observation contains the quarantined provider offering",
				}
			}
			providerIssues[index].SourceID = observation.SourceID
			providerIssues[index].SourceObservationID = observation.ID
			providerIssues[index].SourceRevision = observation.Revision
			providerIssues[index].EvidenceChecksum = observation.EvidenceChecksum
		}
		provider.Models = providerModelMap(resolved)
		if err := catalog.SetProvider(provider); err != nil {
			return nil, errors.WrapResource("set", "provider", string(provider.ID), err)
		}
		issues = append(issues, providerIssues...)
	}
	slices.SortFunc(issues, evidence.CompareReviewCandidates)
	return issues, nil
}

func resolvableProviderModels(
	providerID catalogs.ProviderID,
	models []*catalogs.Model,
	authored map[catalogs.ModelDefinitionID]struct{},
	baseline map[string]*catalogs.Model,
	aliases *catalogs.CanonicalAliasIndex,
) ([]*catalogs.Model, []evidence.ReviewCandidate, error) {
	resolved := make([]*catalogs.Model, 0, len(models))
	issues := make([]evidence.ReviewCandidate, 0)
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
			if reference, found := resolvedModelReference(model.ModelRef, authored, aliases); found {
				carried := *model
				carried.ModelRef = reference
				resolved = append(resolved, &carried)
				continue
			}
		}
		priorReviewedModelLink := ""
		if baselineModel := baseline[model.ID]; baselineModel != nil {
			priorReviewedModelLink = string(baselineModel.ModelRef)
			if reference, found := resolvedModelReference(baselineModel.ModelRef, authored, aliases); found {
				carried := *model
				carried.ModelRef = reference
				resolved = append(resolved, &carried)
				continue
			}
		}
		issues = append(issues, evidence.ReviewCandidate{
			Code:                   evidence.ReviewCandidateUnresolvedModelReference,
			ProviderID:             string(providerID),
			ProviderModelID:        model.ID,
			Reason:                 unresolvedModelReferenceMessage,
			PriorReviewedModelLink: priorReviewedModelLink,
		})
	}
	return resolved, issues, nil
}

// resolvedModelReference normalizes retained facts without granting access through a removed client alias.
func resolvedModelReference(reference catalogs.ModelDefinitionID, authored map[catalogs.ModelDefinitionID]struct{}, aliases *catalogs.CanonicalAliasIndex) (catalogs.ModelDefinitionID, bool) {
	if terminal, _, found := aliases.Lookup(reference); found {
		reference = terminal
	}
	_, found := authored[reference]
	return reference, found
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
	issues []evidence.ReviewCandidate,
) {
	for _, issue := range issues {
		if issue.Code != evidence.ReviewCandidateUnresolvedModelReference {
			continue
		}
		prefix := string(evidence.ResourceTypeModel) + ":" + provenance.ModelResourceID(
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
