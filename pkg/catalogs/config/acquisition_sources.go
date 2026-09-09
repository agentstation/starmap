package config

import (
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/runtime"
)

func acquisitionSourcesOption(value string) (runtime.Option, error) {
	ids := acquisitionSourceIDs(value)
	if err := sources.ValidateAcquisitionSelection(ids); err != nil {
		return nil, err
	}
	return runtime.WithAcquisitionSources(ids...), nil
}
func acquisitionSourceIDs(value string) []sources.ID {
	values := splitList(value)
	ids := make([]sources.ID, len(values))
	for index, id := range values {
		ids[index] = sources.ID(id)
	}
	return ids
}

// AcquisitionSourceSelection returns the explicit source IDs and their presence.
// Each call returns an independent list. Omission keeps the host's collector defaults.
func (c Config) AcquisitionSourceSelection() ([]sources.ID, bool) {
	value, present := c.Value(AcquisitionSources)
	if !present {
		return nil, false
	}
	return acquisitionSourceIDs(value), true
}

func modelsDevGitCommitOption(value string) (runtime.Option, error) {
	if value != "" && !sources.IsExactGitCommit(value) {
		return nil, &errors.ValidationError{Field: ModelsDevGitCommit, Message: "must be an exact 40- or 64-character hexadecimal Git commit"}
	}
	return runtime.WithModelsDevGitCommit(value), nil
}
