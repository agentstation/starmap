package runtime

import (
	"context"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/internal/privatefiles"
)

func isRecordPublicationDirectory(path string) bool {
	if filepath.Base(path) == privatefiles.PublicationDirectoryName && materializationJournalDirectory(filepath.ToSlash(filepath.Dir(path))) {
		return true
	}
	switch filepath.ToSlash(path) {
	case layerDirectoryName + "/" + privatefiles.PublicationDirectoryName,
		layerDirectoryName + "/" + providerLayerDirectoryName + "/" + privatefiles.PublicationDirectoryName,
		layerDirectoryName + "/" + providerLayerDirectoryName + "/" + bindingLayerDirectoryName + "/" + privatefiles.PublicationDirectoryName,
		layerDirectoryName + "/" + inputPublicationDirectory + "/" + privatefiles.PublicationDirectoryName,
		layerDirectoryName + "/" + generationInputsDirectory + "/" + privatefiles.PublicationDirectoryName,
		layerDirectoryName + "/" + generationBaselinesDirectory + "/" + privatefiles.PublicationDirectoryName,
		"github-catalog-source/" + privatefiles.PublicationDirectoryName:
		return true
	default:
		return false
	}
}

func (s *layerStore) recoverRecordPublications(ctx context.Context) error {
	if !s.durable() {
		return ctx.Err()
	}
	directories := []*privatefiles.Directory{s.directory, s.providers, s.bindings}
	inputs, err := s.directory.ExistingChild(inputPublicationDirectory)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if inputs != nil {
		directories = append(directories, inputs)
	}
	baseline, err := s.directory.ExistingChild(generationBaselinesDirectory)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if baseline != nil {
		directories = append(directories, baseline)
	}
	recovery, err := s.directory.ExistingChild(generationInputsDirectory)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if recovery != nil {
		directories = append(directories, recovery)
	}
	for _, directory := range directories {
		if directory != nil {
			if err := directory.RecoverPublications(ctx); err != nil {
				return err
			}
		}
	}
	return ctx.Err()
}
