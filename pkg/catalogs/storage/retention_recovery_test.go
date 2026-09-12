package storage

import (
	"bufio"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestFilesystemRetentionRecoversProcessExit(t *testing.T) {
	for _, phase := range []string{"journaled", "retired", "removed:catalog.json", "removed:manifest.json"} {
		t.Run(phase, func(t *testing.T) {
			store := retentionFixture(t)
			command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestFilesystemRetentionProcessHelper$")
			command.Env = append(os.Environ(), "STARMAP_TEST_RETENTION_ROOT="+store.Root(), "STARMAP_TEST_RETENTION_PHASE="+phase)
			output, err := command.CombinedOutput()
			var status *exec.ExitError
			if !stderrors.As(err, &status) || status.ExitCode() != 37 {
				t.Fatalf("helper failed at %s: %v %s", phase, err, output)
			}
			reopened, err := NewFilesystem(store.Root())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := reopened.Collect(t.Context(), retentionRequest()); err != nil {
				t.Fatal(err)
			}
			current, err := reopened.Current(t.Context())
			if err != nil || current.Manifest.GenerationID != "current" {
				t.Fatalf("current changed: %v", err)
			}
			if _, err := reopened.Get(t.Context(), "old"); !errors.IsNotFound(err) {
				t.Fatalf("expired content remains: %v", err)
			}
			entries, err := os.ReadDir(filepath.Join(store.Root(), "generations"))
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 2 || entries[0].Name() != privatefiles.PublicationDirectoryName || entries[1].Name() != generationDirectoryID("current") {
				t.Fatalf("retirement artifacts remain: %v", entries)
			}
		})
	}
}

func TestFilesystemRetentionProcessHelper(t *testing.T) {
	root := os.Getenv("STARMAP_TEST_RETENTION_ROOT")
	if root == "" {
		return
	}
	store, err := NewFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	phase := os.Getenv("STARMAP_TEST_RETENTION_PHASE")
	if phase == "lease" {
		_, release, err := store.AcquireGeneration(t.Context(), "old")
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println("ready")
		_, _ = os.Stdin.Read(make([]byte, 1))
		if err := release(); err != nil {
			t.Fatal(err)
		}
		return
	}
	store.beforeRetentionStep = func(got, id string) error {
		if got == phase {
			os.Exit(37)
		}
		return nil
	}
	if _, err := store.Collect(t.Context(), retentionRequest()); err != nil {
		t.Fatal(err)
	}
	t.Fatal("retirement did not reach the requested phase")
}

func TestFilesystemRetentionHonorsAnotherProcessLease(t *testing.T) {
	store := retentionFixture(t)
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestFilesystemRetentionProcessHelper$")
	command.Env = append(os.Environ(), "STARMAP_TEST_RETENTION_ROOT="+store.Root(), "STARMAP_TEST_RETENTION_PHASE=lease")
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = input.Close(); _ = command.Process.Kill() })
	line, err := bufio.NewReader(output).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "ready" {
		t.Fatalf("lease helper did not start: %q %v", line, err)
	}
	report, err := store.Collect(t.Context(), retentionRequest())
	if err != nil || !report.OverLimit || len(report.Removed) != 0 {
		t.Fatalf("live reader lost its generation: %+v %v", report, err)
	}
	if _, err := input.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatal(err)
	}
	report, err = store.Collect(t.Context(), retentionRequest())
	if err != nil || len(report.Removed) != 1 || report.Removed[0] != "old" {
		t.Fatalf("released generation remains: %+v %v", report, err)
	}
}

