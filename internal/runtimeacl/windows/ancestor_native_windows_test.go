package windows

import (
	native "golang.org/x/sys/windows"
	"testing"
)

func TestWindowsAncestorNativeConstants(t *testing.T) {
	want := uint32(native.FILE_GENERIC_READ | native.FILE_GENERIC_EXECUTE | native.FILE_APPEND_DATA | native.GENERIC_READ | native.GENERIC_EXECUTE)
	if ancestorReadAndCreateDirectory != want {
		t.Fatalf("rights mask = %x, native = %x", ancestorReadAndCreateDirectory, want)
	}
	sid, _, _, err := native.LookupSID("", `NT SERVICE\TrustedInstaller`)
	if err != nil {
		t.Fatal(err)
	}
	if sid.String() != windowsInstallerSID {
		t.Fatalf("installer SID = %q, want %q", sid.String(), windowsInstallerSID)
	}
}
