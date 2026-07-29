package reconciler

import (
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// reconcileAuthoredCorpus applies the provider-independent construction inputs
// before provider offerings are reconciled. The durable baseline remains the
// last-known-good fallback, the verified embedded corpus may add definitions,
// and a complete human workspace is the editable authority.
//
// Provider observations are deliberately absent from this flow. They may link
// an offering to an authored model, but they cannot infer who authored it.
func reconcileAuthoredCorpus(
	target *catalogs.Builder,
	baseline *catalogs.Catalog,
	collector *collector,
) error {
	bootstrap := collector.authoredBootstrap()
	local := collector.catalog(sources.LocalCatalogID)

	if err := upsertAuthors(target, bootstrap); err != nil {
		return err
	}
	if err := upsertAuthors(target, local); err != nil {
		return err
	}

	baselineModels := authoredModelsByID(baseline)
	bootstrapModels := authoredModelsByID(bootstrap)
	localModels := authoredModelsByID(local)

	// Embedded data is a bootstrap and additive update source. It cannot
	// overwrite a retained or human-edited definition without field-level
	// authored-definition provenance.
	for id, record := range bootstrapModels {
		if _, retained := baselineModels[id]; retained {
			continue
		}
		if err := setAuthoredModel(target, record); err != nil {
			return err
		}
	}

	// The human workspace is explicit editable input, so matching records
	// replace the durable baseline and embedded bootstrap.
	for _, record := range localModels {
		if err := setAuthoredModel(target, record); err != nil {
			return err
		}
	}

	// Absence is deletion evidence only for a complete, successful human
	// observation, and only for definitions no upstream bootstrap can restore.
	if _, complete := collector.completeObservation(sources.LocalCatalogID); complete {
		for id, record := range baselineModels {
			if _, present := localModels[id]; present {
				continue
			}
			if _, upstream := bootstrapModels[id]; upstream {
				continue
			}
			if err := target.DeleteAuthorModel(record.AuthorID, record.Model.ID); err != nil {
				return errors.WrapResource("delete", "authored model", string(id), err)
			}
		}
	}

	return nil
}

func upsertAuthors(target *catalogs.Builder, source *catalogs.Catalog) error {
	if source == nil {
		return nil
	}
	for _, author := range source.Authors().List() {
		if err := target.SetAuthor(author); err != nil {
			return errors.WrapResource("set", "author", string(author.ID), err)
		}
	}
	return nil
}

func authoredModelsByID(source *catalogs.Catalog) map[catalogs.ModelDefinitionID]catalogs.AuthoredModel {
	if source == nil {
		return nil
	}
	records := source.AuthoredModels()
	result := make(map[catalogs.ModelDefinitionID]catalogs.AuthoredModel, len(records))
	for _, record := range records {
		result[record.ID()] = record
	}
	return result
}

func setAuthoredModel(target *catalogs.Builder, record catalogs.AuthoredModel) error {
	if err := target.SetAuthorModel(record.AuthorID, record.Model); err != nil {
		return errors.WrapResource("set", "authored model", string(record.ID()), err)
	}
	return nil
}
