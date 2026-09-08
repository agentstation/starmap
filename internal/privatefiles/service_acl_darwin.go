package privatefiles

import (
	"math"
	"os"

	"github.com/agentstation/starmap/pkg/errors"
)

// These file read and synchronization rights follow the macOS SDK sys/kauth.h.
const darwinConfigurationReadRights uint32 = 1<<1 | 1<<7 | 1<<9 | 1<<11 | 1<<20

func validateDarwinServiceGrants(api darwinACLFunctions, buffer []byte, field string) error {
	grants, err := darwinACLGrantsForRights(buffer, ^darwinConfigurationReadRights)
	if err != nil || len(grants) == 0 {
		return err
	}
	current := os.Geteuid()
	if current < 0 || current > math.MaxUint32 {
		return &errors.ConfigError{Component: field, Message: "effective user ID is outside the supported native range"}
	}
	trusted := make(map[[16]byte]bool)
	for _, uid := range []uint32{0, uint32(current)} {
		var principal [16]byte
		if api.ownerUUID(uid, &principal) != 0 || principal == [16]byte{} {
			return &errors.ConfigError{Component: field, Message: "native trusted owner identity is unavailable"}
		}
		trusted[principal] = true
	}
	for _, principal := range grants {
		if !trusted[principal] {
			return &errors.ValidationError{Field: field, Message: "ACL permits an untrusted account to change configuration. Review native grants before retrying"}
		}
	}
	return nil
}
