package catalogs

import "reflect"

// MergeModels combines two models and retains existing values when updated has
// an empty or nil value.
func MergeModels(existing, updated Model) Model {
	result := existing // Start with existing model

	mergeModelDescription(&result, &updated)
	mergeModelFeaturesByPresence(&result, &updated)
	mergeModelLimitsByPresence(&result, &updated)
	mergeModelMetadataByPresence(&result, &updated)

	// Use reflection to merge non-zero fields from updated model
	existingVal := reflect.ValueOf(&result).Elem()
	newVal := reflect.ValueOf(updated)

	mergeFields(existingVal, newVal)

	return result
}

// mergeFields recursively merges fields from source to dest, only overwriting if source has non-zero values.
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
		if src.Bool() {
			dest.SetBool(true)
		}
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
	case reflect.Pointer:
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

func mergeModelDescription(result, updated *Model) {
	description, state := updated.DescriptionValue()
	switch state {
	case ValueKnown:
		result.SetDescription(description)
	case ValueUnknown:
		if _, existingState := result.DescriptionValue(); existingState == ValueMissing {
			result.SetDescriptionUnknown()
		}
	}
	updated.UnsetDescription()
}

func mergeModelFeaturesByPresence(result, updated *Model) {
	if updated.Features == nil {
		return
	}
	if result.Features == nil {
		result.Features = &ModelFeatures{}
	}
	if len(updated.Features.Modalities.Input) > 0 {
		result.Features.Modalities.Input = append(
			[]ModelModality(nil),
			updated.Features.Modalities.Input...,
		)
	}
	if len(updated.Features.Modalities.Output) > 0 {
		result.Features.Modalities.Output = append(
			[]ModelModality(nil),
			updated.Features.Modalities.Output...,
		)
	}
	for _, feature := range modelFeatures() {
		value, state := updated.Features.Support(feature)
		switch state {
		case ValueKnown:
			result.Features.SetSupport(feature, value)
		case ValueUnknown:
			if _, existingState := result.Features.Support(feature); existingState == ValueMissing {
				result.Features.SetSupportUnknown(feature)
			}
		}
	}
	updated.Features = nil
}

func mergeModelLimitsByPresence(result, updated *Model) {
	if updated.Limits == nil {
		return
	}
	if result.Limits == nil {
		result.Limits = &ModelLimits{}
	}
	for _, limit := range modelLimitOrder {
		value, state := updated.Limits.Value(limit)
		switch state {
		case ValueKnown:
			result.Limits.Set(limit, value)
		case ValueUnknown:
			if _, existingState := result.Limits.Value(limit); existingState == ValueMissing {
				result.Limits.SetUnknown(limit)
			}
		}
	}
	updated.Limits = nil
}

func mergeModelMetadataByPresence(result, updated *Model) {
	if updated.Metadata == nil {
		return
	}
	if result.Metadata == nil {
		result.Metadata = &ModelMetadata{}
	}
	open, state := updated.Metadata.OpenWeightsValue()
	switch state {
	case ValueKnown:
		result.Metadata.SetOpenWeights(open)
	case ValueUnknown:
		if _, existingState := result.Metadata.OpenWeightsValue(); existingState == ValueMissing {
			result.Metadata.SetOpenWeightsUnknown()
		}
	}
	updated.Metadata.UnsetOpenWeights()
}
