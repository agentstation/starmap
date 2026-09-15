package publication

import (
	"bytes"
	"encoding/json"
	"maps"
	"slices"
	"strings"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
)

type authoredCatalogView map[string]json.RawMessage

// mergeAuthoredBaseline applies edits to the original baseline using the last published catalog as their reference.
// Entity collections use their exact IDs. Arrays within a field retain their replacement semantics.
func mergeAuthoredBaseline(original, published, edited catalogs.Generation) (catalogs.Generation, error) {
	views := make([]authoredCatalogView, 0, 3)
	var aliases *catalogs.CanonicalAliasIndex
	for _, generation := range []catalogs.Generation{original, published, edited} {
		catalog, err := catalogs.DecodeCatalogGeneration(generation)
		if err != nil {
			return catalogs.Generation{}, err
		}
		payload, err := catalogs.EncodeCatalogPayload(catalog)
		if err != nil {
			return catalogs.Generation{}, err
		}
		view, err := publicationEditView(payload)
		if err != nil {
			return catalogs.Generation{}, err
		}
		views = append(views, view)
		aliases = catalog.CanonicalAliases()
	}
	merged, err := applyAuthoredObject(views[0], views[1], views[2])
	if err != nil {
		return catalogs.Generation{}, err
	}
	for _, name := range []string{"membership_scopes", "removal_policies", "canonical_aliases"} {
		objects := make([]map[string]json.RawMessage, 3)
		for index, view := range views {
			objects[index], err = authoredObject(view[name])
			if err != nil {
				return catalogs.Generation{}, err
			}
		}
		records, err := applyAuthoredMap(objects[0], objects[1], objects[2], false)
		if err != nil {
			return catalogs.Generation{}, err
		}
		merged[name], err = json.Marshal(records)
		if err != nil {
			return catalogs.Generation{}, err
		}
	}
	payload, err := publicationEditPayload(merged, views[2], aliases)
	if err != nil {
		return catalogs.Generation{}, err
	}
	catalog, err := catalogs.DecodeCatalogPayload(payload)
	if err != nil {
		return catalogs.Generation{}, err
	}
	payload, err = catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		return catalogs.Generation{}, err
	}
	evidence, err := authoredBaselineEvidence(catalog, original.Manifest, published.Manifest, edited.Manifest)
	if err != nil {
		return catalogs.Generation{}, err
	}
	identity, err := json.Marshal(struct {
		Payload  string
		Evidence starmap.CandidateEvidence
	}{catalogs.DescribeCatalogPayload(payload).Checksum, evidence})
	if err != nil {
		return catalogs.Generation{}, err
	}
	candidate, err := starmap.NewCandidate(catalog, evidence,
		starmap.WithCandidateGenerationID("authored-baseline-"+strings.TrimPrefix(receiptChecksum(identity), "sha256:")))
	if err != nil {
		return catalogs.Generation{}, err
	}
	return candidate.Generation(edited.Manifest.SyncRunID, edited.Manifest.GeneratedAt)
}

func publicationEditView(payload []byte) (authoredCatalogView, error) {
	var view authoredCatalogView
	if err := json.Unmarshal(payload, &view); err != nil {
		return nil, admissionError("baseline.payload", "cannot decode catalog fields")
	}
	for name, keys := range publicationCollectionKeys() {
		records, err := indexAuthoredRecords(view[name], keys)
		if err != nil {
			return nil, err
		}
		view[name], err = json.Marshal(records)
		if err != nil {
			return nil, err
		}
	}
	for _, name := range []string{"provider_models", "author_models"} {
		var groups map[string]json.RawMessage
		if err := json.Unmarshal(view[name], &groups); err != nil {
			return nil, admissionError("baseline.models", "cannot decode model groups")
		}
		for owner, raw := range groups {
			records, err := indexAuthoredRecords(raw, []string{"id"})
			if err != nil {
				return nil, err
			}
			groups[owner], err = json.Marshal(records)
			if err != nil {
				return nil, err
			}
		}
		raw, err := json.Marshal(groups)
		if err != nil {
			return nil, err
		}
		view[name] = raw
	}
	return view, nil
}

func publicationCollectionKeys() map[string][]string {
	return map[string][]string{
		"providers": {"id"}, "authors": {"id"}, "canonical_aliases": {"id"},
		"membership_scopes": {"publisher_id", "binding_id"}, "removal_policies": {"publisher_id"},
	}
}

