package bootstrap

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/errors"
)

func assertBaselineRecoveryIdle(t *testing.T, directory string, baselines int) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != baselines+1 {
		t.Fatalf("baseline directory retains unexpected entries: %v, %v", entries, err)
	}
	records, err := os.ReadDir(filepath.Join(directory, baselineRecoveryDirectory))
	if err != nil || len(records) != 1 || records[0].Name() != baselineRecoveryLock {
		t.Fatalf("completed recovery retains journals or staging: %v, %v", records, err)
	}
}

func recordedBaselineStage(t *testing.T, directory string) (string, string, baselineJournalRecord) {
	t.Helper()
	if err := privatefiles.CreateDirectory(directory); err != nil {
		t.Fatal(err)
	}
	parent, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = parent.Close() }()
	recovery, err := openBaselineRecovery(t.Context(), parent, directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = recovery.close() }()
	name := baselineStagePrefix + rand.Text()
	if err := privatefiles.CreateChild(parent, name); err != nil {
		t.Fatal(err)
	}
	stage, err := openBaselineStage(parent, name)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stage.close() }()
	if err := recovery.newJournal(t.Context(), stage, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{baselineManifestName, baselinePayloadName} {
		if err := stage.write(file, []byte("owned "+file)); err != nil {
			t.Fatal(err)
		}
	}
	if err := stage.journal.save(t.Context(), stage, baselineJournalWriting); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(directory, name), filepath.Join(directory, baselineRecoveryDirectory, stage.journal.name), stage.journal.record
}

func crashDuringBaselineRecovery(t *testing.T, directory, point string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^TestBaselineExportCrashChild$")
	command.Env = append(os.Environ(), "STARMAP_BASELINE_TEST_DIRECTORY="+directory, "STARMAP_BASELINE_TEST_POINT="+point)
	output, err := command.CombinedOutput()
	var exited *exec.ExitError
	if !stderrors.As(err, &exited) || exited.ExitCode() != 86 {
		t.Fatalf("recovery did not exit at %s: %v %s", point, err, output)
	}
}

func TestBaselineRecoveryResumesInterruptedCollection(t *testing.T) {
	for _, point := range []string{"recovery-collecting", "recovery-file-removed", "recovery-stage-removed"} {
		t.Run(point, func(t *testing.T) {
			directory := filepath.Join(t.TempDir(), "baseline")
			stage, journal, _ := recordedBaselineStage(t, directory)
			crashDuringBaselineRecovery(t, directory, point)
			if _, err := os.Stat(journal); err != nil {
				t.Fatalf("interrupted cleanup lost its journal: %v", err)
			}
			result, err := Export(t.Context(), directory)
			if err != nil || result.Recovery.RecoveredOperations != 1 || len(result.Recovery.PreservedPaths) != 0 {
				t.Fatalf("cleanup did not resume: %+v, %v", result, err)
			}
			if _, err := os.Stat(stage); !os.IsNotExist(err) {
				t.Fatalf("recovery retained its stage: %v", err)
			}
			assertBaselineRecoveryIdle(t, directory, 1)
		})
	}
}

func TestBaselineRecoveryPreservesUnrecognizedJournal(t *testing.T) {
	for _, change := range []string{"version", "truncated", "path", "unsealed file"} {
		t.Run(change, func(t *testing.T) {
			directory := filepath.Join(t.TempDir(), "baseline")
			stage, journal, record := recordedBaselineStage(t, directory)
			switch change {
			case "version":
				record.Version++
			case "path":
				record.Stage = "../operator-files"
			case "unsealed file":
				if err := os.WriteFile(filepath.Join(stage, "unsealed"), []byte("partial"), baselineFileMode); err != nil {
					t.Fatal(err)
				}
			}
			data, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			if change == "truncated" {
				data = data[:len(data)/2]
			}
			if err := os.WriteFile(journal, data, baselineFileMode); err != nil {
				t.Fatal(err)
			}
			result, err := Export(t.Context(), directory)
			if err != nil || result.Recovery.RecoveredOperations != 0 || len(result.Recovery.PreservedPaths) != 2 {
				t.Fatalf("unrecognized recovery state was not preserved: %+v, %v", result, err)
			}
			if !slices.Contains(result.Recovery.PreservedPaths, filepath.Base(stage)) {
				t.Fatal("preservation report omits the stage")
			}
			for _, file := range []string{baselineManifestName, baselinePayloadName} {
				actual, err := os.ReadFile(filepath.Join(stage, file))
				if err != nil || string(actual) != "owned "+file {
					t.Fatalf("unrecognized stage changed %s: %v", file, err)
				}
			}
			after, err := os.ReadFile(journal)
			if err != nil || !bytes.Equal(data, after) {
				t.Fatalf("recovery changed the unrecognized journal: %v", err)
			}
		})
	}
}

