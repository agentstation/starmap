package workspace

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/constants"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestValidateHumanLayoutDetectsEveryLegacyGenerationEntry(t *testing.T) {
	t.Parallel()

	for _, entry := range legacyGenerationEntries {
		t.Run(entry, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "catalog")
			if err := os.MkdirAll(path, constants.DirPermissions); err != nil {
				t.Fatalf("MkdirAll: %v", err)
			}
			entryPath := filepath.Join(path, entry)
			if entry == "generations" {
				if err := os.Mkdir(entryPath, constants.DirPermissions); err != nil {
					t.Fatalf("Mkdir generations: %v", err)
				}
			} else if err := os.WriteFile(entryPath, []byte("legacy\n"), constants.FilePermissions); err != nil {
				t.Fatalf("WriteFile %s: %v", entry, err)
			}

			err := ValidateHumanLayout(path, "/machine/state")
			var layoutErr *pkgerrors.LegacyCatalogLayoutError
			if !stderrors.As(err, &layoutErr) {
				t.Fatalf("error = %T %v, want *errors.LegacyCatalogLayoutError", err, err)
			}
			if layoutErr.Path != path || layoutErr.MigrationTarget != "/machine/state" {
				t.Fatalf("layout error = %#v", layoutErr)
			}
			if len(layoutErr.Entries) != 1 || layoutErr.Entries[0] != entry {
				t.Fatalf("entries = %v, want [%s]", layoutErr.Entries, entry)
			}
		})
	}
}

func TestProjectRejectsLegacyGenerationLayoutBeforeStaging(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	path := filepath.Join(parent, "catalog")
	if err := os.MkdirAll(filepath.Join(path, "generations"), constants.DirPermissions); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	current := filepath.Join(path, "current")
	if err := os.WriteFile(current, []byte("legacy-generation\n"), constants.FilePermissions); err != nil {
		t.Fatalf("Write current: %v", err)
	}
	catalog, identity := testCatalog(t, "candidate", "Candidate")

	_, err := Project(context.Background(), path, catalog, identity)
	var layoutErr *pkgerrors.LegacyCatalogLayoutError
	if !stderrors.As(err, &layoutErr) {
		t.Fatalf("Project error = %T %v, want *errors.LegacyCatalogLayoutError", err, err)
	}
	data, readErr := os.ReadFile(current)
	if readErr != nil || string(data) != "legacy-generation\n" {
		t.Fatalf("legacy current = %q, %v", data, readErr)
	}
	matches, globErr := filepath.Glob(filepath.Join(parent, ".catalog.candidate-*"))
	if globErr != nil || len(matches) != 0 {
		t.Fatalf("staging paths = %v, %v; want none", matches, globErr)
	}
}

func TestValidateHumanLayoutAcceptsMissingAndProviderYAMLPaths(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "missing")
	if err := ValidateHumanLayout(missing, ""); err != nil {
		t.Fatalf("missing path: %v", err)
	}
	path := filepath.Join(t.TempDir(), "catalog")
	if err := os.MkdirAll(path, constants.DirPermissions); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "providers.yaml"), []byte("providers: []\n"), constants.FilePermissions); err != nil {
		t.Fatalf("Write providers: %v", err)
	}
	if err := ValidateHumanLayout(path, ""); err != nil {
		t.Fatalf("provider YAML workspace: %v", err)
	}
}
