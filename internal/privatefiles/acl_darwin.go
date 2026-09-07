package privatefiles

import (
	"encoding/binary"
	"io/fs"
	"math"
	"os"
	"sync"
	"syscall"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"

	"github.com/agentstation/starmap/pkg/errors"
)

// These layouts follow the macOS SDK sys/attr.h, sys/kauth.h, and membership.h.
const (
	darwinAttributeBufferSize = 8192
	darwinExtendedSecurity    = 0x00400000
	darwinReportFullSize      = 0x00000004
	darwinFileSecurityMagic   = 0x012cc16d
	darwinMaximumACLEntries   = 128
)

type darwinAttributeList struct {
	BitmapCount uint16
	Reserved    uint16
	Common      uint32
	Volume      uint32
	Directory   uint32
	File        uint32
	Fork        uint32
}

type darwinACLFunctions struct {
	attributes func(int32, *darwinAttributeList, *byte, uintptr, uintptr) int32
	ownerUUID  func(uint32, *[16]byte) int32
}

// The process retains one library reference while these function pointers remain live.
var nativeDarwinACL = sync.OnceValues(func() (darwinACLFunctions, error) {
	var api darwinACLFunctions
	library, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return api, err
	}
	attributes, err := purego.Dlsym(library, "fgetattrlist")
	if err != nil {
		_ = purego.Dlclose(library)
		return api, err
	}
	ownerUUID, err := purego.Dlsym(library, "mbr_uid_to_uuid")
	if err != nil {
		_ = purego.Dlclose(library)
		return api, err
	}
	purego.RegisterFunc(&api.attributes, attributes)
	purego.RegisterFunc(&api.ownerUUID, ownerUUID)
	return api, nil
})

// ValidateACL checks native grants on the expected file without changing its ACL.
func ValidateACL(root *os.Root, name string, expected fs.FileInfo, field string) error {
	flags := os.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK
	if expected.IsDir() {
		flags |= unix.O_DIRECTORY
	}
	file, err := root.OpenFile(name, flags, 0)
	if err != nil {
		return errors.WrapResource("inspect", "runtime ACL", field, err)
	}
	defer func() { _ = file.Close() }()
	actual, err := file.Stat()
	if err != nil {
		return err
	}
	current, err := root.Lstat(name)
	if err != nil || !os.SameFile(expected, actual) || !os.SameFile(expected, current) || current.Mode()&os.ModeSymlink != 0 {
		return &errors.ConflictError{Resource: field, Message: "file changed during ACL inspection"}
	}
	if err := ValidateMetadata(actual, field); err != nil {
		return err
	}
	api, err := nativeDarwinACL()
	if err != nil {
		return errors.WrapResource("load", "native ACL API", field, err)
	}
	buffer, err := darwinACLBuffer(api, file, field)
	if err != nil {
		return err
	}
	grants, err := darwinACLGrants(buffer)
	if err != nil {
		return errors.WrapResource("parse", "native ACL", field, err)
	}
	if len(grants) == 0 {
		return nil
	}
	stat, ok := actual.Sys().(*syscall.Stat_t)
	if !ok {
		return &errors.ConfigError{Component: field, Message: "native owner metadata is unavailable"}
	}
	var owner [16]byte
	if api.ownerUUID(stat.Uid, &owner) != 0 || owner == [16]byte{} {
		return &errors.ConfigError{Component: field, Message: "native ACL owner identity is unavailable"}
	}
	for _, principal := range grants {
		if principal != owner {
			return &errors.ValidationError{Field: field, Message: "ACL grants access beyond the file owner. Review native ACL entries before retrying"}
		}
	}
	return nil
}

// darwinACLGrants validates the complete kernel buffer before returning grant principals.
// Deny entries remain valid. Inherited and inheritance-only grants follow the same owner rule.
func darwinACLGrants(buffer []byte) ([][16]byte, error) {
	return darwinACLGrantsForRights(buffer, math.MaxUint32)
}

func darwinACLBuffer(api darwinACLFunctions, file *os.File, field string) ([]byte, error) {
	attributes := darwinAttributeList{BitmapCount: 5, Common: darwinExtendedSecurity}
	buffer := make([]byte, darwinAttributeBufferSize)
	descriptor := file.Fd()
	if descriptor > math.MaxInt32 {
		return nil, &errors.ConfigError{Component: field, Message: "native file descriptor is outside the supported range"}
	}
	if api.attributes(int32(descriptor), &attributes, &buffer[0], uintptr(len(buffer)), darwinReportFullSize) != 0 {
		return nil, &errors.ConfigError{Component: field, Message: "native ACL verification failed. Check filesystem ACL support and read access"}
	}
	return buffer, nil
}

func darwinACLGrantsForRights(buffer []byte, selectedRights uint32) ([][16]byte, error) {
	invalid := func() error {
		return &errors.ValidationError{Field: "runtime.acl", Message: "native ACL metadata is incomplete or unsupported"}
	}
	if len(buffer) < 12 {
		return nil, invalid()
	}
	total := int64(binary.LittleEndian.Uint32(buffer[:4]))
	offset := int64(binary.LittleEndian.Uint32(buffer[4:8]))
	length := int64(binary.LittleEndian.Uint32(buffer[8:12]))
	if total < 12 || total > int64(len(buffer)) || offset < 8 || offset > math.MaxInt32 || 4+offset+length > total {
		return nil, invalid()
	}
	if length == 0 {
		return nil, nil
	}
	acl := buffer[4+offset : 4+offset+length]
	if len(acl) < 44 || binary.LittleEndian.Uint32(acl[:4]) != darwinFileSecurityMagic {
		return nil, invalid()
	}
	count := binary.LittleEndian.Uint32(acl[36:40])
	if count == 0xffffffff {
		count = 0
	}
	if count > darwinMaximumACLEntries || len(acl) != 44+int(count)*24 {
		return nil, invalid()
	}
	var grants [][16]byte
	for index := range count {
		entry := acl[44+int(index)*24 : 44+int(index+1)*24]
		kind := binary.LittleEndian.Uint32(entry[16:20]) & 0xf
		rights := binary.LittleEndian.Uint32(entry[20:24])
		switch kind {
		case 1:
			if rights&selectedRights != 0 {
				grants = append(grants, [16]byte(entry[:16]))
			}
		case 2:
		default:
			return nil, invalid()
		}
	}
	return grants, nil
}
