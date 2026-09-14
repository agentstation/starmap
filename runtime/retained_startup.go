package runtime

import "context"

func (r *Runtime) initializeRetainedState(ctx context.Context) error {
	var err error
	r.store, err = newLayerStore(r.config.stateDirectory)
	if err != nil {
		return err
	}
	if err := r.store.recoverRecordPublications(ctx); err != nil {
		return err
	}
	if err := r.store.recoverInputPublication(ctx, r.client.CurrentCatalogState()); err != nil {
		return err
	}
	if err := r.loadRetainedLayers(ctx); err != nil {
		return err
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
