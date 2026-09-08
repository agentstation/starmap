package workspace

import (
	"encoding/json"
	"io/fs"
	"os"

	"golang.org/x/sys/windows"

	"github.com/agentstation/starmap/pkg/errors"
)

const workspaceWindowsSecurity = windows.OWNER_SECURITY_INFORMATION | windows.GROUP_SECURITY_INFORMATION | windows.DACL_SECURITY_INFORMATION |
	windows.LABEL_SECURITY_INFORMATION | windows.ATTRIBUTE_SECURITY_INFORMATION | windows.SCOPE_SECURITY_INFORMATION

func nativeEntryAccess(file *os.File) ([]byte, error) {
	sd, err := windows.GetSecurityInfo(windows.Handle(file.Fd()), windows.SE_FILE_OBJECT,
		workspaceWindowsSecurity)
	if err != nil {
		return nil, err
	}
	if sd == nil || !sd.IsValid() {
		return nil, invalidWorkspaceDescriptor()
	}
	control, _, err := sd.Control()
	if err != nil {
		return nil, err
	}
	value := sd.String()
	if value == "" || len(value) > workspaceACLMaxBytes {
		return nil, invalidWorkspaceDescriptor()
	}
	return json.Marshal(struct {
		Control uint16 `json:"control"`
		SDDL    string `json:"sddl"`
	}{uint16(control), value})
}

func invalidWorkspaceDescriptor() error {
	return &errors.ConfigError{Component: "workspace access", Message: "native security descriptor is unavailable or exceeds the size limit"}
}

func openSnapshotEntry(root *os.Root, name string, _ fs.FileInfo) (*os.File, error) {
	return root.Open(name)
}
