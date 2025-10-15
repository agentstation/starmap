package filter

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// TestParseModelFilter tests query parameter parsing into ModelFilter struct.
func TestParseModelFilter(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected ModelFilter
	}{
		{
			name:  "empty query",
			query: "",
			expected: ModelFilter{
				Features:   map[string]bool{},
				Limit:      100,
				Offset:     0,
				MaxResults: 1000,
			},
		},
		{
			name:  "basic filters",
			query: "id=gpt-4&name=GPT-4&provider=openai",
			expected: ModelFilter{
				ID:         "gpt-4",
				Name:       "GPT-4",
				Provider:   "openai",
				Features:   map[string]bool{},
				Limit:      100,
				MaxResults: 1000,
			},
		},
		{
			name:  "name contains filter",
			query: "name_contains=gpt",
			expected: ModelFilter{
				NameContains: "gpt",
				Features:     map[string]bool{},
				Limit:        100,
				MaxResults:   1000,
			},
		},
		{
			name:  "modality filters",
			query: "modality_input=text,image&modality_output=text",
			expected: ModelFilter{
				ModalityInput:  []string{"text", "image"},
				ModalityOutput: []string{"text"},
				Features:       map[string]bool{},
				Limit:          100,
				MaxResults:     1000,
			},
		},
		{
			name:  "feature filters - explicit",
			query: "feature_streaming=true&feature_tool_calls=false",
			expected: ModelFilter{
				Features: map[string]bool{
					"streaming":  true,
					"tool_calls": false,
				},
				Limit:      100,
				MaxResults: 1000,
			},
		},
		{
			name:  "feature filters - shorthand",
			query: "feature=streaming",
			expected: ModelFilter{
				Features: map[string]bool{
					"streaming": true,
				},
				Limit:      100,
				MaxResults: 1000,
			},
		},
		{
			name:  "tags filter",
			query: "tag=audio,vision",
			expected: ModelFilter{
				Tags:       []string{"audio", "vision"},
				Features:   map[string]bool{},
				Limit:      100,
				MaxResults: 1000,
			},
		},
		{
			name:  "open weights filter",
			query: "open_weights=true",
			expected: ModelFilter{
				OpenWeights: boolPtr(true),
				Features:    map[string]bool{},
				Limit:       100,
				MaxResults:  1000,
			},
		},
		{
			name:  "context window range",
			query: "min_context=4096&max_context=128000",
			expected: ModelFilter{
				MinContext: 4096,
				MaxContext: 128000,
				Features:   map[string]bool{},
				Limit:      100,
				MaxResults: 1000,
			},
		},
		{
			name:  "output tokens range",
			query: "min_output=1024&max_output=4096",
			expected: ModelFilter{
				MinOutput:  1024,
				MaxOutput:  4096,
				Features:   map[string]bool{},
				Limit:      100,
				MaxResults: 1000,
			},
		},
		{
			name:  "pagination",
			query: "limit=50&offset=100&max_results=500",
			expected: ModelFilter{
				Features:   map[string]bool{},
				Limit:      50,
				Offset:     100,
				MaxResults: 500,
			},
		},
		{
			name:  "sort and order",
			query: "sort=name&order=desc",
			expected: ModelFilter{
				Sort:       "name",
				Order:      "desc",
				Features:   map[string]bool{},
				Limit:      100,
				MaxResults: 1000,
			},
		},
		{
			name:  "date range filters",
			query: "released_after=2024-01-01T00:00:00Z&released_before=2024-12-31T23:59:59Z",
			expected: ModelFilter{
				ReleasedAfter:  timePtr(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
				ReleasedBefore: timePtr(time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)),
				Features:       map[string]bool{},
				Limit:          100,
				MaxResults:     1000,
			},
		},
		{
			name:  "combined complex filters",
			query: "name_contains=gpt&provider=openai&modality_input=text&feature_streaming=true&min_context=8000&limit=25",
			expected: ModelFilter{
				NameContains:  "gpt",
				Provider:      "openai",
				ModalityInput: []string{"text"},
				Features: map[string]bool{
					"streaming": true,
				},
				MinContext: 8000,
				Limit:      25,
				MaxResults: 1000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock request with query params
			req := httptest.NewRequest("GET", "/models?"+tt.query, nil)

			// Parse filter
			result := ParseModelFilter(req)

			// Verify basic fields
			if result.ID != tt.expected.ID {
				t.Errorf("ID: got %q, want %q", result.ID, tt.expected.ID)
			}
			if result.Name != tt.expected.Name {
				t.Errorf("Name: got %q, want %q", result.Name, tt.expected.Name)
			}
			if result.NameContains != tt.expected.NameContains {
				t.Errorf("NameContains: got %q, want %q", result.NameContains, tt.expected.NameContains)
			}
			if result.Provider != tt.expected.Provider {
				t.Errorf("Provider: got %q, want %q", result.Provider, tt.expected.Provider)
			}

			// Verify slices
			if !stringSliceEqual(result.ModalityInput, tt.expected.ModalityInput) {
				t.Errorf("ModalityInput: got %v, want %v", result.ModalityInput, tt.expected.ModalityInput)
			}
			if !stringSliceEqual(result.ModalityOutput, tt.expected.ModalityOutput) {
				t.Errorf("ModalityOutput: got %v, want %v", result.ModalityOutput, tt.expected.ModalityOutput)
			}
			if !stringSliceEqual(result.Tags, tt.expected.Tags) {
				t.Errorf("Tags: got %v, want %v", result.Tags, tt.expected.Tags)
			}

			// Verify maps
			if !boolMapEqual(result.Features, tt.expected.Features) {
				t.Errorf("Features: got %v, want %v", result.Features, tt.expected.Features)
			}

			// Verify pointers
			if !boolPtrEqual(result.OpenWeights, tt.expected.OpenWeights) {
				t.Errorf("OpenWeights: got %v, want %v", result.OpenWeights, tt.expected.OpenWeights)
			}
			if !timePtrEqual(result.ReleasedAfter, tt.expected.ReleasedAfter) {
				t.Errorf("ReleasedAfter: got %v, want %v", result.ReleasedAfter, tt.expected.ReleasedAfter)
			}
			if !timePtrEqual(result.ReleasedBefore, tt.expected.ReleasedBefore) {
				t.Errorf("ReleasedBefore: got %v, want %v", result.ReleasedBefore, tt.expected.ReleasedBefore)
			}

			// Verify numeric fields
			if result.MinContext != tt.expected.MinContext {
				t.Errorf("MinContext: got %d, want %d", result.MinContext, tt.expected.MinContext)
			}
			if result.MaxContext != tt.expected.MaxContext {
				t.Errorf("MaxContext: got %d, want %d", result.MaxContext, tt.expected.MaxContext)
			}
			if result.MinOutput != tt.expected.MinOutput {
				t.Errorf("MinOutput: got %d, want %d", result.MinOutput, tt.expected.MinOutput)
			}
			if result.MaxOutput != tt.expected.MaxOutput {
				t.Errorf("MaxOutput: got %d, want %d", result.MaxOutput, tt.expected.MaxOutput)
			}
			if result.Limit != tt.expected.Limit {
				t.Errorf("Limit: got %d, want %d", result.Limit, tt.expected.Limit)
			}
			if result.Offset != tt.expected.Offset {
				t.Errorf("Offset: got %d, want %d", result.Offset, tt.expected.Offset)
			}
			if result.MaxResults != tt.expected.MaxResults {
				t.Errorf("MaxResults: got %d, want %d", result.MaxResults, tt.expected.MaxResults)
			}

			// Verify string fields
			if result.Sort != tt.expected.Sort {
				t.Errorf("Sort: got %q, want %q", result.Sort, tt.expected.Sort)
			}
			if result.Order != tt.expected.Order {
				t.Errorf("Order: got %q, want %q", result.Order, tt.expected.Order)
			}
		})
	}
}

