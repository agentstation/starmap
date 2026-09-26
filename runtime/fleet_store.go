package runtime

import (
	"context"
	"math"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// FleetIdentity binds operations to an independently approved backend incarnation.
// The host gets this identity from its recovery authority, outside the catalog store.
// An address or a record restored inside that backend cannot establish this identity.
type FleetIdentity struct {
	DeploymentID  string `json:"deployment_id"`
	RecoveryEpoch uint64 `json:"recovery_epoch"`
	BackendID     string `json:"backend_id"`
}

// Validate requires a complete identity. It does not establish external approval.
func (i FleetIdentity) Validate() error {
	if i.DeploymentID == "" || i.RecoveryEpoch == 0 || i.BackendID == "" {
		return fleetConflict("the approved deployment, recovery epoch, and backend identity are required")
	}
	return nil
}

// FleetHead identifies one accepted publication, including its private recovery inputs.
// Revision increases for every publication, even when the catalog generation stays unchanged.
// The zero value selects an empty store. A populated head cannot use revision zero.
type FleetHead struct {
	Identity         FleetIdentity `json:"identity"`
	Revision         uint64        `json:"revision"`
	GenerationID     string        `json:"generation_id"`
	RecoveryChecksum string        `json:"recovery_checksum"`
}

// Validate accepts an empty head or a complete publication identity.
func (h FleetHead) Validate() error {
	if h == (FleetHead{}) {
		return nil
	}
	if err := h.Identity.Validate(); err != nil {
		return err
	}
	if h.Revision == 0 || h.GenerationID == "" || !validFleetChecksum(h.RecoveryChecksum) {
		return fleetConflict("the publication head has an incomplete revision or recovery identity")
	}
	return nil
}

// FleetPublication carries the original grant and predecessor for one commit attempt.
// A retry preserves all fields. It must not substitute a newer grant or predecessor.
type FleetPublication struct {
	Generation catalogs.Generation `json:"generation"`
	Recovery   FleetRecovery       `json:"recovery"`
	Grant      Lease               `json:"grant"`
	Expected   FleetHead           `json:"expected"`
}

// Validate checks publication content and identity before a backend operation.
// The backend must still compare the live grant, expiry, approved identity, and head atomically.
// No local clock reading can replace that comparison.
func (p FleetPublication) Validate() error {
	if err := p.Generation.Validate(); err != nil {
		return err
	}
	if err := p.Recovery.Validate(p.Generation); err != nil {
		return err
	}
	if err := p.Grant.validateFleet(); err != nil {
		return err
	}
	if err := p.Expected.Validate(); err != nil {
		return err
	}
	if p.Expected != (FleetHead{}) && p.Expected.Identity != p.Grant.Identity {
		return fleetConflict("the predecessor belongs to a different recovery identity")
	}
	if p.Expected.Revision == math.MaxUint64 {
		return fleetConflict("the publication revision is exhausted")
	}
	return nil
}

// nextHead describes the result of one successful commit without granting ownership.
func (p FleetPublication) nextHead() FleetHead {
	return FleetHead{Identity: p.Grant.Identity, Revision: p.Expected.Revision + 1,
		GenerationID: p.Generation.Manifest.GenerationID, RecoveryChecksum: p.Recovery.Checksum}
}

// FleetSnapshot retains the accepted publication and its original ownership evidence.
// Reading a snapshot does not prove that its grant remains valid.
type FleetSnapshot struct {
	Head        FleetHead        `json:"head"`
	Publication FleetPublication `json:"publication"`
}

// Validate checks that the head selects exactly this publication and its recovery inputs.
func (s FleetSnapshot) Validate() error {
	if err := s.Publication.Validate(); err != nil {
		return err
	}
	if s.Head != s.Publication.nextHead() {
		return fleetConflict("the selected head does not match the retained publication")
	}
	return nil
}

// FleetStore owns shared publication, retained inputs, and refresh ownership.
// Hosts supply the adapter. Standalone stores keep the separate storage.Store contract.
// Each method uses one deployment namespace and the approved backend incarnation.
// New or recovered connections must validate that incarnation before application operations.
type FleetStore interface {
	LeaseStore

	// CurrentPublication reads the complete durable head and the exact selected bytes.
	// Missing immutable bytes or recovery inputs are errors, never an empty-store result.
	CurrentPublication(context.Context) (FleetSnapshot, error)

	// Publication retrieves one retained publication by its complete head identity.
	Publication(context.Context, FleetHead) (FleetSnapshot, error)

	// Get retrieves a retained immutable catalog without exposing recovery inputs.
	Get(context.Context, string) (catalogs.Generation, error)

	// CommitPublication selects the catalog and recovery reference in one backend transaction.
	// It compares Expected, the exact holder, process session, epoch, live expiry, and recovery identity.
	// A refusal changes neither the head nor the recovery reference. Staged bytes confer no permission.
	// An identical successful retry returns its original head without another publication.
	// The store must prove that retry from the retained request, including its original grant.
	//
	// Lease expiry must not erase the durable epoch. Reuse of an active holder by another session fails.
	CommitPublication(context.Context, FleetPublication) (FleetHead, error)
}

func (l Lease) validateFleet() error {
	if l.Holder == "" || l.Epoch == 0 || l.SessionID == "" {
		return fleetConflict("the original holder, process session, and lease epoch are required")
	}
	return l.Identity.Validate()
}

func fleetConflict(message string) error {
	return &errors.ConflictError{Resource: "fleet publication", Message: message}
}