func TestBaselineRecoveryScanLimitPreservesEveryStage(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "baseline")
	stage, journal, _ := recordedBaselineStage(t, directory)
	before, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}
	for index := range baselineRecoveryEntryLimit {
		if err := os.WriteFile(filepath.Join(directory, fmt.Sprintf("unmatched-%04d", index)), nil, baselineFileMode); err != nil {
			t.Fatal(err)
		}
	}
	result, err := Export(t.Context(), directory)
	var invalid *errors.ValidationError
	if !stderrors.As(err, &invalid) || invalid.Field != "baseline.recovery.entry_limit" || result.Created {
		t.Fatalf("oversized directory bypassed the scan limit: %+v, %v", result, err)
	}
	after, err := os.ReadFile(journal)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("scan-limit refusal changed the journal: %v", err)
	}
	for _, file := range []string{baselineManifestName, baselinePayloadName} {
		actual, err := os.ReadFile(filepath.Join(stage, file))
		if err != nil || string(actual) != "owned "+file {
			t.Fatalf("scan-limit refusal deleted %s: %v", file, err)
		}
	}
}

func TestBaselineRecoveryContentBudgetPreservesStage(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "baseline")
	stage, _, record := recordedBaselineStage(t, directory)
	parent, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = parent.Close() }()
	recovery, err := openBaselineRecovery(t.Context(), parent, directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = recovery.close() }()
	remaining := int64(0)
	_, err = recovery.restoreRecordedStage(filepath.Base(stage), record, &remaining)
	var invalid *errors.ValidationError
	if !stderrors.As(err, &invalid) || invalid.Field != "baseline.recovery.byte_limit" || remaining != 0 {
		t.Fatalf("content budget did not refuse the read: remaining=%d, %v", remaining, err)
	}
	for _, file := range []string{baselineManifestName, baselinePayloadName} {
		data, err := os.ReadFile(filepath.Join(stage, file))
		if err != nil || string(data) != "owned "+file {
			t.Fatalf("content budget changed %s: %v", file, err)
		}
	}
}

func TestBaselineRecoveryPreservesStagesAfterLockReplacement(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "baseline")
	stage, journal, _ := recordedBaselineStage(t, directory)
	before, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(directory, baselineRecoveryDirectory, baselineRecoveryLock)
	if err := os.Rename(lock, filepath.Join(t.TempDir(), "preserved-lock")); err != nil {
		t.Fatal(err)
	}
	result, err := Export(t.Context(), directory)
	if err != nil || result.Recovery.RecoveredOperations != 0 || !slices.Contains(result.Recovery.PreservedPaths, filepath.Base(stage)) {
		t.Fatalf("replacement lock adopted an earlier writer's stage: %+v, %v", result, err)
	}
	after, err := os.ReadFile(journal)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("replacement lock changed the old recovery journal: %v", err)
	}
	for _, file := range []string{baselineManifestName, baselinePayloadName} {
		data, err := os.ReadFile(filepath.Join(stage, file))
		if err != nil || string(data) != "owned "+file {
			t.Fatalf("replacement lock changed %s: %v", file, err)
		}
	}
}

func TestBaselineRecoveryWaitsForLiveWriter(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "baseline")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^TestBaselineExportCrashChild$")
	command.Env = append(os.Environ(), "STARMAP_BASELINE_TEST_DIRECTORY="+directory, "STARMAP_BASELINE_TEST_POINT=held")
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	var diagnostic bytes.Buffer
	command.Stderr = &diagnostic
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	defer func() {
		_ = input.Close()
		if !waited {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	}()
	ready, err := bufio.NewReader(output).ReadString('\n')
	if err != nil || ready != "baseline-ready\n" {
		t.Fatalf("writer did not acquire its stage: %q, %v", ready, err)
	}
	stage := baselineStagePath(t, directory)
	before, err := os.Stat(stage)
	if err != nil {
		t.Fatal(err)
	}
	blocked, stop := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer stop()
	if _, err := Export(blocked, directory); !stderrors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("recovery bypassed the live writer: %v", err)
	}
	after, err := os.Stat(stage)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("waiting recovery changed the live stage: %v", err)
	}
	if _, err := input.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	_ = input.Close()
	err = command.Wait()
	waited = true
	if err != nil {
		t.Fatalf("active writer failed after release: %v, %s", err, diagnostic.String())
	}
	assertBaselineRecoveryIdle(t, directory, 1)
}
