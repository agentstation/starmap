package app

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/errors"
)

// catalogValuesFromFile validates file keys before environment lookup can hide them.
func catalogValuesFromFile(values map[string]any) (catalogInput, error) {
	applicationKeys := []string{"verbose", "quiet", "no-color", "output", "catalog_path",
		"embedded_bootstrap_max_age", "embedded_bootstrap_max_size_bytes", "remote_server_url",
		"remote_server_api_key", "remote_server_only", "credential_sources", "log_level", "log_format", "log_output"}
	descriptors := make(map[string]catalogconfig.Descriptor)
	for _, descriptor := range catalogconfig.Descriptors() {
		descriptors[descriptor.Key] = descriptor
	}
	result := make(map[string]string)
	for key, value := range values {
		if slices.Contains(applicationKeys, key) || pathKeyKnown(key) {
			continue
		}
		descriptor, known := descriptors[key]
		if !known {
			return catalogInput{}, &errors.ValidationError{Field: key, Message: "is not a supported configuration key"}
		}
		text, err := catalogFileValue(descriptor, value)
		if err != nil {
			return catalogInput{}, err
		}
		result[descriptor.Name] = text
	}
	legacy := make(map[string]string)
	var names []string
	for _, alias := range []struct{ name, target string }{
		{"remote_server_url", catalogconfig.SourceURL}, {"remote_server_api_key", catalogconfig.SourceAPIKey},
	} {
		if value, present := values[alias.name]; present {
			text, ok := value.(string)
			if !ok && value != nil {
				return catalogInput{}, &errors.ValidationError{Field: alias.name, Message: "must be text"}
			}
			legacy[alias.target] = text
			names = append(names, alias.name)
		}
	}
	return catalogInput{values: normalizeLegacyCatalogSource(result, legacy), legacyNames: names}, nil
}

func catalogFileValue(descriptor catalogconfig.Descriptor, value any) (string, error) {
	invalid := &errors.ValidationError{Field: descriptor.Key, Message: "has the wrong configuration value type"}
	switch typed := value.(type) {
	case nil:
		return "", nil
	case string:
		return typed, nil
	case bool:
		if descriptor.Type == catalogconfig.BooleanValue {
			return strconv.FormatBool(typed), nil
		}
	case int, int64, uint64, float64:
		if descriptor.Type == catalogconfig.IntegerValue || descriptor.Type == catalogconfig.DurationValue {
			return fmt.Sprint(typed), nil
		}
	case []any:
		if descriptor.Type == catalogconfig.ProviderBindingsValue {
			encoded, err := json.Marshal(typed)
			if err != nil {
				return "", invalid
			}
			return string(encoded), nil
		}
		if descriptor.Type != catalogconfig.ListValue {
			return "", invalid
		}
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok || strings.Contains(text, ",") {
				return "", invalid
			}
			items = append(items, text)
		}
		return strings.Join(items, ","), nil
	case []string:
		if descriptor.Type == catalogconfig.ListValue {
			return strings.Join(typed, ","), nil
		}
	}
	return "", invalid
}
