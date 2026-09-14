package workspace

import (
	"bytes"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/errors"
)

func TestProjectionCleanupPreservesChangedStaging(t *testing.T) {
	for _, phase := range []string{"candidate", "exchanged-backup"} {
		for _, change := range []string{"unknown-file", "changed-file", "replaced-file", "replaced-root"} {
			t.Run(phase+"/"+change, func(t *testing.T) {
				if phase == "exchanged-backup" && journalWorkspaceReplacement {
					t.Skip("native exchange is unavailable; journal recovery has separate tests")
				}
				path := filepath.Join(t.TempDir(), "workspace")
				old, oldIdentity := testCatalog(t, "old", "Old")
				if _, err := Project(t.Context(), path, old, oldIdentity); err != nil {
					t.Fatal(err)
				}
				fault := stderrors.New("stop after operator edit")
				var preserved string
				var want []byte
				edit := func() error {
					matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".workspace.candidate-*"))
					if err != nil || len(matches) != 1 {
						t.Fatalf("candidate: %v, %v", matches, err)
					}
					stage := matches[0]
					preserved = filepath.Join(stage, "providers.yaml")
					want, err = os.ReadFile(preserved)
					if err != nil {
						return err
					}
					switch change {
					case "unknown-file":
						preserved = filepath.Join(stage, "operator-note")
						want = []byte("keep this note")
					case "changed-file":
						want = []byte("operator edit")
					case "replaced-file":
						if err := os.Rename(preserved, filepath.Join(filepath.Dir(path), "original-provider-file")); err != nil {
							return err
						}
					case "replaced-root":
						if err := os.Rename(stage, stage+".moved"); err != nil {
							return err
						}
						if err := os.Mkdir(stage, directoryMode); err != nil {
							return err
						}
					}
					if err := os.WriteFile(preserved, want, fileMode); err != nil {
						return err
					}
					return fault
				}
				p := projector{beforePromote: edit}
				if phase == "exchanged-backup" {
					p = projector{beforeMarker: edit}
				}
				next, identity := testCatalog(t, "new", "New")
				receipt, err := p.project(t.Context(), path, next, identity, InputExpectation{})
				if !stderrors.Is(err, fault) {
					t.Fatalf("projection error: %v", err)
				}
				actual, readErr := os.ReadFile(preserved)
				if readErr != nil || !bytes.Equal(actual, want) {
					t.Fatalf("cleanup removed operator content: %q, %v", actual, readErr)
				}
				var conflict *errors.ConflictError
				if !stderrors.As(err, &conflict) {
					t.Fatalf("cleanup did not report its conflict: %v", err)
				}
				if phase == "candidate" {
					assertWorkspaceModel(t, path, "old", "Old")
				} else {
					if receipt.GenerationID != identity.GenerationID {
						t.Fatalf("cleanup lost the visible publication receipt: %+v", receipt)
					}
					assertWorkspaceModel(t, path, "new", "New")
				}
			})
		}
	}
}

func TestRepairPreservesInputChangedAfterRendering(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "missing"
		if existing {
			name = "prior-projection"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "workspace")
			if existing {
				old, oldIdentity := testCatalog(t, "old", "Old")
				if _, err := Project(t.Context(), path, old, oldIdentity); err != nil {
					t.Fatal(err)
				}
			}
			preserved := filepath.Join(path, "operator-note")
			p := projector{afterStageRender: func(string) error {
				if !existing {
					if err := os.Mkdir(path, directoryMode); err != nil {
						return err
					}
				}
				return os.WriteFile(preserved, []byte("keep this note"), fileMode)
			}}
			catalog, identity := testCatalog(t, "new", "New")
			_, err := p.repair(t.Context(), path, catalog, identity)
			var conflict *errors.ConflictError
			if !stderrors.As(err, &conflict) {
				t.Fatalf("repair accepted changed input: %v", err)
			}
			data, err := os.ReadFile(preserved)
			if err != nil || string(data) != "keep this note" {
				t.Fatalf("repair removed operator content: %q, %v", data, err)
			}
			if existing {
				assertWorkspaceModel(t, path, "old", "Old")
			}
			assertNoProjectionStaging(t, path)
		})
	}
}

func TestRepairBuildsSingleCandidate(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "missing"
		if existing {
			name = "prior-projection"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "workspace")
			if existing {
				old, oldIdentity := testCatalog(t, "old", "Old")
				if _, err := Project(t.Context(), path, old, oldIdentity); err != nil {
					t.Fatal(err)
				}
			}
			renders := 0
			p := projector{afterStageRender: func(string) error { renders++; return nil }}
			catalog, identity := testCatalog(t, "new", "New")
			result, err := p.repair(t.Context(), path, catalog, identity)
			if err != nil || result.Status != RepairStatusRepaired {
				t.Fatalf("repair: %+v, %v", result, err)
			}
			if renders != 1 {
				t.Fatalf("repair rendered %d candidates, want one validated candidate", renders)
			}
			assertWorkspaceModel(t, path, "new", "New")
			assertNoProjectionStaging(t, path)
		})
	}
}
