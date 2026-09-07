package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// inputPublicationName holds the current retention transaction or an idle marker.
	inputPublicationName = "publication.json"
	// inputPublicationDirectory holds immutable records referenced by the transaction.
	inputPublicationDirectory = "publication-inputs"
	inputPublicationVersion   = 1
	inputPublicationPrepared  = "prepared"
	inputPublicationCommitted = "committed"
	inputPublicationIdle      = "idle"
)

// inputPublication connects staged inputs to the catalog commit that accepts them.
// Prepared records change no active retained input. Committed records require replay.
type inputPublication struct {
	Version          int      `json:"version"`
	Phase            string   `json:"phase"`
	ExpectedID       string   `json:"expected_generation_id,omitempty"`
	ExpectedChecksum string   `json:"expected_payload_checksum,omitempty"`
	GenerationID     string   `json:"generation_id,omitempty"`
	PayloadChecksum  string   `json:"payload_checksum,omitempty"`
	Source           string   `json:"source,omitempty"`
	Providers        []string `json:"providers,omitempty"`
}

func invalidInputPublication(message string) error {
	return &errors.ConflictError{Resource: "runtime input publication", Message: message}
}

func (s *layerStore) loadInputPublication() (*inputPublication, error) {
	if !s.durable() {
		return nil, nil
	}
	raw, err := readLayerFile(s.directory, inputPublicationName)
	if err != nil || raw == nil {
		return nil, err
	}
	var record inputPublication
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return nil, errors.WrapParse("runtime input publication", inputPublicationName, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, invalidInputPublication("record contains trailing data")
	}
	if record.Version != inputPublicationVersion {
		return nil, invalidInputPublication("unsupported record version")
	}
	if record.Phase == inputPublicationIdle {
		if record.ExpectedID != "" || record.ExpectedChecksum != "" || record.GenerationID != "" || record.PayloadChecksum != "" || record.Source != "" || len(record.Providers) != 0 {
			return nil, invalidInputPublication("idle record contains pending inputs")
		}
		return nil, nil
	}
	if record.Phase != inputPublicationPrepared && record.Phase != inputPublicationCommitted {
		return nil, invalidInputPublication("unsupported record phase")
	}
	if record.GenerationID == "" || record.PayloadChecksum == "" || (record.Source == "" && len(record.Providers) == 0) {
		return nil, invalidInputPublication("incomplete record")
	}
	return &record, nil
}

func (s *layerStore) writeInputPublication(ctx context.Context, record inputPublication) error {
	if !s.durable() {
		return ctx.Err()
	}
	return s.writeContext(ctx, s.directory, inputPublicationName, record)
}

func (s *layerStore) clearInputPublication(ctx context.Context) error {
	return s.writeInputPublication(ctx, inputPublication{Version: inputPublicationVersion, Phase: inputPublicationIdle})
}

func (s *layerStore) stageInput(ctx context.Context, value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", errors.WrapResource("encode", "runtime publication input", "", err)
	}
	if len(raw) > maxLayerBytes {
		return "", invalidInputPublication("input exceeds the retained layer bound")
	}
	digest := sha256.Sum256(raw)
	name := hex.EncodeToString(digest[:]) + ".json"
	if !s.durable() {
		return name, nil
	}
	directory, err := s.directory.Child(inputPublicationDirectory)
	if err != nil {
		return "", err
	}
	existing, err := readLayerFile(directory, name)
	if err != nil {
		return "", err
	}
	if existing != nil {
		if !bytes.Equal(existing, raw) {
			return "", invalidInputPublication("immutable input identity contains different bytes")
		}
		return name, nil
	}
	if err := directory.WriteFileContext(ctx, name, raw, ".input-"); err != nil {
		return "", err
	}
	return name, nil
}

