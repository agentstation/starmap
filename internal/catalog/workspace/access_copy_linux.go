package workspace

import (
	"encoding/json"
	stderrors "errors"
	"os"

	"golang.org/x/sys/unix"
)

func setNativeEntryACL(file *os.File, data []byte) error {
	var attributes map[string][]byte
	if len(data) != 0 {
		if err := json.Unmarshal(data, &attributes); err != nil {
			return err
		}
	}
	for _, name := range []string{"system.posix_acl_access", "system.posix_acl_default"} {
		value := attributes[name]
		if len(value) == 0 {
			err := unix.Fremovexattr(int(file.Fd()), name)
			if err != nil && !stderrors.Is(err, unix.ENODATA) && !stderrors.Is(err, unix.ENOTSUP) {
				return err
			}
		} else if err := unix.Fsetxattr(int(file.Fd()), name, value, 0); err != nil {
			return err
		}
	}
	return nil
}
