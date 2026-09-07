package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWorkspaceReplacementPreservesAccessPolicy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	old, oldIdentity := testCatalog(t, "old", "Old")
	if _, err := Project(t.Context(), path, old, oldIdentity); err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	base := "D:P(A;;FA;;;" + user.User.Sid.String() + ")(A;;FA;;;SY)(A;;FA;;;BA)(A;;RA;;;WD)"
	for _, name := range []string{".", "providers.yaml"} {
		setWorkspaceTestDACL(t, filepath.Join(path, name), base)
	}
	label, err := windows.SecurityDescriptorFromString("S:(ML;;NW;;;LW)")
	if err != nil {
		t.Fatal(err)
	}
	sacl, _, err := label.SACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(filepath.Join(path, "providers.yaml"), windows.SE_FILE_OBJECT, windows.LABEL_SECURITY_INFORMATION, nil, nil, nil, sacl); err != nil {
		t.Fatal(err)
	}
	before, err := snapshotTree(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	next, identity := testCatalog(t, "new", "New")
	if _, err := Project(t.Context(), path, next, identity); err != nil {
		t.Fatal(err)
	}
	after, err := snapshotTree(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	want := make(map[string]string)
	for _, entry := range before.Entries {
		want[entry.Path] = entry.AccessSHA256
	}
	for _, entry := range after.Entries {
		if old, ok := want[entry.Path]; ok && old != entry.AccessSHA256 {
			t.Errorf("%s access changed", entry.Path)
		}
	}
	assertReplacementFinished(t, path)
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspaceReplacementPreservesInheritedAccessPolicy(t *testing.T) {
	parent := t.TempDir()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	base := "D:P(A;OICI;FA;;;" + user.User.Sid.String() + ")(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;GR;;;AU)"
	setWorkspaceTestDACL(t, parent, base)
	path := filepath.Join(parent, "workspace")
	old, oldIdentity := testCatalog(t, "old", "Old")
	if _, err := Project(t.Context(), path, old, oldIdentity); err != nil {
		t.Fatal(err)
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := sd.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED != 0 {
		t.Fatal("fixture did not retain enabled inheritance", err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	inherited := false
	for index := uint32(0); index < uint32(dacl.AceCount); index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, index, &ace); err != nil {
			t.Fatal(err)
		}
		if ace.Header.AceFlags&windows.INHERITED_ACE != 0 {
			inherited = true
		}
	}
	if !inherited {
		t.Fatal("fixture has no inherited grants")
	}
	before, err := snapshotTree(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	next, identity := testCatalog(t, "new", "New")
	if _, err := Project(t.Context(), path, next, identity); err != nil {
		t.Fatal(err)
	}
	after, err := snapshotTree(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	want := make(map[string]string)
	for _, entry := range before.Entries {
		want[entry.Path] = entry.AccessSHA256
	}
	for _, entry := range after.Entries {
		if old, ok := want[entry.Path]; ok && old != entry.AccessSHA256 {
			t.Errorf("%s inherited access changed", entry.Path)
		}
	}
	assertReplacementFinished(t, path)
}
