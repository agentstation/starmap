package policy

import (
	"io/fs"
	"testing"
)

func TestConfigurationAccessSelection(t *testing.T) {
	for _, test := range []struct {
		name              string
		explicit, allowed bool
	}{
		{"", false, true}, {OwnerOnly, false, true}, {ServiceManaged, false, false},
		{ServiceManaged, true, true}, {"deployment-controlled", true, false}, {"invalid", true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := Configuration(test.name, test.explicit)
			if (err == nil) != test.allowed {
				t.Fatalf("selected=%q, error=%v", got, err)
			}
		})
	}
}

func TestServicePOSIXOwnershipAndWrites(t *testing.T) {
	for _, test := range []struct {
		name                    string
		mode                    fs.FileMode
		known, trusted, allowed bool
	}{
		{"root group readable", 0o640, true, true, true}, {"read only", 0o440, true, true, true},
		{"untrusted owner", 0o600, true, false, false}, {"unknown owner", 0o600, false, false, false},
		{"group writer", 0o660, true, true, false}, {"other writer", 0o602, true, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			reason := ServicePOSIXReason(POSIXMetadata{Mode: test.mode, OwnerKnown: test.known, OwnerTrusted: test.trusted})
			if (reason == "") != test.allowed {
				t.Fatalf("reason=%q", reason)
			}
		})
	}
}
