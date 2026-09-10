// Command starmap-openapi-presence preserves wire contracts in generated schemas.
package main

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/parser"
)

const nullableAnnotation = "x-starmap-nullable"

type replacement struct {
	start, end int
	value      []byte
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) (resultErr error) {
	if len(args) != 2 {
		return fmt.Errorf("usage: starmap-openapi-presence <spec.json> <spec.yaml>")
	}
	jsonRoot, jsonName, err := openSpec(args[0])
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, jsonRoot.Close()) }()
	yamlRoot, yamlName, err := openSpec(args[1])
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, yamlRoot.Close()) }()
	originalJSON, err := jsonRoot.ReadFile(jsonName)
	if err != nil {
		return err
	}
	originalYAML, err := yamlRoot.ReadFile(yamlName)
	if err != nil {
		return err
	}
	updatedJSON, updatedYAML, err := normalize(originalJSON, originalYAML)
	if err != nil {
		return err
	}
	if err := jsonRoot.WriteFile(jsonName, updatedJSON, 0o600); err != nil {
		return err
	}
	return yamlRoot.WriteFile(yamlName, updatedYAML, 0o600)
}

func openSpec(path string) (*os.Root, string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, "", err
	}
	root, err := os.OpenRoot(filepath.Dir(absolute))
	if err != nil {
		return nil, "", err
	}
	return root, filepath.Base(absolute), nil
}

func normalize(originalJSON, originalYAML []byte) ([]byte, []byte, error) {
	var document map[string]any
	input := json.NewDecoder(bytes.NewReader(originalJSON))
	input.UseNumber()
	if err := input.Decode(&document); err != nil {
		return nil, nil, err
	}
	if len(bytes.TrimSpace(originalJSON[input.InputOffset():])) != 0 {
		return nil, nil, fmt.Errorf("JSON specification contains trailing content")
	}
	version, _ := document["openapi"].(string)
	if !strings.HasPrefix(version, "3.1.") {
		return nil, nil, fmt.Errorf("OpenAPI 3.1 is required, got %q", version)
	}
	var yamlDocument map[string]any
	if err := yaml.Unmarshal(originalYAML, &yamlDocument); err != nil {
		return nil, nil, err
	}
	yamlJSON, err := json.Marshal(yamlDocument)
	if err != nil {
		return nil, nil, err
	}
	canonicalJSON, _ := json.Marshal(document)
	if !bytes.Equal(canonicalJSON, yamlJSON) {
		return nil, nil, fmt.Errorf("JSON and YAML specifications differ")
	}
	var paths [][]string
	if err := markNullable(document["components"], []string{"components"}, &paths); err != nil {
		return nil, nil, err
	}
	recordPaths, err := markNullableCapabilityRecords(document)
	if err != nil {
		return nil, nil, err
	}
	permissionChanged, err := normalizePermissionContent(document)
	if err != nil {
		return nil, nil, err
	}
	if len(paths) == 0 && len(recordPaths) == 0 && !permissionChanged {
		return originalJSON, originalYAML, nil
	}
	edits, err := nullableYAMLEdits(originalYAML, paths, recordPaths)
	if err != nil {
		return nil, nil, err
	}
	if permissionChanged {
		edit, err := permissionContentYAMLEdit(originalYAML)
		if err != nil {
			return nil, nil, err
		}
		edits = append(edits, edit)
	}
	updatedYAML := replace(originalYAML, edits)
	// Replace only top-level JSON values. Preserve the generator's existing layout.
	decoder := json.NewDecoder(bytes.NewReader(originalJSON))
	if _, err := decoder.Token(); err != nil {
		return nil, nil, err
	}
	edits = nil
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, nil, err
		}
		key, ok := token.(string)
		if !ok {
			return nil, nil, fmt.Errorf("JSON object key is not a string")
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return nil, nil, err
		}
		if key != "components" && (key != "paths" || !permissionChanged) {
			continue
		}
		value, err := json.Marshal(document[key])
		if err != nil {
			return nil, nil, err
		}
		end := int(decoder.InputOffset())
		edits = append(edits, replacement{start: end - len(raw), end: end, value: value})
	}
	updatedJSON := replace(originalJSON, edits)
	var checked map[string]any
	if err := yaml.Unmarshal(updatedYAML, &checked); err != nil {
		return nil, nil, err
	}
	rendered, err := json.Marshal(checked)
	if err != nil {
		return nil, nil, err
	}
	expected, err := json.Marshal(document)
	if err != nil {
		return nil, nil, err
	}
	if !bytes.Equal(rendered, expected) {
		return nil, nil, fmt.Errorf("normalized JSON and YAML specifications differ")
	}
	return updatedJSON, updatedYAML, nil
}

