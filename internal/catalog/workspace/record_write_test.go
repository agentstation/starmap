package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/errors"
)

func TestWorkspaceRecordCleanupPreservesOperatorFiles(t *testing.T) {
	for _, flow := range []string{"projection", "journal", "journal-marker"} {
		for _, change := range []string{"changed-bytes", "changed-access", "replaced-identical", "recreated-after-publication", "replaced-after-partial-write"} {
			t.Run(flow+"/"+change, func(t *testing.T) {
				root := t.TempDir()
				target := filepath.Join(root, "workspace")
				if flow != "projection" {
					catalog, identity := testCatalog(t, "old", "Old")
					if _, err := Project(t.Context(), target, catalog, identity); err != nil {
						t.Fatal(err)
					}
				}
				fault := stderrors.New("stop record publication")
				var preserved string
				var want []byte
				var identity os.FileInfo
				selected := func(name string) bool {
					if flow == "journal" {
						return strings.Contains(filepath.Base(name), ".starmap-replacement.json.")
					}
					return strings.Contains(filepath.Base(name), ".starmap-projection.json.")
				}
				edit := func(name string) error {
					if !selected(name) {
						return nil
					}
					preserved = name
					want = []byte("operator content must survive\n")
					if change == "replaced-identical" || change == "changed-access" {
						var err error
						want, err = os.ReadFile(name)
						if err != nil {
							return err
						}
					}
					if change != "changed-bytes" && change != "changed-access" {
						if err := os.Rename(name, filepath.Join(root, "original-record")); err != nil && !os.IsNotExist(err) {
							return err
						}
					}
					if err := os.WriteFile(name, want, fileMode); err != nil {
						return err
					}
					if change == "changed-access" {
						if err := os.Chmod(name, 0o444); err != nil {
							return err
						}
						t.Cleanup(func() { _ = os.Chmod(name, fileMode) })
					}
					var err error
					identity, err = os.Stat(name)
					if err != nil {
						return err
					}
					return fault
				}
				hooks := workspaceRecordWriter{}
				switch change {
				case "recreated-after-publication":
					hooks.afterPublish = edit
				case "replaced-after-partial-write":
					hooks.writeBytes = func(file *os.File, data []byte) (int, error) {
						if !selected(file.Name()) {
							return file.Write(data)
						}
						n, err := file.Write(data[:3])
						if err != nil {
							return n, err
						}
						return n, edit(file.Name())
					}
				default:
					hooks.beforePublish = edit
				}
				catalog, generation := testCatalog(t, "new", "New")
				_, err := (projector{journalReplacement: flow != "projection", recordWrites: hooks}).
					project(t.Context(), target, catalog, generation, InputExpectation{})
				if !stderrors.Is(err, fault) || preserved == "" || identity == nil {
					t.Fatalf("fault did not reach the selected record: %v", err)
				}
				got, readErr := os.ReadFile(preserved)
				if readErr != nil || !bytes.Equal(got, want) {
					t.Fatalf("cleanup removed operator content at %s: %v", preserved, readErr)
				}
				after, err := os.Stat(preserved)
				if err != nil || !os.SameFile(identity, after) {
					t.Fatalf("cleanup replaced the operator file: %v", err)
				}
			})
		}
	}
}