func TestFilesystemRetentionPreservesChangedRetirement(t *testing.T) {
	for _, change := range []string{"unknown", "replaced-file", "modified-file", "replaced-directory", "replaced-writer"} {
		t.Run(change, func(t *testing.T) {
			store := retentionFixture(t)
			stop := stderrors.New("stop after retirement")
			store.beforeRetentionStep = func(phase, id string) error {
				if phase == "retired" {
					return stop
				}
				return nil
			}
			if _, err := store.Collect(t.Context(), retentionRequest()); !stderrors.Is(err, stop) {
				t.Fatalf("did not stop: %v", err)
			}
			store.beforeRetentionStep = nil
			entries, err := os.ReadDir(filepath.Join(store.Root(), "generations"))
			if err != nil {
				t.Fatal(err)
			}
			var retired string
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), retiredPrefix) {
					retired = filepath.Join(store.Root(), "generations", entry.Name())
				}
			}
			if retired == "" {
				t.Fatal("missing retired directory")
			}
			payload := filepath.Join(retired, payloadFilename)
			original, err := os.ReadFile(payload)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "unknown":
				err = os.WriteFile(filepath.Join(retired, "note.txt"), []byte("keep"), privatefiles.FileMode)
			case "replaced-file":
				err = os.Rename(payload, payload+".original")
				if err == nil {
					err = os.WriteFile(payload, original, privatefiles.FileMode)
				}
				if err == nil {
					err = os.Rename(payload+".original", filepath.Join(store.Root(), "saved-payload"))
				}
			case "modified-file":
				err = os.WriteFile(payload, []byte("changed"), privatefiles.FileMode)
			case "replaced-directory":
				err = os.Rename(retired, retired+".saved")
				if err == nil {
					err = privatefiles.CreateDirectory(retired)
				}
			case "replaced-writer":
				path := filepath.Join(store.Root(), ".commit.lock")
				err = os.Rename(path, path+".saved")
				if err == nil {
					err = os.WriteFile(path, nil, privatefiles.FileMode)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			reopened, err := NewFilesystem(store.Root())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := reopened.Collect(t.Context(), retentionRequest()); err == nil {
				t.Fatal("changed retirement was accepted")
			}
			if _, err := os.Stat(retired); err != nil {
				t.Fatalf("changed directory disappeared: %v", err)
			}
			current, err := reopened.Current(t.Context())
			if err != nil || current.Manifest.GenerationID != "current" {
				t.Fatalf("accepted head changed: %v", err)
			}
			if change == "unknown" {
				if data, err := os.ReadFile(payload); err != nil || string(data) != string(original) {
					t.Fatalf("recognized content changed beside unknown file: %v", err)
				}
			}
		})
	}
}

func TestFilesystemRetentionRefusesBeforeDeletion(t *testing.T) {
	for _, kind := range []string{"stale", "missing-required", "scan-limit", "canceled", "dry-run", "invalid-manifest-size"} {
		t.Run(kind, func(t *testing.T) {
			store := retentionFixture(t)
			request := retentionRequest()
			ctx := t.Context()
			switch kind {
			case "stale":
				request.ExpectedGenerationID = "old"
			case "missing-required":
				request.RequiredGenerationIDs = []string{"missing"}
			case "scan-limit":
				request.ScanEntries = 1
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "dry-run":
				request.DryRun = true
			case "invalid-manifest-size":
				manifest := testGeneration("old", "old").Manifest
				manifest.Payload.SizeBytes = 1 << 40
				data, err := marshalManifest(manifest)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(store.generationDir("old"), manifestFilename), data, privatefiles.FileMode); err != nil {
					t.Fatal(err)
				}
			}
			report, err := store.Collect(ctx, request)
			if kind == "dry-run" {
				if err != nil || len(report.Removed) != 0 || report.After != report.Before {
					t.Fatalf("dry run: %+v %v", report, err)
				}
			} else if err == nil {
				t.Fatal("invalid request did not fail")
			}
			for _, id := range []string{"old", "current"} {
				if _, err := os.Stat(filepath.Join(store.generationDir(id), payloadFilename)); err != nil {
					t.Fatalf("content disappeared after %s: %v", kind, err)
				}
			}
			if _, err := os.Stat(filepath.Join(store.generationDir("old"), generationReadLock)); !os.IsNotExist(err) {
				t.Fatalf("inspection created lease state: %v", err)
			}
		})
	}
}

