// Package windowstoken controls privileges for synchronous Windows access tests.
package windowstoken

import (
	"bytes"
	"encoding/binary"
	"runtime"
	"testing"

	"golang.org/x/sys/windows"
)

// WithoutPrivileges runs check on a thread with a private token and no enabled privileges.
// The callback must not start goroutines or subtests that need this token.
func WithoutPrivileges(t *testing.T, check func()) {
	t.Helper()
	runtime.LockOSThread()
	restore := false
	var original, source windows.Token
	defer func() {
		defer func() {
			if original != 0 {
				_ = original.Close()
			} else if source != 0 {
				_ = source.Close()
			}
		}()
		if restore {
			if err := windows.SetThreadToken(nil, original); err != nil {
				t.Errorf("restore thread token: %v", err)
				return
			}
		}
		runtime.UnlockOSThread()
	}()
	err := windows.OpenThreadToken(windows.CurrentThread(), windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE|windows.TOKEN_IMPERSONATE, true, &original)
	if err != nil && err != windows.ERROR_NO_TOKEN {
		t.Fatal(err)
	}
	source = original
	if original == 0 {
		if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE, &source); err != nil {
			t.Fatal(err)
		}
	}
	var token windows.Token
	if err := windows.DuplicateTokenEx(source, windows.TOKEN_QUERY|windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_IMPERSONATE, nil, windows.SecurityImpersonation, windows.TokenImpersonation, &token); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = token.Close() }()
	before := tokenPrivileges(t, token)
	for _, name := range []string{"SeBackupPrivilege", "SeRestorePrivilege"} {
		native, err := windows.UTF16PtrFromString(name)
		if err != nil {
			t.Fatal(err)
		}
		var luid windows.LUID
		if err := windows.LookupPrivilegeValue(nil, native, &luid); err != nil {
			t.Fatal(err)
		}
		enabled := false
		for _, privilege := range before {
			if privilege.Luid == luid && privilege.Attributes&windows.SE_PRIVILEGE_ENABLED != 0 {
				enabled = true
			}
		}
		t.Logf("inherited %s enabled=%v", name, enabled)
	}
	if err := windows.AdjustTokenPrivileges(token, true, nil, 0, nil, nil); err != nil {
		t.Fatal(err)
	}
	for _, privilege := range tokenPrivileges(t, token) {
		if privilege.Attributes&windows.SE_PRIVILEGE_ENABLED != 0 {
			t.Fatal("test token retained an enabled privilege")
		}
	}
	if err := windows.SetThreadToken(nil, token); err != nil {
		t.Fatal(err)
	}
	restore = true
	check()
}

func tokenPrivileges(t *testing.T, token windows.Token) []windows.LUIDAndAttributes {
	t.Helper()
	var size uint32
	if err := windows.GetTokenInformation(token, windows.TokenPrivileges, nil, 0, &size); err != windows.ERROR_INSUFFICIENT_BUFFER {
		t.Fatalf("size token privileges: %v", err)
	}
	data := make([]byte, size)
	if err := windows.GetTokenInformation(token, windows.TokenPrivileges, &data[0], size, &size); err != nil {
		t.Fatal(err)
	}
	if len(data) < 4 {
		t.Fatal("token privilege count is missing")
	}
	count := binary.LittleEndian.Uint32(data[:4])
	if uint64(count)*12+4 > uint64(len(data)) {
		t.Fatal("token privilege entries exceed the returned buffer")
	}
	privileges := make([]windows.LUIDAndAttributes, count)
	for index := range privileges {
		entry := data[4+index*12 : 4+(index+1)*12]
		// Each LUID has one signed and one unsigned 32-bit field.
		if err := binary.Read(bytes.NewReader(entry), binary.LittleEndian, &privileges[index]); err != nil {
			t.Fatal(err)
		}
	}
	return privileges
}
