package catalogs

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
)

const (
	maxExtensionInt64 = int64(^uint64(0) >> 1)
	minExtensionInt64 = -maxExtensionInt64 - 1
)

// SourceExtensions stores source-specific attributes that Starmap preserves
// without treating as canonical schema fields.
type SourceExtensions map[string]SourceExtension

// Copy returns a deep copy of the source extension map.
func (se SourceExtensions) Copy() SourceExtensions {
	if se == nil {
		return nil
	}
	copied := make(SourceExtensions, len(se))
	for source, extension := range se {
		copied[source] = extension.Copy()
	}
	return copied
}

// SourceExtension stores controlled non-canonical fields reported by one source.
type SourceExtension struct {
	Fields map[string]any `json:"fields,omitempty" yaml:"fields,omitempty"` // Preserved source-specific fields
}

// NormalizeSourceExtensions returns a copy with JSON/YAML-stable dynamic value
// types for equality checks and sync idempotency.
func NormalizeSourceExtensions(extensions SourceExtensions) SourceExtensions {
	if extensions == nil {
		return nil
	}
	normalized := make(SourceExtensions, len(extensions))
	for source, extension := range extensions {
		normalized[source] = SourceExtension{
			Fields: NormalizeExtensionFields(extension.Fields),
		}
	}
	return normalized
}

// NormalizeExtensionFields returns a copy with maps, slices, and numbers
// normalized to stable dynamic types after JSON/YAML round trips.
func NormalizeExtensionFields(fields map[string]any) map[string]any {
	if fields == nil {
		return nil
	}
	normalized := make(map[string]any, len(fields))
	for key, value := range fields {
		normalized[key] = normalizeExtensionValue(value)
	}
	return normalized
}

// UnmarshalJSON normalizes dynamic extension field types after JSON decoding.
func (se *SourceExtension) UnmarshalJSON(data []byte) error {
	type sourceExtension SourceExtension
	var raw sourceExtension
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	se.Fields = NormalizeExtensionFields(raw.Fields)
	return nil
}

// UnmarshalYAML normalizes dynamic extension field types after YAML decoding.
func (se *SourceExtension) UnmarshalYAML(unmarshal func(any) error) error {
	type sourceExtension SourceExtension
	var raw sourceExtension
	if err := unmarshal(&raw); err != nil {
		return err
	}
	se.Fields = NormalizeExtensionFields(raw.Fields)
	return nil
}

// Copy returns a deep copy of the source extension.
func (se SourceExtension) Copy() SourceExtension {
	return SourceExtension{
		Fields: deepCopyExtensionMap(se.Fields),
	}
}

func deepCopyExtensionMap(fields map[string]any) map[string]any {
	if fields == nil {
		return nil
	}
	copied := make(map[string]any, len(fields))
	for key, value := range fields {
		copied[key] = deepCopyExtensionValue(value)
	}
	return copied
}

func normalizeExtensionValue(value any) any {
	switch v := value.(type) {
	case nil, string, bool:
		return v
	case int:
		return int64(v)
	case int8:
		return int64(v)
	case int16:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case uint:
		return normalizeExtensionUint(uint64(v))
	case uint8:
		return int64(v)
	case uint16:
		return int64(v)
	case uint32:
		return int64(v)
	case uint64:
		return normalizeExtensionUint(v)
	case float32:
		return normalizeExtensionFloat(float64(v))
	case float64:
		return normalizeExtensionFloat(v)
	case map[string]any:
		return NormalizeExtensionFields(v)
	case []any:
		normalized := make([]any, len(v))
		for i, item := range v {
			normalized[i] = normalizeExtensionValue(item)
		}
		return normalized
	}

	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return nil
	}
	switch rv.Kind() {
	case reflect.Map:
		if rv.IsNil() {
			return nil
		}
		normalized := make(map[string]any, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			normalized[fmt.Sprint(iter.Key().Interface())] = normalizeExtensionValue(iter.Value().Interface())
		}
		return normalized
	case reflect.Slice, reflect.Array:
		if rv.Kind() == reflect.Slice && rv.IsNil() {
			return nil
		}
		normalized := make([]any, rv.Len())
		for i := range rv.Len() {
			normalized[i] = normalizeExtensionValue(rv.Index(i).Interface())
		}
		return normalized
	case reflect.Pointer, reflect.Interface:
		if rv.IsNil() {
			return nil
		}
		return normalizeExtensionValue(rv.Elem().Interface())
	default:
		return value
	}
}

func normalizeExtensionUint(value uint64) any {
	if value <= uint64(maxExtensionInt64) {
		return int64(value)
	}
	return strconv.FormatUint(value, 10)
}

func normalizeExtensionFloat(value float64) any {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return value
	}
	if math.Trunc(value) == value && value >= float64(minExtensionInt64) && value <= float64(maxExtensionInt64) {
		return int64(value)
	}
	return value
}

func deepCopyExtensionValue(value any) any {
	copied := deepCopyExtensionReflectValue(reflect.ValueOf(value))
	if !copied.IsValid() {
		return nil
	}
	return copied.Interface()
}

func deepCopyExtensionReflectValue(value reflect.Value) reflect.Value {
	if !value.IsValid() {
		return reflect.Value{}
	}

	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		return deepCopyExtensionReflectValue(value.Elem())
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		copied := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			copied.SetMapIndex(
				deepCopyExtensionReflectValue(iter.Key()),
				deepCopyExtensionReflectValue(iter.Value()),
			)
		}
		return copied
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		copied := reflect.MakeSlice(value.Type(), value.Len(), value.Cap())
		for i := range value.Len() {
			copied.Index(i).Set(deepCopyExtensionReflectValue(value.Index(i)))
		}
		return copied
	case reflect.Array:
		copied := reflect.New(value.Type()).Elem()
		for i := range value.Len() {
			copied.Index(i).Set(deepCopyExtensionReflectValue(value.Index(i)))
		}
		return copied
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		copied := reflect.New(value.Type().Elem())
		copied.Elem().Set(deepCopyExtensionReflectValue(value.Elem()))
		return copied
	default:
		return value
	}
}