// TestModelFilter_Apply tests the filtering logic.
func TestModelFilter_Apply(t *testing.T) {
	// Create test models
	models := []catalogs.Model{
		{
			ID:   "gpt-4",
			Name: "GPT-4",
			Features: &catalogs.ModelFeatures{
				Streaming: true,
				ToolCalls: true,
				Modalities: catalogs.ModelModalities{
					Input:  []catalogs.ModelModality{"text"},
					Output: []catalogs.ModelModality{"text"},
				},
			},
			Limits: &catalogs.ModelLimits{
				ContextWindow: 128000,
				OutputTokens:  4096,
			},
			Metadata: &catalogs.ModelMetadata{
				Tags:        []catalogs.ModelTag{"chat"},
				OpenWeights: false,
			},
		},
		{
			ID:   "claude-3-opus",
			Name: "Claude 3 Opus",
			Features: &catalogs.ModelFeatures{
				Streaming: true,
				ToolCalls: true,
				Modalities: catalogs.ModelModalities{
					Input:  []catalogs.ModelModality{"text", "image"},
					Output: []catalogs.ModelModality{"text"},
				},
			},
			Limits: &catalogs.ModelLimits{
				ContextWindow: 200000,
				OutputTokens:  4096,
			},
			Metadata: &catalogs.ModelMetadata{
				Tags:        []catalogs.ModelTag{"chat", "vision"},
				OpenWeights: false,
			},
		},
		{
			ID:   "llama-3-70b",
			Name: "Llama 3 70B",
			Features: &catalogs.ModelFeatures{
				Streaming: true,
				ToolCalls: false,
				Modalities: catalogs.ModelModalities{
					Input:  []catalogs.ModelModality{"text"},
					Output: []catalogs.ModelModality{"text"},
				},
			},
			Limits: &catalogs.ModelLimits{
				ContextWindow: 8192,
				OutputTokens:  2048,
			},
			Metadata: &catalogs.ModelMetadata{
				Tags:        []catalogs.ModelTag{"chat", "open"},
				OpenWeights: true,
			},
		},
	}

	tests := []struct {
		name     string
		filter   ModelFilter
		expected []string // Expected model IDs in result
	}{
		{
			name:     "no filters - return all",
			filter:   ModelFilter{Features: map[string]bool{}},
			expected: []string{"gpt-4", "claude-3-opus", "llama-3-70b"},
		},
		{
			name: "filter by ID",
			filter: ModelFilter{
				ID:       "gpt-4",
				Features: map[string]bool{},
			},
			expected: []string{"gpt-4"},
		},
		{
			name: "filter by name (case insensitive)",
			filter: ModelFilter{
				Name:     "gpt-4",
				Features: map[string]bool{},
			},
			expected: []string{"gpt-4"},
		},
		{
			name: "filter by name contains",
			filter: ModelFilter{
				NameContains: "claude",
				Features:     map[string]bool{},
			},
			expected: []string{"claude-3-opus"},
		},
		{
			name: "filter by input modality",
			filter: ModelFilter{
				ModalityInput: []string{"image"},
				Features:      map[string]bool{},
			},
			expected: []string{"claude-3-opus"},
		},
		{
			name: "filter by streaming feature",
			filter: ModelFilter{
				Features: map[string]bool{
					"streaming": true,
				},
			},
			expected: []string{"gpt-4", "claude-3-opus", "llama-3-70b"},
		},
		{
			name: "filter by tool_calls feature",
			filter: ModelFilter{
				Features: map[string]bool{
					"tool_calls": true,
				},
			},
			expected: []string{"gpt-4", "claude-3-opus"},
		},
		{
			name: "filter by open weights",
			filter: ModelFilter{
				OpenWeights: boolPtr(true),
				Features:    map[string]bool{},
			},
			expected: []string{"llama-3-70b"},
		},
		{
			name: "filter by tags",
			filter: ModelFilter{
				Tags:     []string{"vision"},
				Features: map[string]bool{},
			},
			expected: []string{"claude-3-opus"},
		},
		{
			name: "filter by min context window",
			filter: ModelFilter{
				MinContext: 100000,
				Features:   map[string]bool{},
			},
			expected: []string{"gpt-4", "claude-3-opus"},
		},
		{
			name: "filter by max context window",
			filter: ModelFilter{
				MaxContext: 10000,
				Features:   map[string]bool{},
			},
			expected: []string{"llama-3-70b"},
		},
		{
			name: "combined filters",
			filter: ModelFilter{
				NameContains: "gpt",
				Features: map[string]bool{
					"streaming": true,
				},
				MinContext: 50000,
			},
			expected: []string{"gpt-4"},
		},
		{
			name: "no matches",
			filter: ModelFilter{
				NameContains: "nonexistent",
				Features:     map[string]bool{},
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.filter.Apply(models)

			// Extract IDs from result
			var resultIDs []string
			for _, m := range result {
				resultIDs = append(resultIDs, m.ID)
			}

			// Compare
			if !stringSliceEqual(resultIDs, tt.expected) {
				t.Errorf("got %v, want %v", resultIDs, tt.expected)
			}
		})
	}
}

