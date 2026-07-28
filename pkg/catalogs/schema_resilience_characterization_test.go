package catalogs

import (
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/agentstation/starmap/pkg/constants"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

// TestF009CharacterizationMalformedLocalSiblingAbortsCatalogWalk pins the
// local YAML walk boundary. P4.8 must quarantine the bad record and retain the
// valid sibling while preserving a fail-closed policy for invalid catalog
// structure.
func TestF009CharacterizationMalformedLocalSiblingAbortsCatalogWalk(t *testing.T) {
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
	if err == nil {
		t.Fatal("F-009 characterization changed: malformed local sibling did not abort catalog load")
	}
	if catalog != nil {
		t.Fatalf("F-009 characterization changed: partial local catalog escaped: %#v", catalog)
	}
	var parseErr *pkgerrors.ParseError
	if !stderrors.As(err, &parseErr) {
		t.Fatalf("error = %T: %v, want *errors.ParseError", err, err)
	}
}

// TestF009CharacterizationInvalidLocalWorkspaceFailsClosed pins the selected
// safety expectation: a configured, present, invalid workspace is not treated
// as optional absence. P4.8 may quarantine invalid records, but structural YAML
// failure must still return a typed error and no catalog.
func TestF009CharacterizationInvalidLocalWorkspaceFailsClosed(t *testing.T) {
	path := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(path, "providers.yaml"),
		[]byte("- id: provider\n  name: [unterminated\n"),
		constants.FilePermissions,
	); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	catalog, err := NewLocal(path)
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
