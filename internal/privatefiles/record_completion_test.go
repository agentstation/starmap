package privatefiles

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPrivateRecordExactCompletionAfterProcessExit(t *testing.T) {
	const body = "original-native-phase"
	if directoryPath := os.Getenv("STARMAP_TEST_EXACT_COMPLETION_PATH"); directoryPath != "" {
		d, err := ExistingDirectory(directoryPath)
		if err != nil {
			t.Fatal(err)
		}
		writer, err := d.acquirePublicationWriter(t.Context(), true)
		if err != nil {
			t.Fatal(err)
		}
		phase := os.Getenv("STARMAP_TEST_EXACT_COMPLETION_PHASE")
		name, prefix := "phase.json", ".product-stage-"
		if phase == "foreign" {
			name = "other.json"
		}
		if phase == "wrong-prefix" {
			prefix = ".other-"
		}
		journal, err := writer.newJournal(name, prefix)
		if err != nil {
			t.Fatal(err)
		}
		if phase == "header" {
			os.Exit(88)
		}
		file, err := CreateFile(writer.root, journal.header.Stage)
		if err != nil {
			t.Fatal(err)
		}
		if phase == "unowned" {
			os.Exit(88)
		}
		empty, err := publicationRecordOf(writer.root, journal.header.Stage, 0)
		if err != nil {
			t.Fatal(err)
		}
		if err := journal.append(writer, publicationEvent{Record: &empty}); err != nil {
			t.Fatal(err)
		}
		if phase == "empty" {
			os.Exit(88)
		}
		data := []byte(body)
		if phase == "wrong-bytes" {
			data = []byte("different-native-phase")
		}
		if _, err := file.Write(data); err != nil {
			t.Fatal(err)
		}
		if err := file.Sync(); err != nil {
			t.Fatal(err)
		}
		if phase == "incomplete" {
			os.Exit(88)
		}
		written, err := publicationRecordOf(writer.root, journal.header.Stage, 128)
		if err != nil {
			t.Fatal(err)
		}
		if err := journal.append(writer, publicationEvent{Record: &written}); err != nil {
			t.Fatal(err)
		}
		if phase == "published" {
			if _, err := d.promotePublication(t.Context(), writer, journal, name, nil, written); err != nil {
				t.Fatal(err)
			}
		}
		os.Exit(88)
	}
	for _, phase := range []string{"header", "empty", "prepared", "published", "unowned", "incomplete", "foreign", "wrong-prefix", "wrong-bytes", "callback-refusal", "changed-stage"} {
		t.Run(phase, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "records")
			d, err := NewDirectory(path)
			if err != nil {
				t.Fatal(err)
			}
			childPhase := phase
			if phase == "callback-refusal" || phase == "changed-stage" {
				childPhase = "prepared"
			}
			command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestPrivateRecordExactCompletionAfterProcessExit$")
			command.Env = append(os.Environ(), "STARMAP_TEST_EXACT_COMPLETION_PATH="+path, "STARMAP_TEST_EXACT_COMPLETION_PHASE="+childPhase)
			out, err := command.CombinedOutput()
			if command.ProcessState == nil || command.ProcessState.ExitCode() != 88 {
				t.Fatalf("child exit: %v, %s", err, out)
			}
			if phase == "changed-stage" {
				writer, err := d.acquirePublicationWriter(t.Context(), false)
				if err != nil {
					t.Fatal(err)
				}
				names, err := writer.journalNames(t.Context())
				if err != nil || len(names) != 1 {
					t.Fatalf("journals: %v %v", names, err)
				}
				journal, err := writer.readJournal(names[0])
				if err != nil {
					t.Fatal(err)
				}
				if err := writer.root.Remove(journal.header.Stage); err != nil {
					t.Fatal(err)
				}
				f, err := CreateFile(writer.root, journal.header.Stage)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := f.Write([]byte(body)); err != nil {
					t.Fatal(err)
				}
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}
				writer.close()
			}
			refused := phase == "unowned" || phase == "incomplete" || phase == "foreign" || phase == "wrong-prefix" || phase == "wrong-bytes" || phase == "callback-refusal" || phase == "changed-stage"
			stages, inspectErr := d.InspectPublication(t.Context(), "phase.json", nil, []byte(body), ".product-stage-")
			if refused && phase != "callback-refusal" {
				if inspectErr == nil {
					t.Fatal("invalid pending evidence inspected")
				}
			} else if inspectErr != nil {
				t.Fatal(inspectErr)
			} else if phase != "published" && phase != "header" && len(stages) != 1 {
				t.Fatalf("checked stage census: %v", stages)
			}
			if err := d.CheckNoPendingPublications(t.Context()); err == nil {
				t.Fatal("passive inspection removed pending evidence")
			}
			called := 0
			err = d.CompletePublication(t.Context(), "phase.json", nil, []byte(body), ".product-stage-", func(context.Context) error {
				called++
				if phase == "callback-refusal" {
					return errors.New("native receipt refused")
				}
				return nil
			})
			if refused {
				if err == nil {
					t.Fatal("invalid completion accepted")
				}
				if err := d.CheckNoPendingPublications(t.Context()); err == nil {
					t.Fatal("refusal discarded original pending evidence")
				}
				if _, err := d.ReadFile("phase.json", 128); !os.IsNotExist(err) {
					t.Fatalf("refusal published phase: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if called < 1 {
				t.Fatal("native completion check omitted")
			}
			data, err := d.ReadFile("phase.json", 128)
			if err != nil || string(data) != body {
				t.Fatalf("phase: %q %v", data, err)
			}
			if err := d.CheckNoPendingPublications(t.Context()); err != nil {
				t.Fatal(err)
			}
			if err := d.CompletePublication(t.Context(), "phase.json", nil, []byte(body), ".product-stage-", func(context.Context) error { return nil }); err != nil {
				t.Fatalf("exact retry: %v", err)
			}
		})
	}
}
