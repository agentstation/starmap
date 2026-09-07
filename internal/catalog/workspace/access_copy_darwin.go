package workspace

import (
	"encoding/binary"
	"math"
	"os"
	"sync"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/ebitengine/purego"
)

// The setter consumes an attrreference followed by native extended-security data.
var workspaceDarwinSetACL = sync.OnceValues(func() (workspaceDarwinAttributes, error) {
	library, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, err
	}
	address, err := purego.Dlsym(library, "fsetattrlist")
	if err != nil {
		_ = purego.Dlclose(library)
		return nil, err
	}
	var api workspaceDarwinAttributes
	purego.RegisterFunc(&api, address)
	return api, nil
})

func setNativeEntryACL(file *os.File, data []byte) error {
	if len(data) == 0 {
		data = make([]byte, workspaceDarwinSecurityHeader)
		binary.LittleEndian.PutUint32(data[:4], workspaceDarwinSecurityMagic)
		binary.LittleEndian.PutUint32(data[36:40], math.MaxUint32)
	}
	size := len(data)
	if size < 0 || size > workspaceACLMaxBytes {
		return workspaceACLError()
	}
	buffer := make([]byte, 8+len(data))
	binary.LittleEndian.PutUint32(buffer[:4], 8)
	binary.LittleEndian.PutUint32(buffer[4:8], uint32(size))
	copy(buffer[8:], data)
	api, err := workspaceDarwinSetACL()
	if err != nil {
		return err
	}
	descriptor := file.Fd()
	if descriptor > math.MaxInt32 {
		return workspaceACLError()
	}
	attributes := workspaceDarwinAttributeList{BitmapCount: 5, Common: workspaceDarwinExtendedSecurity}
	if api(int32(descriptor), &attributes, &buffer[0], uintptr(len(buffer)), 0) != 0 {
		return &errors.ConfigError{Component: "workspace access", Message: "native ACL restoration failed. Check ownership and ACL write access"}
	}
	return nil
}
