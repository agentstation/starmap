package windows

import (
	"testing"

	native "golang.org/x/sys/windows"
)

func TestNativeDescriptorPreservesDACLStateAndFlags(t *testing.T) {
	const owner = "S-1-5-21-1-2-3-1001"
	for _, test := range []struct {
		name, sddl    string
		present, null bool
		count         int
	}{
		{"absent", "O:" + owner, false, false, 0},
		{"null", "O:" + owner + "D:NO_ACCESS_CONTROL", true, true, 0},
		{"empty", "O:" + owner + "D:P", true, false, 0},
		{"inherited", "O:" + owner + "D:P(A;OICIIO;GR;;;WD)", true, false, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			sd, err := native.SecurityDescriptorFromString(test.sddl)
			if err != nil {
				t.Fatal(err)
			}
			result, err := DecodeDescriptor(sd, "test")
			if err != nil || result.Owner != owner || result.Present != test.present || result.Null != test.null || len(result.Entries) != test.count {
				t.Fatalf("descriptor state changed: %+v %v", result, err)
			}
			if test.count != 0 && result.Entries[0].Flags&native.INHERIT_ONLY_ACE == 0 {
				t.Fatal("decoder lost native inheritance flags")
			}
		})
	}
	if _, err := DecodeDescriptor(nil, "test"); err == nil {
		t.Fatal("nil native descriptor returned a usable observation")
	}
}