// TestMatchFeature tests individual feature matching.
func TestMatchFeature(t *testing.T) {
	features := &catalogs.ModelFeatures{
		Streaming:   true,
		ToolCalls:   true,
		Tools:       false,
		ToolChoice:  false,
		Reasoning:   true,
		Temperature: true,
		MaxTokens:   true,
	}

	tests := []struct {
		name     string
		feature  string
		required bool
		expected bool
	}{
		{"streaming matches true", "streaming", true, true},
		{"streaming matches false", "streaming", false, false},
		{"tool_calls matches true", "tool_calls", true, true},
		{"tools matches false", "tools", false, true},
		{"tools doesn't match true", "tools", true, false},
		{"unknown feature defaults to true", "unknown", true, true},
		{"unknown feature defaults to true", "unknown", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchFeature(features, tt.feature, tt.required)
			if result != tt.expected {
				t.Errorf("matchFeature(%q, %v) = %v, want %v", tt.feature, tt.required, result, tt.expected)
			}
		})
	}
}

// TestModalityContainsAll tests modality matching.
func TestModalityContainsAll(t *testing.T) {
	tests := []struct {
		name     string
		slice    []catalogs.ModelModality
		required []string
		expected bool
	}{
		{
			name:     "empty required always matches",
			slice:    []catalogs.ModelModality{"text"},
			required: []string{},
			expected: true,
		},
		{
			name:     "single match",
			slice:    []catalogs.ModelModality{"text"},
			required: []string{"text"},
			expected: true,
		},
		{
			name:     "case insensitive match",
			slice:    []catalogs.ModelModality{"text"},
			required: []string{"TEXT"},
			expected: true,
		},
		{
			name:     "multiple matches",
			slice:    []catalogs.ModelModality{"text", "image", "audio"},
			required: []string{"text", "image"},
			expected: true,
		},
		{
			name:     "missing required modality",
			slice:    []catalogs.ModelModality{"text"},
			required: []string{"text", "image"},
			expected: false,
		},
		{
			name:     "no match",
			slice:    []catalogs.ModelModality{"text"},
			required: []string{"image"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := modalityContainsAll(tt.slice, tt.required)
			if result != tt.expected {
				t.Errorf("modalityContainsAll(%v, %v) = %v, want %v", tt.slice, tt.required, result, tt.expected)
			}
		})
	}
}

