package runtime

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// FleetRecoveryOriginVersion identifies the explicit closed-import evidence schema.
const FleetRecoveryOriginVersion = 1

// FleetRecoveryOrigin binds catalog facts to a host-owned closed recovery import.
// The host must independently verify the accepted decision and closed import before exposing the publication.
// These digests describe retained evidence. They grant no refresh lease, authority receipt, or inference permission.
type FleetRecoveryOrigin struct {
	Version                  int           `json:"version"`
	Identity                 FleetIdentity `json:"identity"`
	OperationID              string        `json:"operation_id"`
	AcceptedDecisionSHA256   string        `json:"accepted_decision_sha256"`
	ClosedImportSHA256       string        `json:"closed_import_sha256"`
	GenerationManifestSHA256 string        `json:"generation_manifest_sha256"`
	RecoverySHA256           string        `json:"recovery_sha256"`
}

func (o FleetRecoveryOrigin) validate(publication FleetPublication) error {
	if publication.Grant != (Lease{}) {
		return fleetConflict("recovery origin must not carry a refresh grant")
	}
	if o.Version != FleetRecoveryOriginVersion || o.OperationID == "" || len(o.OperationID) > deploymentIDMaxBytes || strings.TrimSpace(o.OperationID) != o.OperationID || !utf8.ValidString(o.OperationID) || strings.ContainsFunc(o.OperationID, unicode.IsControl) {
		return fleetConflict("recovery origin requires its version and a bounded operation identity")
	}
	if err := o.Identity.Validate(); err != nil {
		return err
	}
	if !validFleetChecksum(o.AcceptedDecisionSHA256) || !validFleetChecksum(o.ClosedImportSHA256) || !validFleetChecksum(o.GenerationManifestSHA256) || !validFleetChecksum(o.RecoverySHA256) {
		return fleetConflict("recovery origin requires complete decision, import, and catalog digests")
	}
	manifest, err := catalogManifestChecksum(publication.Generation)
	if err != nil {
		return err
	}
	if manifest != o.GenerationManifestSHA256 || o.RecoverySHA256 != publication.Recovery.Checksum {
		return fleetConflict("recovery origin differs from its original generation or reconstruction inputs")
	}
	return nil
}
