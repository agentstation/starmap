package administration

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Action identifies an administrative effect recorded before execution.
type Action string

const (
	// Bootstrap creates the first local administrator.
	Bootstrap Action = "bootstrap"
	// CreateIdentity adds an administrator or subscriber.
	CreateIdentity Action = "create_identity"
	// RotateIdentity replaces a credential with a bounded overlap.
	RotateIdentity Action = "rotate_identity"
	// RevokeIdentity withdraws all credentials for an identity.
	RevokeIdentity Action = "revoke_identity"
	// RefreshCatalog starts explicit catalog acquisition.
	RefreshCatalog Action = "refresh_catalog"
	// CancelOperation asks an accepted operation to stop.
	CancelOperation Action = "cancel_operation"
)

// State records the durable outcome of an administrative operation.
type State string

const (
	// Accepted means the audit intent is durable and effects may start.
	Accepted State = "accepted"
	// Succeeded means the effect and its audit outcome are durable.
	Succeeded State = "succeeded"
	// Failed means the operation reported a failure. Effects can be partial.
	Failed State = "failed"
	// Interrupted means recovery cannot prove the effect's outcome. It never replays the effect.
	Interrupted State = "interrupted"
)

// Receipt reports one administrative operation without credential material.
type Receipt struct {
	ID           string    `json:"id"`
	Audience     string    `json:"audience"`
	Actor        string    `json:"actor"`
	Action       Action    `json:"action"`
	Subject      string    `json:"subject"`
	State        State     `json:"state"`
	AcceptedAt   time.Time `json:"accepted_at"`
	CompletedAt  time.Time `json:"completed_at"`
	Revision     uint64    `json:"revision"`
	BeforeDigest string    `json:"before_digest,omitempty"`
	AfterDigest  string    `json:"after_digest,omitempty"`
}

func (a Action) valid() bool {
	switch a {
	case Bootstrap, CreateIdentity, RotateIdentity, RevokeIdentity, RefreshCatalog, CancelOperation:
		return true
	default:
		return false
	}
}

func (s State) valid() bool {
	return s == Accepted || s == Succeeded || s == Failed || s == Interrupted
}

func (r Receipt) valid(audience string) bool {
	if !validIdentity(r.ID) || r.Audience != audience || !validIdentity(r.Actor) || !r.Action.valid() || !validIdentity(r.Subject) ||
		!r.State.valid() || r.Revision == 0 || r.AcceptedAt.IsZero() {
		return false
	}
	if (r.State == Accepted) != r.CompletedAt.IsZero() || !r.CompletedAt.IsZero() && r.CompletedAt.Before(r.AcceptedAt) {
		return false
	}
	if r.AfterDigest == "" {
		return r.BeforeDigest == ""
	}
	return validDigest(r.AfterDigest) && (r.BeforeDigest == "" || validDigest(r.BeforeDigest))
}

func validDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && hex.EncodeToString(decoded) == value
}

func contentDigest(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func receiptName(id string) string { return contentDigest([]byte(id)) + ".json" }
