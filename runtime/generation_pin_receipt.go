package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	generationPinRecordFile     = "generation-pin.json"
	generationPinRecordVersion  = 1
	maxGenerationPinRecordBytes = 32 << 10
	pinPrepared                 = "prepared"
	pinAccepted                 = "accepted"
	pinReleased                 = "released"
)

// GenerationPinAcceptance identifies one verified selection and its publication.
// An origin rollback publishes the selected payload under a new authority generation.
// RequestedAt and AcceptedAt record local acceptance, not provider observation time.
type GenerationPinAcceptance struct {
	OperationID          string                        `json:"operation_id"`
	SelectedGenerationID string                        `json:"selected_generation_id"`
	AcceptedGenerationID string                        `json:"accepted_generation_id"`
	PreviousGenerationID string                        `json:"previous_generation_id"`
	PayloadChecksum      string                        `json:"payload_checksum"`
	AuthorityHead        catalogs.CatalogAuthorityHead `json:"authority_head,omitzero"`
	RequestedAt          time.Time                     `json:"requested_at"`
	AcceptedAt           time.Time                     `json:"accepted_at,omitzero"`
}

type generationPinRecord struct {
	Version    int                     `json:"version"`
	Binding    string                  `json:"binding"`
	Phase      string                  `json:"phase"`
	Receipt    GenerationPinAcceptance `json:"receipt"`
	ReleasedAt time.Time               `json:"released_at,omitzero"`
}

// PinAcceptance returns the current acceptance and whether the runtime retained it on disk.
// A missing acceptance returns a zero record and false. This method reads memory only.
func (r *Runtime) PinAcceptance() (GenerationPinAcceptance, bool) {
	if r == nil {
		return GenerationPinAcceptance{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.pinRecord == nil || r.pinRecord.Phase != pinAccepted || r.config.generationPin == "" {
		return GenerationPinAcceptance{}, false
	}
	return r.pinRecord.Receipt, r.store.durable()
}

// pinBinding excludes credentials and schedule settings from the authority identity.
func (r *Runtime) pinBinding() string {
	source := r.config.source
	originAuthority, originPolicy := "", ""
	if r.config.origin != nil {
		originAuthority, originPolicy = r.config.origin.config.AuthorityID, r.config.origin.config.PolicyID
	}
	identity := source.SafeIdentity()
	if r.source != nil {
		identity = r.source.Identity()
	}
	fields := []string{"pin-authority-binding/v1", string(source.Kind), identity, originAuthority, originPolicy}
	if source.Kind == SourcePublic || source.Kind == SourceGitHub {
		fields = append(fields, source.Repository, source.Channel, source.SignerWorkflow)
	}
	if r.requiresAuthority() {
		fields = append(fields, string(StartupRequireAuthority), source.AuthorityID, source.PolicyID)
	}
	encoded, _ := json.Marshal(fields)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func (r *Runtime) initializePinRecord() error {
	record, err := r.store.loadPinRecord()
	if err != nil {
		return err
	}
	r.pinRecord = record
	if record == nil || record.Phase == pinReleased {
		return nil
	}
	if record.Phase == pinPrepared && r.config.generationPin != record.Receipt.SelectedGenerationID {
		return pinRecordConflict("resolve the pending selection before changing the configured pin")
	}
	if r.config.generationPin == record.Receipt.SelectedGenerationID && record.Binding != r.pinBinding() {
		return pinRecordConflict("the pin belongs to a different source or authority configuration")
	}
	return nil
}

func (s *layerStore) loadPinRecord() (*generationPinRecord, error) {
	if !s.durable() {
		return nil, nil
	}
	raw, err := s.directory.ReadFile(generationPinRecordFile, maxGenerationPinRecordBytes)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.WrapIO("read pin acceptance", generationPinRecordFile, err)
	}
	var record generationPinRecord
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return nil, errors.WrapParse("pin acceptance", "", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, pinRecordConflict("the acceptance file must contain one complete record")
	}
	if err := record.validate(); err != nil {
		return nil, err
	}
	return &record, nil
}

func (record generationPinRecord) validate() error {
	if record.Version != generationPinRecordVersion {
		return pinRecordConflict("the acceptance version is unsupported")
	}
	binding, err := hex.DecodeString(record.Binding)
	if err != nil || len(binding) != sha256.Size {
		return pinRecordConflict("the authority binding is invalid")
	}
	receipt := record.Receipt
	if receipt.OperationID == "" || receipt.SelectedGenerationID == "" || receipt.AcceptedGenerationID == "" || receipt.PayloadChecksum == "" || receipt.RequestedAt.IsZero() {
		return pinRecordConflict("the acceptance identity is incomplete")
	}
	if receipt.AuthorityHead != (catalogs.CatalogAuthorityHead{}) {
		if err := receipt.AuthorityHead.Validate(); err != nil {
			return err
		}
		if receipt.AuthorityHead.GenerationID != receipt.AcceptedGenerationID || receipt.AuthorityHead.PayloadChecksum != receipt.PayloadChecksum {
			return pinRecordConflict("the authority head differs from the accepted artifact")
		}
	}
	switch record.Phase {
	case pinPrepared:
		if !receipt.AcceptedAt.IsZero() || !record.ReleasedAt.IsZero() {
			return pinRecordConflict("a prepared selection cannot claim acceptance or release")
		}
	case pinAccepted:
		if receipt.AcceptedAt.IsZero() || !record.ReleasedAt.IsZero() {
			return pinRecordConflict("accepted selection times are incomplete")
		}
	case pinReleased:
		if receipt.AcceptedAt.IsZero() || record.ReleasedAt.IsZero() {
			return pinRecordConflict("released selection times are incomplete")
		}
	default:
		return pinRecordConflict("the acceptance phase is unsupported")
	}
	return nil
}

func (s *layerStore) savePinRecord(ctx context.Context, record generationPinRecord) error {
	if err := record.validate(); err != nil {
		return err
	}
	if !s.durable() {
		return ctx.Err()
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if len(encoded) > maxGenerationPinRecordBytes {
		return pinRecordConflict("the acceptance exceeds its storage limit")
	}
	return s.directory.WriteFileContext(ctx, generationPinRecordFile, encoded, ".layer-")
}

func (r *Runtime) finishPinRelease(ctx context.Context) error {
	if r.config.generationPin != "" || r.pinRecord == nil || r.pinRecord.Phase != pinAccepted {
		return nil
	}
	record := *r.pinRecord
	record.Phase, record.ReleasedAt = pinReleased, r.config.now().UTC()
	if err := r.store.savePinRecord(ctx, record); err != nil {
		return err
	}
	r.pinRecord = &record
	return nil
}

func (r *Runtime) releasesAcceptedPin(current string) bool {
	return r.config.generationPin == "" && r.pinRecord != nil && r.pinRecord.Phase == pinAccepted && r.pinRecord.Receipt.AcceptedGenerationID == current
}

func pinRecordConflict(message string) error {
	return &errors.ConflictError{Resource: "generation pin acceptance", Message: message}
}
