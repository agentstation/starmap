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

// FleetPublication retains one refresh grant or explicit recovery origin and its predecessor.
// A retry preserves all fields. It must not substitute newer ownership evidence or a predecessor.
type FleetPublication struct {
	Generation     catalogs.Generation  `json:"generation"`
	Recovery       FleetRecovery        `json:"recovery"`
	Grant          Lease                `json:"grant"`
	Expected       FleetHead            `json:"expected"`
	RecoveryOrigin *FleetRecoveryOrigin `json:"recovery_origin,omitempty"`
}

// Validate checks retained publication content and structural ownership evidence.
// It does not establish independent recovery approval, live lease ownership, or serving permission.
// Ordinary commit adapters must use ValidateRefreshPublication before their native transaction.
func (p FleetPublication) Validate() error {
	if err := p.Generation.Validate(); err != nil {
		return err
	}
	if err := p.Recovery.Validate(p.Generation); err != nil {
		return err
	}
	if p.RecoveryOrigin == nil {
		if err := p.Grant.validateFleet(); err != nil {
			return err
		}
	} else if err := p.RecoveryOrigin.validate(p); err != nil {
		return err
	}
	if err := p.Expected.Validate(); err != nil {
		return err
	}
	if p.Expected != (FleetHead{}) && p.Expected.Identity != p.publicationIdentity() {
		return fleetConflict("the predecessor belongs to a different recovery identity")
	}
	if p.Expected.Revision == math.MaxUint64 {
		return fleetConflict("the publication revision is exhausted")
	}
	return nil
}

// ValidateRefreshPublication refuses recovery imports through the ordinary refresh commit path.
// The backend must still compare the original live grant, expiry, approved identity, and head atomically.
func (p FleetPublication) ValidateRefreshPublication() error {
	if p.RecoveryOrigin != nil {
		return fleetConflict("recovery publication requires the host's closed import procedure")
	}
	return p.Validate()
}

func (p FleetPublication) publicationIdentity() FleetIdentity {
	if p.RecoveryOrigin != nil {
		return p.RecoveryOrigin.Identity
	}
	return p.Grant.Identity
}

// nextHead describes the result of one successful commit without granting ownership.
func (p FleetPublication) nextHead() FleetHead {
	return FleetHead{Identity: p.publicationIdentity(), Revision: p.Expected.Revision + 1,
		GenerationID: p.Generation.Manifest.GenerationID, RecoveryChecksum: p.Recovery.Checksum}
}

// FleetSnapshot retains the accepted publication and its original ownership evidence.
// Reading a snapshot proves neither a live grant nor independent recovery approval.
type FleetSnapshot struct {
	Head        FleetHead        `json:"head"`
	Publication FleetPublication `json:"publication"`
	Adoption    *FleetAdoption   `json:"adoption,omitempty"`
}

// FleetAdoption identifies explicit host recovery without changing the original publication ownership evidence.
// The host must verify the receipt and independent approval before exposing this snapshot.
// This record grants no refresh lease or inference permission.
type FleetAdoption struct {
	Previous FleetHead `json:"previous"`
	Receipt  string    `json:"receipt"`
}

// Validate checks that the head selects exactly this publication and its recovery inputs.
func (s FleetSnapshot) Validate() error {
	if err := s.Publication.Validate(); err != nil {
		return err
	}
	original := s.Publication.nextHead()
	if s.Adoption != nil {
		return s.Adoption.validate(original, s.Head)
	}
	if s.Head != original {
		return fleetConflict("the selected head does not match the retained publication")
	}
	return nil
}

func (a FleetAdoption) validate(original, selected FleetHead) error {
	if err := a.Previous.Validate(); err != nil {
		return err
	}
	if err := selected.Validate(); err != nil {
		return err
	}
	if !validFleetChecksum(a.Receipt) || a.Previous == (FleetHead{}) || selected == (FleetHead{}) ||
		a.Previous.Identity.DeploymentID != original.Identity.DeploymentID ||
		selected.Identity.DeploymentID != original.Identity.DeploymentID ||
		a.Previous.Identity.RecoveryEpoch < original.Identity.RecoveryEpoch ||
		(a.Previous.Identity.RecoveryEpoch == original.Identity.RecoveryEpoch && a.Previous.Identity != original.Identity) ||
		selected.Identity.RecoveryEpoch <= a.Previous.Identity.RecoveryEpoch {
		return fleetConflict("catalog adoption requires a receipt and a newer approved recovery identity")
	}
	previous, current := a.Previous, selected
	previous.Identity, current.Identity = original.Identity, original.Identity
	if previous != original || current != original {
		return fleetConflict("catalog adoption changed the original publication selection")
	}
	return nil
}

// FleetStore owns shared publication, retained inputs, and refresh ownership.
// Hosts supply the adapter. Standalone stores keep the separate storage.Store contract.
// Each method uses one deployment namespace and the approved backend incarnation.
// New or recovered connections must validate that incarnation before application operations.
type FleetStore interface {
	LeaseStore

	// CurrentHead reads publication metadata without transferring catalog or recovery payloads.
	CurrentHead(context.Context) (FleetHead, error)

	// CurrentPublication reads the complete durable head and the exact selected bytes.
	// Missing immutable bytes or recovery inputs are errors, never an empty-store result.
	CurrentPublication(context.Context) (FleetSnapshot, error)

	// Publication retrieves one retained publication by its complete head identity.
	Publication(context.Context, FleetHead) (FleetSnapshot, error)

	// Get retrieves a retained immutable catalog without exposing recovery inputs.
	Get(context.Context, string) (catalogs.Generation, error)

	// CommitPublication selects a refresh publication and recovery reference in one backend transaction.
	// It refuses RecoveryOrigin through ValidateRefreshPublication. Hosts import recovery separately.
	// It compares Expected, the exact holder, process session, epoch, live expiry, and recovery identity.
	// A refusal changes neither the head nor the recovery reference. Staged bytes confer no permission.
	//
	// An identical successful retry returns its original head while its receipt remains retained.
	// A retry after receipt collection refuses without another publication.
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
