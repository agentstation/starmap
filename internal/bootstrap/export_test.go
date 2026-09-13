package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	stderrors "errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

func TestBaselineExportPreservesDirectoryCreatedDuringStaging(t *testing.T) {
	generation, err := Generation()
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(generation.Manifest.GenerationID))
	directory := filepath.Join(t.TempDir(), "baseline")
	target := filepath.Join(directory, hex.EncodeToString(digest[:]))
	var existing os.FileInfo
	result, err := exportBaseline(t.Context(), directory, func(point string) error {
		if point != "payload-written" {
			return nil
		}
		if err := os.Mkdir(target, 0o700); err != nil {
			return err
		}
		var err error
		existing, err = os.Stat(target)
		return err
	})
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) || result.Created {
		t.Fatalf("export did not preserve a conflicting directory: created=%v, error=%v", result.Created, err)
	}
	after, err := os.Stat(target)
	if err != nil || !os.SameFile(existing, after) {
		t.Fatal("concurrent directory identity changed")
	}
	entries, err := os.ReadDir(target)
	if err != nil || len(entries) != 0 {
		t.Fatal("export changed the concurrent directory")
	}
}

func TestBaselineExportIsIdempotentAndPreservesConflicts(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "baseline")
	first, err := Export(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Created {
		t.Fatal("first export did not report creation")
	}
	second, err := Export(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}
	if second.Created || second.Directory != first.Directory || second.GenerationID != first.GenerationID {
		t.Fatal("repeated export changed identity")
	}
	poisoned := filepath.Join(first.Directory, baselinePayloadName)
	if err := os.WriteFile(poisoned, []byte("operator-content-must-remain"), baselineFileMode); err != nil {
		t.Fatal(err)
	}
	if _, err := Export(t.Context(), directory); err == nil {
		t.Fatal("corrupt existing export was accepted")
	}
	retained, err := os.ReadFile(poisoned)
	if err != nil {
		t.Fatal(err)
	}
	if string(retained) != "operator-content-must-remain" {
		t.Fatal("conflicting existing content was overwritten")
	}
}

func TestConcurrentBaselineExportPublishesOneCompleteGeneration(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "baseline")
	failures := make(chan error, 4)
	created := make(chan bool, 4)
	var work sync.WaitGroup
	for range 4 {
		work.Go(func() { result, err := Export(t.Context(), directory); failures <- err; created <- result.Created })
	}
	work.Wait()
	close(failures)
	close(created)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	count := 0
	for value := range created {
		if value {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("published %d baseline directories, want one", count)
	}
	assertBaselineRecoveryIdle(t, directory, 1)
	if _, err := Export(t.Context(), directory); err != nil {
		t.Fatal(err)
	}
}

func TestBaselineExportRejectsSymlinksAndCancellation(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "baseline")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Export(ctx, directory); err == nil {
		t.Fatal("canceled export succeeded")
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatal("canceled export created storage")
	}
	result, err := Export(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(result.Directory, baselineManifestName)
	if err := os.Remove(manifest); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "sentinel")
	if err := os.WriteFile(target, []byte("unchanged"), baselineFileMode); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, manifest); err != nil {
		t.Fatal(err)
	}
	if _, err := Export(t.Context(), directory); err == nil {
		t.Fatal("symlinked export was accepted")
	}
	bytes, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(bytes) != "unchanged" {
		t.Fatal("export changed a symlink target")
	}
}

func TestBaselineExportRecoversAfterProcessInterruption(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, point := range []string{"created", "manifest-written", "payload-written", "promoted"} {
		t.Run(point, func(t *testing.T) {
			directory := filepath.Join(t.TempDir(), "baseline")
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			command := exec.CommandContext(ctx, executable, "-test.run=^TestBaselineExportCrashChild$")
			command.Env = append(os.Environ(), "STARMAP_BASELINE_TEST_DIRECTORY="+directory, "STARMAP_BASELINE_TEST_POINT="+point)
			output, err := command.CombinedOutput()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 86 {
				t.Fatalf("child did not exit at its checkpoint: %v %s", err, output)
			}
			before, err := os.ReadDir(directory)
			if err != nil {
				t.Fatal(err)
			}
			result, err := Export(t.Context(), directory)
			if err != nil {
				t.Fatal(err)
			}
			if result.Created == (point == "promoted") {
				t.Fatal("recovery did not distinguish staged and published state")
			}
			// Recovery collects the interrupted writer's recorded stage.
			for _, entry := range before {
				if strings.HasPrefix(entry.Name(), ".baseline-") {
					if _, err := os.Stat(filepath.Join(directory, entry.Name())); !os.IsNotExist(err) {
						t.Fatal("recovery retained the exited writer's staging directory")
					}
				}
			}
		})
	}
}

func TestBaselineRecoveryPreservesUnrecordedStage(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "baseline")
	stage := filepath.Join(directory, ".baseline-unknown")
	if err := os.MkdirAll(stage, baselineDirectoryMode); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(stage, "operator.txt")
	if err := os.WriteFile(file, []byte("preserve"), baselineFileMode); err != nil {
		t.Fatal(err)
	}
	if _, err := Export(t.Context(), directory); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil || string(data) != "preserve" {
		t.Fatalf("recovery changed unrecorded staging: %v", err)
	}
}

func TestBaselineExportCrashChild(t *testing.T) {
	directory := os.Getenv("STARMAP_BASELINE_TEST_DIRECTORY")
	if directory == "" {
		return
	}
	point := os.Getenv("STARMAP_BASELINE_TEST_POINT")
	_, err := exportBaseline(t.Context(), directory, func(at string) error {
		if point == "held" && at == "created" {
			if _, err := io.WriteString(os.Stdout, "baseline-ready\n"); err != nil {
				return err
			}
			var release [1]byte
			_, err := io.ReadFull(os.Stdin, release[:])
			return err
		}
		if at == point {
			os.Exit(86)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if point == "held" {
		return
	}
	t.Fatal("child missed its interruption checkpoint")
}
