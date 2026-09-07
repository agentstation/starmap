package windowstoken

import (
	"reflect"
	"runtime"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWithoutPrivilegesRestoresTokens(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	processBefore := tokenPrivileges(t, windows.GetCurrentProcessToken())
	threadBefore := tokenPrivileges(t, windows.GetCurrentThreadEffectiveToken())
	called := false
	WithoutPrivileges(t, func() {
		called = true
		for _, privilege := range tokenPrivileges(t, windows.GetCurrentThreadEffectiveToken()) {
			if privilege.Attributes&windows.SE_PRIVILEGE_ENABLED != 0 {
				t.Fatal("callback retained enabled privileges")
			}
		}
		outer := tokenPrivileges(t, windows.GetCurrentThreadEffectiveToken())
		nested := false
		WithoutPrivileges(t, func() { nested = true })
		if !nested || !reflect.DeepEqual(outer, tokenPrivileges(t, windows.GetCurrentThreadEffectiveToken())) {
			t.Fatal("nested callback did not restore the existing thread token")
		}
	})
	if !called {
		t.Fatal("callback did not run")
	}
	if !reflect.DeepEqual(threadBefore, tokenPrivileges(t, windows.GetCurrentThreadEffectiveToken())) {
		t.Fatal("thread privileges changed after callback")
	}
	if !reflect.DeepEqual(processBefore, tokenPrivileges(t, windows.GetCurrentProcessToken())) {
		t.Fatal("process privileges changed during callback")
	}
}
