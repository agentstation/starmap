package main

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
)

// nullableCapabilityProperties mirrors the optional records in the public read view.
// The generator attaches reference extensions to shared types, so mark these properties directly.
var nullableCapabilityProperties = []string{"features", "attachments", "generation", "reasoning", "reasoning_tokens", "verbosity", "tools", "delivery"}

func markNullableCapabilityRecords(document map[string]any) ([][]string, error) {
	capability, err := markNullableRecordProperties(document, "catalogs.ModelDefinitionCapabilities", nullableCapabilityProperties)
	if err != nil {
		return nil, err
	}
	generation, err := markNullableRecordProperties(document, "catalogs.ModelGeneration", nullableGenerationProperties)
	if err != nil {
		return nil, err
	}
	return append(capability, generation...), nil
}

var nullableGenerationProperties = []string{"temperature", "top_p", "top_k", "top_a", "min_p", "typical_p", "tfs", "frequency_penalty", "presence_penalty", "repetition_penalty", "no_repeat_ngram_size", "length_penalty", "n", "best_of", "mirostat_tau", "mirostat_eta", "contrastive_search_penalty_alpha", "num_beams", "diversity_penalty"}

func markNullableRecordProperties(document map[string]any, schemaName string, fields []string) ([][]string, error) {
	components, _ := document["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	definition, exists := schemas[schemaName]
	if !exists {
		return nil, nil
	}
	definitionSchema, ok := definition.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("capability definition must be an object schema")
	}
	properties, ok := definitionSchema["properties"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("capability definition has no properties")
	}
	var paths [][]string
	for _, field := range fields {
		property, ok := properties[field].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("capability %s has no property schema", field)
		}
		for key := range property {
			if key != "$ref" && key != "description" && key != "title" && key != "anyOf" {
				return nil, fmt.Errorf("capability %s has unsupported reference constraint %s", field, key)
			}
		}
		if union, exists := property["anyOf"]; exists {
			if _, exists := property["$ref"]; exists {
				return nil, fmt.Errorf("capability %s retains an outer reference", field)
			}
			options, ok := union.([]any)
			if !ok || len(options) != 2 {
				return nil, fmt.Errorf("capability %s has an invalid nullable union", field)
			}
			reference, ok := options[0].(map[string]any)
			null, validNull := options[1].(map[string]any)
			ref, validRef := reference["$ref"].(string)
			if !ok || !validNull || !validRef || !strings.HasPrefix(ref, "#/components/schemas/") || len(reference) != 1 || len(null) != 1 || null["type"] != "null" {
				return nil, fmt.Errorf("capability %s has an invalid nullable reference", field)
			}
			continue
		}
		ref, ok := property["$ref"].(string)
		if !ok || !strings.HasPrefix(ref, "#/components/schemas/") {
			return nil, fmt.Errorf("capability %s requires a local schema reference", field)
		}
		delete(property, "$ref")
		property["anyOf"] = []any{map[string]any{"$ref": ref}, map[string]any{"type": "null"}}
		paths = append(paths, []string{"components", "schemas", schemaName, "properties", field})
	}
	return paths, nil
}

func nullableCapabilityYAMLEdit(file *ast.File, original []byte, path []string) (replacement, error) {
	builder := new(yaml.PathBuilder).Root()
	for _, part := range path {
		builder = builder.Child(part)
	}
	selector, err := yaml.PathString(builder.Build().String() + ".'$ref'")
	if err != nil {
		return replacement{}, err
	}
	node, err := selector.FilterFile(file)
	if err != nil {
		return replacement{}, err
	}
	runes := []rune(string(original))
	runeOffset := node.GetToken().Position.Offset - 1
	if runeOffset < 0 || runeOffset > len(runes) {
		return replacement{}, fmt.Errorf("invalid YAML reference offset")
	}
	offset := len(string(runes[:runeOffset]))
	start := bytes.LastIndexByte(original[:offset], '\n') + 1
	end := bytes.IndexByte(original[offset:], '\n')
	if end < 0 {
		end = len(original)
	} else {
		end += offset
	}
	line := string(original[start:end])
	trimmed := strings.TrimLeft(line, " ")
	if !strings.HasPrefix(trimmed, "$ref:") || strings.Contains(trimmed, "\t") {
		return replacement{}, fmt.Errorf("capability reference must occupy its own YAML line")
	}
	indent := line[:len(line)-len(trimmed)]
	value := indent + "anyOf:\n" + indent + "  - " + trimmed + "\n" + indent + "  - type: \"null\""
	return replacement{start: start, end: end, value: []byte(value)}, nil
}
