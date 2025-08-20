package catalogs

import "time"

// SyncOptions configures sync operations.
type SyncOptions struct {
	// Provider filtering
	ProviderID *ProviderID // If set, sync only this provider

	// Sync behavior
	DryRun             bool // Show changes without applying them
	Fresh              bool // Delete existing models and write all API models
	AutoApprove        bool // Skip confirmation prompts
	CleanModelsDevRepo bool // Remove models.dev repository after sync

	// Output configuration
	OutputDir string // Custom output directory for providers

	// Timeout configuration
	Timeout time.Duration // Timeout for API calls
}

// SyncOption is a function that configures sync options
type SyncOption func(*SyncOptions)

// SyncWithProvider configures syncing for a specific provider only
func SyncWithProvider(providerID ProviderID) SyncOption {
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

// SyncWithTimeout sets the timeout for API calls
func SyncWithTimeout(timeout time.Duration) SyncOption {
	return func(opts *SyncOptions) {
		opts.Timeout = timeout
	}
}

// NewSyncOptions creates SyncOptions with defaults
func NewSyncOptions(opts ...SyncOption) *SyncOptions {
	options := &SyncOptions{
		DryRun:             false,
		Fresh:              false,
		AutoApprove:        false,
		CleanModelsDevRepo: false,
		Timeout:            30 * time.Second,
	}

	for _, opt := range opts {
		opt(options)
	}

	return options
}
