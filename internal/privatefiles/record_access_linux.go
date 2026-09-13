package privatefiles

import (
	"encoding/json"
	stderrors "errors"
	"os"

	"golang.org/x/sys/unix"
)

func nativePublicationACL(file *os.File) ([]byte, error) {
	attributes := make(map[string][]byte, 2)
	for _, name := range []string{"system.posix_acl_access", "system.posix_acl_default"} {
		buffer := make([]byte, publicationACLMaxBytes)
		n, err := unix.Fgetxattr(int(file.Fd()), name, buffer)
		switch {
		case stderrors.Is(err, unix.ENODATA):
			attributes[name] = nil
		case stderrors.Is(err, unix.ENOTSUP):
			attributes[name+".unsupported"] = nil
		case err != nil:
			return nil, err
		default:
			attributes[name] = buffer[:n]
		}
	}
	return json.Marshal(attributes)
}
