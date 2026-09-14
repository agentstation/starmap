package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/errors"
)

// providerHistoryShape separates replacement values from their replay context.
// Membership, canonical links, pricing, unions, and unknown fields remain in context.
type providerHistoryShape struct {
	context [sha256.Size]byte
	values  map[string][sha256.Size]byte
}

func retainedProviderShape(ctx context.Context, observation manualObservation) (providerHistoryShape, error) {
	shape := providerHistoryShape{values: make(map[string][sha256.Size]byte)}
	if err := ctx.Err(); err != nil {
		return shape, err
	}
	if _, err := observation.restore(); err != nil {
		return shape, err
	}
	var payload map[string]any
	decoder := json.NewDecoder(bytes.NewReader(observation.Payload))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return shape, errors.WrapResource("decode", "retained provider payload", observation.Receipt.Link.ObservationID, err)
	}
	policies := authority.New()
	providers, _ := payload["providers"].([]any)
	for _, item := range providers {
		if err := ctx.Err(); err != nil {
			return shape, err
		}
		provider, _ := item.(map[string]any)
		id, _ := provider["id"].(string)
		if err := shape.maskPolicies(provider, reflect.TypeFor[catalogs.Provider](), "provider/"+strconv.Quote(id), policies.Policies(evidence.ResourceTypeProvider)); err != nil {
			return shape, err
		}
	}
	models, _ := payload["provider_models"].(map[string]any)
	for provider, items := range models {
		list, _ := items.([]any)
		for _, item := range list {
			if err := ctx.Err(); err != nil {
				return shape, err
			}
			model, _ := item.(map[string]any)
			id, _ := model["id"].(string)
			prefix := "model/" + strconv.Quote(provider) + "/" + strconv.Quote(id)
			if err := shape.maskPolicies(model, reflect.TypeFor[catalogs.Model](), prefix, policies.Policies(evidence.ResourceTypeModel)); err != nil {
				return shape, err
			}
		}
	}
	keys := slices.Sorted(maps.Keys(shape.values))
	raw, err := json.Marshal(struct {
		Payload map[string]any
		Fields  []string
	}{payload, keys})
	if err != nil {
		return shape, errors.WrapResource("encode", "retained provider shape", observation.Receipt.Link.ObservationID, err)
	}
	shape.context = sha256.Sum256(raw)
	return shape, ctx.Err()
}

func (s *providerHistoryShape) maskPolicies(record map[string]any, recordType reflect.Type, prefix string, policies []authority.Policy) error {
	for _, policy := range policies {
		// Canonical references control future resolution. Pricing also depends on evaluation time.
		if policy.Path == "ModelRef" || policy.Path == "Pricing" {
			continue
		}
		container, key, valueType, ok := historyPolicyField(record, recordType, strings.Split(policy.Path, "."))
		if !ok || container[key] == nil {
			continue
		}
		path := prefix + "/" + policy.Path
		switch policy.Merge {
		case authority.MergeReplace:
			if policy.Empty == authority.EmptyAuthoritative || !historyValueEmpty(container[key]) {
				if err := s.maskValue(container, key, path); err != nil {
					return err
				}
			}
		case authority.MergeFillMissing:
			if err := s.maskReplacementLeaves(container[key], valueType, path); err != nil {
				return err
			}
		}
	}
	return nil
}

func historyPolicyField(record map[string]any, recordType reflect.Type, path []string) (map[string]any, string, reflect.Type, bool) {
	for index, name := range path {
		for recordType.Kind() == reflect.Pointer {
			recordType = recordType.Elem()
		}
		if recordType.Kind() != reflect.Struct {
			return nil, "", nil, false
		}
		field, ok := recordType.FieldByName(name)
		if !ok {
			return nil, "", nil, false
		}
		key := strings.Split(field.Tag.Get("json"), ",")[0]
		if key == "" || key == "-" {
			return nil, "", nil, false
		}
		if index == len(path)-1 {
			_, exists := record[key]
			return record, key, field.Type, exists
		}
		record, _ = record[key].(map[string]any)
		recordType = field.Type
	}
	return nil, "", nil, false
}

func (s *providerHistoryShape) maskReplacementLeaves(value any, valueType reflect.Type, path string) error {
	for valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	record, ok := value.(map[string]any)
	if !ok || valueType.Kind() != reflect.Struct {
		return nil
	}
	for index := range valueType.NumField() {
		field := valueType.Field(index)
		key := strings.Split(field.Tag.Get("json"), ",")[0]
		if !field.IsExported() || key == "" || key == "-" || historyValueEmpty(record[key]) {
			continue
		}
		fieldPath := path + "." + field.Name
		switch record[key].(type) {
		case string, bool, json.Number:
			if err := s.maskValue(record, key, fieldPath); err != nil {
				return err
			}
		case map[string]any:
			if err := s.maskReplacementLeaves(record[key], field.Type, fieldPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *providerHistoryShape) maskValue(record map[string]any, key, path string) error {
	raw, err := json.Marshal(record[key])
	if err != nil {
		return errors.WrapResource("encode", "retained catalog field", path, err)
	}
	if _, exists := s.values[path]; exists {
		return invalidInputPublication("retention encountered a repeated field identity")
	}
	s.values[path] = sha256.Sum256(raw)
	record[key] = nil
	return nil
}

func historyValueEmpty(value any) bool {
	switch value := value.(type) {
	case nil:
		return true
	case string:
		return value == ""
	case bool:
		return !value
	case json.Number:
		number, err := strconv.ParseFloat(string(value), 64)
		return number == 0 || err != nil
	case map[string]any:
		return len(value) == 0
	case []any:
		return len(value) == 0
	default:
		return false
	}
}
