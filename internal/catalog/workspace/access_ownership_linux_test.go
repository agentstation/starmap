//go:build linux && starmap_ownership_test

package workspace

import (
	"bytes"
	stderrors "errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// TestWorkspaceForeignOwnership requires the isolated native ownership fixture.
func TestWorkspaceForeignOwnership(t *testing.T) {
	if os.Getenv("STARMAP_OWNERSHIP_FIXTURE") != "1" {
		t.Fatal("use native-linux-suites.py --workspace-ownership")
	}
	if action := os.Getenv("STARMAP_OWNERSHIP_ACTION"); action != "" {
		runOwnershipChild(t, action)
		return
	}
	if os.Geteuid() != 0 {
		t.Fatal("fixture parent requires scoped container capabilities")
	}
	for _, journal := range []bool{false, true} {
		t.Run(map[bool]string{false: "atomic", true: "journal"}[journal], func(t *testing.T) {
			for _, fixture := range []struct {
				name, path string
				uid, gid   int
			}{
				{"file-owner", "providers.yaml", 65533, 65532},
				{"file-group", "providers.yaml", 65532, 65533},
				{"root-owner", ".", 65533, 65532},
				{"root-group", ".", 65532, 65533},
				{"directory-owner", "providers", 65533, 65532},
				{"operator-note-owner", "operator-note.txt", 65533, 65532},
			} {
				t.Run(fixture.name, func(t *testing.T) {
					parent, err := os.MkdirTemp("", "workspace-ownership-")
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := os.RemoveAll(parent); err != nil {
							t.Error(err)
						}
					})
					if err := os.Chown(parent, 65532, 65532); err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(parent, "workspace")
					invokeOwnershipChild(t, "seed", path, journal)
					target := filepath.Join(path, fixture.path)
					if err := os.Chown(target, fixture.uid, fixture.gid); err != nil {
						t.Fatal(err)
					}
					before, err := snapshotTree(t.Context(), path)
					if err != nil {
						t.Fatal(err)
					}
					invokeOwnershipChild(t, "refuse", path, journal)
					after, err := snapshotTree(t.Context(), path)
					if err != nil || !sameTree(before, after) {
						t.Fatal("refused update changed original tree or access", err)
					}
					assertWorkspaceModel(t, path, "old", "Old")
					assertReplacementFinished(t, path)
					if fixture.gid == 65533 {
						invokeOwnershipChild(t, "group-update", path, journal)
						assertWorkspaceModel(t, path, "middle", "Middle")
					}
					next, identity := testCatalog(t, "new", "New")
					if _, err := (projector{journalReplacement: journal}).project(t.Context(), path, next, identity, InputExpectation{}); err != nil {
						t.Fatal("authorized replacement", err)
					}
					after, err = snapshotTree(t.Context(), path)
					if err != nil {
						t.Fatal(err)
					}
					access := make(map[string]string)
					for _, entry := range before.Entries {
						access[entry.Path] = entry.AccessSHA256
					}
					for _, entry := range after.Entries {
						if want, found := access[entry.Path]; found && entry.AccessSHA256 != want {
							t.Errorf("replacement changed ownership or access for %s", entry.Path)
						}
					}
					data, err := os.ReadFile(filepath.Join(path, "operator-note.txt"))
					if err != nil || !bytes.Equal(data, []byte("operator input")) {
						t.Fatal("replacement changed operator note", err)
					}
					assertWorkspaceModel(t, path, "new", "New")
					assertReplacementFinished(t, path)
				})
			}
		})
	}
}

func invokeOwnershipChild(t *testing.T, action, path string, journal bool) {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), binary, "-test.run=^TestWorkspaceForeignOwnership$", "-test.timeout=30s")
	command.Env = append(os.Environ(), "STARMAP_OWNERSHIP_ACTION="+action, "STARMAP_OWNERSHIP_PATH="+path,
		"STARMAP_OWNERSHIP_JOURNAL="+map[bool]string{false: "0", true: "1"}[journal])
	command.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65532, Gid: 65532, Groups: []uint32{}}}
	if action == "group-update" {
		command.SysProcAttr.Credential.Groups = []uint32{65533}
	}
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("service child %s: %v\n%s", action, err, output)
	}
	t.Logf("service child %s: %s", action, output)
}

func runOwnershipChild(t *testing.T, action string) {
	t.Helper()
	status, err := os.ReadFile("/proc/self/status")
	if err != nil || os.Geteuid() != 65532 || os.Getegid() != 65532 || !strings.Contains(string(status), "CapEff:\t0000000000000000\n") {
		t.Fatal("service child must have UID/GID 65532 and no effective capabilities", err)
	}
	groups, err := os.Getgroups()
	if err != nil || (action == "group-update" && (len(groups) != 1 || groups[0] != 65533)) || (action != "group-update" && len(groups) != 0) {
		t.Fatal("unexpected service supplementary groups", groups, err)
	}
	path := os.Getenv("STARMAP_OWNERSHIP_PATH")
	journal := os.Getenv("STARMAP_OWNERSHIP_JOURNAL") == "1"
	switch action {
	case "seed":
		old, identity := testCatalog(t, "old", "Old")
		if _, err := Project(t.Context(), path, old, identity); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "operator-note.txt"), []byte("operator input"), 0o644); err != nil {
			t.Fatal(err)
		}
	case "group-update":
		next, identity := testCatalog(t, "middle", "Middle")
		if _, err := (projector{journalReplacement: journal}).project(t.Context(), path, next, identity, InputExpectation{}); err != nil {
			t.Fatal("group-authorized replacement", err)
		}
	case "refuse":
		next, identity := testCatalog(t, "new", "New")
		_, err := (projector{journalReplacement: journal}).project(t.Context(), path, next, identity, InputExpectation{})
		if !stderrors.Is(err, syscall.EPERM) {
			t.Fatalf("expected ownership restoration refusal, got %v", err)
		}
		t.Logf("ownership restoration refused: %v", err)
	default:
		t.Fatal("unknown ownership fixture action", action)
	}
}
