package reconciler

import (
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

// mergeAuthorshipContributions records each author membership independently.
func (merger *merger) mergeAuthorshipContributions(identity modelIdentity, target *catalogs.Model, policy authority.Policy, models map[sources.ID]*catalogs.Model, history *map[string]provenance.Field) {
	ids := make([]catalogs.AuthorID, 0)
	seen := make(map[catalogs.AuthorID]bool)
	for _, source := range policy.SourceOrder {
		if model := models[source]; model != nil {
			for _, author := range model.Authors {
				if !seen[author.ID] {
					ids = append(ids, author.ID)
					seen[author.ID] = true
				}
			}
		}
	}
	var baseline *catalogs.Model
	if models[sources.LocalCatalogID] == nil {
		baseline = merger.baselineModel(identity.providerID, identity.modelID)
		if baseline != nil {
			for _, author := range baseline.Authors {
				if !seen[author.ID] {
					ids = append(ids, author.ID)
					seen[author.ID] = true
				}
			}
		}
	}
	if len(ids) == 0 {
		return
	}
	result := make([]catalogs.Author, 0, len(ids))
	for _, id := range ids {
		author, present := merger.selectAuthorContribution(identity, policy, models, history, id)
		if !present {
			author, present = modelAuthor(baseline, id)
		}
		if present {
			result = append(result, author)
		} else {
			merger.clearAuthorshipEvidence(history, policy.Evidence()+compositeMapKey(string(id))+".present", false)
		}
	}
	target.Authors = catalogs.DeepCopyModel(catalogs.Model{Authors: result}).Authors
	if len(result) == 0 {
		merger.clearAuthorshipEvidence(history, policy.Evidence(), target.Authors)
		return
	}
	merger.recordCompositeSummary(history, policy, target.Authors)
}

// clearAuthorshipEvidence replaces a rejected current claim, not source history.
func (merger *merger) clearAuthorshipEvidence(history *map[string]provenance.Field, field string, value any) {
	if history == nil {
		return
	}
	(*history)[field] = provenance.Field{Current: provenance.Entry{
		Field: field, Value: value, Timestamp: merger.changeTime(),
		Reason: "cleared by authorship authority policy",
	}}
}

func modelAuthor(model *catalogs.Model, id catalogs.AuthorID) (catalogs.Author, bool) {
	if model != nil {
		for _, author := range model.Authors {
			if author.ID == id {
				return author, true
			}
		}
	}
	return catalogs.Author{}, false
}

// selectAuthorContribution selects membership and details from one resolved input.
func (merger *merger) selectAuthorContribution(identity modelIdentity, policy authority.Policy, models map[sources.ID]*catalogs.Model, history *map[string]provenance.Field, id catalogs.AuthorID) (catalogs.Author, bool) {
	field := policy
	field.EvidencePath = policy.Evidence() + compositeMapKey(string(id)) + ".present"
	resolved := merger.modelSourcesForValue(identity.providerID, identity.modelID, field, models, func(model *catalogs.Model) any {
		if _, present := modelAuthor(model, id); present {
			return true
		}
		return nil
	})
	for _, source := range policy.SourceOrder {
		author, present := modelAuthor(resolved[source], id)
		if !present {
			continue
		}
		merger.recordModelHistory(identity, history, field, source, true, "selected author membership with its original receipt")
		return author, true
	}
	return catalogs.Author{}, false
}
