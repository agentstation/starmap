package bootstrap

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

func TestBaselineExportPreservesChangedStaging(t *testing.T) {
	for _, change := range []string{"added file", "changed file", "changed timestamp", "replaced file", "replaced directory"} {
		t.Run(change, func(t *testing.T) {
			for _, abort := range []bool{false, true} {
				t.Run(map[bool]string{false: "publication", true: "cancellation"}[abort], func(t *testing.T) {
					directory := filepath.Join(t.TempDir(), "baseline")
					var sentinel string
					var expected []byte
					var original os.FileInfo
					result, err := exportBaseline(t.Context(), directory, func(point string) error {
						if point != "payload-written" {
							return nil
						}
						stage := baselineStagePath(t, directory)
						sentinel = filepath.Join(stage, baselinePayloadName)
						expected = []byte("operator content")
						switch change {
						case "added file":
							sentinel = filepath.Join(stage, "operator.txt")
						case "replaced directory":
							if err := os.Rename(stage, stage+"-preserved"); err != nil {
								t.Fatal(err)
							}
							if err := os.Mkdir(stage, baselineDirectoryMode); err != nil {
								t.Fatal(err)
							}
						case "changed timestamp":
							var err error
							expected, err = os.ReadFile(sentinel)
							if err != nil {
								t.Fatal(err)
							}
						case "replaced file":
							var err error
							expected, err = os.ReadFile(sentinel)
							if err != nil {
								t.Fatal(err)
							}
							if err := os.Rename(sentinel, filepath.Join(directory, "preserved-payload")); err != nil {
								t.Fatal(err)
							}
						}
						if err := os.WriteFile(sentinel, expected, baselineFileMode); err != nil {
							t.Fatal(err)
						}
						if change == "changed timestamp" {
							modified := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
							if err := os.Chtimes(sentinel, modified, modified); err != nil {
								t.Fatal(err)
							}
						}
						var err error
						original, err = os.Stat(sentinel)
						if err != nil {
							t.Fatal(err)
						}
						if abort {
							return context.Canceled
						}
						return nil
					})
					var conflict *errors.ConflictError
					if !stderrors.As(err, &conflict) || result.Created {
						t.Errorf("changed stage: created=%v, error=%v, want conflict before publication", result.Created, err)
					}
					if abort && !stderrors.Is(err, context.Canceled) {
						t.Errorf("cleanup lost cancellation: %v", err)
					}
					actual, err := os.ReadFile(sentinel)
					if err != nil || string(actual) != string(expected) {
						t.Fatalf("operator file changed or disappeared: %v", err)
					}
					after, err := os.Stat(sentinel)
					if err != nil || !os.SameFile(original, after) {
						t.Fatalf("operator file identity changed: %v", err)
					}
					if _, err := os.Stat(result.Directory); !os.IsNotExist(err) {
						t.Fatalf("changed staging reached publication: %v", err)
					}
					recovered, err := Export(t.Context(), directory)
					if err != nil || len(recovered.Recovery.PreservedPaths) == 0 {
						t.Fatalf("recovery did not report the changed stage: %+v, %v", recovered, err)
					}
					actual, err = os.ReadFile(sentinel)
					if err != nil || string(actual) != string(expected) {
						t.Fatalf("recovery changed operator content: %v", err)
					}
					after, err = os.Stat(sentinel)
					if err != nil || !os.SameFile(original, after) {
						t.Fatalf("recovery replaced the changed file: %v", err)
					}
				})
			}
		})
	}
}

func TestBaselineExportPreservesReusedStageNameAfterPublication(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "baseline")
	var stage string
	result, err := exportBaseline(t.Context(), directory, func(point string) error {
		switch point {
		case "created":
			stage = baselineStagePath(t, directory)
		case "promoted":
			if err := os.Mkdir(stage, baselineDirectoryMode); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(stage, "operator.txt"), []byte("preserve"), baselineFileMode)
		}
		return nil
	})
	if err != nil || !result.Created {
		t.Fatalf("export failed: %+v, %v", result, err)
	}
	actual, err := os.ReadFile(filepath.Join(stage, "operator.txt"))
	if err != nil || string(actual) != "preserve" {
		t.Fatalf("cleanup removed the reused stage name: %v", err)
	}
}

func TestBaselineExportCleansOnlyUnchangedOwnedStage(t *testing.T) {
	for _, checkpoint := range []string{"created", "manifest-written", "payload-written"} {
		t.Run(checkpoint, func(t *testing.T) {
			directory := filepath.Join(t.TempDir(), "baseline")
			_, err := exportBaseline(t.Context(), directory, func(point string) error {
				if point == checkpoint {
					return context.Canceled
				}
				return nil
			})
			if !stderrors.Is(err, context.Canceled) {
				t.Fatalf("export lost cancellation: %v", err)
			}
			assertBaselineRecoveryIdle(t, directory, 0)
		})
	}
}

func TestBaselineExportCancellationAfterStagingPreventsPublication(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "baseline")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result, err := exportBaseline(ctx, directory, func(point string) error {
		if point == "payload-written" {
			cancel()
		}
		return nil
	})
	if !stderrors.Is(err, context.Canceled) || result.Created {
		t.Errorf("canceled export reached publication: %+v, %v", result, err)
	}
	assertBaselineRecoveryIdle(t, directory, 0)
}

func baselineStagePath(t *testing.T, directory string) string {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".baseline-") {
			return filepath.Join(directory, entry.Name())
		}
	}
	t.Fatal("baseline stage is missing")
	return ""
}
