package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/errors"
)

// loadProviders returns retained unscoped evidence and separate binding revisions.
func (s *layerStore) loadProviders() (map[providerEvidenceKey]ProviderLayer, error) {
	layers := make(map[providerEvidenceKey]ProviderLayer)
	if !s.durable() {
		return layers, nil
	}
	if err := s.loadProviderDirectory(s.providers, false, layers); err != nil {
		return nil, err
	}
	if err := s.loadProviderDirectory(s.bindings, true, layers); err != nil {
		return nil, err
	}
	if err := validateProviderScopes(nil, layers); err != nil {
		return nil, err
	}
	return layers, nil
}

func (s *layerStore) loadProviderDirectory(directory *privatefiles.Directory, scoped bool, layers map[providerEvidenceKey]ProviderLayer) error {
	if directory == nil {
		return nil
	}
	entries, err := directory.ReadDir()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.WrapIO("read provider evidence", s.root, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		raw, err := readLayerFile(directory, entry.Name())
		if err != nil {
			return err
		}
		if raw == nil {
			continue
		}
		var layer ProviderLayer
		if err := json.Unmarshal(raw, &layer); err != nil {
			return errors.WrapParse("retained provider layer", entry.Name(), err)
		}
		if err := layer.validate(); err != nil {
			return err
		}
		key := layer.evidenceKey()
		if key.scoped() != scoped || key.filename() != entry.Name() {
			return invalidProviderEvidence("provider_id", "does not match the retained scope directory and filename")
		}
		layers[key] = layer
	}
	return nil
}

// saveProvider writes one private record without replacing another binding revision.
func (s *layerStore) saveProvider(ctx context.Context, layer ProviderLayer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !s.durable() {
		return nil
	}
	if err := validateProviderLayerID(layer.ProviderID); err != nil {
		return err
	}
	key := layer.evidenceKey()
	directory := s.providers
	if key.scoped() {
		directory = s.bindings
	}
	return s.writeContext(ctx, directory, key.filename(), layer)
}
