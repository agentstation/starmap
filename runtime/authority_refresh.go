package runtime

import (
	"context"
	stderrors "errors"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/runtime/source"
)

const (
	// Permission reads have their own deadline and cadence, independent of payload transfers.
	permissionReadTimeout  = 15 * time.Second
	permissionPollInterval = time.Minute
)

// RefreshPermission reads and retains the configured authority's requirement.
// It runs independently of catalog refresh and the shared acquisition lease.
func (r *Runtime) RefreshPermission(ctx context.Context) error {
	if r == nil || ctx == nil {
		return &errors.ValidationError{Field: "permission_refresh", Message: "requires a runtime and context"}
	}
	if !r.requiresAuthority() {
		return &errors.ConfigError{Component: "catalog authority", Message: "require_authority is not selected"}
	}
	run, owner, err := r.permissionRuns.start(ctx, r.ctx, runKindPermission, "permission", 0)
	if err != nil {
		return err
	}
	if !owner {
		_, err := run.join(ctx)
		return err
	}
	stop := context.AfterFunc(ctx, run.cancel)
	defer stop()
	bounded, cancel := context.WithTimeout(run.ctx, permissionReadTimeout)
	defer cancel()
	err = r.readPermission(bounded)
	r.permissionRuns.finish(run, RefreshReport{}, err)
	return err
}

func (r *Runtime) readPermission(ctx context.Context) error {
	r.permissionIO.Lock()
	defer r.permissionIO.Unlock()
	reader, ok := r.source.(source.PermissionReader)
	if !ok {
		return &errors.ConfigError{Component: "catalog authority", Message: "source has no independent permission reader"}
	}
	r.mu.RLock()
	p := r.permissions
	r.mu.RUnlock()
	if err := r.store.savePermission(ctx, p, true); err != nil {
		r.mu.Lock()
		r.permissions.retained, r.permissions.pending = false, false
		r.mu.Unlock()
		return err
	}
	receipt, err := reader.ReadPermission(ctx)
	if err != nil {
		// A failed transfer does not extend the prior in-memory receipt.
		// The pending checkpoint requires fresh evidence after a restart.
		var parse *errors.ParseError
		if errors.IsValidationError(err) || stderrors.As(err, &parse) {
			r.mu.Lock()
			r.permissions.retained, r.permissions.pending = false, false
			r.mu.Unlock()
		}
		return err
	}
	r.mu.Lock()
	next, observeErr := r.permissions.observe(receipt)
	r.permissions = next
	r.mu.Unlock()
	if observeErr != nil {
		return observeErr
	}
	if err := r.store.savePermission(ctx, next, true); err != nil {
		return err
	}
	r.mu.Lock()
	next, err = r.permissions.confirmRetention(receipt)
	if err == nil {
		r.permissions = next
		r.activateAuthorityLocked(r.layers.source)
	}
	r.mu.Unlock()
	return err
}

// observeSourceAuthority learns a manifest requirement before payload activation.
// Permission reads remain independent and are the only source of finite receipts.
func (r *Runtime) observeSourceAuthority(ctx context.Context, manifest catalogs.GenerationManifest) error {
	if !r.requiresAuthority() {
		return nil
	}
	head := manifest.AuthorityHead
	if manifest.ManifestVersion != catalogs.AuthorityGenerationManifestVersion || head.GenerationID != manifest.GenerationID || head.PayloadChecksum != manifest.Payload.Checksum {
		r.mu.Lock()
		r.permissions.retained, r.permissions.pending = false, false
		r.mu.Unlock()
		return &errors.ValidationError{Field: "catalog_authority.manifest", Message: "must bind the authority head to the source generation"}
	}
	r.mu.Lock()
	p, err := r.permissions.observeHead(head)
	r.permissions = p
	r.mu.Unlock()
	if err != nil {
		return err
	}
	// Invalidate admission before waiting for an independent permission transfer.
	r.permissionIO.Lock()
	defer r.permissionIO.Unlock()
	r.mu.RLock()
	p = r.permissions
	r.mu.RUnlock()
	if p.retained && !p.pending {
		return nil
	}
	if err := r.store.savePermission(ctx, p, true); err != nil {
		return err
	}
	r.mu.Lock()
	next, err := r.permissions.confirmCheckpoint(p.required, p.highest)
	if err == nil {
		r.permissions = next
	}
	r.mu.Unlock()
	return err
}

func (r *Runtime) startPermissionSchedule() {
	if !r.requiresAuthority() {
		return
	}
	r.work.Go(func() {
		for {
			if err := r.RefreshPermission(r.ctx); err != nil && r.ctx.Err() == nil {
				r.logScheduledFailure(string(runKindPermission), err)
			}
			if !r.sleep(permissionPollInterval) {
				return
			}
		}
	})
}
