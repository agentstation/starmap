package main

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

const (
	permissionPath      = "/api/v1/catalog/permission"
	permissionMediaType = "application/vnd.agentstation.starmap.catalog-permission+json"
	permissionSchemaRef = "#/components/schemas/catalogs.CatalogPermissionEnvelope"
)

// normalizePermissionContent corrects the pinned generator's vendor JSON schema.
// Swag v2 rc4 puts the object reference under application/json and adds a string for the vendor type.
func normalizePermissionContent(document map[string]any) (bool, error) {
	paths, _ := document["paths"].(map[string]any)
	value, exists := paths[permissionPath]
	if !exists {
		return false, nil
	}
	for _, key := range []string{"get", "responses", "200", "content"} {
		object, ok := value.(map[string]any)
		if !ok {
			return false, fmt.Errorf("permission response has no %s object", key)
		}
		value = object[key]
	}
	content, ok := value.(map[string]any)
	if !ok {
		return false, fmt.Errorf("permission response has no content object")
	}
	object := map[string]any{"schema": map[string]any{"$ref": permissionSchemaRef}}
	correct := map[string]any{permissionMediaType: object}
	if reflect.DeepEqual(content, correct) {
		return false, nil
	}
	generated := map[string]any{"application/json": object, permissionMediaType: map[string]any{"schema": map[string]any{"type": "string"}}}
	if !reflect.DeepEqual(content, generated) {
		return false, fmt.Errorf("permission response differs from the pinned generator contract")
	}
	clear(content)
	content[permissionMediaType] = object
	return true, nil
}

func permissionContentYAMLEdit(original []byte) (replacement, error) {
	file, err := parser.ParseBytes(original, 0)
	if err != nil {
		return replacement{}, err
	}
	selector, err := yaml.PathString("$.paths.'/api/v1/catalog/permission'.get.responses.'200'.content")
	if err != nil {
		return replacement{}, err
	}
	node, err := selector.FilterFile(file)
	if err != nil {
		return replacement{}, err
	}
	mapping, ok := node.(*ast.MappingNode)
	if !ok || len(mapping.Values) != 2 {
		return replacement{}, fmt.Errorf("permission content requires the two generated media types")
	}
	position := mapping.Values[0].Key.GetToken().Position
	offset, err := yamlSourceOffset(original, position.Line, position.Column)
	if err != nil {
		return replacement{}, err
	}
	start := bytes.LastIndexByte(original[:offset], '\n') + 1
	indent := string(original[start:offset])
	if len(indent) == 0 || strings.Trim(indent, " ") != "" {
		return replacement{}, fmt.Errorf("permission content requires an indented YAML mapping")
	}
	end := start
	for end < len(original) {
		lineEnd := bytes.IndexByte(original[end:], '\n')
		if lineEnd < 0 {
			lineEnd = len(original) - end
		}
		line := original[end : end+lineEnd]
		if len(bytes.TrimSpace(line)) != 0 && len(line)-len(bytes.TrimLeft(line, " ")) < len(indent) {
			break
		}
		end += lineEnd
		if end < len(original) {
			end++
		}
	}
	value := indent + permissionMediaType + ":\n" + indent + "  schema:\n" + indent + "    $ref: '" + permissionSchemaRef + "'\n"
	return replacement{start: start, end: end, value: []byte(value)}, nil
}
