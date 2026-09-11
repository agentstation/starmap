package starmap

import (
	"context"

	"github.com/agentstation/starmap/pkg/errors"
)

// PublicationGuard authorizes a mutation before candidate work or activation.
// A hosting runtime can reserve publication while exposing the client for reads and hooks.
// A queued operation checks guards again after entering its mutation transaction.
// Guards can run concurrently and must not mutate the client.
type PublicationGuard func(context.Context) error

// WithPublicationGuard adds a guard to Update, Activate, Reload, and Rollback. Every configured guard must permit the operation.
// Construction and catalog lookups do not call guards. A guard must not call a mutation on the same client.
func WithPublicationGuard(guard PublicationGuard) Option {
	return func(o *options) error {
		if guard == nil {
			return &errors.ValidationError{Field: "publication_guard", Message: "is required"}
		}
		o.publicationGuards = append(o.publicationGuards, guard)
		return nil
	}
}

func (c *Client) authorizePublication(ctx context.Context) error {
	for _, guard := range c.options.publicationGuards {
		if err := guard(ctx); err != nil {
			return err
		}
	}
	return nil
}
