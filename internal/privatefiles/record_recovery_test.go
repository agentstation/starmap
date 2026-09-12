package privatefiles

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrivateRecordRecoveryAfterProcessExit(t *testing.T) {
	if path := os.Getenv("STARMAP_TEST_PRIVATE_RECORD_EXIT"); path != "" {
		directory, err := ExistingDirectory(path)
		if err != nil {
			t.Fatal(err)
		}
		phase := os.Getenv("STARMAP_TEST_PRIVATE_RECORD_PHASE")

		if strings.HasPrefix(phase, "receipt-") {
			writer, err := directory.acquirePublicationWriter(t.Context(), true)
			if err != nil {
				t.Fatal(err)
			}
			journal, err := writer.newJournal("source.json", ".layer-")
			if err != nil {
				t.Fatal(err)
			}
			if phase == "receipt-header" {
				os.Exit(88)
			}
			file, err := CreateFile(writer.root, journal.header.Stage)
			if err != nil {
				t.Fatal(err)
			}
			if phase == "receipt-unowned" {
				os.Exit(88)
			}
			empty, err := publicationRecordOf(writer.root, journal.header.Stage, 0)
			if err != nil {
				t.Fatal(err)
			}
			if err := journal.append(writer, publicationEvent{Record: &empty}); err != nil {
				t.Fatal(err)
			}
			if phase == "receipt-empty" {
				os.Exit(88)
			}
			if _, err := file.Write([]byte("part")); err != nil {
				t.Fatal(err)
			}
			if err := file.Sync(); err != nil {
				t.Fatal(err)
			}
			if phase == "receipt-incomplete" {
				os.Exit(88)
			}
			partial, err := publicationRecordOf(writer.root, journal.header.Stage, 128)
			if err != nil {
				t.Fatal(err)
			}
			if err := journal.append(writer, publicationEvent{Record: &partial}); err != nil {
				t.Fatal(err)
			}
			os.Exit(88)
		}
		var syncDirectory func(*os.Root) error
		if phase == "prepared" || phase == "new-prepared" {
			directory.beforePublish = func(string) error { os.Exit(88); return nil }
		} else {
			syncDirectory = func(*os.Root) error { os.Exit(88); return nil }
		}
		err = directory.publishFileContext(t.Context(), "source.json", []byte("candidate"), ".layer-", syncDirectory)
		t.Fatalf("child did not exit: %v", err)
	}
	for _, phase := range []string{"prepared", "published", "new-prepared", "new-published", "receipt-header", "receipt-empty", "receipt-partial", "receipt-unowned", "receipt-incomplete"} {
		t.Run(phase, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "records")
			directory, err := NewDirectory(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(phase, "new-") {
				if err := directory.WriteFile("source.json", []byte("retained"), ".layer-"); err != nil {
					t.Fatal(err)
				}
			}
			command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestPrivateRecordRecoveryAfterProcessExit$")
			command.Env = append(os.Environ(), "STARMAP_TEST_PRIVATE_RECORD_EXIT="+path, "STARMAP_TEST_PRIVATE_RECORD_PHASE="+phase)
			output, err := command.CombinedOutput()
			if command.ProcessState == nil || command.ProcessState.ExitCode() != 88 {
				t.Fatalf("child exit: %v, %s", err, output)
			}
			reopened, err := ExistingDirectory(path)
			if err != nil {
				t.Fatal(err)
			}

			preserve := phase == "receipt-unowned" || phase == "receipt-incomplete"
			err = reopened.RecoverPublications(t.Context())
			if preserve {
				if err == nil {
					t.Fatal("incomplete ownership was accepted")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			want := "retained"
			if phase == "published" || phase == "new-published" {
				want = "candidate"
			}
			data, err := reopened.ReadFile("source.json", 128)
			if phase == "new-prepared" {
				if !os.IsNotExist(err) {
					t.Fatalf("unpublished destination: %q, %v", data, err)
				}
			} else if err != nil || string(data) != want {
				t.Fatalf("recovery changed accepted bytes: %q, %v", data, err)
			}
			entries, err := os.ReadDir(path)
			if err != nil {
				t.Fatal(err)
			}
			retained := 0
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".layer-") {
					retained++
				}
				if strings.HasPrefix(entry.Name(), ".layer-") && !preserve {
					t.Fatalf("recovery retained abandoned file %s", entry.Name())
				}
			}
			if preserve && retained != 1 {
				t.Fatalf("unowned staging file was lost: %d", retained)
			}
		})
	}
}

