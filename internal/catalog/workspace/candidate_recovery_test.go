package workspace

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCandidateRecoveryAfterProcessExit(t *testing.T) {
	if target := os.Getenv("STARMAP_TEST_CANDIDATE_EXIT"); target != "" {
		catalog, identity := testCatalog(t, "new", "New Model")
		p := projector{}
		stop := func() error { os.Exit(88); return nil }
		phase := os.Getenv("STARMAP_TEST_CANDIDATE_PHASE")
		if strings.HasPrefix(phase, "prepared") {
			p.beforePromote = stop
		} else if strings.HasPrefix(phase, "journal-") {
			p.journalReplacement = true
			wanted := map[string]replacementPhase{
				"journal-saved":     replacementJournalSaved,
				"journal-installed": replacementInstalled,
				"journal-marker":    replacementMarkerSaved,
				"journal-cleanup":   replacementBackupRemoved,
			}[phase]
			p.afterReplacementPhase = func(current replacementPhase) error {
				if current == wanted {
					return stop()
				}
				return nil
			}
		} else {
			p.beforeMarker = stop
		}
		_, err := p.project(t.Context(), target, catalog, identity, InputExpectation{})
		t.Fatalf("child did not exit during candidate publication: %v", err)
	}
	for _, phase := range []string{"prepared", "installed", "prepared-first", "installed-first", "journal-saved", "journal-installed", "journal-marker", "journal-cleanup"} {
		t.Run(phase, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "workspace")
			crashCandidate(t, target, phase)
			catalog, identity := testCatalog(t, "new", "New Model")
			if _, err := Repair(t.Context(), target, catalog, identity); err != nil {
				t.Fatal(err)
			}
			assertWorkspaceModel(t, target, "new", "New Model")
			assertNoProjectionStaging(t, target)
		})
	}
}

func crashCandidate(t *testing.T, target, phase string) {
	t.Helper()
	if !strings.HasSuffix(phase, "-first") {
		old, oldID := testCatalog(t, "old", "Old Model")
		if _, err := Project(t.Context(), target, old, oldID); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestCandidateRecoveryAfterProcessExit$")
	command.Env = append(os.Environ(), "STARMAP_TEST_CANDIDATE_EXIT="+target, "STARMAP_TEST_CANDIDATE_PHASE="+phase)
	output, err := command.CombinedOutput()
	if command.ProcessState == nil || command.ProcessState.ExitCode() != 88 {
		t.Fatalf("child: %v, %s", err, output)
	}
}

func TestCandidateRecoveryPreservesChangedState(t *testing.T) {
	for _, phase := range []string{"prepared", "installed"} {
		t.Run(phase, func(t *testing.T) {
			for _, change := range []string{"entry-bytes", "entry-identity", "unknown-file"} {
				t.Run(change, func(t *testing.T) {
					target := filepath.Join(t.TempDir(), "workspace")
					crashCandidate(t, target, phase)
					pattern := ".workspace.candidate-*"
					if phase == "installed" && journalWorkspaceReplacement {
						pattern = ".workspace.backup-*"
					}
					paths, err := filepath.Glob(filepath.Join(filepath.Dir(target), pattern))
					if err != nil || len(paths) != 1 {
						t.Fatalf("candidate or backup: %v, %v", paths, err)
					}
					path := filepath.Join(paths[0], "providers.yaml")
					data, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					switch change {
					case "entry-bytes":
						data = []byte("operator change")
					case "entry-identity":
						if err := os.Rename(path, filepath.Join(filepath.Dir(target), "original-provider")); err != nil {
							t.Fatal(err)
						}
					case "unknown-file":
						path = filepath.Join(paths[0], "operator-note")
					}
					if err := os.WriteFile(path, data, fileMode); err != nil {
						t.Fatal(err)
					}
					catalog, identity := testCatalog(t, "new", "New Model")
					if _, err := Repair(t.Context(), target, catalog, identity); err == nil {
						t.Fatal("recovery accepted changed candidate")
					}
					actual, err := os.ReadFile(path)
					if err != nil || !bytes.Equal(actual, data) {
						t.Fatalf("recovery changed operator file: %q, %v", actual, err)
					}
				})
			}
		})
	}
}

func TestCandidateRecoveryDefersToReplacementJournal(t *testing.T) {
	target := filepath.Join(t.TempDir(), "workspace")
	crashCandidate(t, target, "journal-saved")
	writer := preparationTestWriter(t, target)
	before := migrationTree(t, filepath.Dir(target))
	if err := recoverPreparations(t.Context(), target, writer); err == nil {
		t.Fatal("preparation cleanup bypassed replacement journal")
	}
	if !reflect.DeepEqual(before, migrationTree(t, filepath.Dir(target))) {
		t.Fatal("preparation cleanup changed replacement state")
	}
	if _, err := recoverReplacement(t.Context(), target, writer); err != nil {
		t.Fatal(err)
	}
	if err := recoverPreparations(t.Context(), target, writer); err != nil {
		t.Fatal(err)
	}
	assertNoProjectionStaging(t, target)
	assertWorkspaceModel(t, target, "new", "New Model")
}

func TestCandidateRecoveryRejectsInvalidHandoff(t *testing.T) {
	for _, change := range []string{"candidate-path", "new-identity", "original-identities", "suffix", "version-downgrade"} {
		t.Run(change, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "workspace")
			crashCandidate(t, target, "prepared")
			stages, err := filepath.Glob(filepath.Join(filepath.Dir(target), ".workspace.preparing-*"))
			if err != nil || len(stages) != 1 {
				t.Fatalf("preparation: %v, %v", stages, err)
			}
			path := filepath.Join(stages[0], preparationJournalName)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			lines := bytes.Split(bytes.TrimSuffix(data, []byte{'\n'}), []byte{'\n'})
			var event preparationEvent
			if err := json.Unmarshal(lines[len(lines)-1], &event); err != nil || event.Handoff == nil {
				t.Fatalf("handoff: %v", err)
			}
			switch change {
			case "candidate-path":
				event.Handoff.Candidate = filepath.Base(target)
			case "new-identity":
				event.Handoff.NewIdentity = "unknown"
			case "original-identities":
				delete(event.Handoff.OriginalIdentities, ".")
			}
			lines[len(lines)-1], err = json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			if change == "suffix" {
				lines = append(lines, lines[len(lines)-1])
			}
			if change == "version-downgrade" {
				lines[0] = bytes.Replace(lines[0], []byte(`"version":2`), []byte(`"version":1`), 1)
			}
			data = append(bytes.Join(lines, []byte{'\n'}), '\n')
			if err := os.WriteFile(path, data, fileMode); err != nil {
				t.Fatal(err)
			}
			writer := preparationTestWriter(t, target)
			before := migrationTree(t, filepath.Dir(target))
			if err := recoverPreparations(t.Context(), target, writer); err == nil {
				t.Fatal("invalid handoff accepted")
			}
			if !reflect.DeepEqual(before, migrationTree(t, filepath.Dir(target))) {
				t.Fatal("invalid handoff changed workspace or candidate")
			}
		})
	}
}

