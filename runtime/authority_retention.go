package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	permissionCheckpointFile     = "permission.json"
	permissionCheckpointVersion  = 1
	maxPermissionCheckpointBytes = 32 << 10
)

// permissionCheckpoint stays uncertain for the lifetime of a connected runtime.
// A completed shutdown clears it only after every reader stops and known requirements are durable.
type permissionCheckpoint struct {
	Version     int                           `json:"version"`
	AuthorityID string                        `json:"authority_id"`
	PolicyID    string                        `json:"policy_id"`
	Receipt     json.RawMessage               `json:"receipt,omitempty"`
	Head        catalogs.CatalogAuthorityHead `json:"head,omitzero"`
	Uncertain   bool                          `json:"uncertain"`
}

func (s *layerStore) savePermission(ctx context.Context, p authorityPermissions, uncertain bool) error {
	if !s.durable() {
		return ctx.Err()
	}
	record := permissionCheckpoint{Version: permissionCheckpointVersion, AuthorityID: p.authorityID, PolicyID: p.policyID, Head: p.highest, Uncertain: uncertain}
	if p.required.Version != 0 {
		encoded, err := json.Marshal(p.required)
		if err != nil {
			return errors.WrapParse("encode permission receipt", "", err)
		}
		record.Receipt = encoded
	}
	return s.writeContext(ctx, s.directory, permissionCheckpointFile, record)
}

func (s *layerStore) loadPermission(p authorityPermissions) (authorityPermissions, error) {
	if !s.durable() {
		return p, nil
	}
	raw, err := s.directory.ReadFile(permissionCheckpointFile, maxPermissionCheckpointBytes)
	if os.IsNotExist(err) {
		return p, nil
	}
	if err != nil {
		return p, errors.WrapIO("read permission checkpoint", permissionCheckpointFile, err)
	}
	var record permissionCheckpoint
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return p, errors.WrapParse("permission checkpoint", "", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return p, &errors.ValidationError{Field: "permission_checkpoint", Message: "must contain one complete record"}
	}
	if record.Version != permissionCheckpointVersion {
		return p, &errors.ValidationError{Field: "permission_checkpoint.version", Message: "is unsupported"}
	}
	if record.AuthorityID != p.authorityID || record.PolicyID != p.policyID {
		// New configuration establishes a new authority context. Old permission cannot authorize it.
		return p, nil
	}
	if len(record.Receipt) != 0 {
		receipt, err := catalogs.ParseCatalogPermissionEnvelope(record.Receipt)
		if err != nil {
			return p, err
		}
		p, err = p.observe(receipt)
		if err != nil {
			return p, err
		}
	}
	if record.Head != (catalogs.CatalogAuthorityHead{}) {
		p, err = p.observeHead(record.Head)
		if err != nil {
			return p, err
		}
	}
	if record.Uncertain {
		p.pending = false
		return p, nil
	}
	if !p.pending {
		return p, nil
	}
	return p.confirmCheckpoint(p.required, p.highest)
}

func (r *Runtime) initializeAuthority() error {
	if !r.requiresAuthority() {
		return nil
	}
	p := authorityPermissions{authorityID: r.config.source.AuthorityID, policyID: r.config.source.PolicyID}
	p, err := r.store.loadPermission(p)
	if err != nil {
		return err
	}
	if err := r.store.savePermission(r.ctx, p, true); err != nil {
		return err
	}
	r.mu.Lock()
	r.permissions = p
	r.activateAuthorityLocked(r.layers.source)
	r.permissionInitialized = true
	r.mu.Unlock()
	return nil
}

func (r *Runtime) sealAuthority(ctx context.Context) error {
	if !r.requiresAuthority() {
		return nil
	}
	r.mu.RLock()
	p := r.permissions
	r.mu.RUnlock()
	return r.store.savePermission(ctx, p, !p.retained || p.pending)
}
