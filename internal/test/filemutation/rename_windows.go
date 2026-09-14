package filemutation

import (
	"errors"

	"golang.org/x/sys/windows"
)

func nativeRenameRefusal(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_LOCK_VIOLATION)
}