func indexAuthoredRecords(raw json.RawMessage, fields []string) (map[string]json.RawMessage, error) {
	var records []json.RawMessage
	if len(raw) != 0 {
		if err := json.Unmarshal(raw, &records); err != nil {
			return nil, admissionError("baseline.records", "cannot decode catalog records")
		}
	}
	result := make(map[string]json.RawMessage, len(records))
	for _, record := range records {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(record, &object); err != nil {
			return nil, admissionError("baseline.record", "must be an object")
		}
		identity := make([]string, 0, len(fields))
		for _, field := range fields {
			var id string
			if err := json.Unmarshal(object[field], &id); err != nil || id == "" {
				return nil, admissionError("baseline.record", "requires its catalog identity")
			}
			identity = append(identity, id)
		}
		key, err := json.Marshal(identity)
		if err != nil {
			return nil, err
		}
		if _, exists := result[string(key)]; exists {
			return nil, admissionError("baseline.record", "contains a duplicate catalog identity")
		}
		result[string(key)] = record
	}
	return result, nil
}

func applyAuthoredObject(baseline, published, edited map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	return applyAuthoredMap(baseline, published, edited, true)
}

func applyAuthoredMap(baseline, published, edited map[string]json.RawMessage, recursive bool) (map[string]json.RawMessage, error) {
	result := maps.Clone(baseline)
	if result == nil {
		result = make(map[string]json.RawMessage)
	}
	for key, previous := range published {
		if _, exists := edited[key]; !exists && len(previous) != 0 {
			delete(result, key)
		}
	}
	for key, value := range edited {
		if bytes.Equal(published[key], value) {
			continue
		}
		if recursive && bytes.HasPrefix(bytes.TrimSpace(value), []byte("{")) && bytes.HasPrefix(bytes.TrimSpace(published[key]), []byte("{")) {
			objects := make([]map[string]json.RawMessage, 3)
			for index, raw := range []json.RawMessage{baseline[key], published[key], value} {
				if bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) {
					if err := json.Unmarshal(raw, &objects[index]); err != nil {
						return nil, admissionError("baseline.field", "cannot apply object edits to another field type")
					}
				}
			}
			merged, err := applyAuthoredObject(objects[0], objects[1], objects[2])
			if err != nil {
				return nil, err
			}
			result[key], err = json.Marshal(merged)
			if err != nil {
				return nil, err
			}
		} else {
			result[key] = value
		}
	}
	return result, nil
}

func publicationEditPayload(view, edited authoredCatalogView, aliases *catalogs.CanonicalAliasIndex) ([]byte, error) {
	if err := completeAuthoredIdentities(view, edited, aliases); err != nil {
		return nil, err
	}
	for _, name := range []string{"provider_models", "author_models"} {
		var groups map[string]json.RawMessage
		if err := json.Unmarshal(view[name], &groups); err != nil {
			return nil, admissionError("baseline.models", "cannot decode edited model groups")
		}
		for owner, raw := range groups {
			records, err := authoredRecordArray(raw)
			if err != nil {
				return nil, err
			}
			groups[owner], err = json.Marshal(records)
			if err != nil {
				return nil, err
			}
		}
		raw, err := json.Marshal(groups)
		if err != nil {
			return nil, err
		}
		view[name] = raw
	}
	for name := range publicationCollectionKeys() {
		records, err := authoredRecordArray(view[name])
		if err != nil {
			return nil, err
		}
		view[name], err = json.Marshal(records)
		if err != nil {
			return nil, err
		}
	}
	version, err := json.Marshal(catalogs.CurrentCatalogSchemaVersion)
	if err != nil {
		return nil, err
	}
	view["schema_version"] = version
	return json.Marshal(view)
}

func authoredRecordArray(raw json.RawMessage) ([]json.RawMessage, error) {
	var records map[string]json.RawMessage
	if err := json.Unmarshal(raw, &records); err != nil {
		return nil, admissionError("baseline.records", "cannot decode edited catalog records")
	}
	keys := slices.Sorted(maps.Keys(records))
	result := make([]json.RawMessage, 0, len(keys))
	for _, key := range keys {
		result = append(result, records[key])
	}
	return result, nil
}
