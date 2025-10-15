// Package filter provides query parameter parsing and filtering for API endpoints.
package filter

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// ModelFilter contains all possible filter criteria for models.
type ModelFilter struct {
	// Basic filters
	ID           string
	Name         string
	NameContains string
	Provider     string

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

// ParseModelFilter extracts model filter parameters from HTTP request.
func ParseModelFilter(r *http.Request) ModelFilter {
	q := r.URL.Query()

	filter := ModelFilter{
		ID:           q.Get("id"),
		Name:         q.Get("name"),
		NameContains: q.Get("name_contains"),
		Provider:     q.Get("provider"),
		Sort:         q.Get("sort"),
		Order:        q.Get("order"),
		Limit:        parseIntOrDefault(q.Get("limit"), 100),
		Offset:       parseIntOrDefault(q.Get("offset"), 0),
		MaxResults:   parseIntOrDefault(q.Get("max_results"), 1000),
	}

	// Parse modalities
	if modalInput := q.Get("modality_input"); modalInput != "" {
		filter.ModalityInput = strings.Split(modalInput, ",")
	}
	if modalOutput := q.Get("modality_output"); modalOutput != "" {
		filter.ModalityOutput = strings.Split(modalOutput, ",")
	}

	// Parse features
	filter.Features = make(map[string]bool)
	for _, feature := range []string{"streaming", "tool_calls", "tools", "tool_choice", "reasoning", "temperature", "max_tokens"} {
		if val := q.Get("feature_" + feature); val != "" {
			if b, err := strconv.ParseBool(val); err == nil {
				filter.Features[feature] = b
			}
		}
	}
	// Also support shorthand "feature=streaming" format
	if feature := q.Get("feature"); feature != "" {
		filter.Features[feature] = true
	}

	// Parse tags
	if tags := q.Get("tag"); tags != "" {
		filter.Tags = strings.Split(tags, ",")
	}

	// Parse open_weights
	if ow := q.Get("open_weights"); ow != "" {
		if b, err := strconv.ParseBool(ow); err == nil {
			filter.OpenWeights = &b
		}
	}

	// Parse context window ranges
	if minCtx := q.Get("min_context"); minCtx != "" {
		if i, err := strconv.ParseInt(minCtx, 10, 64); err == nil {
			filter.MinContext = i
		}
	}
	if maxCtx := q.Get("max_context"); maxCtx != "" {
		if i, err := strconv.ParseInt(maxCtx, 10, 64); err == nil {
			filter.MaxContext = i
		}
	}

	// Parse output token ranges
	if minOut := q.Get("min_output"); minOut != "" {
		if i, err := strconv.ParseInt(minOut, 10, 64); err == nil {
			filter.MinOutput = i
		}
	}
	if maxOut := q.Get("max_output"); maxOut != "" {
		if i, err := strconv.ParseInt(maxOut, 10, 64); err == nil {
			filter.MaxOutput = i
		}
	}

	// Parse date ranges
	if after := q.Get("released_after"); after != "" {
		if t, err := time.Parse(time.RFC3339, after); err == nil {
			filter.ReleasedAfter = &t
		}
	}
	if before := q.Get("released_before"); before != "" {
		if t, err := time.Parse(time.RFC3339, before); err == nil {
			filter.ReleasedBefore = &t
		}
	}

	return filter
}

// Apply applies the filter to a list of models and returns filtered results.
func (f ModelFilter) Apply(models []catalogs.Model) []catalogs.Model {
	var results []catalogs.Model

	for _, model := range models {
		if f.matches(model) {
			results = append(results, model)
		}
	}

	// Apply sorting
	if f.Sort != "" {
		results = f.sort(results)
	}

	return results
}

// matches checks if a model matches the filter criteria.
func (f ModelFilter) matches(model catalogs.Model) bool {
	return f.matchesBasicFilters(model) &&
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

// matchesModalityFilters checks input and output modality filters.
func (f ModelFilter) matchesModalityFilters(model catalogs.Model) bool {
	if len(f.ModalityInput) > 0 && model.Features != nil {
		if !modalityContainsAll(model.Features.Modalities.Input, f.ModalityInput) {
			return false
		}
	}
	if len(f.ModalityOutput) > 0 && model.Features != nil {
		if !modalityContainsAll(model.Features.Modalities.Output, f.ModalityOutput) {
			return false
		}
	}
	return true
}

// matchesFeaturesFilter checks feature capability filters.
func (f ModelFilter) matchesFeaturesFilter(model catalogs.Model) bool {
	if len(f.Features) > 0 && model.Features != nil {
		for feature, required := range f.Features {
			if !matchFeature(model.Features, feature, required) {
				return false
			}
		}
	}
	return true
}

// matchesMetadataFilters checks tags and open weights filters.
func (f ModelFilter) matchesMetadataFilters(model catalogs.Model) bool {
	if len(f.Tags) > 0 && model.Metadata != nil {
		if !tagContainsAny(model.Metadata.Tags, f.Tags) {
			return false
		}
	}
	if f.OpenWeights != nil && model.Metadata != nil {
		if model.Metadata.OpenWeights != *f.OpenWeights {
			return false
		}
	}
	return true
}

// matchesLimitFilters checks context window and output token range filters.
func (f ModelFilter) matchesLimitFilters(model catalogs.Model) bool {
	if model.Limits == nil {
		return true
	}
	if f.MinContext > 0 && model.Limits.ContextWindow < f.MinContext {
		return false
	}
	if f.MaxContext > 0 && model.Limits.ContextWindow > f.MaxContext {
		return false
	}
	if f.MinOutput > 0 && model.Limits.OutputTokens < f.MinOutput {
		return false
	}
	if f.MaxOutput > 0 && model.Limits.OutputTokens > f.MaxOutput {
		return false
	}
	return true
}

// matchesDateFilters checks release date range filters.
func (f ModelFilter) matchesDateFilters(model catalogs.Model) bool {
	if model.Metadata == nil || model.Metadata.ReleaseDate.IsZero() {
		return true
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
	// Simple implementation - for production, use more sophisticated sorting
	// This is a placeholder that maintains current order
	return models
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
		return true
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

// parseIntOrDefault parses an integer or returns default.
func parseIntOrDefault(s string, def int) int {
	if s == "" {
		return def
	}
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	return def
}
