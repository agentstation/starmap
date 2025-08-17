package operations

import (
	"reflect"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// SmartMergeModels performs a smart merge of two models, keeping existing values where the new model has empty/nil values
func SmartMergeModels(existing, new catalogs.Model) catalogs.Model {
	result := existing // Start with existing model

	// Use reflection to merge non-zero fields from new model
	existingVal := reflect.ValueOf(&result).Elem()
	newVal := reflect.ValueOf(new)

	mergeFields(existingVal, newVal)

	return result
}

// mergeFields recursively merges fields from source to dest, only overwriting if source has non-zero values
func mergeFields(dest, src reflect.Value) {
	if !dest.CanSet() || !src.IsValid() {
		return
	}

	switch src.Kind() {
	case reflect.String:
		if src.String() != "" {
			dest.SetString(src.String())
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if src.Int() != 0 {
			dest.SetInt(src.Int())
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if src.Uint() != 0 {
			dest.SetUint(src.Uint())
		}
	case reflect.Float32, reflect.Float64:
		if src.Float() != 0 {
			dest.SetFloat(src.Float())
		}
	case reflect.Bool:
		// For booleans, always take the new value since false is a valid override
		dest.SetBool(src.Bool())
	case reflect.Slice:
		if !src.IsNil() && src.Len() > 0 {
			dest.Set(src)
		}
	case reflect.Map:
		if !src.IsNil() && src.Len() > 0 {
			if dest.IsNil() {
				dest.Set(reflect.MakeMap(dest.Type()))
			}
			for _, key := range src.MapKeys() {
				dest.SetMapIndex(key, src.MapIndex(key))
			}
		}
	case reflect.Ptr:
		if !src.IsNil() {
			if dest.IsNil() {
				dest.Set(reflect.New(dest.Type().Elem()))
			}
			mergeFields(dest.Elem(), src.Elem())
		}
	case reflect.Struct:
		for i := 0; i < src.NumField(); i++ {
			srcField := src.Field(i)
			destField := dest.Field(i)
			if destField.CanSet() {
				mergeFields(destField, srcField)
			}
		}
	default:
		// For other types, just set if not zero
		if !src.IsZero() {
			dest.Set(src)
		}
	}
}
