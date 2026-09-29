package runtime

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"slices"
	"strings"

	"github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/pkg/catalogs"
)

const generationInputsDirectory = "generation-inputs"
const maxCatalogRecoveryBytes = MaxFleetRecoveryBytes

// CatalogRecovery binds retained private inputs to one complete catalog manifest.
// It records reconstruction evidence, not publication success or serving permission.
// Inputs use the same private encoding as fleet publications.
type CatalogRecovery struct {
	ManifestChecksum string        `json:"manifest_checksum"`
	Inputs           FleetRecovery `json:"inputs"`
}

// Validate checks the complete generation binding and bounded input bytes.
func (r CatalogRecovery) Validate(generation catalogs.Generation) error {
	if err := generation.Validate(); err != nil {
		return err
	}
	digest, err := catalogManifestChecksum(generation)
	if err != nil {
		return err
	}
	if r.ManifestChecksum != digest {
		return invalidInputPublication("recovery inputs belong to another catalog manifest")
	}
	return r.Inputs.Validate(generation)
}

func catalogManifestChecksum(generation catalogs.Generation) (string, error) {
	data, err := json.Marshal(generation.Manifest, json.Deterministic(true))
	if err != nil {
		return "", err
	}
	return fleetRecoveryChecksum(data), nil
}

func validCatalogRecoveryName(name string) bool {
	base, ok := strings.CutSuffix(name, ".json.gz")
	if !ok {
		return false
	}
	manifest, inputs, ok := strings.Cut(base, "-")
	return ok && validFleetChecksum(manifest) && validFleetChecksum(inputs)
}

type localRecoveryContextKey struct{}
type localRecoveryAttempt struct {
	store    *layerStore
	inputs   fleetRecoveryRecord
	baseline catalogRecoveryBaseline
}

func (r *Runtime) prepareLocalRecovery(ctx context.Context, layers layerSet, pin *generationPinRecord) (context.Context, error) {
	if r.config.fleetStore != nil || !r.store.durable() || !r.client.PublishesDurably() {
		return ctx, nil
	}
	if layers.embeddedManifest == nil {
		manifest, err := bootstrap.GenerationManifest()
		if err != nil {
			return nil, err
		}
		layers.embedded, layers.embeddedManifest = r.client.EmbeddedCatalogState(), &manifest
	}
	layers.requireAuthority = r.requiresAuthority()
	layers.providerBindings, layers.acquisitionSources = r.config.providerBindings, r.config.acquisitionSources
	layers.publisherAliases, layers.sourceConfiguration = slices.Clone(r.config.source.Aliases), slices.Clone(r.config.sourceConfiguration)
	inputs, err := makeFleetRecoveryRecord(ctx, layers, pin)
	if err != nil {
		return nil, err
	}
	baseline, err := localRecoveryBaseline(layers)
	if err != nil {
		return nil, err
	}
	return context.WithValue(ctx, localRecoveryContextKey{}, &localRecoveryAttempt{store: r.store, inputs: inputs, baseline: baseline}), nil
}

func retainLocalGeneration(ctx context.Context, generation catalogs.Generation) error {
	attempt, _ := ctx.Value(localRecoveryContextKey{}).(*localRecoveryAttempt)
	if attempt == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := generation.Validate(); err != nil {
		return err
	}
	digest, err := catalogManifestChecksum(generation)
	if err != nil {
		return err
	}
	record := localCatalogRecovery{Version: localCatalogRecoveryVersion, ManifestChecksum: digest, GenerationID: generation.Manifest.GenerationID, PayloadChecksum: generation.Manifest.Payload.Checksum, BaselineChecksum: attempt.baseline.manifestChecksum, Inputs: attempt.inputs}
	if err := record.validate(generation); err != nil {
		return err
	}
	return attempt.store.retainCatalogRecovery(ctx, record, attempt.baseline)
}

// ReadCatalogRecovery reads exact private inputs without opening a runtime.
// The caller must fence writers and supply an independently retained generation and record checksum.
// Missing history refuses recovery. Current directory inputs never replace missing historical inputs.
func ReadCatalogRecovery(ctx context.Context, path string, owner DirectoryOwner, identity string, generation catalogs.Generation, recordChecksum string) (CatalogRecovery, error) {
	if ctx == nil {
		return CatalogRecovery{}, invalidInputPublication("recovery requires a context")
	}
	if err := ctx.Err(); err != nil {
		return CatalogRecovery{}, err
	}
	if !validFleetChecksum(recordChecksum) {
		return CatalogRecovery{}, invalidInputPublication("recovery requires an exact record checksum")
	}
	if err := InspectRetainedDirectory(ctx, path, owner, identity); err != nil {
		return CatalogRecovery{}, err
	}
	digest, err := catalogManifestChecksum(generation)
	if err != nil {
		return CatalogRecovery{}, err
	}
	store, err := existingLayerStore(path)
	if err != nil {
		return CatalogRecovery{}, err
	}
	if !store.durable() {
		return CatalogRecovery{}, invalidInputPublication("catalog recovery is not retained")
	}
	directory, err := store.directory.ExistingChild(generationInputsDirectory)
	if err != nil {
		return CatalogRecovery{}, err
	}
	record, _, err := readLocalCatalogRecovery(ctx, directory, digest+"-"+recordChecksum+".json.gz", maxCatalogRecoveryBytes)
	if err != nil {
		return CatalogRecovery{}, err
	}
	if err := record.validate(generation); err != nil {
		return CatalogRecovery{}, err
	}
	baseline, _, err := store.readCatalogRecoveryBaseline(ctx, record.BaselineChecksum)
	if err != nil {
		return CatalogRecovery{}, err
	}
	inputs := record.Inputs
	inputs.Baseline = baseline
	data, err := encodeFleetRecoveryRecord(inputs)
	if err != nil {
		return CatalogRecovery{}, err
	}
	result := CatalogRecovery{ManifestChecksum: digest, Inputs: FleetRecovery{GenerationID: record.GenerationID, PayloadChecksum: record.PayloadChecksum, Checksum: fleetRecoveryChecksum(data), Data: data}}
	return result, ctx.Err()
}

// CaptureFleetCatalogRecovery copies a validated fleet publication's private inputs.
// The result preserves the complete generation binding without inventing local ownership.
// The host must retain the original fleet selection and verify deployment settings separately.
func CaptureFleetCatalogRecovery(ctx context.Context, snapshot FleetSnapshot) (CatalogRecovery, error) {
	if err := ValidateFleetRecovery(ctx, snapshot); err != nil {
		return CatalogRecovery{}, err
	}
	manifest, err := catalogManifestChecksum(snapshot.Publication.Generation)
	if err != nil {
		return CatalogRecovery{}, err
	}
	inputs := snapshot.Publication.Recovery
	inputs.Data = bytes.Clone(inputs.Data)
	return CatalogRecovery{ManifestChecksum: manifest, Inputs: inputs}, ctx.Err()
}
