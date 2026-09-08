package workspace

import (
	"encoding/binary"
	"math"
	"os"
	"sync"

	"github.com/ebitengine/purego"

	"github.com/agentstation/starmap/pkg/errors"
)

// These layouts follow the macOS SDK sys/attr.h and sys/kauth.h.
const (
	workspaceDarwinExtendedSecurity = 0x00400000
	workspaceDarwinReportFullSize   = 0x00000004
	workspaceDarwinSecurityMagic    = 0x012cc16d
	workspaceDarwinSecurityHeader   = 44
	workspaceDarwinACLEntryBytes    = 24
)

type workspaceDarwinAttributeList struct {
	BitmapCount uint16
	Reserved    uint16
	Common      uint32
	Volume      uint32
	Directory   uint32
	File        uint32
	Fork        uint32
}

type workspaceDarwinAttributes func(int32, *workspaceDarwinAttributeList, *byte, uintptr, uintptr) int32

// The process retains the library while its function pointer remains live.
var workspaceDarwinACL = sync.OnceValues(func() (workspaceDarwinAttributes, error) {
	library, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, err
	}
	address, err := purego.Dlsym(library, "fgetattrlist")
	if err != nil {
		_ = purego.Dlclose(library)
		return nil, err
	}
	var api workspaceDarwinAttributes
	purego.RegisterFunc(&api, address)
	return api, nil
})

func nativeEntryACL(file *os.File) ([]byte, error) {
	api, err := workspaceDarwinACL()
	if err != nil {
		return nil, errors.WrapResource("load", "workspace ACL API", file.Name(), err)
	}
	descriptor := file.Fd()
	if descriptor > math.MaxInt32 {
		return nil, workspaceACLError()
	}
	attributes := workspaceDarwinAttributeList{BitmapCount: 5, Common: workspaceDarwinExtendedSecurity}
	buffer := make([]byte, workspaceACLMaxBytes)
	if api(int32(descriptor), &attributes, &buffer[0], uintptr(len(buffer)), workspaceDarwinReportFullSize) != 0 {
		return nil, workspaceACLError()
	}
	return workspaceDarwinACLBytes(buffer)
}

func workspaceDarwinACLBytes(buffer []byte) ([]byte, error) {
	if len(buffer) < 12 {
		return nil, workspaceACLError()
	}
	total := int64(binary.LittleEndian.Uint32(buffer[:4]))
	offset := int64(binary.LittleEndian.Uint32(buffer[4:8]))
	length := int64(binary.LittleEndian.Uint32(buffer[8:12]))
	if total < 12 || total > int64(len(buffer)) || offset < 8 || offset > math.MaxInt32 || 4+offset+length > total {
		return nil, workspaceACLError()
	}
	if length == 0 {
		return nil, nil
	}
	data := buffer[4+offset : 4+offset+length]
	if len(data) < workspaceDarwinSecurityHeader || binary.LittleEndian.Uint32(data[:4]) != workspaceDarwinSecurityMagic {
		return nil, workspaceACLError()
	}
	count := binary.LittleEndian.Uint32(data[36:40])
	if count == math.MaxUint32 {
		count = 0
	}
	if int64(count)*workspaceDarwinACLEntryBytes+workspaceDarwinSecurityHeader != length {
		return nil, workspaceACLError()
	}
	return data, nil
}

func workspaceACLError() error {
	return &errors.ConfigError{Component: "workspace access", Message: "native ACL snapshot is unavailable or exceeds the size limit"}
}
