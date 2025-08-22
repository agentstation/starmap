package sources

import (
	"fmt"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// SyncOptions configures sync operations.
type SyncOptions struct {
	// Provider filtering
	ProviderID *catalogs.ProviderID // If set, sync only this provider

	// Sync behavior
	DryRun             bool // Show changes without applying them
	Fresh              bool // Delete existing models and write all API models
	AutoApprove        bool // Skip confirmation prompts
	CleanModelsDevRepo bool // Remove models.dev repository after sync
	ForceFormat        bool // Force reformat of providers.yaml even if no changes detected

	// Output configuration
	OutputDir string // Custom output directory for providers

	// Timeout configuration
	Timeout time.Duration // Timeout for API calls

	// Source control
	DisableProviderAPI   bool // Disable provider API source
	DisableModelsDevGit  bool // Disable models.dev git source
	DisableModelsDevHTTP bool // Disable models.dev HTTP source
	DisableLocalCatalog  bool // Disable local catalog source

	// Field authorities (new)
	CustomFieldAuthorities []FieldAuthority // Custom field authorities to override defaults

	// Provenance tracking (new)
	TrackProvenance bool   // Enable provenance tracking
	ProvenanceFile  string // File to save provenance information
}

// SyncOption is a function that configures sync options
type SyncOption func(*SyncOptions)

// SyncWithProvider configures syncing for a specific provider only
func SyncWithProvider(providerID catalogs.ProviderID) SyncOption {
	return func(opts *SyncOptions) {
		opts.ProviderID = &providerID
	}
}

// SyncWithDryRun enables dry run mode (show changes without applying)
func SyncWithDryRun(enabled bool) SyncOption {
	return func(opts *SyncOptions) {
		opts.DryRun = enabled
	}
}

// SyncWithFreshSync enables fresh sync mode (delete and recreate all models)
func SyncWithFreshSync(enabled bool) SyncOption {
	return func(opts *SyncOptions) {
		opts.Fresh = enabled
	}
}

// SyncWithAutoApprove skips confirmation prompts
func SyncWithAutoApprove(enabled bool) SyncOption {
	return func(opts *SyncOptions) {
		opts.AutoApprove = enabled
	}
}

// SyncWithOutputDir sets a custom output directory for providers
func SyncWithOutputDir(dir string) SyncOption {
	return func(opts *SyncOptions) {
		opts.OutputDir = dir
	}
}

// SyncWithCleanModelsDevRepo enables cleanup of models.dev repository after sync
func SyncWithCleanModelsDevRepo(enabled bool) SyncOption {
	return func(opts *SyncOptions) {
		opts.CleanModelsDevRepo = enabled
	}
}

// SyncWithForceFormat forces reformat of providers.yaml even if no changes detected
func SyncWithForceFormat(enabled bool) SyncOption {
	return func(opts *SyncOptions) {
		opts.ForceFormat = enabled
	}
}

// SyncWithTimeout sets the timeout for API calls
func SyncWithTimeout(timeout time.Duration) SyncOption {
	return func(opts *SyncOptions) {
		opts.Timeout = timeout
	}
}

// SyncWithFieldAuthority adds a custom field authority
func SyncWithFieldAuthority(field string, source Type, priority int) SyncOption {
	return func(opts *SyncOptions) {
		opts.CustomFieldAuthorities = append(opts.CustomFieldAuthorities, FieldAuthority{
			FieldPath: field,
			Source:    source,
			Priority:  priority,
		})
	}
}

// SyncWithDisabledSource disables a specific source type
func SyncWithDisabledSource(source Type) SyncOption {
	return func(opts *SyncOptions) {
		switch source {
		case ProviderAPI:
			opts.DisableProviderAPI = true
		case ModelsDevGit:
			opts.DisableModelsDevGit = true
		case ModelsDevHTTP:
			opts.DisableModelsDevHTTP = true
		case LocalCatalog:
			opts.DisableLocalCatalog = true
		}
	}
}

// SyncWithModelsDevGitDisabled disables the models.dev git source
func SyncWithModelsDevGitDisabled(disabled bool) SyncOption {
	return func(opts *SyncOptions) {
		opts.DisableModelsDevGit = disabled
	}
}

// SyncWithModelsDevHTTPDisabled disables the models.dev HTTP source
func SyncWithModelsDevHTTPDisabled(disabled bool) SyncOption {
	return func(opts *SyncOptions) {
		opts.DisableModelsDevHTTP = disabled
	}
}

// SyncWithProvenance enables provenance tracking and optionally sets output file
func SyncWithProvenance(file string) SyncOption {
	return func(opts *SyncOptions) {
		opts.TrackProvenance = true
		if file != "" {
			opts.ProvenanceFile = file
		}
	}
}

// SyncWithProvenanceEnabled enables/disables provenance tracking
func SyncWithProvenanceEnabled(enabled bool) SyncOption {
	return func(opts *SyncOptions) {
		opts.TrackProvenance = enabled
	}
}

// NewSyncOptions creates SyncOptions with defaults
func NewSyncOptions(opts ...SyncOption) *SyncOptions {
	options := &SyncOptions{
		DryRun:             false,
		Fresh:              false,
		AutoApprove:        false,
		CleanModelsDevRepo: false,
		ForceFormat:        false,
		Timeout:            30 * time.Second,

		// Source control defaults
		DisableProviderAPI:   false,
		DisableModelsDevGit:  true,  // Default to disabled (use HTTP instead)
		DisableModelsDevHTTP: false, // Default to enabled (faster)
		DisableLocalCatalog:  false,

		// Provenance defaults
		TrackProvenance: false,
	}

	for _, opt := range opts {
		opt(options)
	}

	return options
}

// DefaultSyncOptions returns default sync options suitable for Update() operations
func DefaultSyncOptions() *SyncOptions {
	return &SyncOptions{
		AutoApprove: true,
		DryRun:      false,
		Timeout:     30 * time.Second,
	}
}

// Copy creates a deep copy of the sync options
func (opts *SyncOptions) Copy() *SyncOptions {
	if opts == nil {
		return nil
	}

	copy := &SyncOptions{
		ProviderID:           opts.ProviderID,
		DryRun:               opts.DryRun,
		Fresh:                opts.Fresh,
		AutoApprove:          opts.AutoApprove,
		CleanModelsDevRepo:   opts.CleanModelsDevRepo,
		ForceFormat:          opts.ForceFormat,
		OutputDir:            opts.OutputDir,
		Timeout:              opts.Timeout,
		DisableProviderAPI:   opts.DisableProviderAPI,
		DisableModelsDevGit:  opts.DisableModelsDevGit,
		DisableModelsDevHTTP: opts.DisableModelsDevHTTP,
		DisableLocalCatalog:  opts.DisableLocalCatalog,
		TrackProvenance:      opts.TrackProvenance,
		ProvenanceFile:       opts.ProvenanceFile,
	}

	// Deep copy slices
	if opts.CustomFieldAuthorities != nil {
		copy.CustomFieldAuthorities = make([]FieldAuthority, len(opts.CustomFieldAuthorities))
		for i, authority := range opts.CustomFieldAuthorities {
			copy.CustomFieldAuthorities[i] = authority
		}
	}

	return copy
}

// Validate checks if the sync options are valid
func (opts *SyncOptions) Validate() error {
	if opts == nil {
		return nil
	}

	// Validate timeout
	if opts.Timeout < 0 {
		return fmt.Errorf("timeout cannot be negative")
	}

	// Validate field authorities
	for i, authority := range opts.CustomFieldAuthorities {
		if authority.FieldPath == "" {
			return fmt.Errorf("field authority %d has empty field path", i)
		}
		if authority.Priority < 0 {
			return fmt.Errorf("field authority %d has negative priority", i)
		}
	}

	// Validate provenance settings
	if opts.ProvenanceFile != "" && !opts.TrackProvenance {
		return fmt.Errorf("provenance_file specified but track_provenance is false")
	}

	return nil
}
