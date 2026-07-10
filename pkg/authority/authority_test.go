package authority

import (
	"testing"

	"github.com/agentstation/starmap/pkg/sources"
)

func TestFindResolvesExpectedDefaultAuthorities(t *testing.T) {
	auth := New()

	tests := []struct {
		name         string
		resourceType sources.ResourceType
		fieldPath    string
		wantSource   sources.ID
		wantPriority int
	}{
		{
			name:         "model pricing prefers provider observation",
			resourceType: sources.ResourceTypeModel,
			fieldPath:    "Pricing",
			wantSource:   sources.ProvidersID,
			wantPriority: 110,
		},
		{
			name:         "provider catalog nested fields prefer local catalog",
			resourceType: sources.ResourceTypeProvider,
			fieldPath:    "Catalog.Endpoint.URL",
			wantSource:   sources.LocalCatalogID,
			wantPriority: 95,
		},
		{
			name:         "provider policy nested fields prefer models.dev HTTP",
			resourceType: sources.ResourceTypeProvider,
			fieldPath:    "PrivacyPolicy.URL",
			wantSource:   sources.ModelsDevHTTPID,
			wantPriority: 90,
		},
		{
			name:         "author name prefers local catalog",
			resourceType: sources.ResourceTypeAuthor,
			fieldPath:    "Name",
			wantSource:   sources.LocalCatalogID,
			wantPriority: 90,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := auth.Find(tt.resourceType, tt.fieldPath)
			if field == nil {
				t.Fatalf("Find(%q, %q) returned nil", tt.resourceType, tt.fieldPath)
			}
			if field.Source != tt.wantSource {
				t.Fatalf("source = %q, want %q", field.Source, tt.wantSource)
			}
			if field.Priority != tt.wantPriority {
				t.Fatalf("priority = %d, want %d", field.Priority, tt.wantPriority)
			}
		})
	}
}

func TestFindUnknownResourceOrFieldReturnsNil(t *testing.T) {
	auth := New()

	if got := auth.Find(sources.ResourceType("unknown"), "Name"); got != nil {
		t.Fatalf("unknown resource returned %#v, want nil", got)
	}
	if got := auth.Find(sources.ResourceTypeModel, "UnknownField"); got != nil {
		t.Fatalf("unknown field returned %#v, want nil", got)
	}
}

func TestFieldListsAreDefensiveCopies(t *testing.T) {
	auth := New()

	fields := auth.ModelFields()
	fields[0] = Field{Path: "Pricing", Source: sources.LocalCatalogID, Priority: 999}

	field := auth.Find(sources.ResourceTypeModel, "Pricing")
	if field == nil {
		t.Fatal("Find(Pricing) returned nil")
	}
	if field.Source != sources.ProvidersID {
		t.Fatalf("mutating returned field slice changed authority source to %q", field.Source)
	}
	if field.Priority != 110 {
		t.Fatalf("mutating returned field slice changed authority priority to %d", field.Priority)
	}
}

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		name      string
		fieldPath string
		pattern   string
		want      bool
	}{
		{name: "exact", fieldPath: "Pricing", pattern: "Pricing", want: true},
		{name: "prefix wildcard", fieldPath: "Catalog.Endpoint.URL", pattern: "Catalog.*", want: true},
		{name: "filepath wildcard", fieldPath: "Metadata.ReleaseDate", pattern: "Metadata.*Date", want: true},
		{name: "no match", fieldPath: "Metadata.ReleaseDate", pattern: "Pricing.*", want: false},
		{name: "invalid pattern", fieldPath: "Metadata.ReleaseDate", pattern: "[", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchesPattern(tt.fieldPath, tt.pattern); got != tt.want {
				t.Fatalf("MatchesPattern(%q, %q) = %v, want %v", tt.fieldPath, tt.pattern, got, tt.want)
			}
		})
	}
}