func TestCandidateRecoveryResumesPartialCleanup(t *testing.T) {
	target := filepath.Join(t.TempDir(), "workspace")
	crashCandidate(t, target, "prepared")
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(target), ".workspace.candidate-*"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("candidate: %v, %v", paths, err)
	}
	if err := os.Remove(filepath.Join(paths[0], "providers.yaml")); err != nil {
		t.Fatal(err)
	}
	writer := preparationTestWriter(t, target)
	if err := recoverPreparations(t.Context(), target, writer); err != nil {
		t.Fatal(err)
	}
	assertNoProjectionStaging(t, target)
	assertWorkspaceModel(t, target, "old", "Old Model")
}

func TestCandidatePublicationRefusesChangedOwnership(t *testing.T) {
	for _, journal := range []bool{false, true} {
		t.Run(map[bool]string{false: "native", true: "journal"}[journal], func(t *testing.T) {
			for _, change := range []string{"entry-bytes", "entry-identity", "journal-bytes"} {
				t.Run(change, func(t *testing.T) {
					target := filepath.Join(t.TempDir(), "workspace")
					old, oldID := testCatalog(t, "old", "Old Model")
					if _, err := Project(t.Context(), target, old, oldID); err != nil {
						t.Fatal(err)
					}
					catalog, identity := testCatalog(t, "new", "New Model")
					p := projector{journalReplacement: journal, beforePromote: func() error {
						candidates, err := filepath.Glob(filepath.Join(filepath.Dir(target), ".workspace.candidate-*"))
						if err != nil || len(candidates) != 1 {
							t.Fatalf("candidate: %v, %v", candidates, err)
						}
						path := filepath.Join(candidates[0], "providers.yaml")
						data, err := os.ReadFile(path)
						if err != nil {
							return err
						}
						switch change {
						case "entry-bytes":
							data = []byte("operator bytes")
						case "entry-identity":
							if err := os.Rename(path, filepath.Join(filepath.Dir(target), "original-provider")); err != nil {
								return err
							}
						case "journal-bytes":
							stages, err := filepath.Glob(filepath.Join(filepath.Dir(target), ".workspace.preparing-*"))
							if err != nil || len(stages) != 1 {
								t.Fatalf("preparation: %v, %v", stages, err)
							}
							path = filepath.Join(stages[0], preparationJournalName)
							data, err = os.ReadFile(path)
							if err != nil {
								return err
							}
							data = append([]byte{' '}, data...)
						}
						return os.WriteFile(path, data, fileMode)
					}}
					receipt, err := p.project(t.Context(), target, catalog, identity, InputExpectation{})
					if err == nil || receipt.GenerationID != "" {
						t.Fatalf("changed ownership published: receipt=%+v, err=%v", receipt, err)
					}
					assertWorkspaceModel(t, target, "old", "Old Model")
				})
			}
		})
	}
}
