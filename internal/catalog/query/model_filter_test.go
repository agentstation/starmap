package query

import (
	"testing"
	"time"

	"github.com/agentstation/utc"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestModelFilterSortImplementsDeclaredFieldsWithMissingValuesLast(t *testing.T) {
	models := []catalogs.Model{
		{ID: "b", Name: "Alpha", CreatedAt: utc.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)), UpdatedAt: utc.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)), Limits: &catalogs.ModelLimits{ContextWindow: 20}, Metadata: &catalogs.ModelMetadata{ReleaseDate: utc.New(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))}},
		{ID: "a", Name: "Zulu", CreatedAt: utc.New(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)), UpdatedAt: utc.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)), Limits: &catalogs.ModelLimits{ContextWindow: 10}, Metadata: &catalogs.ModelMetadata{ReleaseDate: utc.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))}},
		{ID: "c", Name: ""},
	}
	for _, test := range []struct {
		name  string
		sort  string
		order string
		want  []string
	}{
		{name: "id asc", sort: "id", order: "asc", want: []string{"a", "b", "c"}},
		{name: "name desc", sort: "name", order: "desc", want: []string{"a", "b", "c"}},
		{name: "release desc", sort: "release_date", order: "desc", want: []string{"a", "b", "c"}},
		{name: "context asc", sort: "context_window", order: "asc", want: []string{"a", "b", "c"}},
		{name: "created asc", sort: "created_at", order: "asc", want: []string{"b", "a", "c"}},
		{name: "updated desc", sort: "updated_at", order: "desc", want: []string{"b", "a", "c"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			filter := ModelFilter{Sort: test.sort, Order: test.order, Limit: 100, MaxResults: 1000}
			if err := filter.Validate(); err != nil {
				t.Fatalf("Validate: %v", err)
			}
			gotModels := filter.Apply(models)
			got := make([]string, len(gotModels))
			for index := range gotModels {
				got[index] = gotModels[index].ID
			}
			if !stringSliceEqual(got, test.want) {
				t.Fatalf("sort = %v, want %v", got, test.want)
			}
		})
	}
}

func TestModelFilterValidateRejectsUnsupportedAndAmbiguousInputs(t *testing.T) {
	for _, filter := range []ModelFilter{
		{Sort: "price", Limit: 100},
		{Order: "desc", Limit: 100},
		{Sort: "id", Order: "sideways", Limit: 100},
		{Limit: 1001},
		{Limit: 100, Offset: -1},
		{Limit: 100, MinContext: 10, MaxContext: 5},
		{Limit: 100, Features: map[string]bool{"invented": true}},
		{Limit: 100, ModalityInput: []string{"hologram"}},
		{Limit: 100, Status: "retired"},
	} {
		if err := filter.Validate(); err == nil {
			t.Fatalf("filter %#v passed validation", filter)
		}
	}
}

func TestModelFilterApply(t *testing.T) {
	models := []catalogs.Model{
		{
			ID:     "gpt-4",
			Name:   "GPT-4",
			Status: catalogs.ModelStatusActive,
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
				InputTokens:   96000,
				OutputTokens:  4096,
			},
			Metadata: &catalogs.ModelMetadata{
				Tags:        []catalogs.ModelTag{"chat"},
				OpenWeights: false,
			},
		},
		{
			ID:     "claude-3-opus",
			Name:   "Claude 3 Opus",
			Status: catalogs.ModelStatusBeta,
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
				InputTokens:   160000,
				OutputTokens:  4096,
			},
			Metadata: &catalogs.ModelMetadata{
				Tags:        []catalogs.ModelTag{"chat", "vision"},
				OpenWeights: false,
			},
		},
		{
			ID:     "llama-3-70b",
			Name:   "Llama 3 70B",
			Status: catalogs.ModelStatusDeprecated,
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
				InputTokens:   4096,
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
		expected []string
	}{
		{
			name:     "no filters",
			filter:   ModelFilter{Features: map[string]bool{}},
			expected: []string{"gpt-4", "claude-3-opus", "llama-3-70b"},
		},
		{
			name:     "filter by id",
			filter:   ModelFilter{ID: "gpt-4", Features: map[string]bool{}},
			expected: []string{"gpt-4"},
		},
		{
			name:     "filter by name case insensitive",
			filter:   ModelFilter{Name: "gpt-4", Features: map[string]bool{}},
			expected: []string{"gpt-4"},
		},
		{
			name:     "filter by name contains",
			filter:   ModelFilter{NameContains: "claude", Features: map[string]bool{}},
			expected: []string{"claude-3-opus"},
		},
		{
			name:     "filter by input modality",
			filter:   ModelFilter{ModalityInput: []string{"image"}, Features: map[string]bool{}},
			expected: []string{"claude-3-opus"},
		},
		{
			name: "filter by feature",
			filter: ModelFilter{Features: map[string]bool{
				"tool_calls": true,
			}},
			expected: []string{"gpt-4", "claude-3-opus"},
		},
		{
			name:     "filter by open weights",
			filter:   ModelFilter{OpenWeights: boolPtr(true), Features: map[string]bool{}},
			expected: []string{"llama-3-70b"},
		},
		{
			name:     "filter by tag",
			filter:   ModelFilter{Tags: []string{"vision"}, Features: map[string]bool{}},
			expected: []string{"claude-3-opus"},
		},
		{
			name:     "filter by min context",
			filter:   ModelFilter{MinContext: 100000, Features: map[string]bool{}},
			expected: []string{"gpt-4", "claude-3-opus"},
		},
		{
			name:     "filter by max context",
			filter:   ModelFilter{MaxContext: 10000, Features: map[string]bool{}},
			expected: []string{"llama-3-70b"},
		},
		{
			name:     "filter by status",
			filter:   ModelFilter{Status: "beta", Features: map[string]bool{}},
			expected: []string{"claude-3-opus"},
		},
		{
			name:     "filter by min input tokens",
			filter:   ModelFilter{MinInput: 100000, Features: map[string]bool{}},
			expected: []string{"claude-3-opus"},
		},
		{
			name:     "filter by max input tokens",
			filter:   ModelFilter{MaxInput: 5000, Features: map[string]bool{}},
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
			name:     "no matches",
			filter:   ModelFilter{NameContains: "nonexistent", Features: map[string]bool{}},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.filter.Apply(models)
			resultIDs := make([]string, 0, len(result))
			for _, model := range result {
				resultIDs = append(resultIDs, model.ID)
			}
			if !stringSliceEqual(resultIDs, tt.expected) {
				t.Fatalf("got %v, want %v", resultIDs, tt.expected)
			}
		})
	}
}

