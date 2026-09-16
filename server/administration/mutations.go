package administration

import (
	"context"
	"slices"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

// MaximumRotationOverlap bounds the interval that accepts old and new credentials.
const MaximumRotationOverlap = 24 * time.Hour

// Create adds a distinct identity and returns its credential after durable audit completion.
func (m *Manager) Create(ctx context.Context, actor Principal, id string, role Role) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.authorize(actor); err != nil {
		return "", err
	}
	if !validIdentity(id) || !validRole(role) {
		return "", invalidIdentityRecord()
	}
	if m.findIdentity(id) >= 0 {
		return "", &errors.ConflictError{Resource: "server identity", Message: "identity already exists"}
	}
	if len(m.record.Identities) >= maximumIdentities {
		return "", invalidIdentityRecord()
	}
	token, key, err := newCredential(role, m.clock())
	if err != nil {
		return "", err
	}
	next := m.copyRecord()
	next.Identities = append(next.Identities, identityRecord{ID: id, Role: role, Keys: []credentialRecord{key}})
	if err := m.changeIdentities(ctx, actor.id, CreateIdentity, id, next); err != nil {
		return "", err
	}
	return token, nil
}

// Rotate returns a new credential and retains the prior active key for the selected overlap.
// A second rotation cannot discard a key whose overlap has not ended.
func (m *Manager) Rotate(ctx context.Context, actor Principal, id string, overlap time.Duration) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.authorize(actor); err != nil {
		return "", err
	}
	if overlap < 0 || overlap > MaximumRotationOverlap {
		return "", &errors.ValidationError{Field: "server.rotation_overlap", Message: "must be between zero and 24 hours"}
	}
	index := m.findIdentity(id)
	if index < 0 || m.record.Identities[index].Revoked {
		return "", &errors.NotFoundError{Resource: "active server identity", ID: id}
	}
	now := m.clock()
	next := m.copyRecord()
	identity := &next.Identities[index]
	active := slices.DeleteFunc(identity.Keys, func(key credentialRecord) bool { return !key.NotAfter.IsZero() && !now.Before(key.NotAfter) })
	if len(active) > 1 {
		return "", &errors.ConflictError{Resource: "credential rotation", Message: "the previous overlap has not ended"}
	}
	token, key, err := newCredential(identity.Role, now)
	if err != nil {
		return "", err
	}
	identity.Keys = []credentialRecord{key}
	if overlap > 0 && len(active) == 1 {
		previous := active[0]
		end := now.Add(overlap)
		if previous.NotAfter.IsZero() || previous.NotAfter.After(end) {
			previous.NotAfter = end
		}
		identity.Keys = append(identity.Keys, previous)
	}
	if err := m.changeIdentities(ctx, actor.id, RotateIdentity, id, next); err != nil {
		return "", err
	}
	return token, nil
}

// Revoke withdraws an identity while retaining at least one active administrator.
func (m *Manager) Revoke(ctx context.Context, actor Principal, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.authorize(actor); err != nil {
		return err
	}
	index := m.findIdentity(id)
	if index < 0 {
		return &errors.NotFoundError{Resource: "server identity", ID: id}
	}
	if m.record.Identities[index].Revoked {
		return nil
	}
	next := m.copyRecord()
	next.Identities[index].Revoked = true
	snapshot, err := buildIdentitySnapshot(next, m.audience)
	if err != nil {
		return err
	}
	if !hasAdministrator(snapshot, m.clock()) {
		return &errors.ConflictError{Resource: "server administrator", Message: "cannot revoke the last active administrator"}
	}
	return m.changeIdentities(ctx, actor.id, RevokeIdentity, id, next)
}

func (m *Manager) findIdentity(id string) int {
	return slices.IndexFunc(m.record.Identities, func(identity identityRecord) bool { return identity.ID == id })
}

func (m *Manager) copyRecord() identitiesRecord {
	next := m.record
	next.Revision++
	next.Identities = slices.Clone(m.record.Identities)
	for i := range next.Identities {
		next.Identities[i].Keys = slices.Clone(next.Identities[i].Keys)
	}
	return next
}

func hasAdministrator(snapshot *identitySnapshot, now time.Time) bool {
	for digest := range snapshot.keys {
		principal, valid := snapshot.authenticateDigest(digest, now)
		if valid && principal.role == Administrator {
			return true
		}
	}
	return false
}
