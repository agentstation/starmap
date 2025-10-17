package reconciler

import (
	"github.com/agentstation/starmap/pkg/authority"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/enhancer"
	"github.com/agentstation/starmap/pkg/errors"
)

// Options configures a reconciler.
type options struct {
	strategy    Strategy
	authorities authority.Authority
	enhancers   []enhancer.Enhancer
	tracking    bool
	baseline    catalogs.Catalog // Existing catalog for comparison
}

func defaultOptions() *options {
	authorities := authority.New()
	return &options{
		strategy:    NewAuthorityStrategy(authorities),
		authorities: authorities,
		enhancers:   []enhancer.Enhancer{},
		tracking:    true,
	}
}

// Option is a function that configures a Reconciler.
type Option func(*options) error

func (options *options) apply(opts ...Option) (*options, error) {
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return nil, err
		}
	}
	return options, nil
}

// newOptions returns reconciler options with default values.
func newOptions(opts ...Option) (*options, error) {
	return defaultOptions().apply(opts...)
}

// WithStrategy sets the merge strategy.
func WithStrategy(strategy Strategy) Option {
	return func(r *options) error {
		if strategy == nil {
			return &errors.ValidationError{
				Field:   "strategy",
				Message: "cannot be nil",
			}
		}
		r.strategy = strategy
		return nil
	}
}

// WithAuthorities sets the field authorities.
func WithAuthorities(authorities authority.Authority) Option {
	return func(r *options) error {
		if authorities == nil {
			return &errors.ValidationError{
				Field:   "authorities",
				Message: "cannot be nil",
			}
		}
		r.authorities = authorities
		if r.strategy != nil || r.strategy.Type() != StrategyTypeFieldAuthority {
			r.strategy = NewAuthorityStrategy(authorities)
		}
		return nil
	}
}

// WithProvenance enables field-level tracking.
func WithProvenance(enabled bool) Option {
	return func(r *options) error {
		r.tracking = enabled
		return nil
	}
}

// WithEnhancers adds model enhancers to the pipeline.
func WithEnhancers(enhancers ...enhancer.Enhancer) Option {
	return func(r *options) error {
		r.enhancers = enhancers
		return nil
	}
}

// WithBaseline sets an existing catalog to compare against for change detection.
func WithBaseline(catalog catalogs.Catalog) Option {
	return func(r *options) error {
		r.baseline = catalog
		return nil
	}
}
