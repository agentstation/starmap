package acquisition

import (
	"slices"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// WithProviderBindings limits manual acquisition to the declared provider scopes.
// An explicit empty set disables provider acquisition. Omission retains legacy selection.
// Source and provider options can restrict this set, but cannot add bindings.
func WithProviderBindings(bindings ...sources.ProviderAcquisitionBinding) Option {
	owned := slices.Clone(bindings)
	return func(options *options) error {
		seen := make(map[string]bool, len(owned))
		for _, binding := range owned {
			if err := binding.Validate(); err != nil {
				return err
			}
			if seen[binding.ID] {
				return &errors.ValidationError{Field: "acquisition.provider_bindings", Message: "each binding identity must have exactly one active declaration"}
			}
			seen[binding.ID] = true
		}
		selected := slices.Clone(owned)
		options.providerBindings = &selected
		return nil
	}
}
