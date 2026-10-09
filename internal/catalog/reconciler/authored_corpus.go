package reconciler

import (
	"reflect"
	"slices"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// reconcileAuthoredCorpus applies provider-independent construction inputs
// before reconciling provider offerings. The durable baseline remains the
// last-known-good fallback, the verified embedded corpus may add definitions,
// and a complete human catalog workspace is the editable authority.
//
// Provider observations are deliberately absent from this flow. They may link
// an offering to an authored model, but they cannot infer who authored it.
func reconcileAuthoredCorpus(
	target *catalogs.Builder,
	baseline *catalogs.Catalog,
	collector *collector,
	strategy *AuthorityStrategy,
) error {
	bootstrap := collector.authoredBootstrap()
	local := collector.catalog(sources.LocalCatalogID)
	var aliases *catalogs.CanonicalAliasIndex
	if baseline != nil {
		aliases = baseline.CanonicalAliases()
	}

	if err := addMissingAuthors(target, bootstrap); err != nil {
		return err
	}
	if err := upsertAuthors(target, local, strategy); err != nil {
		return err
	}

	baselineModels := authoredModelsByID(baseline)
	bootstrapModels := authoredModelsByID(bootstrap)
	localModels := authoredModelsByID(local)

	// Embedded data is a bootstrap and additive update source. It cannot
	// overwrite a retained or human-edited definition without field-level
	// authored-definition provenance.
	for id, record := range bootstrapModels {
		if _, _, retired := aliases.Lookup(id); retired {
			continue
		}
		if _, retained := baselineModels[id]; retained {
			continue
		}
		if err := setAuthoredModel(target, record); err != nil {
			return err
		}
	}

	// The human catalog workspace is explicit editable input, so matching records
	// replace the durable baseline and embedded bootstrap.
	for _, record := range localModels {
		if _, _, retired := aliases.Lookup(record.ID()); retired {
			return &errors.ConflictError{Resource: "local authored model", Actual: string(record.ID()), Message: "use the current canonical ID instead of a retired ID"}
		}
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

func addMissingAuthors(target *catalogs.Builder, source *catalogs.Catalog) error {
	if source == nil {
		return nil
	}
	for _, author := range source.Authors().List() {
		if _, found := target.Authors().Resolve(author.ID); found {
			continue
		}
		if err := target.SetAuthor(author); err != nil {
			return errors.WrapResource("set", "author", string(author.ID), err)
		}
	}
	return nil
}

func upsertAuthors(target *catalogs.Builder, source *catalogs.Catalog, strategy *AuthorityStrategy) error {
	if source == nil {
		return nil
	}
	for _, author := range source.Authors().List() {
		if retained, found := target.Authors().Resolve(author.ID); found && retained != nil {
			author = mergeLocalAuthor(*retained, author, strategy)
		}
		if err := target.SetAuthor(author); err != nil {
			return errors.WrapResource("set", "author", string(author.ID), err)
		}
	}
	return nil
}

// mergeLocalAuthor applies present workspace facts and retains absent fields.
func mergeLocalAuthor(retained, incoming catalogs.Author, strategy *AuthorityStrategy) catalogs.Author {
	result := catalogs.DeepCopyAuthor(retained)
	target := reflect.ValueOf(&result).Elem()
	input := reflect.ValueOf(incoming)
	for index := range input.NumField() {
		name := input.Type().Field(index).Name
		policy, found := strategy.authorities.Find(evidence.ResourceTypeAuthor, name)
		if !found || !slices.Contains(policy.SourceOrder, sources.LocalCatalogID) {
			continue
		}
		field := input.Field(index)
		if !policyAccepts(policy, field.Interface()) {
			continue
		}
		if policy.Empty == authority.EmptyAbsent && field.Kind() == reflect.Slice && field.Len() == 0 {
			continue
		}
		if name == "Aliases" && policy.Merge == authority.MergeSetUnion {
			result.Aliases = nil
			for _, alias := range append(slices.Clone(incoming.Aliases), retained.Aliases...) {
				if !slices.Contains(result.Aliases, alias) {
					result.Aliases = append(result.Aliases, alias)
				}
			}
			continue
		}
		target.Field(index).Set(field)
	}
	return catalogs.DeepCopyAuthor(result)
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
