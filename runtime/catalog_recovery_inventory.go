package runtime

import (
	"bytes"
	"context"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

// CatalogRecoveryChecksums lists immutable input-record identities for an exact generation.
// These checksums name compressed local descriptors, not exported FleetRecovery bytes.
// The caller must fence writers. Multiple results require an explicit selection.
// Listing never proves which publication committed or whether independent history is complete.
func CatalogRecoveryChecksums(ctx context.Context, path string, owner DirectoryOwner, identity string, generation catalogs.Generation) ([]string, error) {
	if ctx == nil {
		return nil, invalidInputPublication("recovery requires a context")
	}
	if err := generation.Validate(); err != nil {
		return nil, err
	}
	if err := InspectRetainedDirectory(ctx, path, owner, identity); err != nil {
		return nil, err
	}
	manifest, err := catalogManifestChecksum(generation)
	if err != nil {
		return nil, err
	}
	store, err := existingLayerStore(path)
	if err != nil {
		return nil, err
	}
	if !store.durable() {
		return nil, nil
	}
	directory, err := store.directory.ExistingChild(generationInputsDirectory)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	entries, err := catalogRecoveryEntries(directory, storage.DefaultRetentionScanEntries)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !strings.HasPrefix(entry.Name(), manifest+"-") {
			continue
		}
		record, data, err := readLocalCatalogRecovery(ctx, directory, entry.Name(), maxCatalogRecoveryBytes)
		if err != nil {
			return nil, err
		}
		if err := record.validate(generation); err != nil {
			return nil, err
		}
		result = append(result, fleetRecoveryChecksum(data))
	}
	return result, ctx.Err()
}

