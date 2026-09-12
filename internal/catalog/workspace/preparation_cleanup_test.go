package workspace

import (
	"bytes"
	"context"
	stderrors "errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/errors"
)

func TestPreparationCleanupPreservesOperatorChanges(t *testing.T) {
	for _, change := range []string{"unknown-child", "changed-render", "replaced-render-file", "replaced-enclosure"} {
		t.Run(change, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "workspace")
			catalog, identity := testCatalog(t, "new", "New")
			fault := stderrors.New("stop after operator change")
			var preserved string
			var want []byte
			_, err := (projector{afterStageRender: func(render string) error {
				enclosure := filepath.Dir(render)
				preserved = filepath.Join(render, "providers.yaml")
				var err error
				want, err = os.ReadFile(preserved)
				if err != nil {
					return err
				}
				switch change {
				case "unknown-child":
					preserved = filepath.Join(enclosure, "operator-note")
					want = []byte("preserve this note")
				case "changed-render":
					want = []byte("operator change")
				case "replaced-render-file":
					if err := os.Rename(preserved, filepath.Join(filepath.Dir(enclosure), "original-provider-file")); err != nil {
						return err
					}
				case "replaced-enclosure":
					if err := os.Rename(enclosure, enclosure+".moved"); err != nil {
						return err
					}
					if err := os.Mkdir(enclosure, directoryMode); err != nil {
						return err
					}
					preserved = filepath.Join(enclosure, "operator-note")
					want = []byte("preserve this replacement")
				}
				if err := os.WriteFile(preserved, want, fileMode); err != nil {
					return err
				}
				return fault
			}}).project(t.Context(), path, catalog, identity, InputExpectation{})
			if !stderrors.Is(err, fault) {
				t.Fatalf("projection error: %v", err)
			}
			data, err := os.ReadFile(preserved)
			if err != nil || !bytes.Equal(data, want) {
				t.Fatalf("preparation removed operator content: %q, %v", data, err)
			}
			if _, err := os.Lstat(path); !os.IsNotExist(err) {
				t.Fatalf("failed preparation published a workspace: %v", err)
			}
		})
	}
}

func TestPreparationOwnsPartialWrites(t *testing.T) {
	for _, changed := range []bool{false, true} {
		name := "unchanged"
		if changed {
			name = "operator-edit"
		}
		t.Run(name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "workspace")
			stage, err := prepareWorkspaceStage(t.Context(), target, preparationTestWriter(t, target))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			err = stage.trees["render"].writeFrom(ctx, "nested/partial", strings.NewReader("partial"), 100, "not-reached", nil)
			if !stderrors.Is(err, io.EOF) {
				t.Fatalf("partial copy: %v", err)
			}
			preserved := filepath.Join(stage.renderPath(), "nested", "partial")
			if changed {
				if err := os.WriteFile(preserved, []byte("operator bytes"), fileMode); err != nil {
					t.Fatal(err)
				}
			}
			cancel()
			err = stage.close(ctx)
			if changed {
				var conflict *errors.ConflictError
				if !stderrors.As(err, &conflict) {
					t.Fatalf("changed partial cleanup: %v", err)
				}
				data, err := os.ReadFile(preserved)
				if err != nil || string(data) != "operator bytes" {
					t.Fatalf("lost partial edit: %q, %v", data, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				assertNoProjectionStaging(t, target)
			}
		})
	}
}

func TestVerificationCleanupPreservesOperatorChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	catalog, identity := testCatalog(t, "new", "New")
	fault := stderrors.New("stop verification")
	var preserved string
	_, err := (projector{afterVerificationRender: func(verification string) error {
		preserved = filepath.Join(verification, "operator-note")
		if err := os.WriteFile(preserved, []byte("operator bytes"), fileMode); err != nil {
			return err
		}
		return fault
	}}).project(t.Context(), path, catalog, identity, InputExpectation{})
	if !stderrors.Is(err, fault) {
		t.Fatalf("verification error: %v", err)
	}
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("verification cleanup did not report its conflict: %v", err)
	}
	data, err := os.ReadFile(preserved)
	if err != nil || string(data) != "operator bytes" {
		t.Fatalf("lost verification edit: %q, %v", data, err)
	}
}

func TestAssemblyBoundsChangedRenderedDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	catalog, identity := testCatalog(t, "new", "New")
	var render string
	copiedUnexpected := false
	_, err := (projector{
		afterStageRender: func(value string) error { render = value; return nil },
		beforeAccessRestore: func(name string) error {
			if name == "." {
				return os.WriteFile(filepath.Join(render, "unexpected"), []byte("operator bytes"), fileMode)
			}
			if name == "unexpected" {
				copiedUnexpected = true
			}
			return nil
		},
	}).project(t.Context(), path, catalog, identity, InputExpectation{})
	var limit *errors.ValidationError
	if !stderrors.As(err, &limit) || copiedUnexpected {
		t.Fatalf("assembly did not refuse the expanded directory before copying: copied=%v, %v", copiedUnexpected, err)
	}
	data, readErr := os.ReadFile(filepath.Join(render, "unexpected"))
	if readErr != nil || string(data) != "operator bytes" {
		t.Fatalf("lost expanded-directory content: %q, %v", data, readErr)
	}
}

func TestPreparationLimitRefusesCreation(t *testing.T) {
	target := filepath.Join(t.TempDir(), "workspace")
	stage, err := prepareWorkspaceStage(t.Context(), target, preparationTestWriter(t, target))
	if err != nil {
		t.Fatal(err)
	}
	tree := stage.trees["render"]
	tree.nameBytes = replacementMaxNameBytes
	err = tree.writeFile(t.Context(), "unowned", []byte("bytes"))
	var limit *errors.ValidationError
	if !stderrors.As(err, &limit) {
		t.Fatalf("name limit: %v", err)
	}
	if _, err := tree.root.Lstat("unowned"); !os.IsNotExist(err) {
		t.Fatalf("limit created an unrecorded file: %v", err)
	}
	if err := stage.close(t.Context()); err != nil {
		t.Fatal(err)
	}
	assertNoProjectionStaging(t, target)
}
