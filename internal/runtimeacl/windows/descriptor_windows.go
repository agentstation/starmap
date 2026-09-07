package windows

import (
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/agentstation/starmap/pkg/errors"
)

// DecodeDescriptor copies native owner and basic DACL entries into a policy value.
// Unsupported descriptors return an error without an access verdict.
func DecodeDescriptor(sd *windows.SECURITY_DESCRIPTOR, field string) (Descriptor, error) {
	invalid := func() error {
		return &errors.ValidationError{Field: field, Message: "native DACL metadata is incomplete or unsupported"}
	}
	if sd == nil || !sd.IsValid() {
		return Descriptor{}, invalid()
	}
	owner, _, err := sd.Owner()
	if err != nil || owner == nil || !owner.IsValid() {
		return Descriptor{}, invalid()
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		if err == windows.ERROR_OBJECT_NOT_FOUND {
			return Descriptor{Owner: owner.String()}, nil
		}
		return Descriptor{Owner: owner.String()}, errors.WrapResource("read", "runtime DACL", field, err)
	}
	if dacl == nil {
		return Descriptor{Owner: owner.String(), Present: true, Null: true}, nil
	}
	entries := make([]Entry, 0, int(dacl.AceCount))
	for index := uint32(0); index < uint32(dacl.AceCount); index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, index, &ace); err != nil {
			return Descriptor{}, invalid()
		}
		if ace == nil || (ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE && ace.Header.AceType != windows.ACCESS_DENIED_ACE_TYPE) {
			return Descriptor{}, invalid()
		}
		// A basic ACE contains an eight-byte header and an SID of at least eight bytes.
		if ace.Header.AceSize < 16 {
			return Descriptor{}, invalid()
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart)) //nolint:gosec // G103: GetAce supplies the SID field within a validated basic ACE.
		if !sid.IsValid() || sid.Len() > int(ace.Header.AceSize)-8 {
			return Descriptor{}, invalid()
		}
		entries = append(entries, Entry{Kind: ace.Header.AceType, Flags: ace.Header.AceFlags, Principal: sid.String(), Rights: uint32(ace.Mask)})
	}
	return Descriptor{Owner: owner.String(), Present: true, Entries: entries}, nil
}
