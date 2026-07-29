package catalogs

import (
	"path"
	"slices"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

func deriveDefinitionAuthors(reader Reader, candidates []providerModelCandidate) ([]AuthorID, error) {
	ids := make([]AuthorID, 0)
	for _, candidate := range candidates {
		for _, author := range candidate.model.Authors {
			ids = append(ids, canonicalAuthorID(reader, author.ID))
		}
		if candidate.providerID == "" {
			continue
		}
		attributed, err := attributedAuthorIDs(reader, candidate.providerID, candidate.model.ID)
		if err != nil {
			return nil, err
		}
		ids = append(ids, attributed...)
	}
	slices.Sort(ids)
	return slices.Compact(ids), nil
}

func canonicalAuthorID(reader Reader, id AuthorID) AuthorID {
	if author, found := reader.Authors().Resolve(id); found && author != nil {
		return author.ID
	}
	return id
}

func attributedAuthorIDs(reader Reader, providerID ProviderID, modelID string) ([]AuthorID, error) {
	var ids []AuthorID
	for _, author := range reader.Authors().List() {
		if author.Catalog == nil || author.Catalog.Attribution == nil {
			continue
		}
		attribution := author.Catalog.Attribution
		if attribution.ProviderID != "" && attribution.ProviderID != providerID {
			continue
		}
		if len(attribution.Patterns) == 0 {
			if attribution.ProviderID == providerID {
				ids = append(ids, author.ID)
			}
			continue
		}
		for _, pattern := range attribution.Patterns {
			matched, err := path.Match(strings.ToLower(pattern), strings.ToLower(modelID))
			if err != nil {
				return nil, errors.WrapParse("glob", "author "+string(author.ID)+" pattern", err)
			}
			if matched {
				ids = append(ids, author.ID)
				break
			}
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids), nil
}

func deriveAuthorDefinitions(
	reader Reader,
	definitions map[ModelDefinitionID]ModelDefinition,
) map[AuthorID][]ModelDefinitionID {
	index := make(map[AuthorID][]ModelDefinitionID)
	for _, author := range reader.Authors().List() {
		index[author.ID] = nil
		for _, alias := range author.Aliases {
			index[alias] = nil
		}
	}
	for definitionID, definition := range definitions {
		for _, authorID := range definition.AuthorIDs {
			canonical := canonicalAuthorID(reader, authorID)
			index[canonical] = append(index[canonical], definitionID)
		}
	}
	for authorID, definitionIDs := range index {
		slices.Sort(definitionIDs)
		index[authorID] = slices.Compact(definitionIDs)
	}
	for _, author := range reader.Authors().List() {
		definitionIDs := index[author.ID]
		for _, alias := range author.Aliases {
			index[alias] = definitionIDs
		}
	}
	return index
}
