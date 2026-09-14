package storage

import (
	"context"
	"path/filepath"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// confirmCurrentDurability retries synchronization without replacing the selected pointer.
// The caller holds the store commit lock after complete generation validation.
func (s *Filesystem) confirmCurrentDurability(ctx context.Context, generation catalogs.Generation, directory *privatefiles.Directory) error {
	id := generation.Manifest.GenerationID
	if err := s.syncRetainedGeneration(ctx, generation); err != nil {
		return &errors.PublicationError{Resource: "catalog current generation", ID: id, Err: err}
	}
	if err := ctx.Err(); err != nil {
		return &errors.PublicationError{Resource: "catalog current generation", ID: id, Err: err}
	}
	root, err := directory.Open()
	if err != nil {
		return &errors.PublicationError{Resource: "catalog current generation", ID: id, Err: err}
	}
	defer func() { _ = root.Close() }()
	synchronize := s.syncCurrentDirectory
	if synchronize == nil {
		synchronize = filepublish.SyncDirectory
	}
	if err := synchronize(root); err != nil {
		return &errors.PublicationError{Resource: "catalog current generation", ID: id, Err: err}
	}
	return nil
}

// syncRetainedGeneration confirms prior directory publication before selecting retained content.
func (s *Filesystem) syncRetainedGeneration(ctx context.Context, generation catalogs.Generation) error {
	for _, path := range []string{s.generationDir(generation.Manifest.GenerationID), filepath.Join(s.root, "generations")} {
		if err := ctx.Err(); err != nil {
			return err
		}
		directory, err := privatefiles.ExistingDirectory(path)
		if err != nil {
			return err
		}
		root, err := directory.Open()
		if err != nil {
			return err
		}
		err = filepublish.SyncDirectory(root)
		closeErr := root.Close()
		if err != nil {
			return errors.WrapIO("sync", path, err)
		}
		if closeErr != nil {
			return errors.WrapIO("close", path, closeErr)
		}
	}
	return nil
}
