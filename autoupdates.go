package starmap

import (
	"context"
	stderrors "errors"
	"time"

	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
)

// Compile-time interface check to ensure proper implementation.
var _ AutoUpdater = (*starmap)(nil)

// AutoUpdater provides controls for automatic catalog updates.
type AutoUpdater interface {
	// AutoUpdatesOn begins automatic updates if configured
	AutoUpdatesOn() error

	// AutoUpdatesOff stops automatic updates
	AutoUpdatesOff() error
}

// AutoUpdatesOn begins automatic updates if configured.
func (s *starmap) AutoUpdatesOn() error {
	if s.options.autoUpdateInterval <= 0 {
		return &errors.ValidationError{
			Field:   "autoUpdateInterval",
			Value:   s.options.autoUpdateInterval,
			Message: "update interval must be positive",
		}
	}

	// Stop any existing auto-updates to prevent resource leaks
	if err := s.AutoUpdatesOff(); err != nil {
		return err
	}

	// Recreate stopCh since it was closed in AutoUpdatesOff
	s.stopCh = make(chan struct{})

	s.updateTicker = time.NewTicker(s.options.autoUpdateInterval)

	// Create a cancellable context for the update goroutine
	ctx, cancel := context.WithCancel(context.Background())
	s.updateCancel = cancel

	go func(parentCtx context.Context) {
		for {
			select {
			case <-s.updateTicker.C:
				// Create a timeout context for each update (5 minutes default)
				updateCtx, updateCancel := context.WithTimeout(parentCtx, constants.UpdateContextTimeout)
				err := s.Update(updateCtx)
				updateCancel() // Always cancel to release resources

				if err != nil {
					// Check if context was canceled - if so, exit the loop
					if stderrors.Is(err, context.Canceled) || stderrors.Is(err, context.DeadlineExceeded) {
						return
					}
					// Log other errors but continue
					logging.Error().Err(err).Msg("Auto-update failed")
				}
			case <-parentCtx.Done():
				return
			case <-s.stopCh:
				return
			}
		}
	}(ctx)

	return nil
}

// AutoUpdatesOff stops automatic updates.
func (s *starmap) AutoUpdatesOff() error {
	if s.updateTicker != nil {
		s.updateTicker.Stop()
		s.updateTicker = nil
	}
	if s.updateCancel != nil {
		s.updateCancel()
		s.updateCancel = nil
	}
	select {
	case <-s.stopCh:
		// Already closed
	default:
		close(s.stopCh)
	}
	return nil
}
