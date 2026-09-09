package runtime

import (
	"context"
	"encoding/json"
	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/sources"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/productpaths/policy"
)

const (
	// layerDirectoryName holds every retained runtime layer.
	layerDirectoryName = "catalog-runtime"

	// providerLayerDirectoryName holds one file per retained provider layer.
	providerLayerDirectoryName = "providers"

	// bindingLayerDirectoryName separates scoped records from legacy provider filenames.
	bindingLayerDirectoryName = "bindings"

	// sourceLayerFileName holds the retained upstream source layer.
	sourceLayerFileName = "source.json"

	// maxLayerBytes bounds one retained layer record. A larger record is
	// unsafe input, so the runtime rejects it instead of loading it.
	maxLayerBytes = 64 << 20
)

// sourceLayer is the retained upstream generation. The runtime keeps the
// verified payload, so a restart serves the last upstream catalog without a
// network reply.
type sourceLayer struct {
	Identity         string      `json:"identity"`
	GenerationID     string      `json:"generation_id"`
	Checksum         string      `json:"checksum"`
	Payload          []byte      `json:"payload"`
	PublishedAt      time.Time   `json:"published_at"`
	ChannelUpdatedAt time.Time   `json:"channel_updated_at"`
	ObservedAt       time.Time   `json:"observed_at"`
	Chain            []SourceHop `json:"chain,omitempty"`
}

// layerSet holds the inputs that produce the effective catalog: the embedded
// baseline, selected upstream source, provider observations, and manual history.
type layerSet struct {
	embedded           starmap.CatalogState
	source             *sourceLayer
	providers          map[providerEvidenceKey]ProviderLayer
	manual             *manualBatch
	sequence           uint64
	providerBindings   *providerBindingPolicy
	acquisitionSources *acquisitionSourcePolicy
	buildEvidence      starmap.CandidateEvidence
	acceptedSources    []sources.ID
}

// empty reports whether any retained layer sits above the embedded baseline.
func (l *layerSet) empty() bool {
	return l.source == nil && len(l.providers) == 0 && l.manual == nil
}

// providerOrder returns the retained provider identities in stable order, so
// two rebuilds of the same layers produce the same catalog.
func (l *layerSet) providerOrder() []providerEvidenceKey {
	order := make([]providerEvidenceKey, 0, len(l.providers))
	for id := range l.providers {
		order = append(order, id)
	}
	slices.SortFunc(order, compareProviderEvidenceKeys)
	return order
}

// activeProviderOrder selects permitted evidence without deleting retained records.
func (l *layerSet) activeProviderOrder() []providerEvidenceKey {
	order := l.providerOrder()
	return slices.DeleteFunc(order, func(key providerEvidenceKey) bool {
		return !l.acquisitionSources.permits(sources.ProvidersID) || !l.providerBindings.permits(l.providers[key])
	})
}

// setProvider replaces evidence only within one provider binding revision.
func (l *layerSet) setProvider(layer ProviderLayer) {
	if l.providers == nil {
		l.providers = make(map[providerEvidenceKey]ProviderLayer)
	}
	l.providers[layer.evidenceKey()] = layer
}

