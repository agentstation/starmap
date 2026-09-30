package runtime

import (
	"bytes"
	"context"
	jsonv1 "encoding/json"
	"encoding/json/v2"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

const localCatalogRecoveryVersion = 1
const generationBaselinesDirectory = "generation-baselines"

// localCatalogRecovery stores the same typed inputs as a fleet record.
// A separate immutable artifact supplies its baseline when the host exports it.
type localCatalogRecovery struct {
	Version          int                 `json:"version"`
	ManifestChecksum string              `json:"manifest_checksum"`
	GenerationID     string              `json:"generation_id"`
	PayloadChecksum  string              `json:"payload_checksum"`
	BaselineChecksum string              `json:"baseline_checksum"`
	Inputs           fleetRecoveryRecord `json:"inputs"`
}

func (r localCatalogRecovery) validate(generation catalogs.Generation) error {
	if err := r.validateStructure(); err != nil {
		return err
	}
	digest, err := catalogManifestChecksum(generation)
	if err != nil {
		return err
	}
	if r.ManifestChecksum != digest || r.GenerationID != generation.Manifest.GenerationID || r.PayloadChecksum != generation.Manifest.Payload.Checksum {
		return invalidInputPublication("retained inputs belong to another catalog generation")
	}
	if r.Inputs.Pin != nil && !pinRecordMatches(*r.Inputs.Pin, generation) {
		return pinRecordConflict("retained inputs select another pinned catalog")
	}
	return generation.Validate()
}

func (r localCatalogRecovery) validateStructure() error {
	if r.Version != localCatalogRecoveryVersion || !validFleetChecksum(r.ManifestChecksum) || !validFleetChecksum(r.BaselineChecksum) || r.GenerationID == "" || !validFleetChecksum(strings.TrimPrefix(r.PayloadChecksum, "sha256:")) || !strings.HasPrefix(r.PayloadChecksum, "sha256:") {
		return invalidInputPublication("catalog recovery has an incomplete identity")
	}
	if !reflect.DeepEqual(r.Inputs.Baseline, catalogs.Generation{}) {
		return invalidInputPublication("local recovery must reference its separate baseline")
	}
	return validateFleetRecoveryRecord(r.Inputs)
}

type catalogRecoveryBaseline struct {
	manifestChecksum string
	data             []byte
}

var embeddedRecoveryBaseline = sync.OnceValues(func() (catalogRecoveryBaseline, error) {
	baseline, err := bootstrap.Generation()
	if err != nil {
		return catalogRecoveryBaseline{}, err
	}
	return encodeCatalogRecoveryBaseline(baseline)
})

func localRecoveryBaseline(layers layerSet) (catalogRecoveryBaseline, error) {
	embedded, err := bootstrap.GenerationManifest()
	if err != nil {
		return catalogRecoveryBaseline{}, err
	}
	if layers.embeddedManifest != nil && reflect.DeepEqual(*layers.embeddedManifest, embedded) && layers.embedded.PayloadChecksum == embedded.Payload.Checksum {
		return embeddedRecoveryBaseline()
	}
	baseline, err := fleetBaseline(layers)
	if err != nil {
		return catalogRecoveryBaseline{}, err
	}
	return encodeCatalogRecoveryBaseline(baseline)
}

func encodeCatalogRecoveryBaseline(generation catalogs.Generation) (catalogRecoveryBaseline, error) {
	if err := generation.Validate(); err != nil {
		return catalogRecoveryBaseline{}, err
	}
	manifest, err := catalogManifestChecksum(generation)
	if err != nil {
		return catalogRecoveryBaseline{}, err
	}
	encoded, err := json.Marshal(generation, json.Deterministic(true))
	if err != nil {
		return catalogRecoveryBaseline{}, err
	}
	data, err := compressFleetRecovery(encoded)
	if err != nil {
		return catalogRecoveryBaseline{}, err
	}
	return catalogRecoveryBaseline{manifestChecksum: manifest, data: data}, nil
}

func (s *layerStore) retainCatalogRecovery(ctx context.Context, record localCatalogRecovery, baseline catalogRecoveryBaseline) error {
	baselines, err := s.directory.Child(generationBaselinesDirectory)
	if err != nil {
		return err
	}
	name := baseline.manifestChecksum + ".json.gz"
	previous, err := baselines.ReadFile(name, MaxFleetRecoveryBytes)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil && !bytes.Equal(previous, baseline.data) {
		if _, err := decodeCatalogRecoveryBaseline(ctx, previous, baseline.manifestChecksum); err != nil {
			return err
		}
		baseline.data = previous
	}
	if os.IsNotExist(err) {
		if err := s.checkRecoveryCapacity(ctx, int64(len(baseline.data))); err != nil {
			return err
		}
	}
	if err := baselines.CompareAndPublishFileContext(ctx, name, previous, baseline.data, ".baseline-"); err != nil {
		return err
	}
	inputs, err := s.directory.Child(generationInputsDirectory)
	if err != nil {
		return err
	}
	data, err := json.Marshal(record, json.Deterministic(true), jsonv1.FormatDurationAsNano(true))
	if err != nil {
		return err
	}
	if len(data) > maxCatalogRecoveryBytes {
		return invalidInputPublication("catalog recovery exceeds its record byte limit")
	}
	data, err = compressFleetRecovery(data)
	if err != nil {
		return err
	}
	name = record.ManifestChecksum + "-" + fleetRecoveryChecksum(data) + ".json.gz"
	previous, err = inputs.ReadFile(name, maxCatalogRecoveryBytes)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil && !bytes.Equal(previous, data) {
		return invalidInputPublication("immutable catalog recovery changed")
	}
	if os.IsNotExist(err) {
		if err := s.checkRecoveryCapacity(ctx, int64(len(data))); err != nil {
			return err
		}
	}
	return inputs.CompareAndPublishFileContext(ctx, name, previous, data, ".generation-input-")
}

func readLocalCatalogRecovery(ctx context.Context, directory *privatefiles.Directory, name string, limit int64) (localCatalogRecovery, []byte, error) {
	data, err := directory.ReadFile(name, limit)
	if err != nil {
		return localCatalogRecovery{}, nil, err
	}
	record, err := decodeLocalCatalogRecovery(ctx, name, data)
	if err != nil {
		return record, nil, err
	}
	return record, data, nil
}

func decodeLocalCatalogRecovery(ctx context.Context, name string, data []byte) (localCatalogRecovery, error) {
	if ctx == nil || !validCatalogRecoveryName(name) || len(data) == 0 || len(data) > maxCatalogRecoveryBytes {
		return localCatalogRecovery{}, invalidInputPublication("catalog recovery requires bounded original descriptor bytes")
	}
	if err := ctx.Err(); err != nil {
		return localCatalogRecovery{}, err
	}
	decoded, err := decompressFleetRecovery(ctx, data, maxCatalogRecoveryBytes)
	if err != nil {
		return localCatalogRecovery{}, err
	}
	var record localCatalogRecovery
	if err := json.Unmarshal(decoded, &record, json.RejectUnknownMembers(true), jsonv1.FormatDurationAsNano(true)); err != nil {
		return record, err
	}
	if !validCatalogRecoveryName(name) || name != record.ManifestChecksum+"-"+fleetRecoveryChecksum(data)+".json.gz" {
		return record, invalidInputPublication("catalog recovery differs from its immutable identity")
	}
	if err := record.validateStructure(); err != nil {
		return record, err
	}
	return record, ctx.Err()
}

func (s *layerStore) readCatalogRecoveryBaseline(ctx context.Context, checksum string) (catalogs.Generation, []byte, error) {
	if !validFleetChecksum(checksum) {
		return catalogs.Generation{}, nil, invalidInputPublication("catalog baseline requires an exact checksum")
	}
	directory, err := s.directory.ExistingChild(generationBaselinesDirectory)
	if err != nil {
		return catalogs.Generation{}, nil, err
	}
	data, err := directory.ReadFile(checksum+".json.gz", MaxFleetRecoveryBytes)
	if err != nil {
		return catalogs.Generation{}, nil, err
	}
	generation, err := decodeCatalogRecoveryBaseline(ctx, data, checksum)
	return generation, data, err
}

func decodeCatalogRecoveryBaseline(ctx context.Context, data []byte, checksum string) (catalogs.Generation, error) {
	decoded, err := decompressFleetRecovery(ctx, data, MaxFleetRecoveryBytes)
	if err != nil {
		return catalogs.Generation{}, err
	}
	var generation catalogs.Generation
	if err := json.Unmarshal(decoded, &generation, json.RejectUnknownMembers(true)); err != nil {
		return generation, err
	}
	if err := generation.Validate(); err != nil {
		return generation, err
	}
	digest, err := catalogManifestChecksum(generation)
	if err != nil {
		return generation, err
	}
	if digest != checksum {
		return generation, invalidInputPublication("catalog baseline differs from its immutable identity")
	}
	return generation, ctx.Err()
}

func validCatalogBaselineName(name string) bool {
	base, ok := strings.CutSuffix(name, ".json.gz")
	return ok && validFleetChecksum(base)
}

func (s *layerStore) checkRecoveryCapacity(ctx context.Context, added int64) error {
	if added > storage.DefaultRetentionInputMaxBytes {
		return invalidInputPublication("catalog recovery exceeds its byte limit")
	}
	total := added
	scanned := 1
	for _, name := range []string{generationBaselinesDirectory, generationInputsDirectory} {
		directory, err := s.directory.ExistingChild(name)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		entries, err := catalogRecoveryEntries(directory, storage.DefaultRetentionScanEntries-scanned)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			scanned++
			if err := ctx.Err(); err != nil {
				return err
			}
			if entry.Name() == privatefiles.PublicationDirectoryName && entry.IsDir() {
				continue
			}
			valid := validCatalogRecoveryName(entry.Name())
			if name == generationBaselinesDirectory {
				valid = validCatalogBaselineName(entry.Name())
			}
			if !valid || !entry.Type().IsRegular() {
				return invalidInputPublication("catalog recovery contains an unknown record")
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if info.Size() > storage.DefaultRetentionInputMaxBytes-total {
				return invalidInputPublication("catalog recovery exceeds its byte limit")
			}
			total += info.Size()
		}
	}
	return ctx.Err()
}
