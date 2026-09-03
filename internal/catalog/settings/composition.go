package settings

import (
	"context"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/errors"
)

// Composition builds the connected runtime of one process. It joins the parsed
// canonical settings with the roles that the process injects: the catalog
// store, the provider acquirer, and an optional upstream source
// implementation.
//
// The parser accepts the starmap source kind. This composition step is the one
// place that rejects it, because the Starmap cascade subscriber arrives with
// the source-chain task that follows this one.
type Composition struct {
	// Config holds the canonical settings the process read.
	Config Config

	// Source supplies an upstream source implementation for a kind that the
	// build does not construct on its own. The Starmap cascade needs one.
	Source starmap.Source

	// Acquirer collects provider observations. A runtime without one observes
	// no provider.
	Acquirer starmap.Acquirer

	// LeaseStore fences the durable generation commit of a replicated
	// deployment. A single instance needs none.
	LeaseStore starmap.LeaseStore

	// Base holds options that the process supplies before every setting. A
	// canonical setting overrides a base option of the same name.
	Base []starmap.Option

	// Extra holds options that one command supplies last. The serve command
	// supplies its listen address here, so two replicas on one host take
	// separate schedule phases.
	Extra []starmap.Option
}

// Options returns the runtime options in precedence order: the process base
// options first, then one option for each supplied canonical setting, then the
// injected roles. A later option replaces an earlier one, so a canonical
// setting always wins over a base default.
func (c Composition) Options() ([]starmap.Option, error) {
	if c.Config.SourceKind == starmap.SourceStarmap && c.Source == nil {
		return nil, &errors.ConfigError{
			Component: "catalog source",
			Message: "the starmap catalog source is not yet available; " +
				"select public, github, file, or embedded",
		}
	}
	options := make([]starmap.Option, 0, len(c.Base)+len(c.Config.options)+len(c.Extra)+3)
	options = append(options, c.Base...)
	options = append(options, c.Config.options...)
	if c.Source != nil {
		options = append(options, starmap.WithSource(c.Source))
	}
	if c.Acquirer != nil {
		options = append(options, starmap.WithAcquirer(c.Acquirer))
	}
	if c.LeaseStore != nil {
		options = append(options, starmap.WithLeaseStore(c.LeaseStore))
	}
	options = append(options, c.Extra...)
	return options, nil
}

// Open composes the options and opens the connected runtime. The runtime serves
// the verified embedded catalog immediately and pulls its source in the
// background.
func (c Composition) Open(ctx context.Context) (*starmap.Runtime, error) {
	options, err := c.Options()
	if err != nil {
		return nil, err
	}
	return starmap.Open(ctx, options...)
}