func TestFilesystemRetentionSerializesWithPublication(t *testing.T) {
	store := retentionFixture(t)
	publisher, err := NewFilesystem(store.Root())
	if err != nil {
		t.Fatal(err)
	}
	for index := range 12 {
		previous := "current"
		if index > 0 {
			previous = fmt.Sprintf("next-%d", index-1)
		}
		id := fmt.Sprintf("next-%d", index)
		var workers sync.WaitGroup
		workers.Go(func() {
			if err := publisher.Commit(t.Context(), testGeneration(id, id), previous); err != nil {
				t.Error(err)
			}
		})
		workers.Go(func() {
			_, err := store.Collect(t.Context(), RetentionRequest{ExpectedGenerationID: previous, MaxGenerations: 1, MaxBytes: 1 << 20})
			var conflict *errors.ConflictError
			if err != nil && !stderrors.As(err, &conflict) {
				t.Error(err)
			}
		})
		workers.Go(func() {
			if _, err := store.Current(t.Context()); err != nil {
				t.Error(err)
			}
		})
		workers.Wait()
		if got, err := store.Current(t.Context()); err != nil || got.Manifest.GenerationID != id {
			t.Fatalf("current lost after concurrent collection: %v", err)
		}
	}
}

func retentionFixture(t *testing.T) *Filesystem {
	t.Helper()
	store, err := NewFilesystem(filepath.Join(t.TempDir(), "store"))
	if err != nil {
		t.Fatal(err)
	}
	for index, generation := range []catalogs.Generation{testGeneration("old", "old"), testGeneration("current", "current")} {
		previous := ""
		if index > 0 {
			previous = "old"
		}
		if err := store.Commit(t.Context(), generation, previous); err != nil {
			t.Fatal(err)
		}
	}
	return store
}

func retentionRequest() RetentionRequest {
	return RetentionRequest{ExpectedGenerationID: "current", MaxGenerations: 1, MaxBytes: 1 << 20}
}

func TestFilesystemRetentionPendingPreparationAllowsNewLease(t *testing.T) {
	store := retentionFixture(t)
	stop := stderrors.New("stop before directory move")
	store.beforeRetentionStep = func(phase, id string) error {
		if phase == "journaled" {
			return stop
		}
		return nil
	}
	if _, err := store.Collect(t.Context(), retentionRequest()); !stderrors.Is(err, stop) {
		t.Fatalf("did not stop: %v", err)
	}
	store.beforeRetentionStep = nil
	_, release, err := store.AcquireGeneration(t.Context(), "old")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = release() })
	request := retentionRequest()
	request.RequiredGenerationIDs = []string{"old"}
	report, err := store.Collect(t.Context(), request)
	if err != nil || !report.OverLimit || len(report.Removed) != 0 {
		t.Fatalf("newly required reader lost its generation: %+v %v", report, err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	report, err = store.Collect(t.Context(), retentionRequest())
	if err != nil || len(report.Removed) != 1 {
		t.Fatalf("released generation did not retire: %+v %v", report, err)
	}
}

func TestFilesystemRetentionRejectsUnrecognizedJournal(t *testing.T) {
	for _, kind := range []string{"unknown-version", "unknown-field", "duplicate-record", "missing-payload", "trailing-data", "duplicate-version", "unclaimed-directory"} {
		t.Run(kind, func(t *testing.T) {
			store := retentionFixture(t)
			stop := stderrors.New("stop after retirement")
			store.beforeRetentionStep = func(phase, id string) error {
				if phase == "retired" {
					return stop
				}
				return nil
			}
			if _, err := store.Collect(t.Context(), retentionRequest()); !stderrors.Is(err, stop) {
				t.Fatalf("did not stop: %v", err)
			}
			store.beforeRetentionStep = nil
			parent := filepath.Join(store.Root(), "generations")
			entries, err := os.ReadDir(parent)
			if err != nil {
				t.Fatal(err)
			}
			var name string
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), retirementPrefix) {
					name = entry.Name()
				}
			}
			data, err := os.ReadFile(filepath.Join(parent, name))
			if err != nil {
				t.Fatal(err)
			}
			journal, err := decodeRetirement(name, data)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "unknown-version":
				journal.Version++
			case "duplicate-record":
				journal.Files = append(journal.Files, journal.Files[0])
			case "missing-payload":
				for i, record := range journal.Files {
					if record.Name == payloadFilename {
						journal.Files = append(journal.Files[:i], journal.Files[i+1:]...)
						break
					}
				}
			}
			if kind == "unclaimed-directory" {
				if err := os.Rename(filepath.Join(parent, name), filepath.Join(store.Root(), "saved-journal")); err != nil {
					t.Fatal(err)
				}
			} else {
				data, err = json.Marshal(journal)
				if err != nil {
					t.Fatal(err)
				}
				if kind == "unknown-field" {
					data = append([]byte(`{"unexpected":true,`), data[1:]...)
				}
				if kind == "duplicate-version" {
					data = append([]byte(`{"version":1,`), data[1:]...)
				}
				if kind == "trailing-data" {
					data = append(data, []byte(" {}")...)
				}
				if err := os.WriteFile(filepath.Join(parent, name), data, privatefiles.FileMode); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := store.Collect(t.Context(), retentionRequest()); err == nil {
				t.Fatal("unrecognized recovery input was accepted")
			}
			if _, err := os.Stat(filepath.Join(parent, journal.Retired, payloadFilename)); err != nil {
				t.Fatalf("unrecognized recovery removed payload: %v", err)
			}
		})
	}
}

