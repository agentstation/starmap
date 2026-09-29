package starmap

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// GenerationRetainer retains caller-owned evidence before a catalog store commit.
// An error prevents the commit. Invocation does not prove that the commit succeeded.
// Retainers must support retries and must not call mutations on the same client.
// The generation is a private copy. Changing it cannot change the publication.
type GenerationRetainer func(context.Context, catalogs.Generation) error

// WithGenerationRetainer adds an explicit evidence retention step to catalog commits.
// Construction and reads do not retain evidence. Each callback runs inside the mutation transaction.
func WithGenerationRetainer(retainer GenerationRetainer) Option {
	return func(o *options) error {
		if retainer == nil {
			return &errors.ValidationError{Field: "generation_retainer", Message: "is required"}
		}
		o.generationRetainers = append(o.generationRetainers, retainer)
		return nil
	}
}

func (c *Client) retainGeneration(ctx context.Context, generation catalogs.Generation) error {
	for _, retain := range c.options.generationRetainers {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := retain(ctx, generation.Copy()); err != nil {
			return err
		}
	}
	return ctx.Err()
}
