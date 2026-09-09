package reconciler

import (
	"strconv"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

// rejectsLegacyCompositeValue checks refusal against an older aggregate claim.
// An aggregate cannot establish independent provenance for its child fields.
func (merger *merger) rejectsLegacyCompositeValue(providerID catalogs.ProviderID, modelID string, policy authority.Policy, localValue any) bool {
	if merger.projectedEvidence == nil {
		return false
	}
	root := ""
	switch policy.Path {
	case "Metadata":
		root = "metadata"
	case "Modes":
		root = "modes"
	case "Extensions":
		root = "extensions"
	case "Authors", "Features":
		root = policy.Path
	default:
		return false
	}
	field := policy.Evidence()
	if field == root || !compositeEvidenceField(root, field) {
		return false
	}
	catalog := merger.sourceCatalogs[sources.LocalCatalogID]
	if catalog == nil || len(catalog.Provenance().FindModelField(providerID, modelID, field)) != 0 {
		return false
	}
	entries := catalog.Provenance().FindModelField(providerID, modelID, root)
	if len(entries) == 0 {
		return false
	}
	current := latestCompositeEntry(entries)
	if current.Source == "" || merger.projectedEvidence(providerID, current) {
		return false
	}
	value, err := normalizedSemanticValue("", current.Value)
	if err != nil {
		return false
	}
	value, found := legacyCompositeValue(value, strings.TrimPrefix(field, root))
	return found && semanticValueEqual(field, value, localValue)
}

func latestCompositeEntry(entries []provenance.Entry) provenance.Entry {
	current := entries[0]
	for _, entry := range entries[1:] {
		if entry.Timestamp.After(current.Timestamp) {
			current = entry
		}
	}
	return current
}

// legacyCompositeValue reads the fact named by a generated evidence path.
// Membership compares stable IDs, so a changed display name cannot grant access.
func legacyCompositeValue(value any, path string) (any, bool) {
	for path != "" {
		if path == ".present" {
			return true, value != nil
		}
		var key string
		switch path[0] {
		case '.':
			path = path[1:]
			end := strings.IndexAny(path, ".[")
			if end < 0 {
				key, path = path, ""
			} else {
				key, path = path[:end], path[end:]
			}
		case '[':
			end := 1
			if len(path) < 3 || path[1] != '"' {
				return nil, false
			}
			for end++; end < len(path); end++ {
				if path[end] == '\\' {
					end++
					continue
				}
				if path[end] == '"' {
					break
				}
			}
			if end+1 >= len(path) || path[end+1] != ']' {
				return nil, false
			}
			var err error
			key, err = strconv.Unquote(path[1 : end+1])
			if err != nil {
				return nil, false
			}
			path = path[end+2:]
		default:
			return nil, false
		}
		var found bool
		switch node := value.(type) {
		case map[string]any:
			value, found = node[key]
		case []any:
			for _, item := range node {
				if record, ok := item.(map[string]any); ok && record["id"] == key {
					value, found = record, true
					break
				}
				if item == key {
					value, found = true, true
					break
				}
			}
		}
		if !found {
			return nil, false
		}
	}
	return value, true
}