func TestFilesystemRetentionIndependentLeaseLifetimes(t *testing.T) {
	store := retentionFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	_, first, err := store.AcquireGeneration(ctx, "old")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first() })
	_, second, err := store.AcquireGeneration(ctx, "old")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second() })
	cancel()
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			if err := first(); err != nil {
				t.Error(err)
			}
		})
	}
	workers.Wait()
	report, err := store.Collect(t.Context(), retentionRequest())
	if err != nil || !report.OverLimit || len(report.Removed) != 0 {
		t.Fatalf("one release ended another lease: %+v %v", report, err)
	}
	if err := second(); err != nil {
		t.Fatal(err)
	}
	report, err = store.Collect(t.Context(), retentionRequest())
	if err != nil || len(report.Removed) != 1 {
		t.Fatalf("released generation remains: %+v %v", report, err)
	}
}

func TestFilesystemRetentionSerializesOrdinaryReads(t *testing.T) {
	store := retentionFixture(t)
	reader, err := NewFilesystem(store.Root())
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	finish := func() { once.Do(func() { close(release) }) }
	t.Cleanup(finish)
	store.beforeRetentionStep = func(phase, id string) error {
		if phase == "retired" {
			close(entered)
			<-release
		}
		return nil
	}
	result := make(chan error, 1)
	go func() { _, err := store.Collect(t.Context(), retentionRequest()); result <- err }()
	<-entered
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if _, err := reader.Current(ctx); !stderrors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("ordinary read bypassed active collector: %v", err)
	}
	finish()
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if current, err := reader.Current(t.Context()); err != nil || current.Manifest.GenerationID != "current" {
		t.Fatalf("read did not resume: %v", err)
	}
}

func TestFilesystemRetentionEmptyStores(t *testing.T) {
	for _, kind := range []string{"missing-acquisition", "absent-collection", "empty-collection", "empty-generations", "missing-required", "stale-head"} {
		t.Run(kind, func(t *testing.T) {
			parent := t.TempDir()
			root := filepath.Join(parent, "store")
			if kind == "empty-collection" || kind == "empty-generations" {
				if err := privatefiles.CreateDirectory(root); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "empty-generations" {
				if err := privatefiles.CreateDirectory(filepath.Join(root, "generations")); err != nil {
					t.Fatal(err)
				}
			}
			store, err := NewFilesystem(root)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "missing-acquisition" {
				_, release, err := store.AcquireGeneration(t.Context(), "missing")
				if !errors.IsNotFound(err) || release != nil {
					t.Fatalf("missing generation error or lease: %v", err)
				}
				return
			}
			request := RetentionRequest{MaxGenerations: 1, MaxBytes: 1024}
			if kind == "missing-required" {
				request.RequiredGenerationIDs = []string{"missing"}
			}
			if kind == "stale-head" {
				request.ExpectedGenerationID = "missing"
			}
			report, err := store.Collect(t.Context(), request)
			switch kind {
			case "missing-required":
				if !errors.IsNotFound(err) {
					t.Fatalf("missing requirement error: %v", err)
				}
			case "stale-head":
				var conflict *errors.ConflictError
				if !stderrors.As(err, &conflict) {
					t.Fatalf("stale head error: %v", err)
				}
			default:
				if err != nil || report.After != (RetentionUsage{}) || len(report.Removed) != 0 {
					t.Fatalf("empty collection failed: %+v %v", report, err)
				}
			}
		})
	}
}
