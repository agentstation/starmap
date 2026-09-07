package app

import (
	"maps"
	"slices"
	"strings"

	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
)

// catalogInput keeps normalized values and safe migration diagnostics from one authority.
type catalogInput struct {
	values      map[string]string
	legacyNames []string
}

// normalizeLegacyCatalogSource preserves the source boundary within one input authority.
// A canonical source identity replaces the complete legacy source group.
func normalizeLegacyCatalogSource(canonical, legacy map[string]string) map[string]string {
	result := make(map[string]string, len(canonical)+3)
	maps.Copy(result, canonical)
	for _, descriptor := range catalogconfig.Descriptors() {
		if descriptor.SourceBinding == "" || descriptor.Sensitive {
			continue
		}
		if _, present := canonical[descriptor.Name]; present {
			return result
		}
	}
	if url, present := legacy[catalogconfig.SourceURL]; present {
		result[catalogconfig.Source] = "starmap"
		result[catalogconfig.SourceURL] = url
	}
	if key, present := legacy[catalogconfig.SourceAPIKey]; present {
		if _, explicit := result[catalogconfig.SourceAPIKey]; !explicit {
			result[catalogconfig.SourceAPIKey] = key
		}
	}
	return result
}

func catalogEnvironmentInput(values map[string]string) catalogInput {
	canonical := make(map[string]string)
	for _, name := range catalogconfig.Names() {
		if value, present := values[name]; present {
			canonical[name] = value
		}
	}
	for name, value := range values {
		if strings.HasPrefix(name, "STARMAP_CATALOG_") && !isPathEnvironmentName(name) {
			canonical[name] = value
		}
	}
	legacy := make(map[string]string)
	var names []string
	for _, alias := range []struct{ name, target string }{
		{"REMOTE_SERVER_URL", catalogconfig.SourceURL}, {"REMOTE_SERVER_API_KEY", catalogconfig.SourceAPIKey},
	} {
		if value, present := values[alias.name]; present {
			legacy[alias.target] = value
			names = append(names, alias.name)
		}
	}
	return catalogInput{values: normalizeLegacyCatalogSource(canonical, legacy), legacyNames: names}
}

func isCatalogEnvironmentName(name string) bool {
	return strings.HasPrefix(name, "STARMAP_CATALOG_") || slices.Contains(catalogconfig.Names(), name) || name == "REMOTE_SERVER_URL" || name == "REMOTE_SERVER_API_KEY"
}
