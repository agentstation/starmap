package query

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// ModelFilter contains all possible filter criteria for models.
type ModelFilter struct {
	// Basic filters
	ID           string
	Name         string
	NameContains string
	Provider     string
	Status       string

	// Modality filters
	ModalityInput  []string
	ModalityOutput []string

	// Feature filters
	Features map[string]bool

	// Metadata filters
	Tags        []string
	OpenWeights *bool

	// Numeric range filters
	MinContext int64
	MaxContext int64
	MinInput   int64
	MaxInput   int64
	MinOutput  int64
	MaxOutput  int64

	// Date filters
	ReleasedAfter  *time.Time
	ReleasedBefore *time.Time

	// Pagination
	Sort       string
	Order      string
	Limit      int
	Offset     int
	MaxResults int
}

var validFeatureFilters = map[string]struct{}{
	"streaming": {}, "tool_calls": {}, "tools": {}, "tool_choice": {},
	"reasoning": {}, "temperature": {}, "max_tokens": {},
}

const (
	sortID            = "id"
	sortName          = "name"
	sortReleaseDate   = "release_date"
	sortContextWindow = "context_window"
	sortCreatedAt     = "created_at"
	sortUpdatedAt     = "updated_at"
)

// Validate rejects unsupported or ambiguous filter, sort, range, and page
// values before query execution.
func (f ModelFilter) Validate() error {
	if f.Sort != "" {
		switch f.Sort {
		case sortID, sortName, sortReleaseDate, sortContextWindow, sortCreatedAt, sortUpdatedAt:
		default:
			return &errors.ValidationError{Field: "model_filter.sort", Value: f.Sort, Message: "is not supported"}
		}
	}
	if f.Order != "" && !strings.EqualFold(f.Order, "asc") && !strings.EqualFold(f.Order, "desc") {
		return &errors.ValidationError{Field: "model_filter.order", Value: f.Order, Message: "must be asc or desc"}
	}
	if f.Order != "" && f.Sort == "" {
		return &errors.ValidationError{Field: "model_filter.order", Value: f.Order, Message: "requires a sort field"}
	}
	if f.Limit < 1 || f.Limit > 1000 {
		return &errors.ValidationError{Field: "model_filter.limit", Value: f.Limit, Message: "must be between 1 and 1000"}
	}
	if f.Offset < 0 {
		return &errors.ValidationError{Field: "model_filter.offset", Value: f.Offset, Message: "must not be negative"}
	}
	if f.MaxResults < 0 || f.MaxResults > 1000 {
		return &errors.ValidationError{Field: "model_filter.max_results", Value: f.MaxResults, Message: "must be between 0 and 1000"}
	}
	for _, bounds := range []struct {
		name    string
		minimum int64
		maximum int64
	}{
		{name: "context", minimum: f.MinContext, maximum: f.MaxContext},
		{name: "input", minimum: f.MinInput, maximum: f.MaxInput},
		{name: "output", minimum: f.MinOutput, maximum: f.MaxOutput},
	} {
		if bounds.minimum < 0 || bounds.maximum < 0 || bounds.maximum > 0 && bounds.minimum > bounds.maximum {
			return &errors.ValidationError{Field: "model_filter." + bounds.name + "_range", Value: bounds, Message: "must be non-negative and ordered"}
		}
	}
	for feature := range f.Features {
		if _, found := validFeatureFilters[feature]; !found {
			return &errors.ValidationError{Field: "model_filter.feature", Value: feature, Message: "is not supported"}
		}
	}
	for _, modality := range append(append([]string(nil), f.ModalityInput...), f.ModalityOutput...) {
		switch catalogs.ModelModality(strings.ToLower(strings.TrimSpace(modality))) {
		case catalogs.ModelModalityText, catalogs.ModelModalityAudio, catalogs.ModelModalityImage,
			catalogs.ModelModalityVideo, catalogs.ModelModalityPDF, catalogs.ModelModalityEmbedding:
		default:
			return &errors.ValidationError{Field: "model_filter.modality", Value: modality, Message: "is not supported"}
		}
	}
	if f.Status != "" {
		switch catalogs.ModelStatus(strings.ToLower(f.Status)) {
		case catalogs.ModelStatusActive, catalogs.ModelStatusBeta, catalogs.ModelStatusPreview,
			catalogs.ModelStatusDeprecated, catalogs.ModelStatusUnknown:
		default:
			return &errors.ValidationError{Field: "model_filter.status", Value: f.Status, Message: "is not supported"}
		}
	}
	if f.ReleasedAfter != nil && f.ReleasedBefore != nil && f.ReleasedAfter.After(*f.ReleasedBefore) {
		return &errors.ValidationError{Field: "model_filter.release_range", Message: "released_after must not be after released_before"}
	}
	return nil
}