func TestPrivatePublicationPreservesChangedRecords(t *testing.T) {
	for _, change := range []string{"stage-bytes", "stage-identity", "stage-access", "stage-time", "destination-bytes", "journal-bytes", "journal-identity", "writer-identity", "metadata-identity"} {
		t.Run(change, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "records")
			directory, err := NewDirectory(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := directory.WriteFile("source.json", []byte("retained"), ".layer-"); err != nil {
				t.Fatal(err)
			}
			var stagePath string
			directory.beforePublish = func(stage string) error {
				stagePath = filepath.Join(path, stage)
				target := stagePath
				if strings.HasPrefix(change, "destination") {
					target = filepath.Join(path, "source.json")
				}
				if strings.HasPrefix(change, "journal") {
					entries, err := os.ReadDir(filepath.Join(path, publicationDirectory))
					if err != nil {
						return err
					}
					for _, entry := range entries {
						if strings.HasSuffix(entry.Name(), publicationJournalSuffix) {
							target = filepath.Join(path, publicationDirectory, entry.Name())
						}
					}
				}
				if change == "writer-identity" {
					target = filepath.Join(path, publicationDirectory, publicationLock)
				}
				if change == "metadata-identity" {
					if err := os.Rename(filepath.Join(path, publicationDirectory), filepath.Join(path, "saved-metadata")); err != nil {
						return err
					}
					return CreateDirectory(filepath.Join(path, publicationDirectory))
				}
				switch {
				case strings.HasSuffix(change, "identity"):
					raw, err := os.ReadFile(target)
					if err != nil {
						return err
					}
					if err := os.Rename(target, target+".saved"); err != nil {
						return err
					}
					return os.WriteFile(target, raw, FileMode)
				case change == "stage-access":
					return os.Chmod(target, 0o400)
				case change == "stage-time":
					return os.Chtimes(target, time.Unix(1, 0), time.Unix(1, 0))
				default:
					info, err := os.Stat(target)
					if err != nil {
						return err
					}
					raw, err := os.ReadFile(target)
					if err != nil {
						return err
					}
					if change == "journal-bytes" {
						raw = append([]byte(" "), raw...)
					} else {
						raw[0] ^= 1
					}
					if err := os.WriteFile(target, raw, FileMode); err != nil {
						return err
					}
					return os.Chtimes(target, info.ModTime(), info.ModTime())
				}
			}
			err = directory.PublishFileContext(t.Context(), "source.json", []byte("candidate"), ".layer-")
			if err == nil {
				t.Fatal("publication accepted changed ownership")
			}
			raw, readErr := os.ReadFile(filepath.Join(path, "source.json"))
			want := "retained"
			if change == "destination-bytes" {
				want = "setained"
			}
			if readErr != nil || string(raw) != want {
				t.Fatalf("destination changed: %q, %v", raw, readErr)
			}
			if change != "destination-bytes" {
				if _, err := os.Lstat(stagePath); err != nil {
					t.Fatalf("changed publication lost staging evidence: %v", err)
				}
			}
		})
	}
}

func TestPrivatePublicationExcludesActiveWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records")
	directory, err := NewDirectory(path)
	if err != nil {
		t.Fatal(err)
	}
	directory.beforePublish = func(string) error {
		other, err := ExistingDirectory(path)
		if err != nil {
			return err
		}
		for _, operation := range []func() error{
			func() error { return other.RecoverPublications(t.Context()) },
			func() error { return other.PublishFileContext(t.Context(), "other.json", []byte("other"), ".state-") },
		} {
			var conflict *errors.ConflictError
			if err := operation(); !stderrors.As(err, &conflict) {
				t.Fatalf("active writer: %v", err)
			}
		}
		return nil
	}
	if err := directory.PublishFileContext(t.Context(), "source.json", []byte("candidate"), ".layer-"); err != nil {
		t.Fatal(err)
	}
	if err := directory.RecoverPublications(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestPrivatePublicationCancellationAndAmbiguousFlush(t *testing.T) {
	directory, err := NewDirectory(filepath.Join(t.TempDir(), "records"))
	if err != nil {
		t.Fatal(err)
	}
	if err := directory.PublishFileContext(t.Context(), "source.json", []byte("retained"), ".layer-"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	directory.beforePublish = func(string) error { cancel(); return nil }
	if err := directory.PublishFileContext(ctx, "source.json", []byte("candidate"), ".layer-"); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	raw, err := directory.ReadFile("source.json", 128)
	if err != nil || string(raw) != "retained" {
		t.Fatalf("cancellation changed destination: %q %v", raw, err)
	}
	directory.beforePublish = nil
	sentinel := fmt.Errorf("directory flush failed")
	err = directory.publishFileContext(t.Context(), "source.json", []byte("candidate"), ".layer-", func(*os.Root) error { return sentinel })
	var publication *errors.PublicationError
	if !stderrors.As(err, &publication) || !stderrors.Is(err, sentinel) {
		t.Fatalf("publication result: %v", err)
	}
	if err := directory.RecoverPublications(t.Context()); err != nil {
		t.Fatal(err)
	}
	raw, err = directory.ReadFile("source.json", 128)
	if err != nil || string(raw) != "candidate" {
		t.Fatalf("visible destination: %q %v", raw, err)
	}
	if err := directory.PublishFileContext(t.Context(), "source.json", []byte("candidate"), ".layer-"); err != nil {
		t.Fatal(err)
	}
}

func TestPrivatePublicationRecoveryBoundsAndUnknownFiles(t *testing.T) {
	directory, err := NewDirectory(filepath.Join(t.TempDir(), "records"))
	if err != nil {
		t.Fatal(err)
	}
	if err := directory.RecoverPublications(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(directory.path, publicationDirectory)); !os.IsNotExist(err) {
		t.Fatal("passive recovery created metadata")
	}
	writer, err := directory.acquirePublicationWriter(t.Context(), true)
	if err != nil {
		t.Fatal(err)
	}
	unknown := filepath.Join(directory.path, ".layer-operator")
	if err := os.WriteFile(unknown, []byte("operator"), FileMode); err != nil {
		t.Fatal(err)
	}
	writer.close()
	if err := directory.RecoverPublications(t.Context()); err != nil {
		t.Fatal(err)
	}
	for i := range publicationScanLimit {
		path := filepath.Join(directory.path, publicationDirectory, fmt.Sprintf("unknown-%d", i))
		if err := os.WriteFile(path, nil, FileMode); err != nil {
			t.Fatal(err)
		}
	}
	var invalid *errors.ValidationError
	if err := directory.RecoverPublications(t.Context()); !stderrors.As(err, &invalid) || invalid.Field != "private.publication_entries" {
		t.Fatalf("scan limit: %v", err)
	}
	raw, err := os.ReadFile(unknown)
	if err != nil || string(raw) != "operator" {
		t.Fatalf("unknown file lost: %q %v", raw, err)
	}
}

func TestPrivatePublicationRecoveryPreservesAbandonedChanges(t *testing.T) {
	for _, change := range []string{"stage-bytes", "stage-identity", "stage-time", "journal-empty", "journal-partial", "journal-whitespace", "journal-identity", "writer-missing", "writer-identity", "unknown-metadata", "canceled"} {
		t.Run(change, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "records")
			directory, err := NewDirectory(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := directory.WriteFile("source.json", []byte("retained"), ".layer-"); err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestPrivateRecordRecoveryAfterProcessExit$")
			command.Env = append(os.Environ(), "STARMAP_TEST_PRIVATE_RECORD_EXIT="+path, "STARMAP_TEST_PRIVATE_RECORD_PHASE=prepared")
			output, err := command.CombinedOutput()
			if command.ProcessState == nil || command.ProcessState.ExitCode() != 88 {
				t.Fatalf("child exit: %v %s", err, output)
			}
			var stage, journal string
			entries, err := os.ReadDir(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".layer-") {
					stage = filepath.Join(path, entry.Name())
				}
			}
			entries, err = os.ReadDir(filepath.Join(path, publicationDirectory))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), publicationJournalSuffix) {
					journal = filepath.Join(path, publicationDirectory, entry.Name())
				}
			}
			if stage == "" || journal == "" {
				t.Fatal("missing durable staging receipt")
			}
			target := stage
			if strings.HasPrefix(change, "journal") {
				target = journal
			}
			if strings.HasPrefix(change, "writer") {
				target = filepath.Join(path, publicationDirectory, publicationLock)
			}
			ctx := t.Context()
			switch change {
			case "stage-bytes":
				err = os.WriteFile(stage, []byte("operator"), FileMode)
			case "stage-time":
				err = os.Chtimes(stage, time.Unix(1, 0), time.Unix(1, 0))
			case "stage-identity", "journal-identity", "writer-identity":
				raw, readErr := os.ReadFile(target)
				if readErr != nil {
					t.Fatal(readErr)
				}
				err = os.Rename(target, target+".saved")
				if err == nil {
					err = os.WriteFile(target, raw, FileMode)
				}
			case "writer-missing":
				err = os.Rename(target, target+".saved")
			case "journal-empty":
				err = os.WriteFile(journal, nil, FileMode)
			case "journal-partial", "journal-whitespace":
				raw, readErr := os.ReadFile(journal)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if change == "journal-partial" {
					raw = raw[:len(raw)-1]
				} else {
					raw = append([]byte(" "), raw...)
				}
				err = os.WriteFile(journal, raw, FileMode)
			case "unknown-metadata":
				err = os.WriteFile(filepath.Join(path, publicationDirectory, "operator-note"), []byte("preserve"), FileMode)
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if err != nil {
				t.Fatal(err)
			}
			reopened, err := ExistingDirectory(path)
			if err != nil {
				t.Fatal(err)
			}
			err = reopened.RecoverPublications(ctx)
			if change == "unknown-metadata" {
				if err != nil {
					t.Fatal(err)
				}
				raw, err := os.ReadFile(filepath.Join(path, publicationDirectory, "operator-note"))
				if err != nil || string(raw) != "preserve" {
					t.Fatalf("unknown metadata: %q %v", raw, err)
				}
			} else {
				if err == nil {
					t.Fatal("recovery accepted changed or canceled operation")
				}
				for _, name := range []string{stage, journal} {
					if _, err := os.Lstat(name); err != nil {
						t.Fatalf("recovery removed evidence %s: %v", name, err)
					}
				}
			}
			raw, err := reopened.ReadFile("source.json", 128)
			if err != nil || string(raw) != "retained" {
				t.Fatalf("accepted bytes: %q %v", raw, err)
			}
		})
	}
}
