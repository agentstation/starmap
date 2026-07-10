// Package params provides HTTP request parameter parsing for API handlers.
package params

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/agentstation/starmap/internal/catalog/query"
	"github.com/agentstation/starmap/pkg/errors"
)

// ParseModelFilter extracts model filter parameters from an HTTP request.
func ParseModelFilter(r *http.Request) query.ModelFilter {
	q := r.URL.Query()

	filter := query.ModelFilter{
		ID:           q.Get("id"),
		Name:         q.Get("name"),
		NameContains: q.Get("name_contains"),
		Provider:     q.Get("provider"),
		Status:       q.Get("status"),
		Sort:         q.Get("sort"),
		Order:        q.Get("order"),
		Limit:        parseIntOrDefault(q.Get("limit"), 100),
		Offset:       parseIntOrDefault(q.Get("offset"), 0),
		MaxResults:   parseIntOrDefault(q.Get("max_results"), 1000),
	}

	if modalInput := q.Get("modality_input"); modalInput != "" {
		filter.ModalityInput = strings.Split(modalInput, ",")
	}
	if modalOutput := q.Get("modality_output"); modalOutput != "" {
		filter.ModalityOutput = strings.Split(modalOutput, ",")
	}

	filter.Features = make(map[string]bool)
	for _, feature := range []string{"streaming", "tool_calls", "tools", "tool_choice", "reasoning", "temperature", "max_tokens"} {
		if val := q.Get("feature_" + feature); val != "" {
			if b, err := strconv.ParseBool(val); err == nil {
				filter.Features[feature] = b
			}
		}
	}
	if feature := q.Get("feature"); feature != "" {
		filter.Features[feature] = true
	}

	if tags := q.Get("tag"); tags != "" {
		filter.Tags = strings.Split(tags, ",")
	}

	if ow := q.Get("open_weights"); ow != "" {
		if b, err := strconv.ParseBool(ow); err == nil {
			filter.OpenWeights = &b
		}
	}

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

	if minInput := q.Get("min_input"); minInput != "" {
		if i, err := strconv.ParseInt(minInput, 10, 64); err == nil {
			filter.MinInput = i
		}
	}
	if maxInput := q.Get("max_input"); maxInput != "" {
		if i, err := strconv.ParseInt(maxInput, 10, 64); err == nil {
			filter.MaxInput = i
		}
	}

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

// ParseModelFilterStrict parses and validates every supplied query parameter.
// Unlike the compatibility parser, malformed client input is never silently
// replaced by a default or ignored.
func ParseModelFilterStrict(r *http.Request) (query.ModelFilter, error) {
	q := r.URL.Query()
	for _, field := range []string{
		"limit", "offset", "max_results", "min_context", "max_context",
		"min_input", "max_input", "min_output", "max_output",
	} {
		if value := q.Get(field); value != "" {
			if _, err := strconv.ParseInt(value, 10, 64); err != nil {
				return query.ModelFilter{}, &errors.ValidationError{Field: "model_filter." + field, Value: value, Message: "must be an integer"}
			}
		}
	}
	for _, field := range []string{
		"open_weights", "feature_streaming", "feature_tool_calls", "feature_tools",
		"feature_tool_choice", "feature_reasoning", "feature_temperature", "feature_max_tokens",
	} {
		if value := q.Get(field); value != "" {
			if _, err := strconv.ParseBool(value); err != nil {
				return query.ModelFilter{}, &errors.ValidationError{Field: "model_filter." + field, Value: value, Message: "must be a boolean"}
			}
		}
	}
	for _, field := range []string{"released_after", "released_before"} {
		if value := q.Get(field); value != "" {
			if _, err := time.Parse(time.RFC3339, value); err != nil {
				return query.ModelFilter{}, &errors.ValidationError{Field: "model_filter." + field, Value: value, Message: "must be RFC3339"}
			}
		}
	}
	filter := ParseModelFilter(r)
	if err := filter.Validate(); err != nil {
		return query.ModelFilter{}, err
	}
	return filter, nil
}

func parseIntOrDefault(s string, def int) int {
	if s == "" {
		return def
	}
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	return def
}
