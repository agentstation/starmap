package policy

import "io/fs"

const (
	// GroupOrOtherModeBits identifies mode bits that permit access outside the owner class.
	GroupOrOtherModeBits = "group-or-other-mode-bits"
	// DifferentEffectiveOwner identifies a known owner that differs from the process account.
	DifferentEffectiveOwner = "different-effective-owner"
	// OwnerUnavailable identifies missing native ownership evidence.
	OwnerUnavailable = "owner-unavailable"
)

// POSIXMetadata contains observed mode and ownership facts without filesystem access.
type POSIXMetadata struct {
	Mode         fs.FileMode
	OwnerKnown   bool
	OwnerMatches bool
	OwnerTrusted bool
}

// PrivatePOSIXReason classifies owner-only mode and ownership observations.
// An empty reason establishes only metadata compatibility, not ACL safety or usable access.
func PrivatePOSIXReason(observed POSIXMetadata) string {
	if observed.Mode.Perm()&0o077 != 0 {
		return GroupOrOtherModeBits
	}
	if !observed.OwnerKnown {
		return OwnerUnavailable
	}
	if !observed.OwnerMatches {
		return DifferentEffectiveOwner
	}
	return ""
}

// ServicePOSIXReason checks trusted ownership and rejects shared mutation rights.
// The service policy permits shared reads. Native ACL checks and a successful read remain required.
func ServicePOSIXReason(observed POSIXMetadata) string {
	if !observed.OwnerKnown {
		return OwnerUnavailable
	}
	if !observed.OwnerTrusted {
		return "untrusted-configuration-owner"
	}
	if observed.Mode.Perm()&0o022 != 0 {
		return "group-or-other-write-bits"
	}
	return ""
}
