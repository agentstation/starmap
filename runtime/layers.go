package runtime

import (
	"context"
	"encoding/json"
	"github.com/agentstation/starmap"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
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

// layerSet holds the four layers that produce the effective catalog. The
// layers are the verified embedded baseline, the selected upstream source,
// the retained per-provider observations, and the built immutable result.
type layerSet struct {
	embedded         starmap.CatalogState
	source           *sourceLayer
	providers        map[providerEvidenceKey]ProviderLayer
	sequence         uint64
	providerBindings *providerBindingPolicy
}

// empty reports whether any retained layer sits above the embedded baseline.
func (l *layerSet) empty() bool {
	return l.source == nil && len(l.providers) == 0
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
		return !l.providerBindings.permits(l.providers[key])
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
// enriches the result in stable order, so one failed provider keeps its
// last-known-good records.
func (l *layerSet) build(baseline starmap.CatalogState) (starmap.CatalogState, error) {
	base := baseline.Catalog
	state := starmap.CatalogState{
		GenerationID: baseline.GenerationID,
		GeneratedAt:  baseline.GeneratedAt,
	}
	if l.source != nil {
		decoded, err := catalogs.DecodeCatalogPayload(l.source.Payload)
		if err != nil {
			return starmap.CatalogState{}, errors.WrapResource(
				"decode", "retained source layer", l.source.GenerationID, err)
		}
		base = decoded
		state.GenerationID = l.source.GenerationID
		state.GeneratedAt = l.source.PublishedAt
	}
	if base == nil {
		return starmap.CatalogState{}, &errors.ValidationError{
			Field: "effective catalog", Message: "has no baseline",
		}
	}

	builder := catalogs.NewEmpty()
	if err := builder.MergeWith(base, catalogs.WithStrategy(catalogs.MergeReplaceAll)); err != nil {
		return starmap.CatalogState{}, errors.WrapResource(
			"merge", "effective catalog baseline", state.GenerationID, err)
	}
	active := l.activeProviderOrder()
	for _, id := range active {
		layer := l.providers[id]
		// A retained provider layer holds one provider observation. It carries
		// serving records that name an authored model of the baseline, so the
		// layer alone resolves no canonical authorship. An offering that names
		// no authored model of the merged result stays out of the effective
		// catalog, because a published catalog holds linked offerings only.
		observed, err := catalogs.DecodeSourceObservationPayload(layer.Payload)
		if err != nil {
			return starmap.CatalogState{}, errors.WrapResource(
				"decode", "retained provider layer", string(id.providerID), err)
		}
		linked, unresolved, err := linkProviderOfferings(builder, observed)
		if err != nil {
			return starmap.CatalogState{}, errors.WrapResource(
				"link", "retained provider layer", string(id.providerID), err)
		}
		if err := builder.MergeWith(linked, catalogs.WithStrategy(catalogs.MergeEnrichEmpty)); err != nil {
			return starmap.CatalogState{}, errors.WrapResource(
				"merge", "retained provider layer", string(id.providerID), err)
		}
		if err := mergeAuthoredModels(builder, observed); err != nil {
			return starmap.CatalogState{}, errors.WrapResource(
				"merge", "retained authored models", string(id.providerID), err)
		}
		if unresolved > 0 {
			logging.Info().
				Str("provider_id", string(id.providerID)).
				Int("unresolved_offerings", unresolved).
				Msg("Provider offerings without a canonical model reference stay out of the effective catalog")
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
	// Local acquisition changes the served bytes, so the result is no longer
	// the generation that the layers started from. A reused identity would let
	// a downstream treat two different catalogs as one generation. The hop
	// therefore derives its own identity from that identity and the served
	// digest. The upstream layer supplies it, and the embedded baseline supplies
	// it when the runtime retains no upstream layer. Only the layers decide the
	// derived identity, so two rebuilds of the same layers keep one identity,
	// and a durable commit publishes that same identity. A baseline that names
	// no identity leaves the identity to the publication.
	if l.providerBindings != nil {
		state.GenerationID, err = l.providerBindings.generationID(state.GenerationID, state.PayloadChecksum)
		if err != nil {
			return starmap.CatalogState{}, err
		}
	} else if len(active) > 0 && state.GenerationID != "" {
		state.GenerationID = deriveEffectiveGenerationID(state.GenerationID, state.PayloadChecksum)
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
// served payload differs from the upstream payload.
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

// linkProviderOfferings returns the provider records of one observation that
// name an authored model. The authored models of the builder and of the
// observation both count. An offering without a canonical link takes the link
// of the same offering in the builder. A baseline link therefore survives a
// provider reply that omits it. The result leaves out an offering that still
// names no authored model and counts it. The effective catalog then publishes
// without it, and no provider reply blocks a rebuild.
func linkProviderOfferings(builder *catalogs.Builder, observed catalogs.Reader) (catalogs.Reader, int, error) {
	authored := make(map[catalogs.ModelDefinitionID]struct{})
	for _, record := range builder.AuthoredModels() {
		authored[record.ID()] = struct{}{}
	}
	for _, record := range observed.AuthoredModels() {
		authored[record.ID()] = struct{}{}
	}
	linked, err := catalogs.NewBuilderFrom(observed)
	if err != nil {
		return nil, 0, err
	}
	unresolved := 0
	for _, provider := range linked.Providers().List() {
		var baseline map[string]*catalogs.Model
		if current, err := builder.Provider(provider.ID); err == nil {
			baseline = current.Models
		}
		models := make(map[string]*catalogs.Model, len(provider.Models))
		for modelID, model := range provider.Models {
			if model == nil {
				continue
			}
			offering := catalogs.DeepCopyModel(*model)
			if !resolvesAuthoredModel(authored, offering.ModelRef) {
				offering.ModelRef = ""
				if prior := baseline[modelID]; prior != nil && resolvesAuthoredModel(authored, prior.ModelRef) {
					offering.ModelRef = prior.ModelRef
				}
			}
			if offering.ModelRef == "" {
				unresolved++
				continue
			}
			models[modelID] = &offering
		}
		provider.Models = models
		if err := linked.SetProvider(provider); err != nil {
			return nil, 0, err
		}
	}
	return linked, unresolved, nil
}

// resolvesAuthoredModel reports whether a canonical reference is well formed
// and names an authored model of the merged result.
func resolvesAuthoredModel(authored map[catalogs.ModelDefinitionID]struct{}, ref catalogs.ModelDefinitionID) bool {
	if ref == "" {
		return false
	}
	if _, _, err := catalogs.ParseModelDefinitionID(ref); err != nil {
		return false
	}
	_, found := authored[ref]
	return found
}

// mergeAuthoredModels adds the authored records that a provider layer needs.
// The enrich-empty merge carries providers and authors, so a provider model
// would otherwise reference an authored record that the effective catalog does
// not hold. An existing record wins, because enrichment never overwrites.
func mergeAuthoredModels(builder *catalogs.Builder, source catalogs.Reader) error {
	present := make(map[catalogs.ModelDefinitionID]struct{})
	for _, record := range builder.AuthoredModels() {
		present[record.ID()] = struct{}{}
	}
	for _, record := range source.AuthoredModels() {
		if _, found := present[record.ID()]; found {
			continue
		}
		if err := builder.SetAuthorModel(record.AuthorID, record.Model); err != nil {
			return err
		}
	}
	return nil
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
func (r *Runtime) loadRetainedLayers() error {
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
	r.mu.Lock()
	defer r.mu.Unlock()
	r.layers.source = source
	r.layers.providers = providers
	return nil
}