func catalogRecoveryEntries(directory *privatefiles.Directory, limit int) ([]os.DirEntry, error) {
	root, err := directory.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	listing, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer func() { _ = listing.Close() }()
	entries, err := listing.ReadDir(limit + 1)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if len(entries) > limit {
		return nil, invalidInputPublication("catalog recovery exceeds its entry limit")
	}
	slices.SortFunc(entries, func(a, b os.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return entries, nil
}

type capturedCatalogRecovery struct {
	record localCatalogRecovery
	name   string
	raw    []byte
}
type capturedRecoveryBaseline struct {
	generation catalogs.Generation
	raw        []byte
	layers     layerSet
}
type catalogRecoveryInventory struct {
	inputs, baselines *privatefiles.Directory
	records           []capturedCatalogRecovery
	baseline          map[string][]byte
	scanned           int
	bytes             int64
}

func (s *layerStore) captureCatalogRecovery(ctx context.Context, limit int, maxBytes int64) (catalogRecoveryInventory, error) {
	inventory := catalogRecoveryInventory{baseline: make(map[string][]byte)}
	if !s.durable() {
		return inventory, ctx.Err()
	}
	var decodedBaseline capturedRecoveryBaseline
	var decodedChecksum string
	for _, name := range []string{generationBaselinesDirectory, generationInputsDirectory} {
		directory, err := s.directory.ExistingChild(name)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return inventory, err
		}
		if err := directory.CheckNoPendingPublications(ctx); err != nil {
			return inventory, err
		}
		if name == generationBaselinesDirectory {
			inventory.baselines = directory
		} else {
			inventory.inputs = directory
		}
		entries, err := catalogRecoveryEntries(directory, limit-inventory.scanned)
		if err != nil {
			return inventory, err
		}
		for _, entry := range entries {
			inventory.scanned++
			if err := ctx.Err(); err != nil {
				return inventory, err
			}
			if entry.Name() == privatefiles.PublicationDirectoryName && entry.IsDir() {
				continue
			}
			valid := validCatalogRecoveryName(entry.Name())
			if name == generationBaselinesDirectory {
				valid = validCatalogBaselineName(entry.Name())
			}
			if !valid || !entry.Type().IsRegular() {
				return inventory, invalidInputPublication("catalog recovery contains an unknown record")
			}
			if name == generationBaselinesDirectory {
				raw, err := directory.ReadFile(entry.Name(), min(int64(MaxFleetRecoveryBytes), maxBytes-inventory.bytes))
				if err != nil {
					return inventory, err
				}
				inventory.bytes += int64(len(raw))
				checksum := strings.TrimSuffix(entry.Name(), ".json.gz")
				baseline, err := captureCatalogRecoveryBaseline(ctx, raw, checksum)
				if err != nil {
					return inventory, err
				}
				inventory.baseline[checksum] = raw
				decodedBaseline, decodedChecksum = baseline, checksum
			} else {
				record, raw, err := readLocalCatalogRecovery(ctx, directory, entry.Name(), min(int64(maxCatalogRecoveryBytes), maxBytes-inventory.bytes))
				if err != nil {
					return inventory, err
				}
				inventory.bytes += int64(len(raw))
				baseline, ok := inventory.baseline[record.BaselineChecksum]
				if !ok {
					return inventory, invalidInputPublication("catalog recovery baseline is missing")
				}
				if decodedChecksum != record.BaselineChecksum {
					decodedBaseline, err = captureCatalogRecoveryBaseline(ctx, baseline, record.BaselineChecksum)
					if err != nil {
						return inventory, err
					}
					decodedChecksum = record.BaselineChecksum
				}
				inputs := record.Inputs
				inputs.Baseline = decodedBaseline.generation
				if _, err := decodeFleetRecoveryRecord(ctx, inputs, decodedBaseline.layers); err != nil {
					return inventory, err
				}
				// Collection needs identity and pin bindings. Keep the complete compressed
				// bytes for conditional removal, but release decoded source and provider data.
				record.Inputs = fleetRecoveryRecord{Version: inputs.Version, PublisherID: inputs.PublisherID,
					Compatibility: inputs.Compatibility, Pin: inputs.Pin, ReplayChecksum: inputs.ReplayChecksum}
				inventory.records = append(inventory.records, capturedCatalogRecovery{record: record, name: entry.Name(), raw: raw})
			}
		}
	}
	return inventory, ctx.Err()
}

func (s *layerStore) inspectCatalogRecovery(ctx context.Context) error {
	_, err := s.captureCatalogRecovery(ctx, storage.DefaultRetentionScanEntries, storage.DefaultRetentionInputMaxBytes)
	return err
}

// collectCatalogRecovery runs under publication and provider-retention locks.
// Only a missing catalog generation permits input removal. Referenced baselines remain intact.
func (r *Runtime) collectCatalogRecovery(ctx context.Context, limit int, maxBytes int64) (int, int64, int, error) {
	inventory, err := r.store.captureCatalogRecovery(ctx, limit, maxBytes)
	if err != nil {
		return inventory.scanned, inventory.bytes, 0, err
	}
	keepBaselines := make(map[string]bool)
	var candidates []capturedCatalogRecovery
	for _, record := range inventory.records {
		generation, err := r.client.Generation(ctx, record.record.GenerationID)
		if errors.IsNotFound(err) {
			candidates = append(candidates, record)
			continue
		}
		if err != nil {
			return inventory.scanned, inventory.bytes, 0, err
		}
		if err := record.record.validate(generation); err != nil {
			return inventory.scanned, inventory.bytes, 0, err
		}
		keepBaselines[record.record.BaselineChecksum] = true
	}
	removed := 0
	for _, record := range candidates {
		if _, err := r.client.Generation(ctx, record.record.GenerationID); !errors.IsNotFound(err) {
			if err != nil {
				return inventory.scanned, inventory.bytes, removed, err
			}
			keepBaselines[record.record.BaselineChecksum] = true
			continue
		}
		deleted, err := inventory.inputs.CompareAndRemoveFileContext(ctx, record.name, record.raw)
		if deleted {
			removed++
		}
		if err != nil {
			return inventory.scanned, inventory.bytes, removed, err
		}
	}
	checksums := make([]string, 0, len(inventory.baseline))
	for checksum := range inventory.baseline {
		checksums = append(checksums, checksum)
	}
	slices.Sort(checksums)
	for _, checksum := range checksums {
		if keepBaselines[checksum] {
			continue
		}
		deleted, err := inventory.baselines.CompareAndRemoveFileContext(ctx, checksum+".json.gz", inventory.baseline[checksum])
		if deleted {
			removed++
		}
		if err != nil {
			return inventory.scanned, inventory.bytes, removed, err
		}
	}
	return inventory.scanned, inventory.bytes, removed, ctx.Err()
}

// captureCatalogRecoveryBaseline reuses only an exact verified compiled artifact.
// Older baselines and different encodings retain their complete validation path.
func captureCatalogRecoveryBaseline(ctx context.Context, raw []byte, checksum string) (capturedRecoveryBaseline, error) {
	if err := ctx.Err(); err != nil {
		return capturedRecoveryBaseline{}, err
	}
	compiled, err := embeddedRecoveryBaseline()
	if err != nil {
		return capturedRecoveryBaseline{}, err
	}
	var generation catalogs.Generation
	var catalog *catalogs.Catalog
	if checksum == compiled.manifestChecksum && bytes.Equal(raw, compiled.data) {
		generation, err = bootstrap.Generation()
		if err == nil {
			catalog, _, err = bootstrap.Embedded()
		}
	} else {
		generation, err = decodeCatalogRecoveryBaseline(ctx, raw, checksum)
		if err == nil {
			catalog, err = catalogs.DecodeCatalogGeneration(generation)
		}
	}
	if err != nil {
		return capturedRecoveryBaseline{}, err
	}
	return capturedRecoveryBaseline{generation: generation, raw: raw, layers: layerSet{fleetBaseline: &generation, embedded: starmap.CatalogState{Catalog: catalog}}}, ctx.Err()
}