func TestWorkspaceRecordPublicationGuards(t *testing.T) {
	for _, change := range []string{"candidate", "destination", "cancellation", "partial-write", "short-write"} {
		t.Run(change, func(t *testing.T) {
			path := t.TempDir()
			root, err := os.OpenRoot(path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = root.Close() }()
			const destination = "marker.json"
			original := []byte("previous marker")
			if err := os.WriteFile(filepath.Join(path, destination), original, fileMode); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			operator := []byte("operator edit")
			var stage string
			fault := stderrors.New("partial record write")
			hooks := workspaceRecordWriter{beforePublish: func(name string) error {
				stage = name
				switch change {
				case "candidate":
					return os.WriteFile(name, operator, fileMode)
				case "destination":
					return os.WriteFile(filepath.Join(path, destination), operator, fileMode)
				case "cancellation":
					cancel()
				}
				return nil
			}}
			if change == "partial-write" || change == "short-write" {
				hooks.writeBytes = func(file *os.File, data []byte) (int, error) {
					stage = file.Name()
					n, err := file.Write(data[:3])
					if err != nil {
						return n, err
					}
					if change == "short-write" {
						return n, nil
					}
					return n, fault
				}
			}
			published, err := hooks.publish(ctx, root, destination, []byte("new marker content"), recordPublication{replace: true})
			if err == nil || published.identity != "" || stage == "" {
				t.Fatalf("guard did not stop publication: visible=%v, stage=%q, error=%v", published, stage, err)
			}
			if change == "cancellation" && !stderrors.Is(err, context.Canceled) {
				t.Fatalf("lost cancellation: %v", err)
			}
			if change == "partial-write" && !stderrors.Is(err, fault) {
				t.Fatalf("lost write error: %v", err)
			}
			if change == "short-write" && !stderrors.Is(err, io.ErrShortWrite) {
				t.Fatalf("lost short write: %v", err)
			}
			if change == "candidate" {
				assertWorkspaceRecordBytes(t, stage, operator)
			} else if _, err := os.Stat(stage); !os.IsNotExist(err) {
				t.Fatalf("unchanged temporary file remains: %v", err)
			}
			want := original
			if change == "destination" {
				want = operator
			}
			assertWorkspaceRecordBytes(t, filepath.Join(path, destination), want)
		})
	}
}

func TestWorkspaceRecordPublicationAndExactLimit(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(map[bool]string{false: "journal", true: "marker"}[replace], func(t *testing.T) {
			path := t.TempDir()
			root, err := os.OpenRoot(path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = root.Close() }()
			data := bytes.Repeat([]byte("x"), replacementJournalMax)
			options := recordPublication{replace: replace}
			published, err := (workspaceRecordWriter{}).publish(t.Context(), root, "record.json", data, options)
			if published.identity == "" || err != nil {
				t.Fatalf("publish at the record limit: visible=%v, error=%v", published, err)
			}
			assertWorkspaceRecordBytes(t, filepath.Join(path, "record.json"), data)
			published, err = (workspaceRecordWriter{}).publish(t.Context(), root, "record.json", []byte("replacement"), options)
			if replace {
				if published.identity == "" || err != nil {
					t.Fatalf("replace marker: visible=%v, error=%v", published, err)
				}
				data = []byte("replacement")
			} else {
				var conflict *errors.ConflictError
				if published.identity != "" || !stderrors.As(err, &conflict) {
					t.Fatalf("journal collision: visible=%v, error=%v", published, err)
				}
			}
			published, err = (workspaceRecordWriter{}).publish(t.Context(), root, "too-large.json", make([]byte, replacementJournalMax+1), options)
			var validation *errors.ValidationError
			if published.identity != "" || !stderrors.As(err, &validation) {
				t.Fatalf("oversized publication: visible=%v, error=%v", published, err)
			}
			entries, err := os.ReadDir(path)
			if err != nil || len(entries) != 1 || entries[0].Name() != "record.json" {
				t.Fatalf("record publication left temporary files: %v", err)
			}
			assertWorkspaceRecordBytes(t, filepath.Join(path, "record.json"), data)
		})
	}
}

func TestProjectionMarkerReadRejectsOversize(t *testing.T) {
	target := filepath.Join(t.TempDir(), "workspace")
	marker := projectionMarker{Version: markerVersion, GenerationID: "generation", PayloadChecksum: "payload", WorkspaceChecksum: "workspace", EndpointChecksum: "endpoints"}
	encoded, err := json.Marshal(marker)
	if err != nil {
		t.Fatal(err)
	}
	data := bytes.Repeat([]byte(" "), replacementJournalMax+1)
	copy(data, encoded)
	path := projectionMarkerPath(target)
	if err := os.WriteFile(path, data, fileMode); err != nil {
		t.Fatal(err)
	}
	if _, err := readProjectionMarker(target); err == nil {
		t.Fatal("reader accepted an oversized valid marker")
	}
	assertWorkspaceRecordBytes(t, path, data)
}

func assertWorkspaceRecordBytes(t *testing.T, name string, want []byte) {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil || !bytes.Equal(data, want) {
		t.Fatalf("record content changed at %s: %v", name, err)
	}
}