// Apply applies the filter to a list of models and returns filtered results.
func (f ModelFilter) Apply(models []catalogs.Model) []catalogs.Model {
	var results []catalogs.Model

	for _, model := range models {
		if f.matches(model) {
			results = append(results, model)
		}
	}

	if f.Sort != "" {
		results = f.sort(results)
	}

	return results
}

// matches checks if a model matches the filter criteria.
func (f ModelFilter) matches(model catalogs.Model) bool {
	return f.matchesBasicFilters(model) &&
		f.matchesStatusFilter(model) &&
		f.matchesModalityFilters(model) &&
		f.matchesFeaturesFilter(model) &&
		f.matchesMetadataFilters(model) &&
		f.matchesLimitFilters(model) &&
		f.matchesDateFilters(model)
}

// matchesBasicFilters checks ID, name, and name contains filters.
func (f ModelFilter) matchesBasicFilters(model catalogs.Model) bool {
	if f.ID != "" && model.ID != f.ID {
		return false
	}
	if f.Name != "" && !strings.EqualFold(model.Name, f.Name) {
		return false
	}
	if f.NameContains != "" && !strings.Contains(strings.ToLower(model.Name), strings.ToLower(f.NameContains)) {
		return false
	}
	return true
}

func (f ModelFilter) matchesStatusFilter(model catalogs.Model) bool {
	if f.Status == "" {
		return true
	}
	return strings.EqualFold(model.Status.String(), f.Status)
}

// matchesModalityFilters checks input and output modality filters.
func (f ModelFilter) matchesModalityFilters(model catalogs.Model) bool {
	if len(f.ModalityInput) > 0 {
		if model.Features == nil {
			return false
		}
		if !modalityContainsAll(model.Features.Modalities.Input, f.ModalityInput) {
			return false
		}
	}
	if len(f.ModalityOutput) > 0 {
		if model.Features == nil {
			return false
		}
		if !modalityContainsAll(model.Features.Modalities.Output, f.ModalityOutput) {
			return false
		}
	}
	return true
}

// matchesFeaturesFilter checks feature capability filters.
func (f ModelFilter) matchesFeaturesFilter(model catalogs.Model) bool {
	if len(f.Features) == 0 {
		return true
	}
	if model.Features == nil {
		return false
	}
	for feature, required := range f.Features {
		if !matchFeature(model.Features, feature, required) {
			return false
		}
	}
	return true
}

// matchesMetadataFilters checks tags and open weights filters.
func (f ModelFilter) matchesMetadataFilters(model catalogs.Model) bool {
	if len(f.Tags) > 0 {
		if model.Metadata == nil {
			return false
		}
		if !tagContainsAny(model.Metadata.Tags, f.Tags) {
			return false
		}
	}
	if f.OpenWeights != nil {
		if model.Metadata == nil {
			return false
		}
		if model.Metadata.OpenWeights != *f.OpenWeights {
			return false
		}
	}
	return true
}

// matchesLimitFilters checks context window and output token range filters.
func (f ModelFilter) matchesLimitFilters(model catalogs.Model) bool {
	if model.Limits == nil {
		if f.MinContext > 0 || f.MaxContext > 0 || f.MinInput > 0 || f.MaxInput > 0 || f.MinOutput > 0 || f.MaxOutput > 0 {
			return false
		}
		return true
	}
	if !limitWithinRange(model.Limits.ContextWindow, f.MinContext, f.MaxContext) {
		return false
	}
	if !limitWithinRange(model.Limits.InputTokens, f.MinInput, f.MaxInput) {
		return false
	}
	if !limitWithinRange(model.Limits.OutputTokens, f.MinOutput, f.MaxOutput) {
		return false
	}
	return true
}