func TestModelFilterMissingNestedDataFailsClosed(t *testing.T) {
	models := []catalogs.Model{{
		ID:   "unknown-shape",
		Name: "Unknown Shape",
	}}

	tests := []struct {
		name   string
		filter ModelFilter
	}{
		{
			name:   "input modality requires features",
			filter: ModelFilter{ModalityInput: []string{"image"}, Features: map[string]bool{}},
		},
		{
			name:   "output modality requires features",
			filter: ModelFilter{ModalityOutput: []string{"audio"}, Features: map[string]bool{}},
		},
		{
			name:   "feature requires features",
			filter: ModelFilter{Features: map[string]bool{"streaming": true}},
		},
		{
			name:   "tag requires metadata",
			filter: ModelFilter{Tags: []string{"chat"}, Features: map[string]bool{}},
		},
		{
			name:   "open weights requires metadata",
			filter: ModelFilter{OpenWeights: boolPtr(true), Features: map[string]bool{}},
		},
		{
			name:   "limit requires limits",
			filter: ModelFilter{MinInput: 1, Features: map[string]bool{}},
		},
		{
			name:   "release date requires metadata date",
			filter: ModelFilter{ReleasedAfter: timePtr(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)), Features: map[string]bool{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.Apply(models); len(got) != 0 {
				t.Fatalf("got %d matches, want 0", len(got))
			}
		})
	}
}

func TestModelFilterLimitRangesRequireKnownValues(t *testing.T) {
	models := []catalogs.Model{{
		ID:   "unknown-input",
		Name: "Unknown Input",
		Limits: &catalogs.ModelLimits{
			ContextWindow: 128000,
			OutputTokens:  4096,
		},
	}}

	if got := (ModelFilter{MaxInput: 5000, Features: map[string]bool{}}).Apply(models); len(got) != 0 {
		t.Fatalf("got %d matches for unknown input token limit, want 0", len(got))
	}
}

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
		{name: "streaming true", feature: "streaming", required: true, expected: true},
		{name: "streaming false", feature: "streaming", required: false, expected: false},
		{name: "tools false", feature: "tools", required: false, expected: true},
		{name: "tools true", feature: "tools", required: true, expected: false},
		{name: "unknown true", feature: "unknown", required: true, expected: false},
		{name: "unknown false", feature: "unknown", required: false, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchFeature(features, tt.feature, tt.required); got != tt.expected {
				t.Fatalf("got %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestModalityContainsAll(t *testing.T) {
	tests := []struct {
		name     string
		slice    []catalogs.ModelModality
		required []string
		expected bool
	}{
		{name: "empty required", slice: []catalogs.ModelModality{"text"}, required: []string{}, expected: true},
		{name: "single match", slice: []catalogs.ModelModality{"text"}, required: []string{"text"}, expected: true},
		{name: "case insensitive", slice: []catalogs.ModelModality{"text"}, required: []string{"TEXT"}, expected: true},
		{name: "multiple matches", slice: []catalogs.ModelModality{"text", "image"}, required: []string{"text", "image"}, expected: true},
		{name: "missing required", slice: []catalogs.ModelModality{"text"}, required: []string{"text", "image"}, expected: false},
		{name: "no match", slice: []catalogs.ModelModality{"text"}, required: []string{"image"}, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := modalityContainsAll(tt.slice, tt.required); got != tt.expected {
				t.Fatalf("got %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTagContainsAny(t *testing.T) {
	tests := []struct {
		name     string
		slice    []catalogs.ModelTag
		values   []string
		expected bool
	}{
		{name: "empty values", slice: []catalogs.ModelTag{"chat"}, values: []string{}, expected: false},
		{name: "single match", slice: []catalogs.ModelTag{"chat", "vision"}, values: []string{"vision"}, expected: true},
		{name: "case insensitive", slice: []catalogs.ModelTag{"chat"}, values: []string{"CHAT"}, expected: true},
		{name: "multiple options", slice: []catalogs.ModelTag{"chat"}, values: []string{"vision", "chat"}, expected: true},
		{name: "no match", slice: []catalogs.ModelTag{"chat"}, values: []string{"vision"}, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tagContainsAny(tt.slice, tt.values); got != tt.expected {
				t.Fatalf("got %v, want %v", got, tt.expected)
			}
		})
	}
}

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

func boolPtr(b bool) *bool {
	return &b
}

func timePtr(t time.Time) *time.Time {
	return &t
}
