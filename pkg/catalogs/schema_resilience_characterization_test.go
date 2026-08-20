package catalogs

import (
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/agentstation/starmap/pkg/catalogs/internal/resourcepolicy"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

// TestF009MalformedLocalSiblingIsQuarantined proves the local YAML walk retains
// valid model files while reporting a malformed sibling.
func TestF009MalformedLocalSiblingIsQuarantined(t *testing.T) {
	catalogFS := fstest.MapFS{
		"providers.yaml": {
			Data: []byte("- id: provider\n  name: Provider\n"),
		},
		"providers/provider/models/a-valid.yaml": {
			Data: []byte("id: valid\nname: Valid\n"),
		},
		"providers/provider/models/z-invalid.yaml": {
			Data: []byte("id: invalid\nname: [unterminated\n"),
		},
	}

	catalog, err := New(WithFS(catalogFS))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	models, err := catalog.ProviderModels("provider")
	if err != nil {
		t.Fatalf("ProviderModels: %v", err)
	}
	if model, found := models.Get("valid"); !found || model.Name != "Valid" {
		t.Fatalf("valid model = %#v, found %v", model, found)
	}
	report := catalog.LoadReport()
	if report.Accepted != 1 || report.Rejected != 1 || len(report.Issues) != 1 {
		t.Fatalf("load report = %#v, want accepted 1 rejected 1", report)
	}
	var parseErr *pkgerrors.ParseError
	if !stderrors.As(report.Issues[0].Err, &parseErr) {
		t.Fatalf("issue = %T: %v, want *errors.ParseError", report.Issues[0].Err, report.Issues[0].Err)
	}
}

// TestF009InvalidLocalWorkspaceFailsClosed proves a configured, present,
// structurally invalid workspace is not treated as optional absence.
func TestF009InvalidLocalWorkspaceFailsClosed(t *testing.T) {
	path := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(path, "providers.yaml"),
		[]byte("- id: provider\n  name: [unterminated\n"),
		resourcepolicy.FileMode,
	); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	catalog, err := NewFromPath(path)
	if err == nil {
		t.Fatal("invalid configured workspace was treated as optional absence")
	}
	if catalog != nil {
		t.Fatalf("invalid configured workspace returned partial catalog: %#v", catalog)
	}
	var parseErr *pkgerrors.ParseError
	if !stderrors.As(err, &parseErr) {
		t.Fatalf("error = %T: %v, want *errors.ParseError", err, err)
	}
}

func TestLocalModelWalkStopsAtRecordBudget(t *testing.T) {
	builder := NewEmpty()
	builder.loadReport.Accepted = resourcepolicy.MaxModels
	if !builder.modelLoadLimitReached("providers/provider/models/excess.yaml") {
		t.Fatal("model load did not stop at the record budget")
	}
	report := builder.LoadReport()
	if !report.Truncated || report.Rejected != 1 ||
		len(report.Issues) != 1 || !report.Issues[0].Limit {
		t.Fatalf("report = %#v, want one bounded truncation issue", report)
	}
}