func markNullable(value any, path []string, paths *[][]string) error {
	switch value := value.(type) {
	case map[string]any:
		if marker, exists := value[nullableAnnotation]; exists {
			if marker != true {
				return fmt.Errorf("%s annotation must be true", strings.Join(path, "."))
			}
			switch kind := value["type"].(type) {
			case string:
				if !nullableScalarType(kind) {
					return fmt.Errorf("%s nullable annotation requires a Boolean or numeric scalar", strings.Join(path, "."))
				}
				value["type"] = []string{kind, "null"}
				*paths = append(*paths, slices.Clone(path))
			case []any:
				if len(kind) != 2 || !nullableScalarValue(kind[0]) || kind[1] != "null" {
					return fmt.Errorf("%s has an unsupported nullable type", strings.Join(path, "."))
				}
			default:
				return fmt.Errorf("%s has no supported scalar type", strings.Join(path, "."))
			}
		}
		for key, item := range value {
			if err := markNullable(item, append(slices.Clone(path), key), paths); err != nil {
				return err
			}
		}
	case []any:
		// Generator annotations belong to named properties, never array positions.
		for _, item := range value {
			if object, ok := item.(map[string]any); ok {
				if _, exists := object[nullableAnnotation]; exists {
					return fmt.Errorf("nullable annotations require named properties")
				}
			}
		}
	}
	return nil
}

func replace(original []byte, edits []replacement) []byte {
	slices.SortFunc(edits, func(a, b replacement) int { return cmp.Compare(b.start, a.start) })
	result := bytes.Clone(original)
	for _, edit := range edits {
		result = append(append(append([]byte(nil), result[:edit.start]...), edit.value...), result[edit.end:]...)
	}
	return result
}

func nullableYAMLEdits(originalYAML []byte, paths, recordPaths [][]string) ([]replacement, error) {
	file, err := parser.ParseBytes(originalYAML, 0)
	if err != nil {
		return nil, err
	}
	var edits []replacement
	for _, path := range paths {
		builder := new(yaml.PathBuilder).Root()
		for _, part := range path {
			builder = builder.Child(part)
		}
		// Parsing the rendered path restores quoted selector names in the library.
		selector, err := yaml.PathString(builder.Child("type").Build().String())
		if err != nil {
			return nil, err
		}
		node, err := selector.FilterFile(file)
		if err != nil {
			return nil, err
		}
		token := node.GetToken()
		offset, err := yamlSourceOffset(originalYAML, token.Position.Line, token.Position.Column)
		if err != nil {
			return nil, err
		}
		scalar := ""
		for _, candidate := range []string{"boolean", "number", "integer"} {
			if offset >= 0 && offset+len(candidate) <= len(originalYAML) && string(originalYAML[offset:offset+len(candidate)]) == candidate {
				scalar = candidate
				break
			}
		}
		if scalar == "" {
			return nil, fmt.Errorf("nullable type at %s must use an unquoted supported scalar", selector.String())
		}
		edits = append(edits, replacement{start: offset, end: offset + len(scalar), value: []byte("[" + scalar + `, "null"]`)})
	}
	for _, path := range recordPaths {
		edit, err := nullableCapabilityYAMLEdit(file, originalYAML, path)
		if err != nil {
			return nil, err
		}
		edits = append(edits, edit)
	}
	return edits, nil
}

func nullableScalarType(kind string) bool {
	return kind == "boolean" || kind == "number" || kind == "integer"
}

func nullableScalarValue(value any) bool {
	kind, ok := value.(string)
	return ok && nullableScalarType(kind)
}

func yamlSourceOffset(original []byte, line, column int) (int, error) {
	if line < 1 || column < 1 {
		return 0, fmt.Errorf("invalid YAML source position")
	}
	offset := 0
	for current := 1; current < line; current++ {
		next := bytes.IndexByte(original[offset:], '\n')
		if next < 0 {
			return 0, fmt.Errorf("YAML source line exceeds input")
		}
		offset += next + 1
	}
	end := bytes.IndexByte(original[offset:], '\n')
	if end < 0 {
		end = len(original) - offset
	}
	runes := []rune(string(original[offset : offset+end]))
	if column-1 > len(runes) {
		return 0, fmt.Errorf("YAML source column exceeds input")
	}
	return offset + len(string(runes[:column-1])), nil
}