// TestTagContainsAny tests tag matching.
func TestTagContainsAny(t *testing.T) {
	tests := []struct {
		name     string
		slice    []catalogs.ModelTag
		values   []string
		expected bool
	}{
		{
			name:     "empty values always false",
			slice:    []catalogs.ModelTag{"chat"},
			values:   []string{},
			expected: false,
		},
		{
			name:     "single match",
			slice:    []catalogs.ModelTag{"chat", "vision"},
			values:   []string{"vision"},
			expected: true,
		},
		{
			name:     "case insensitive match",
			slice:    []catalogs.ModelTag{"chat"},
			values:   []string{"CHAT"},
			expected: true,
		},
		{
			name:     "multiple options, one matches",
			slice:    []catalogs.ModelTag{"chat"},
			values:   []string{"vision", "chat", "audio"},
			expected: true,
		},
		{
			name:     "no match",
			slice:    []catalogs.ModelTag{"chat"},
			values:   []string{"vision", "audio"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tagContainsAny(tt.slice, tt.values)
			if result != tt.expected {
				t.Errorf("tagContainsAny(%v, %v) = %v, want %v", tt.slice, tt.values, result, tt.expected)
			}
		})
	}
}

// TestParseIntOrDefault tests integer parsing helper.
func TestParseIntOrDefault(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		def      int
		expected int
	}{
		{"empty string returns default", "", 100, 100},
		{"valid integer", "42", 100, 42},
		{"zero value", "0", 100, 0},
		{"negative value", "-5", 100, -5},
		{"invalid string returns default", "abc", 100, 100},
		{"float returns default", "3.14", 100, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseIntOrDefault(tt.input, tt.def)
			if result != tt.expected {
				t.Errorf("parseIntOrDefault(%q, %d) = %d, want %d", tt.input, tt.def, result, tt.expected)
			}
		})
	}
}

// Helper functions for test comparisons

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func boolMapEqual(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func boolPtrEqual(a, b *bool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func timePtrEqual(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(*b)
}

func boolPtr(b bool) *bool {
	return &b
}

func timePtr(t time.Time) *time.Time {
	return &t
}
