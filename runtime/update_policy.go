package runtime

import (
	"slices"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// SourceRefreshMode controls automatic source reads independently of acquisition.
type SourceRefreshMode string

const (
	// SourceRefreshAutomatic permits startup, periodic, and reactive source reads.
	SourceRefreshAutomatic SourceRefreshMode = "automatic"
	// SourceRefreshManual permits only explicit source reads.
	SourceRefreshManual SourceRefreshMode = "manual"
)

// NetworkMode controls outbound catalog acquisition, not inference or storage.
type NetworkMode string

const (
	// NetworkConfigured permits the selected catalog sources to use the network.
	NetworkConfigured NetworkMode = "configured"
	// NetworkOffline refuses network source reads and acquisition.
	NetworkOffline NetworkMode = "offline"
)

// UpdatePolicy separates automatic source refresh from catalog network access.
type UpdatePolicy struct {
	SourceRefreshMode SourceRefreshMode
	NetworkMode       NetworkMode
}

// DefaultUpdatePolicy permits automatic reads through configured sources.
func DefaultUpdatePolicy() UpdatePolicy {
	return UpdatePolicy{SourceRefreshMode: SourceRefreshAutomatic, NetworkMode: NetworkConfigured}
}

// Validate rejects unknown policy values before runtime construction.
func (p UpdatePolicy) Validate() error {
	if _, err := ParseSourceRefreshMode(string(p.SourceRefreshMode)); err != nil {
		return err
	}
	_, err := ParseNetworkMode(string(p.NetworkMode))
	return err
}

// ParseSourceRefreshMode accepts an explicit automatic or manual selection.
func ParseSourceRefreshMode(value string) (SourceRefreshMode, error) {
	mode := SourceRefreshMode(value)
	if mode != SourceRefreshAutomatic && mode != SourceRefreshManual {
		return "", &errors.ValidationError{Field: "source_refresh_mode", Value: value, Message: "must be automatic or manual"}
	}
	return mode, nil
}

// ParseNetworkMode accepts an explicit configured or offline selection.
func ParseNetworkMode(value string) (NetworkMode, error) {
	mode := NetworkMode(value)
	if mode != NetworkConfigured && mode != NetworkOffline {
		return "", &errors.ValidationError{Field: "catalog_network_mode", Value: value, Message: "must be configured or offline"}
	}
	return mode, nil
}

// WithUpdatePolicy replaces both catalog update controls.
func WithUpdatePolicy(policy UpdatePolicy) Option {
	return func(config *options) error {
		if err := policy.Validate(); err != nil {
			return err
		}
		config.updatePolicy = policy
		return nil
	}
}

// WithSourceRefreshMode selects automatic or explicit-only source refresh.
func WithSourceRefreshMode(value string) Option {
	return func(config *options) error {
		mode, err := ParseSourceRefreshMode(value)
		if err == nil {
			config.updatePolicy.SourceRefreshMode = mode
		}
		return err
	}
}

// WithCatalogNetworkMode selects configured or offline catalog acquisition.
func WithCatalogNetworkMode(value string) Option {
	return func(config *options) error {
		mode, err := ParseNetworkMode(value)
		if err == nil {
			config.updatePolicy.NetworkMode = mode
		}
		return err
	}
}

func (r *Runtime) sourceReadAllowed() bool {
	if r.config.updatePolicy.NetworkMode != NetworkOffline {
		return true
	}
	// An injected source can access the network regardless of its configured kind.
	// Only the built-in embedded and file readers establish local-only access here.
	return r.config.customSource == nil && (r.config.source.Kind == SourceEmbedded || r.config.source.Kind == SourceFile)
}

func (r *Runtime) automaticSourceReads() bool {
	return r.config.generationPin == "" && r.config.updatePolicy.SourceRefreshMode == SourceRefreshAutomatic && r.sourceReadAllowed()
}

func offlineCatalogOperation(operation string) error {
	return &errors.ConfigError{Component: "catalog network policy", Message: "offline mode prohibits " + operation}
}

func (r *Runtime) validateAcquisitionAccess(selected []sources.ID) error {
	if r == nil {
		return &errors.ValidationError{Field: "runtime", Message: "is required"}
	}
	if err := r.validateGenerationMutation(); err != nil {
		return err
	}
	if err := r.validateAuthorityPublication(nil, 1, nil); err != nil {
		return err
	}
	if r.config.updatePolicy.NetworkMode != NetworkOffline {
		return nil
	}
	if len(selected) == 0 {
		return offlineCatalogOperation("acquisition without explicit local sources")
	}
	for _, id := range selected {
		switch id {
		case sources.EmbeddedCatalogID, sources.LocalCatalogID, sources.ReleaseArtifactID:
		default:
			return offlineCatalogOperation("network source and provider acquisition")
		}
	}
	return nil
}

func validateAcquisitionResults(observations []sources.Observation, selected []sources.ID) error {
	if len(selected) == 0 {
		return nil
	}
	for _, observation := range observations {
		if !slices.Contains(selected, observation.SourceID) {
			return &errors.ValidationError{Field: "acquisition.source", Value: observation.SourceID, Message: "result is outside the declared source selection"}
		}
	}
	return nil
}
