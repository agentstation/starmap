package reconciler

import (
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/provenance"
)

// Options configures a reconciler.
type options struct {
	authorities       authority.Reader
	tracking          bool
	changeTime        time.Time
	baseline          *catalogs.Catalog // Existing catalog for comparison
	projectedEvidence func(catalogs.ProviderID, provenance.Entry) bool
	providerSelection providerObservationSelection
}

// WithProjectedEvidencePolicy controls reuse of unchanged facts from a local projection.
// The callback checks the original evidence scope. Operator edits have no matching
// carried value and retain the local source's field authority.
func WithProjectedEvidencePolicy(permit func(catalogs.ProviderID, provenance.Entry) bool) Option {
	return func(options *options) error {
		if permit == nil {
			return &errors.ValidationError{Field: "reconciliation.projected_evidence", Message: "is required"}
		}
		options.projectedEvidence = permit
		return nil
	}
}

func defaultOptions() *options {
	authorities := authority.New()
	return &options{
		authorities: authorities,
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

// WithAuthorities sets the field authorities.
func WithAuthorities(authorities authority.Reader) Option {
	return func(r *options) error {
		if authorities == nil {
			return &errors.ValidationError{
				Field:   "authorities",
				Message: "cannot be nil",
			}
		}
		r.authorities = authorities
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

// WithBaseline sets an existing catalog to compare against for change detection.
func WithBaseline(catalog *catalogs.Catalog) Option {
	return func(r *options) error {
		r.baseline = catalog
		return nil
	}
}

// WithChangeTime supplies stable timestamps for facts derived from retained evidence.
// It leaves original source timestamps and current-time pricing validation unchanged.
func WithChangeTime(at time.Time) Option {
	return func(options *options) error {
		if at.IsZero() {
			return &errors.ValidationError{Field: "reconciliation.change_time", Message: "must be specified"}
		}
		options.changeTime = at.UTC()
		return nil
	}
}