// build rebuilds the immutable effective catalog from the retained layers. The
// upstream source replaces the baseline. Each provider observation then
// uses canonical field authority in stable order, so one failed provider keeps its
// last-known-good records.
func (l *layerSet) build(ctx context.Context, baseline starmap.CatalogState) (starmap.CatalogState, error) {
	if err := ctx.Err(); err != nil {
		return starmap.CatalogState{}, err
	}
	selected, err := l.selectedBaseline(baseline)
	if err != nil {
		return starmap.CatalogState{}, err
	}
	base := selected.Catalog
	state := starmap.CatalogState{
		GenerationID: selected.GenerationID,
		GeneratedAt:  selected.GeneratedAt,
	}

	var builder *catalogs.Builder
	active := l.activeProviderOrder()
	l.buildEvidence = starmap.CandidateEvidence{}
	l.acceptedSources = nil
	if l.manual != nil {
		var err error
		builder, l.buildEvidence, err = l.reconcileManualInputs(ctx, base, state.GeneratedAt, active)
		if err != nil {
			return starmap.CatalogState{}, err
		}
	} else if len(active) > 0 {
		var err error
		builder, l.buildEvidence, err = l.reconcileProviders(ctx, base, state.GeneratedAt, active)
		if err != nil {
			return starmap.CatalogState{}, err
		}
	} else {
		var err error
		builder, err = catalogs.NewBuilderFrom(base)
		if err != nil {
			return starmap.CatalogState{}, errors.WrapResource("copy", "effective catalog baseline", state.GenerationID, err)
		}
	}

	catalog, err := builder.Build()
	if err != nil {
		return starmap.CatalogState{}, errors.WrapResource(
			"publish", "effective catalog", state.GenerationID, err)
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		return starmap.CatalogState{}, errors.WrapResource(
			"encode", "effective catalog", state.GenerationID, err)
	}
	l.sequence++
	state.Catalog = catalog
	state.PayloadChecksum = catalogs.DescribeCatalogPayload(payload).Checksum
	state.Sequence = baseline.Sequence + l.sequence
	// Receipts and review evidence are immutable generation content even when
	// another source supplies every selected catalog field.
	identityChecksum, err := effectiveEvidenceChecksum(state.PayloadChecksum, l.buildEvidence)
	if err != nil {
		return starmap.CatalogState{}, err
	}
	priorChecksum := identityChecksum
	identityChecksum, err = observationResetChecksum(identityChecksum, l.manual)
	if err != nil {
		return starmap.CatalogState{}, err
	}
	if identityChecksum != priorChecksum && state.GenerationID == "" {
		state.GenerationID = "local"
	}
	if l.providerBindings != nil {
		state.GenerationID, err = l.providerBindings.generationID(state.GenerationID, identityChecksum)
		if err != nil {
			return starmap.CatalogState{}, err
		}
	} else if len(l.buildEvidence.SourceObservations) > 0 && state.GenerationID != "" {
		state.GenerationID = deriveEffectiveGenerationID(state.GenerationID, identityChecksum)
	}
	if l.acquisitionSources != nil {
		state.GenerationID, err = l.acquisitionSources.generationID(state.GenerationID, identityChecksum)
		if err != nil {
			return starmap.CatalogState{}, err
		}
	}
	return state, nil
}

// effectiveGenerationSuffixLength bounds the digest suffix of a derived
// identity. It keeps the identity short and still tells two payloads apart.
const effectiveGenerationSuffixLength = 12

// effectiveGenerationLocalSuffix separates the upstream identity from the
// local digest of a derived identity.
const effectiveGenerationLocalSuffix = ".local."

// deriveEffectiveGenerationID returns the identity of a locally enriched
// upstream generation. It never returns the upstream identity, because the
// served payload or its source evidence differs from the upstream generation.
//
// A runtime with a catalog store publishes this identity, and a downstream
// subscriber addresses it as one URL path segment. The suffix therefore stays
// inside the remote protocol vocabulary of letters, digits, dot, dash, and
// underscore.
func deriveEffectiveGenerationID(upstream, checksum string) string {
	fragment := strings.TrimPrefix(checksum, "sha256:")
	if len(fragment) > effectiveGenerationSuffixLength {
		fragment = fragment[:effectiveGenerationSuffixLength]
	}
	if fragment == "" {
		fragment = "local"
	}
	return upstream + effectiveGenerationLocalSuffix + fragment
}

// layerStore retains the runtime layers durably. A runtime without a state
// directory keeps its layers in memory only, so a restart returns to the
// verified embedded baseline.
type layerStore struct {
	root      string
	directory *privatefiles.Directory
	providers *privatefiles.Directory
	bindings  *privatefiles.Directory
}

// newLayerStore prepares the durable layer directory. An empty directory
// selects memory-only retention.
func newLayerStore(directory string) (*layerStore, error) {
	if err := policy.Require("runtime-evidence", policy.OwnerOnly); err != nil {
		return nil, err
	}
	if directory == "" {
		return &layerStore{}, nil
	}
	root := filepath.Join(directory, layerDirectoryName)
	dir, err := privatefiles.NewDirectory(root)
	if err != nil {
		return nil, errors.WrapIO("open private evidence directory", root, err)
	}
	providers, err := dir.Child(providerLayerDirectoryName)
	if err != nil {
		return nil, errors.WrapIO("open private provider directory", root, err)
	}
	bindings, err := providers.Child(bindingLayerDirectoryName)
	if err != nil {
		return nil, errors.WrapIO("open private binding directory", root, err)
	}
	return &layerStore{root: root, directory: dir, providers: providers, bindings: bindings}, nil
}

