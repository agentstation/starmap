package app

import (
	"context"
	"crypto/sha256"
	stderrors "errors"
	"path/filepath"

	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/productpaths"
	"github.com/agentstation/starmap/runtime"
)

// PrepareRuntimeMigration records an inventory with this process's selected owner.
func (a *App) PrepareRuntimeMigration(ctx context.Context, request runtime.DirectoryMigrationRequest) (runtime.DirectoryMigrationPreparation, error) {
	selected, _, err := a.runtimeMigrationRequest(request)
	if err != nil {
		return runtime.DirectoryMigrationPreparation{}, err
	}
	return runtime.PrepareDirectoryMigration(ctx, selected)
}

// StageRuntimeMigration verifies a private copy with this process's selected owner.
func (a *App) StageRuntimeMigration(ctx context.Context, request runtime.DirectoryMigrationRequest) (runtime.DirectoryMigrationStage, error) {
	selected, _, err := a.runtimeMigrationRequest(request)
	if err != nil {
		return runtime.DirectoryMigrationStage{}, err
	}
	return runtime.StageDirectoryMigration(ctx, selected)
}

// PublishRuntimeMigration publishes the replacement without editing host configuration.
func (a *App) PublishRuntimeMigration(ctx context.Context, request runtime.DirectoryMigrationRequest) (runtime.DirectoryMigrationPublication, error) {
	selected, _, err := a.runtimeMigrationRequest(request)
	if err != nil {
		return runtime.DirectoryMigrationPublication{}, err
	}
	return runtime.PublishDirectoryMigration(ctx, selected)
}

// CompleteRuntimeMigration verifies saved selection, opens the replacement, and records completion.
// The command closes its replacement runtime before returning.
func (a *App) CompleteRuntimeMigration(ctx context.Context, request runtime.DirectoryMigrationRequest) (result runtime.DirectoryMigrationPublication, resultErr error) {
	selected, paths, err := a.runtimeMigrationRequest(request)
	if err != nil {
		return result, err
	}
	if paths.Runtime.Path != selected.TargetDirectory || paths.SchedulerIdentity != selected.SourceIdentity {
		return result, migrationSelectionConflict("configuration must select the target directory and retained scheduler identity")
	}
	if err := a.verifySavedMigrationSelection(selected, paths); err != nil {
		return result, err
	}
	if err := runtime.VerifyDirectoryMigrationPublication(ctx, selected); err != nil {
		return result, err
	}
	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	if a.runtime != nil {
		return result, migrationSelectionConflict("stop the existing runtime before completion")
	}
	connected, err := a.openRuntimeLocked(ctx, []runtime.Option{runtime.WithPublishedDirectoryMigration(selected)})
	if err != nil {
		return result, err
	}
	defer func() {
		a.runtime = nil
		resultErr = stderrors.Join(resultErr, connected.Close())
	}()
	if err := a.verifySavedMigrationSelection(selected, paths); err != nil {
		return result, err
	}
	return connected.CompleteDirectoryMigration(ctx, selected)
}

func (a *App) runtimeMigrationRequest(request runtime.DirectoryMigrationRequest) (runtime.DirectoryMigrationRequest, ProductPaths, error) {
	paths, err := a.ResolvedPaths()
	if err != nil {
		return request, paths, err
	}
	owner := runtime.DirectoryOwner{Product: "starmap", Deployment: paths.DeploymentID, Instance: paths.InstanceID}
	if request.Owner != (runtime.DirectoryOwner{}) && request.Owner != owner {
		return request, paths, migrationSelectionConflict("request owner differs from configured ownership")
	}
	request.Owner = owner
	if request.JournalRoot == "" {
		request.JournalRoot = filepath.Join(paths.Roots[productpaths.State].Path, "migrations")
	}
	return request, paths, nil
}

func (a *App) verifySavedMigrationSelection(request runtime.DirectoryMigrationRequest, paths ProductPaths) error {
	config := a.config
	if !config.catalogFileRead || config.ConfigFile == "" {
		return migrationSelectionConflict("save the migration selection in a configuration file before completion")
	}
	data, err := readSelectedConfiguration(config.ConfigFile, config.ConfigAccess)
	if err != nil {
		return errors.WrapIO("read", config.ConfigFile, err)
	}
	if sha256.Sum256(data) != config.configFileDigest {
		return migrationSelectionConflict("configuration file changed after selection")
	}
	state, present := config.CatalogValues[catalogconfig.StateDirectory]
	if !present || state == "" || config.CatalogValues[catalogconfig.SchedulerIdentity] != request.SourceIdentity {
		return migrationSelectionConflict("saved configuration must contain state_dir and scheduler_identity")
	}
	expanded, err := expandHomePath(state)
	if err != nil {
		return err
	}
	saved, err := productpaths.Leaf(paths.Roots[productpaths.Config], expanded, "configuration-file")
	if err != nil {
		return err
	}
	deployment, instance := "local", "default"
	if value, present := config.PathValues["STARMAP_DEPLOYMENT_ID"]; present {
		deployment = value
	}
	if value, present := config.PathValues["STARMAP_INSTANCE_ID"]; present {
		instance = value
	}
	if saved.Path != request.TargetDirectory || deployment != request.Owner.Deployment || instance != request.Owner.Instance {
		return migrationSelectionConflict("saved configuration differs from the target directory or owner")
	}
	return nil
}

func migrationSelectionConflict(message string) error {
	return &errors.ConflictError{Resource: "runtime migration selection", Message: message}
}
