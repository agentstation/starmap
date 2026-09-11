package catalogs

import (
	"strings"
	"testing"
)

func TestChecksumShapePreservesCanonicalDigest(t *testing.T) {
	cases := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "digits", value: "sha256:" + strings.Repeat("0123456789", 6) + "0123", valid: true},
		{name: "lowercase", value: "sha256:" + strings.Repeat("abcdef01", 8), valid: true},
		{name: "uppercase", value: "sha256:" + strings.Repeat("A", 64)},
		{name: "wrong prefix", value: "SHA256:" + strings.Repeat("a", 64)},
		{name: "missing prefix", value: strings.Repeat("a", 64)},
		{name: "short", value: "sha256:" + strings.Repeat("a", 63)},
		{name: "long", value: "sha256:" + strings.Repeat("a", 65)},
		{name: "nonhex", value: "sha256:" + strings.Repeat("g", 64)},
		{name: "unicode", value: "sha256:" + strings.Repeat("é", 32)},
		{name: "control", value: "sha256:" + strings.Repeat("a", 63) + "\x00"},
		{name: "invalid UTF-8", value: "sha256:" + strings.Repeat("a", 63) + "\xff"},
		{name: "empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateChecksum("payload.checksum", tc.value)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%t, error=%v", tc.valid, err)
			}
			if tc.valid {
				allocations := testing.AllocsPerRun(100, func() { _ = validateChecksum("payload.checksum", tc.value) })
				if allocations != 0 {
					t.Fatalf("valid checksum allocated %v times", allocations)
				}
			}
		})
	}
}
