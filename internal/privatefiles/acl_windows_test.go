package privatefiles

import (
	stderrors "errors"
	"golang.org/x/sys/windows"
	"testing"
)

func TestWindowsNativeSecurityDescriptorPolicy(t *testing.T) {
	account, err := windowsProcessSID()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, sddl string
		invalid    bool
	}{
		{"owner", "O:" + account + "D:P(A;;FA;;;" + account + ")", false},
		{"empty", "O:" + account + "D:P", false},
		{"null", "O:" + account + "D:NO_ACCESS_CONTROL", true},
		{"absent", "O:" + account, true},
		{"deny", "O:" + account + "D:P(D;;SD;;;WD)(A;;FA;;;" + account + ")", false},
		{"inherit only public", "O:" + account + "D:P(A;OIIO;GR;;;WD)(A;;FA;;;" + account + ")", true},
		{"wrong owner", "O:BAD:P(A;;FA;;;" + account + ")", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			sd, err := windows.SecurityDescriptorFromString(test.sddl)
			if err != nil {
				t.Fatal(err)
			}
			err = validateWindowsSecurityDescriptor(sd, account, "runtime.directory")
			if test.name == "absent" && !stderrors.Is(err, windows.ERROR_OBJECT_NOT_FOUND) {
				t.Fatalf("absent DACL lost its native error identity: %v", err)
			}
			if (err != nil) != test.invalid {
				t.Fatalf("descriptor error = %v, invalid = %v", err, test.invalid)
			}
		})
	}
}