func limitWithinRange(value, minValue, maxValue int64) bool {
	if minValue == 0 && maxValue == 0 {
		return true
	}
	if value <= 0 {
		return false
	}
	if minValue > 0 && value < minValue {
		return false
	}
	if maxValue > 0 && value > maxValue {
		return false
	}
	return true
}

// matchesDateFilters checks release date range filters.
func (f ModelFilter) matchesDateFilters(model catalogs.Model) bool {
	if f.ReleasedAfter == nil && f.ReleasedBefore == nil {
		return true
	}
	if model.Metadata == nil || model.Metadata.ReleaseDate.IsZero() {
		return false
	}
	if f.ReleasedAfter != nil && model.Metadata.ReleaseDate.Time.Before(*f.ReleasedAfter) {
		return false
	}
	if f.ReleasedBefore != nil && model.Metadata.ReleaseDate.Time.After(*f.ReleasedBefore) {
		return false
	}
	return true
}

// sort sorts models based on the sort field and order.
func (f ModelFilter) sort(models []catalogs.Model) []catalogs.Model {
	field := f.Sort
	if field == "" {
		field = sortID
	}
	descending := strings.EqualFold(f.Order, "desc")
	slices.SortStableFunc(models, func(left, right catalogs.Model) int {
		leftMissing, rightMissing := modelSortMissing(left, field), modelSortMissing(right, field)
		if leftMissing != rightMissing {
			if leftMissing {
				return 1
			}
			return -1
		}
		result := compareModelField(left, right, field)
		if descending {
			result = -result
		}
		if result == 0 {
			result = strings.Compare(left.ID, right.ID)
		}
		return result
	})
	return models
}

func modelSortMissing(model catalogs.Model, field string) bool {
	switch field {
	case sortReleaseDate:
		return modelReleaseDate(model).IsZero()
	case sortContextWindow:
		return modelContextWindow(model) <= 0
	case sortCreatedAt:
		return model.CreatedAt.IsZero()
	case sortUpdatedAt:
		return model.UpdatedAt.IsZero()
	default:
		return false
	}
}

func compareModelField(left, right catalogs.Model, field string) int {
	switch field {
	case sortName:
		return strings.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name))
	case sortReleaseDate:
		return compareOptionalTime(modelReleaseDate(left), modelReleaseDate(right))
	case sortContextWindow:
		return compareOptionalInt(modelContextWindow(left), modelContextWindow(right))
	case sortCreatedAt:
		return compareOptionalTime(left.CreatedAt.Time, right.CreatedAt.Time)
	case sortUpdatedAt:
		return compareOptionalTime(left.UpdatedAt.Time, right.UpdatedAt.Time)
	default:
		return strings.Compare(left.ID, right.ID)
	}
}

func modelReleaseDate(model catalogs.Model) time.Time {
	if model.Metadata == nil {
		return time.Time{}
	}
	return model.Metadata.ReleaseDate.Time
}

func modelContextWindow(model catalogs.Model) int64 {
	if model.Limits == nil {
		return 0
	}
	return model.Limits.ContextWindow
}

func compareOptionalTime(left, right time.Time) int {
	return left.Compare(right)
}

func compareOptionalInt(left, right int64) int {
	return cmp.Compare(left, right)
}

// matchFeature checks if a model has a specific feature.
func matchFeature(features *catalogs.ModelFeatures, feature string, required bool) bool {
	switch feature {
	case "streaming":
		return features.Streaming == required
	case "tool_calls":
		return features.ToolCalls == required
	case "tools":
		return features.Tools == required
	case "tool_choice":
		return features.ToolChoice == required
	case "reasoning":
		return features.Reasoning == required
	case "temperature":
		return features.Temperature == required
	case "max_tokens":
		return features.MaxTokens == required
	default:
		return false
	}
}

// modalityContainsAll checks if modality slice contains all required values.
func modalityContainsAll(slice []catalogs.ModelModality, required []string) bool {
	for _, req := range required {
		found := false
		for _, item := range slice {
			if strings.EqualFold(string(item), req) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// tagContainsAny checks if tag slice contains any of the values.
func tagContainsAny(slice []catalogs.ModelTag, values []string) bool {
	for _, val := range values {
		for _, item := range slice {
			if strings.EqualFold(string(item), val) {
				return true
			}
		}
	}
	return false
}
