package differ

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestModelDiffReportsPresenceTransitions(t *testing.T) {
	existing := &catalogs.Model{
		ID:       "presence",
		Name:     "Presence",
		Metadata: &catalogs.ModelMetadata{},
		Features: &catalogs.ModelFeatures{},
		Limits:   &catalogs.ModelLimits{},
	}
	updated := catalogs.DeepCopyModel(*existing)
	updated.SetDescription("")
	updated.Metadata.SetOpenWeights(false)
	updated.Features.SetSupport(catalogs.ModelFeatureTools, false)
	updated.Limits.Set(catalogs.ModelLimitContextWindow, 0)

	changes := New().Models([]*catalogs.Model{existing}, []*catalogs.Model{&updated})
	if len(changes.Updated) != 1 {
		t.Fatalf("updated = %d, want 1", len(changes.Updated))
	}
	got := make(map[string]FieldChange, len(changes.Updated[0].Changes))
	for _, change := range changes.Updated[0].Changes {
		got[change.Path] = change
	}
	for _, path := range []string{
		"description",
		"metadata.open_weights",
		"features.tools",
		"limits.context_window",
	} {
		change, found := got[path]
		if !found {
			t.Fatalf("missing %s change: %#v", path, changes.Updated[0].Changes)
		}
		if change.OldValue != "missing" {
			t.Fatalf("%s old value = %q, want missing", path, change.OldValue)
		}
	}
}
