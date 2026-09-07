package config

import (
	"encoding/json"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/runtime"
)

// providerBindingsOption preserves an empty array as an explicit deny-all policy.
// The binding decoder validates fields without echoing configured values.
func providerBindingsOption(value string) (runtime.Option, error) {
	var bindings []sources.ProviderAcquisitionBinding
	if err := json.Unmarshal([]byte(value), &bindings); err != nil || bindings == nil {
		return nil, &errors.ValidationError{Field: ProviderBindings, Message: "must be a JSON array of valid provider acquisition bindings"}
	}
	seen := make(map[string]bool, len(bindings))
	for _, binding := range bindings {
		if seen[binding.ID] {
			return nil, &errors.ValidationError{Field: ProviderBindings, Message: "each binding identity must have exactly one active declaration"}
		}
		seen[binding.ID] = true
	}
	return runtime.WithProviderBindings(bindings...), nil
}
