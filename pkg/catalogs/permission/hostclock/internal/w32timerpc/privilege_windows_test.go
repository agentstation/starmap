package w32timerpc

import (
	"os"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// This opt-in probe changes one privilege only in its disposable test process.
// Product code and the mandatory unmodified-account fixture remain unchanged.
func TestWindowsTimeServicePreparedPrivilege(t *testing.T) {
	if os.Getenv("STARMAP_WINDOWS_CLOCK_PRIVILEGE_PROBE") != "1" {
		t.Skip("requires an explicitly selected disposable Windows process")
	}
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY|windows.TOKEN_ADJUST_PRIVILEGES, &token); err != nil {
		t.Fatal(err)
	}
	defer token.Close()
	name, err := windows.UTF16PtrFromString("SeSystemtimePrivilege")
	if err != nil {
		t.Fatal(err)
	}
	var luid windows.LUID
	if err := windows.LookupPrivilegeValue(nil, name, &luid); err != nil {
		t.Fatal(err)
	}
	before := testTimePrivilegeAttributes(t, token, luid)
	_, ordinaryErr := ObserveStatus(t.Context())
	t.Logf("ordinary account: enabled=%t status_error=%v", before&windows.SE_PRIVILEGE_ENABLED != 0, ordinaryErr)
	selected := windows.Tokenprivileges{PrivilegeCount: 1, Privileges: [1]windows.LUIDAndAttributes{{Luid: luid, Attributes: windows.SE_PRIVILEGE_ENABLED}}}
	var previous windows.Tokenprivileges
	var returned uint32
	if err := windows.AdjustTokenPrivileges(token, false, &selected, uint32(unsafe.Sizeof(previous)), &previous, &returned); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := windows.AdjustTokenPrivileges(token, false, &previous, 0, nil, nil); err != nil {
			t.Errorf("restore test process privilege: %v", err)
		}
		if after := testTimePrivilegeAttributes(t, token, luid); after != before {
			t.Error("test process privilege did not return to its original attributes")
		}
	}()
	if testTimePrivilegeAttributes(t, token, luid)&windows.SE_PRIVILEGE_ENABLED == 0 {
		t.Fatal("test process does not hold the requested enabled privilege")
	}
	status, err := ObserveStatus(t.Context())
	if err != nil {
		t.Fatalf("prepared process still cannot read W32Time: %v", err)
	}
	if status == nil || status.Size == 0 {
		t.Fatal("prepared process returned no W32Time status")
	}
	t.Logf("prepared account: status_size=%d", status.Size)
}

func testTimePrivilegeAttributes(t *testing.T, token windows.Token, selected windows.LUID) uint32 {
	t.Helper()
	// The diagnostic accepts at most 4096 bytes of token privilege data.
	var data [4096]byte
	var used uint32
	if err := windows.GetTokenInformation(token, windows.TokenPrivileges, &data[0], uint32(len(data)), &used); err != nil {
		t.Fatal(err)
	}
	if used < uint32(unsafe.Sizeof(windows.Tokenprivileges{})) || used > uint32(len(data)) {
		t.Fatal("unexpected test token privilege buffer size")
	}
	privileges := (*windows.Tokenprivileges)(unsafe.Pointer(&data[0]))
	if privileges.PrivilegeCount > (used-4)/uint32(unsafe.Sizeof(windows.LUIDAndAttributes{})) {
		t.Fatal("unexpected test token privilege count")
	}
	for _, privilege := range privileges.AllPrivileges() {
		if privilege.Luid == selected {
			return privilege.Attributes
		}
	}
	t.Fatal("test account does not hold the time privilege")
	return 0
}
