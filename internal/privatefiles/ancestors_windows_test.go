package privatefiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/productpaths"
	"golang.org/x/sys/windows"
)

func setAncestorFixtureDACL(t *testing.T, path, sddl string) {
	t.Helper()
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsAncestorRefusalAndRecovery(t *testing.T) {
	account, err := windowsProcessSID()
	if err != nil {
		t.Fatal(err)
	}
	for _, right := range []string{"0x2", "0x100", "0x40", "0x10000", "0x40000", "0x80000"} {
		t.Run(right, func(t *testing.T) {
			ancestor := filepath.Join(t.TempDir(), "ancestor")
			directory, err := NewDirectory(filepath.Join(ancestor, "records"))
			if err != nil {
				t.Fatal(err)
			}
			if err := directory.WriteFile("retained.json", []byte("retained"), ".stage-"); err != nil {
				t.Fatal(err)
			}
			private := "D:P(A;;FA;;;" + account + ")(A;;FA;;;SY)(A;;FA;;;BA)"
			setAncestorFixtureDACL(t, ancestor, private+"(A;;"+right+";;;WD)")
			t.Cleanup(func() { setAncestorFixtureDACL(t, ancestor, private) })
			if _, err := directory.ReadFile("retained.json", 100); err == nil {
				t.Fatal("read accepted unsafe ancestor")
			}
			if err := directory.WriteFile("retained.json", []byte("replacement"), ".stage-"); err == nil {
				t.Fatal("write accepted unsafe ancestor")
			}
			missing := filepath.Join(ancestor, "missing", "records")
			if _, err := NewDirectory(missing); err == nil {
				t.Fatal("creation accepted unsafe ancestor")
			}
			if _, err := os.Lstat(filepath.Dir(missing)); !os.IsNotExist(err) {
				t.Fatalf("refused creation changed tree: %v", err)
			}
			data, err := os.ReadFile(filepath.Join(ancestor, "records", "retained.json"))
			if err != nil || string(data) != "retained" {
				t.Fatalf("retained bytes changed: %q %v", data, err)
			}
			setAncestorFixtureDACL(t, ancestor, private)
			if _, err := directory.ReadFile("retained.json", 100); err != nil {
				t.Fatal("explicit correction did not recover", err)
			}
		})
	}
}

func TestWindowsAncestorsPreserveSharedReadAndCreateDirectory(t *testing.T) {
	account, err := windowsProcessSID()
	if err != nil {
		t.Fatal(err)
	}
	for _, grant := range []string{"(A;;GRGX;;;WD)", "(A;;0x4;;;WD)", "(A;OICIIO;FA;;;WD)"} {
		t.Run(grant, func(t *testing.T) {
			parent := filepath.Join(t.TempDir(), "parent")
			if _, err := NewDirectory(parent); err != nil {
				t.Fatal(err)
			}
			private := "D:P(A;OICI;FA;;;" + account + ")(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)"
			setAncestorFixtureDACL(t, parent, private+grant)
			t.Cleanup(func() { setAncestorFixtureDACL(t, parent, private) })
			if _, err := NewDirectory(filepath.Join(parent, "private-child")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWindowsAncestorPathForms(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"config.yaml", `.\config.yaml`, `\config.yaml`, filepath.VolumeName(cwd) + `config.yaml`, `C:\safe\..\config.yaml`, `\\server\share\config.yaml`, `\\server\share`} {
		t.Run(path, func(t *testing.T) {
			got, err := windowsAncestorAbsolute(path)
			if err != nil {
				t.Fatal(err)
			}
			want, err := filepath.Abs(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.EqualFold(filepath.Clean(got), filepath.Clean(want)) {
				t.Fatalf("resolved = %q, want %q", got, want)
			}
		})
	}
	raw := `C:\unsafe\..\config.yaml`
	got, err := windowsAncestorAbsolute(raw)
	if err != nil || got != raw {
		t.Fatalf("traversal erased: %q %v", got, err)
	}

	for input, want := range map[string]string{
		`\\?\C:\config.yaml`:               `C:\config.yaml`,
		`\\.\C:\config.yaml`:               `C:\config.yaml`,
		`\??\C:\config.yaml`:               `C:\config.yaml`,
		`\\?\UNC\server\share\config.yaml`: `\\server\share\config.yaml`,
		`\\?\Volume{01234567-89ab-cdef-0123-456789abcdef}\config.yaml`: `\\?\Volume{01234567-89ab-cdef-0123-456789abcdef}\config.yaml`,
	} {
		got, err := windowsAncestorAbsolute(input)
		if err != nil || got != want {
			t.Fatalf("namespace %q resolved %q, want %q: %v", input, got, want, err)
		}
	}
	for _, path := range []string{`\\.\PhysicalDrive0`, `\\?\GLOBALROOT\Device\HarddiskVolume1`, `\??\Volume{invalid}\file`} {
		if _, err := windowsAncestorAbsolute(path); err == nil {
			t.Fatalf("accepted non-filesystem device path %q", path)
		}
	}
}

func TestWindowsAncestorSecurityDescriptorFlags(t *testing.T) {
	account, err := windowsProcessSID()
	if err != nil {
		t.Fatal(err)
	}
	sd, err := windows.SecurityDescriptorFromString("O:" + account + "D:P(A;OICIIO;FA;;;WD)(A;;FA;;;" + account + ")")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeWindowsSecurityDescriptor(sd, "private.ancestor")
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.Entries) != 2 || decoded.Entries[0].Flags&windows.INHERIT_ONLY_ACE == 0 {
		t.Fatalf("native flags lost: %+v", decoded.Entries)
	}
}

func TestWindowsAncestorSymlinkTargetsAndTraversal(t *testing.T) {
	account, err := windowsProcessSID()
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(t.TempDir(), "routes")
	if _, err := NewDirectory(base); err != nil {
		t.Fatal(err)
	}
	safe := filepath.Join(base, "safe")
	unsafe := filepath.Join(base, "unsafe")
	for _, path := range []string{safe, unsafe} {
		if _, err := NewDirectory(path); err != nil {
			t.Fatal(err)
		}
	}
	leaf := filepath.Join(safe, "settings.yaml")
	if err := os.WriteFile(leaf, []byte("mode: local"), FileMode); err != nil {
		t.Fatal(err)
	}
	hop := filepath.Join(unsafe, "hop")
	if err := os.Symlink(safe, hop); err != nil {
		t.Fatal(err)
	}
	selected := filepath.Join(base, "selected.yaml")
	if err := os.Symlink(filepath.Join(hop, "settings.yaml"), selected); err != nil {
		t.Fatal(err)
	}
	if err := ValidateAncestors(selected); err != nil {
		t.Fatal("trusted target refused", err)
	}
	private := "D:P(A;;FA;;;" + account + ")(A;;FA;;;SY)(A;;FA;;;BA)"
	setAncestorFixtureDACL(t, unsafe, private+"(A;;0x40;;;WD)")
	t.Cleanup(func() { setAncestorFixtureDACL(t, unsafe, private) })
	if err := ValidateAncestors(selected); err == nil {
		t.Fatal("intermediate target ancestor was ignored")
	}
	if err := ValidateAncestors(unsafe + `\..\safe\settings.yaml`); err == nil {
		t.Fatal("normalization erased unsafe traversal")
	}
	setAncestorFixtureDACL(t, unsafe, private)
	if err := ValidateAncestors(selected); err != nil {
		t.Fatal("corrected target refused", err)
	}
}

func TestWindowsCanonicalAncestorsRemainPassive(t *testing.T) {
	for _, product := range []productpaths.Product{productpaths.Starmap, productpaths.Starport} {
		roots, err := productpaths.Resolve(productpaths.UserDefaults(product))
		if err != nil {
			t.Fatal(err)
		}
		for role, path := range roots {
			t.Run(string(product)+"/"+string(role), func(t *testing.T) {
				before, beforeErr := os.Lstat(path.Path)
				if beforeErr != nil && !os.IsNotExist(beforeErr) {
					t.Fatal(beforeErr)
				}
				if err := ValidateAncestors(path.Path); err != nil {
					t.Fatalf("native default %s: %v", path.Path, err)
				}
				after, afterErr := os.Lstat(path.Path)
				if os.IsNotExist(beforeErr) {
					if !os.IsNotExist(afterErr) {
						t.Fatalf("validation created %s: %v", path.Path, afterErr)
					}
				} else if afterErr != nil || !os.SameFile(before, after) {
					t.Fatalf("validation changed %s: %v", path.Path, afterErr)
				}
			})
		}
	}
}
