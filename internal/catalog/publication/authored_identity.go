package publication

import (
	"encoding/json"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// completeAuthoredIdentities retains only the identity needed by newly authored records.
// Required display names fall back to exact IDs when the author did not supply a name.
func completeAuthoredIdentities(view, edited authoredCatalogView, aliases *catalogs.CanonicalAliasIndex) error {
	providers, err := authoredObject(view["providers"])
	if err != nil {
		return err
	}
	authors, err := authoredObject(view["authors"])
	if err != nil {
		return err
	}
	definitions, err := authoredObject(view["author_models"])
	if err != nil {
		return err
	}
	if err := completeAuthoredAliasDefinitions(view, edited, definitions, aliases); err != nil {
		return err
	}
	if err := completeAuthoredModelIdentities(view, edited, definitions, providers, authors); err != nil {
		return err
	}
	return completeAuthoredOwnerIdentities(view, providers, authors)
}

func completeAuthoredAliasDefinitions(view, edited authoredCatalogView, definitions map[string]json.RawMessage, aliases *catalogs.CanonicalAliasIndex) error {
	aliasRecords, err := authoredObject(view["canonical_aliases"])
	if err != nil {
		return err
	}
	for _, raw := range aliasRecords {
		var record catalogs.CanonicalAlias
		if err := json.Unmarshal(raw, &record); err != nil {
			return admissionError("baseline.canonical_alias", "cannot decode the authored alias")
		}
		if record.State != catalogs.CanonicalAliasActive {
			continue
		}
		target, _, found := aliases.Lookup(record.ID)
		if !found {
			return admissionError("baseline.canonical_alias", "requires its authored terminal identity")
		}
		reference, err := json.Marshal(target)
		if err != nil {
			return err
		}
		if err := ensureAuthoredDefinition(definitions, edited, reference); err != nil {
			return err
		}
	}
	return nil
}

func completeAuthoredModelIdentities(view, edited authoredCatalogView, definitions, providers, authors map[string]json.RawMessage) error {
	for _, name := range []string{"provider_models", "author_models"} {
		groups, err := authoredObject(view[name])
		if err != nil {
			return err
		}
		if name == "author_models" {
			groups = definitions
		}
		editedGroups, err := authoredObject(edited[name])
		if err != nil {
			return err
		}
		for owner, raw := range groups {
			records, err := authoredObject(raw)
			if err != nil {
				return err
			}
			editedRecords, err := authoredObject(editedGroups[owner])
			if err != nil {
				return err
			}
			for key, raw := range records {
				record, err := authoredObject(raw)
				if err != nil {
					return err
				}
				input, err := authoredObject(editedRecords[key])
				if err != nil {
					return err
				}
				if len(record["id"]) == 0 {
					record["id"] = input["id"]
				}
				if len(record["name"]) == 0 {
					record["name"] = record["id"]
				}
				if name == "provider_models" {
					if len(record["model"]) == 0 {
						record["model"] = input["model"]
					}
					if err := ensureAuthoredDefinition(definitions, edited, record["model"]); err != nil {
						return err
					}
				} else if len(record["authors"]) == 0 {
					record["authors"], err = json.Marshal([]catalogs.Author{{ID: catalogs.AuthorID(owner), Name: owner}})
					if err != nil {
						return err
					}
				}
				records[key], err = json.Marshal(record)
				if err != nil {
					return err
				}
			}
			if len(records) == 0 {
				continue
			}
			parent := authors
			if name == "provider_models" {
				parent = providers
			}
			if _, exists := parent[authoredRecordKey(owner)]; !exists {
				parent[authoredRecordKey(owner)], err = json.Marshal(map[string]string{"id": owner, "name": owner})
				if err != nil {
					return err
				}
			}
			groups[owner], err = json.Marshal(records)
			if err != nil {
				return err
			}
		}
		view[name], err = json.Marshal(groups)
		if err != nil {
			return err
		}
	}
	return nil
}

func completeAuthoredOwnerIdentities(view authoredCatalogView, providers, authors map[string]json.RawMessage) error {
	var err error
	for name, records := range map[string]map[string]json.RawMessage{"providers": providers, "authors": authors} {
		for key, raw := range records {
			record, err := authoredObject(raw)
			if err != nil {
				return err
			}
			var identity []string
			if err := json.Unmarshal([]byte(key), &identity); err != nil || len(identity) != 1 {
				return admissionError("baseline.identity", "cannot restore the record identity")
			}
			if len(record["id"]) == 0 {
				record["id"], err = json.Marshal(identity[0])
				if err != nil {
					return err
				}
			}
			if len(record["name"]) == 0 {
				record["name"] = record["id"]
			}
			records[key], err = json.Marshal(record)
			if err != nil {
				return err
			}
		}
		view[name], err = json.Marshal(records)
		if err != nil {
			return err
		}
		modelsName := "author_models"
		if name == "providers" {
			modelsName = "provider_models"
		}
		groups, err := authoredObject(view[modelsName])
		if err != nil {
			return err
		}
		for key := range records {
			var identity []string
			if err := json.Unmarshal([]byte(key), &identity); err != nil || len(identity) != 1 {
				return admissionError("baseline.identity", "cannot restore the model group identity")
			}
			if _, exists := groups[identity[0]]; !exists {
				groups[identity[0]] = json.RawMessage(`{}`)
			}
		}
		view[modelsName], err = json.Marshal(groups)
		if err != nil {
			return err
		}
	}
	return nil
}

func ensureAuthoredDefinition(definitions map[string]json.RawMessage, edited authoredCatalogView, reference json.RawMessage) error {
	var id catalogs.ModelDefinitionID
	if err := json.Unmarshal(reference, &id); err != nil {
		return admissionError("baseline.model", "requires its canonical model reference")
	}
	author, slug, err := catalogs.ParseModelDefinitionID(id)
	if err != nil {
		return err
	}
	group, err := authoredObject(definitions[string(author)])
	if err != nil {
		return err
	}
	key := authoredRecordKey(slug)
	if _, exists := group[key]; exists {
		return nil
	}
	available, err := authoredObject(edited["author_models"])
	if err != nil {
		return err
	}
	models, err := authoredObject(available[string(author)])
	if err != nil {
		return err
	}
	if _, exists := models[key]; !exists {
		return admissionError("baseline.model", "references a definition absent from the authored catalog")
	}
	group[key], err = json.Marshal(catalogs.Model{ID: slug, Name: slug, Authors: []catalogs.Author{{ID: author, Name: string(author)}}})
	if err != nil {
		return err
	}
	definitions[string(author)], err = json.Marshal(group)
	return err
}

func authoredObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	result := make(map[string]json.RawMessage)
	if len(raw) != 0 {
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, admissionError("baseline.object", "must contain catalog object fields")
		}
	}
	if result == nil {
		result = make(map[string]json.RawMessage)
	}
	return result, nil
}

func authoredRecordKey(identity ...string) string {
	data, _ := json.Marshal(identity)
	return string(data)
}
