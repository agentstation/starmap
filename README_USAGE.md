# Starmap Usage Examples

## Basic Usage (Unchanged)

```go
// Default behavior - no configuration needed
sm, _ := starmap.New()
err := sm.Update()
```

## Custom Update Configuration  

```go
// Configure sync options for Update()
options := &sources.SyncOptions{
    AutoApprove: true,         // Skip confirmations
    DryRun:      false,        // Apply changes
    Timeout:     time.Minute,  // Custom timeout
}

// Set at creation
sm, _ := starmap.New(starmap.WithSyncOptions(options))
err := sm.Update() // Uses custom options

// Or set at runtime
sm.SetSyncOptions(options)
err = sm.Update() // Uses custom options
```

## Environment-Specific Configurations

```go
// Development options
devOptions := sources.DefaultSyncOptions()
devOptions.DryRun = true                   // Preview only
devOptions.DisableModelsDevGit = true     // Use HTTP source only (faster)

// Production options  
prodOptions := &sources.SyncOptions{
    AutoApprove:     true,
    TrackProvenance: true,
    ProvenanceFile:  "/var/log/starmap.log",
    Timeout:         2 * time.Minute,
}

// Switch configurations as needed
sm.SetSyncOptions(devOptions)
sm.Update() // Development sync

sm.SetSyncOptions(prodOptions) 
sm.Update() // Production sync
```

## Manual Sync (Full Control)

```go
// Manual sync overrides Update() configuration
result, err := sm.Sync(
    sources.SyncWithProvider("openai"),
    sources.SyncWithDryRun(true),
    sources.SyncWithTimeout(30*time.Second),
)
```