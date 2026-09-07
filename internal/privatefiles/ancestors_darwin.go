package privatefiles

import (
	"io/fs"
	"math"
	"os"

	"github.com/agentstation/starmap/pkg/errors"
)

// These read, search, and synchronization rights follow the macOS SDK sys/kauth.h.
const darwinAncestorReadRights uint32 = 1<<1 | 1<<3 | 1<<7 | 1<<9 | 1<<11 | 1<<20

func validateAncestorACL(root *os.Root, expected fs.FileInfo) error {
	file, err := root.Open(".")
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	actual, err := file.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(expected, actual) {
		return changed(root.Name())
	}
	api, err := nativeDarwinACL()
	if err != nil {
		return err
	}
	buffer, err := darwinACLBuffer(api, file, "private.ancestor")
	if err != nil {
		return err
	}
	grants, err := darwinACLGrantsForRights(buffer, ^darwinAncestorReadRights)
	if err != nil || len(grants) == 0 {
		return err
	}
	trusted := make(map[[16]byte]bool)
	currentUID := os.Geteuid()
	if currentUID < 0 || currentUID > math.MaxUint32 {
		return &errors.ConfigError{Component: "private.ancestor", Message: "effective user ID is outside the supported native range"}
	}
	for _, uid := range []uint32{0, uint32(currentUID)} {
		var principal [16]byte
		if api.ownerUUID(uid, &principal) != 0 || principal == [16]byte{} {
			return &errors.ConfigError{Component: "private.ancestor", Message: "native trusted owner identity is unavailable"}
		}
		trusted[principal] = true
	}
	for _, principal := range grants {
		if !trusted[principal] {
			return &errors.ValidationError{Field: "private.ancestor", Message: "ACL permits another account to change the directory. Review parent ACL entries before retrying"}
		}
	}
	return nil
}