func (s *layerStore) readInput(directory *privatefiles.Directory, name string, value any) error {
	stem, ok := strings.CutSuffix(name, ".json")
	digest, err := hex.DecodeString(stem)
	if !ok || err != nil || len(digest) != sha256.Size || hex.EncodeToString(digest) != stem {
		return invalidInputPublication("invalid input reference")
	}
	raw, err := readLayerFile(directory, name)
	if err != nil {
		return err
	}
	if raw == nil {
		return invalidInputPublication("referenced input is missing")
	}
	actual := sha256.Sum256(raw)
	if !bytes.Equal(actual[:], digest) {
		return invalidInputPublication("referenced input digest does not match")
	}
	if err := json.Unmarshal(raw, value); err != nil {
		return errors.WrapParse("runtime publication input", name, err)
	}
	return nil
}

// publicationInputs validates every input before replay can replace a retained file.
func (s *layerStore) publicationInputs(record inputPublication) (*sourceLayer, []ProviderLayer, error) {
	directory, err := s.directory.ExistingChild(inputPublicationDirectory)
	if err != nil {
		return nil, nil, err
	}
	var source *sourceLayer
	if record.Source != "" {
		source = &sourceLayer{}
		if err := s.readInput(directory, record.Source, source); err != nil {
			return nil, nil, err
		}
		if source.GenerationID == "" || source.Identity == "" || source.Checksum != catalogs.DescribeCatalogPayload(source.Payload).Checksum {
			return nil, nil, invalidInputPublication("source identity or digest does not match")
		}
		if _, err := catalogs.DecodeCatalogPayload(source.Payload); err != nil {
			return nil, nil, err
		}
	}
	providers := make([]ProviderLayer, 0, len(record.Providers))
	seen := map[providerEvidenceKey]bool{}
	for _, name := range record.Providers {
		var layer ProviderLayer
		if err := s.readInput(directory, name, &layer); err != nil {
			return nil, nil, err
		}
		if err := layer.validate(); err != nil {
			return nil, nil, err
		}
		key := layer.evidenceKey()
		if seen[key] {
			return nil, nil, invalidInputPublication("duplicate provider input")
		}
		seen[key] = true
		providers = append(providers, layer)
	}
	if err := validateProviderScopes(providers, nil); err != nil {
		return nil, nil, err
	}
	return source, providers, nil
}

func (s *layerStore) applyPublicationInputs(ctx context.Context, source *sourceLayer, providers []ProviderLayer) error {
	if source != nil {
		if err := s.saveSource(ctx, *source); err != nil {
			return err
		}
	}
	for _, layer := range providers {
		if err := s.saveProvider(ctx, layer); err != nil {
			return err
		}
	}
	return s.clearInputPublication(ctx)
}

// recoverInputPublication completes an accepted transaction before startup reads its files.
// An unresolved prepared transaction refuses startup when current state proves neither outcome.
func (s *layerStore) recoverInputPublication(ctx context.Context, current starmap.CatalogState) error {
	record, err := s.loadInputPublication()
	if err != nil || record == nil {
		return err
	}
	if record.Phase == inputPublicationPrepared {
		switch {
		case current.GenerationID == record.ExpectedID && current.PayloadChecksum == record.ExpectedChecksum:
			// An unchanged catalog cannot prove that a retention-only update committed.
			return s.clearInputPublication(ctx)
		case current.GenerationID == record.GenerationID && current.PayloadChecksum == record.PayloadChecksum:
			record.Phase = inputPublicationCommitted
			if err := s.writeInputPublication(ctx, *record); err != nil {
				return err
			}
		default:
			return invalidInputPublication("catalog state cannot resolve the prepared transaction")
		}
	}
	source, providers, err := s.publicationInputs(*record)
	if err != nil {
		return err
	}
	return s.applyPublicationInputs(ctx, source, providers)
}

// refuseInputPublication keeps a later update from replacing unresolved recovery evidence.
func (s *layerStore) refuseInputPublication() error {
	record, err := s.loadInputPublication()
	if err != nil {
		return err
	}
	if record != nil {
		return invalidInputPublication("retention recovery is required before another update")
	}
	return nil
}
