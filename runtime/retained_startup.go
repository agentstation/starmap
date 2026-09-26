package runtime

import (
	"context"

	"github.com/agentstation/starmap"
)

func (r *Runtime) initializeRetainedState(ctx context.Context, initialFleet *FleetSnapshot) error {
	var err error
	r.store, err = newLayerStore(r.config.stateDirectory)
	if err != nil {
		return err
	}
	if err := r.store.recoverRecordPublications(ctx); err != nil {
		return err
	}
	if r.config.fleetStore != nil {
		if err := r.initializeFleetLayers(ctx, initialFleet); err != nil {
			return err
		}
	} else {
		if err := r.store.recoverInputPublication(ctx, r.client.CurrentCatalogState()); err != nil {
			return err
		}
		if err := r.loadRetainedLayers(ctx); err != nil {
			return err
		}
	}
	if err := r.initializePinRecord(); err != nil {
		return err
	}
	if err := r.initializeEffective(ctx); err != nil {
		return err
	}
	if err := r.initializeAuthority(); err != nil {
		return err
	}
	if err := r.initializeSchedule(); err != nil {
		return err
	}
	return nil
}

// openServingClient reads one selected publication and verifies its serving authority.
func (config options) openServingClient(ctx context.Context) (*starmap.Client, *FleetSnapshot, error) {
	clientContext, initialFleet, err := config.readFleetBootstrap(ctx)
	if err != nil {
		return nil, nil, err
	}
	client, err := starmap.NewContext(clientContext, config.acquisitionPolicyClientOptions()...)
	if err != nil {
		return nil, nil, err
	}
	if err := config.validateStoredAuthoritySelection(client.CurrentCatalogState().AuthorityHead); err != nil {
		return nil, nil, err
	}
	if err := repairWorkspaceForStartup(ctx, client); err != nil {
		return nil, nil, err
	}
	return client, initialFleet, nil
}

// initializeRefreshOwnership binds startup publication to the acquired grant.
func (r *Runtime) initializeRefreshOwnership(ctx context.Context) (context.Context, error) {
	r.adoptSourceIdentity()
	r.lease = newLeaseKeeper(r.config.leaseStore, r.schedule.identity.Instance, r.config.now)
	if err := r.lease.start(r.ctx, &r.work, r.onLeaseLost, !r.originFollowed && r.fleetReplayError == nil); err != nil {
		return nil, err
	}
	if r.config.fleetStore != nil && r.lease.status() == leaseHeld {
		return r.captureFleetGrant(ctx, r.lease.epoch())
	}
	return ctx, nil
}
