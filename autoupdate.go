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
var _ AutoUpdater = (*client)(nil)

// AutoUpdater provides controls for automatic catalog updates.
type AutoUpdater interface {
	// AutoUpdatesOn begins automatic updates if configured
	AutoUpdatesOn() error

	// AutoUpdatesOff stops automatic updates
	AutoUpdatesOff() error
}

// AutoUpdatesOn begins automatic updates if configured.
func (c *client) AutoUpdatesOn() error {
	if c.options.autoUpdateInterval <= 0 {
		return &errors.ValidationError{
			Field:   "autoUpdateInterval",
			Value:   c.options.autoUpdateInterval,
			Message: "update interval must be positive",
		}
	}

	// Stop any existing auto-updates to prevent resource leaks
	if err := c.AutoUpdatesOff(); err != nil {
		return err
	}

	// Recreate stopCh since it was closed in AutoUpdatesOff
	c.stopCh = make(chan struct{})

	c.updateTicker = time.NewTicker(c.options.autoUpdateInterval)

	// Create a cancellable context for the update goroutine
	ctx, cancel := context.WithCancel(context.Background())
	c.updateCancel = cancel

	go func(parentCtx context.Context) {
		for {
			select {
			case <-c.updateTicker.C:
				// Create a timeout context for each update (5 minutes default)
				updateCtx, updateCancel := context.WithTimeout(parentCtx, constants.UpdateContextTimeout)
				err := c.Update(updateCtx)
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
			case <-c.stopCh:
				return
			}
		}
	}(ctx)

	return nil
}

// AutoUpdatesOff stops automatic updates.
func (c *client) AutoUpdatesOff() error {
	if c.updateTicker != nil {
		c.updateTicker.Stop()
		c.updateTicker = nil
	}
	if c.updateCancel != nil {
		c.updateCancel()
		c.updateCancel = nil
	}
	select {
	case <-c.stopCh:
		// Already closed
	default:
		close(c.stopCh)
	}
	return nil
}
