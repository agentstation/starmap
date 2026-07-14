package params

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/catalog/query"
)

// TestParseModelFilterStrict tests query parameter parsing into query.ModelFilter.
func TestParseModelFilterStrict(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected query.ModelFilter
	}{
		{
			name:  "empty query",
			query: "",
			expected: query.ModelFilter{
				Features:   map[string]bool{},
				Limit:      100,
				Offset:     0,
				MaxResults: 1000,
			},
		},
		{
			name:  "basic filters",
			query: "id=gpt-4&name=GPT-4&provider=openai&status=active",
			expected: query.ModelFilter{
				ID:         "gpt-4",
				Name:       "GPT-4",
				Provider:   "openai",
				Status:     "active",
				Features:   map[string]bool{},
				Limit:      100,
				MaxResults: 1000,
			},
		},
		{
			name:  "name contains filter",
			query: "name_contains=gpt",
			expected: query.ModelFilter{
				NameContains: "gpt",
				Features:     map[string]bool{},
				Limit:        100,
				MaxResults:   1000,
			},
		},
		{
			name:  "modality filters",
			query: "modality_input=text,image&modality_output=text",
			expected: query.ModelFilter{
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
			expected: query.ModelFilter{
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
			expected: query.ModelFilter{
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
			expected: query.ModelFilter{
				Tags:       []string{"audio", "vision"},
				Features:   map[string]bool{},
				Limit:      100,
				MaxResults: 1000,
			},
		},
		{
			name:  "open weights filter",
			query: "open_weights=true",
			expected: query.ModelFilter{
				OpenWeights: boolPtr(true),
				Features:    map[string]bool{},
				Limit:       100,
				MaxResults:  1000,
			},
		},
		{
			name:  "context window range",
			query: "min_context=4096&max_context=128000",
			expected: query.ModelFilter{
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
			expected: query.ModelFilter{
				MinOutput:  1024,
				MaxOutput:  4096,
				Features:   map[string]bool{},
				Limit:      100,
				MaxResults: 1000,
			},
		},
		{
			name:  "input tokens range",
			query: "min_input=8192&max_input=96000",
			expected: query.ModelFilter{
				MinInput:   8192,
				MaxInput:   96000,
				Features:   map[string]bool{},
				Limit:      100,
				MaxResults: 1000,
			},
		},
		{
			name:  "pagination",
			query: "limit=50&offset=100&max_results=500",
			expected: query.ModelFilter{
				Features:   map[string]bool{},
				Limit:      50,
				Offset:     100,
				MaxResults: 500,
			},
		},
		{
			name:  "sort and order",
			query: "sort=name&order=desc",
			expected: query.ModelFilter{
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
			expected: query.ModelFilter{
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
			expected: query.ModelFilter{
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
			result, err := ParseModelFilterStrict(req)
			if err != nil {
				t.Fatalf("ParseModelFilterStrict: %v", err)
			}

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
			if result.Status != tt.expected.Status {
				t.Errorf("Status: got %q, want %q", result.Status, tt.expected.Status)
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
			if result.MinInput != tt.expected.MinInput {
				t.Errorf("MinInput: got %d, want %d", result.MinInput, tt.expected.MinInput)
			}
			if result.MaxInput != tt.expected.MaxInput {
				t.Errorf("MaxInput: got %d, want %d", result.MaxInput, tt.expected.MaxInput)
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

func TestParseModelFilterStrictRejectsMalformedClientInput(t *testing.T) {
	for _, query := range []string{
		"limit=not-a-number",
		"limit=1001",
		"offset=-1",
		"open_weights=maybe",
		"feature_streaming=maybe",
		"feature=invented",
		"sort=price",
		"order=desc",
		"sort=id&order=sideways",
		"released_after=yesterday",
		"min_context=100&max_context=10",
		"modality_input=hologram",
	} {
		req := httptest.NewRequest("GET", "/models?"+query, nil)
		if _, err := ParseModelFilterStrict(req); err == nil {
			t.Fatalf("query %q passed strict parsing", query)
		}
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