func existingLayerStore(directory string) (*layerStore, error) {
	if err := policy.Require("runtime-evidence", policy.OwnerOnly); err != nil {
		return nil, err
	}
	root := filepath.Join(directory, layerDirectoryName)
	dir, err := privatefiles.ExistingDirectory(root)
	if os.IsNotExist(err) {
		return &layerStore{}, nil
	}
	if err != nil {
		return nil, err
	}
	providers, err := dir.ExistingChild(providerLayerDirectoryName)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	var bindings *privatefiles.Directory
	if providers != nil {
		bindings, err = providers.ExistingChild(bindingLayerDirectoryName)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	return &layerStore{root: root, directory: dir, providers: providers, bindings: bindings}, nil
}

// durable reports whether the store retains layers across a restart.
func (s *layerStore) durable() bool {
	return s != nil && s.root != ""
}

// loadSource returns the retained upstream layer. A missing layer is not an
// error: the runtime starts from the embedded baseline.
func (s *layerStore) loadSource() (*sourceLayer, error) {
	if !s.durable() {
		return nil, nil
	}
	path := filepath.Join(s.root, sourceLayerFileName)
	raw, err := readLayerFile(s.directory, sourceLayerFileName)
	if err != nil || raw == nil {
		return nil, err
	}
	layer := &sourceLayer{}
	if err := json.Unmarshal(raw, layer); err != nil {
		return nil, errors.WrapParse("retained source layer", path, err)
	}
	return layer, nil
}

// saveSource retains the upstream layer durably.
func (s *layerStore) saveSource(ctx context.Context, layer sourceLayer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !s.durable() {
		return nil
	}
	return s.writeContext(ctx, s.directory, sourceLayerFileName, layer)
}

// writeContext stages and flushes one private layer record before publication.
func (s *layerStore) writeContext(ctx context.Context, directory *privatefiles.Directory, path string, record any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return errors.WrapResource("encode", "runtime layer", path, err)
	}
	if len(encoded) > maxLayerBytes {
		return &errors.ResourceError{
			Operation: "retain",
			Resource:  "runtime layer",
			ID:        path,
			Err: &errors.ValidationError{
				Field: "layer_bytes", Value: len(encoded), Message: "exceeds the retained layer bound",
			},
		}
	}
	if err := directory.WriteFileContext(ctx, path, encoded, ".layer-"); err != nil {
		return errors.WrapIO("write private evidence", path, err)
	}
	return nil
}

// readLayerFile returns one bounded layer record. It returns nil bytes when the
// record is absent.
func readLayerFile(directory *privatefiles.Directory, name string) ([]byte, error) {
	raw, err := directory.ReadFile(name, maxLayerBytes)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.WrapIO("read private evidence", name, err)
	}
	return raw, nil
}

// validateProviderLayerID rejects a provider identity that cannot name a file
// safely. The identity reaches the filesystem, so an unsafe value fails early.
func validateProviderLayerID(id catalogs.ProviderID) error {
	name := string(id)
	if name == "" {
		return &errors.ValidationError{Field: "provider_id", Message: "is required"}
	}
	for _, character := range name {
		switch {
		case character >= 'a' && character <= 'z':
		case character >= 'A' && character <= 'Z':
		case character >= '0' && character <= '9':
		case character == '-' || character == '_' || character == '.':
		default:
			return &errors.ValidationError{
				Field: "provider_id", Value: name, Message: "holds an unsafe character",
			}
		}
	}
	if strings.Contains(name, "..") {
		return &errors.ValidationError{
			Field: "provider_id", Value: name, Message: "must not traverse a directory",
		}
	}
	return nil
}

// loadRetainedLayers restores the durable layers that a previous run left.
func (r *Runtime) loadRetainedLayers(ctx context.Context) error {
	source, err := r.store.loadSource()
	if err != nil {
		return err
	}
	providers, err := r.store.loadProviders()
	if err != nil {
		return err
	}
	if err := r.config.providerBindings.validateRetained(providers); err != nil {
		return err
	}
	manual, err := r.store.loadManualHistory(ctx)
	if err != nil {
		return err
	}
	if err := validateManualHistory(manual, r.config.providerBindings); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.layers.source = source
	r.layers.providers = providers
	r.layers.manual = manual
	return nil
}
