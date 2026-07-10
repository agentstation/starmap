package catalogs

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/types"
)

var (
	requiredManifestJSONFields = []string{
		"manifest_version", "schema_version", "generation_id", "generated_at",
		"payload", "validation", "sync_run_id", "source_observations",
		"completeness", "degraded", "consumer_compatibility",
	}
	requiredPayloadJSONFields         = []string{"checksum", "size_bytes", "media_type"}
	requiredValidationJSONFields      = []string{"validator_version", "validated_at", "status", "error_count", "warning_count", "checks"}
	requiredValidationCheckJSONFields = []string{"name", "status"}
	requiredObservationJSONFields     = []string{"source", "observation_id", "observed_at", "revision", "completeness", "status", "evidence_checksum"}
	requiredObservationRevisionFields = []string{"kind"}
	requiredCompatibilityJSONFields   = []string{"min_schema_version", "max_schema_version"}
)

const (
	// CurrentGenerationManifestVersion is the manifest envelope version emitted
	// by this release. It is intentionally independent of the Starmap binary
	// version and the catalog payload schema version.
	CurrentGenerationManifestVersion uint64 = 1

	// CurrentCatalogSchemaVersion identifies the canonical catalog payload
	// schema emitted by this release.
	CurrentCatalogSchemaVersion uint64 = 1

	// CatalogPayloadMediaType identifies the canonical JSON catalog payload.
	CatalogPayloadMediaType = "application/vnd.agentstation.starmap.catalog+json"

	checksumAlgorithmPrefix = "sha256:"
)

// GenerationCompleteness describes whether a generation contains every record
// expected from the observations used to build it.
type GenerationCompleteness string

const (
	// GenerationCompletenessComplete means the generation contains all expected
	// records. It may still be degraded for another reason, such as stale input.
	GenerationCompletenessComplete GenerationCompleteness = "complete"
	// GenerationCompletenessPartial means at least one expected input or record
	// is absent and the generation must also be marked degraded.
	GenerationCompletenessPartial GenerationCompleteness = "partial"
)

// GenerationValidationStatus is the overall result of generation validation.
type GenerationValidationStatus string

const (
	// GenerationValidationPassed means every required validation check passed.
	GenerationValidationPassed GenerationValidationStatus = "passed"
	// GenerationValidationFailed means at least one required check failed. A
	// failed generation is evidence, but is not eligible for publication.
	GenerationValidationFailed GenerationValidationStatus = "failed"
)

// GenerationValidationCheckStatus is the result of one validation check.
type GenerationValidationCheckStatus string

const (
	// GenerationValidationCheckPassed records a successful check.
	GenerationValidationCheckPassed GenerationValidationCheckStatus = "passed"
	// GenerationValidationCheckWarning records a non-fatal validation warning.
	GenerationValidationCheckWarning GenerationValidationCheckStatus = "warning"
	// GenerationValidationCheckFailed records a failed required check.
	GenerationValidationCheckFailed GenerationValidationCheckStatus = "failed"
)

// PayloadDescriptor binds a generation manifest to exact immutable bytes.
type PayloadDescriptor struct {
	Checksum  string `json:"checksum" yaml:"checksum"`
	SizeBytes int64  `json:"size_bytes" yaml:"size_bytes"`
	MediaType string `json:"media_type" yaml:"media_type"`
}

// DescribeCatalogPayload returns the descriptor for canonical catalog bytes.
func DescribeCatalogPayload(payload []byte) PayloadDescriptor {
	digest := sha256.Sum256(payload)
	return PayloadDescriptor{
		Checksum:  checksumAlgorithmPrefix + hex.EncodeToString(digest[:]),
		SizeBytes: int64(len(payload)),
		MediaType: CatalogPayloadMediaType,
	}
}

// Verify checks that payload exactly matches the descriptor.
func (d PayloadDescriptor) Verify(payload []byte) error {
	if int64(len(payload)) != d.SizeBytes {
		return &errors.ValidationError{
			Field:   "payload.size_bytes",
			Value:   len(payload),
			Message: fmt.Sprintf("payload has %d bytes, want %d", len(payload), d.SizeBytes),
		}
	}
	actual := DescribeCatalogPayload(payload)
	if actual.Checksum != d.Checksum {
		return &errors.ValidationError{
			Field:   "payload.checksum",
			Value:   actual.Checksum,
			Message: fmt.Sprintf("payload checksum does not match %s", d.Checksum),
		}
	}
	return nil
}

// GenerationValidationCheck records one deterministic validation decision.
type GenerationValidationCheck struct {
	Name    string                          `json:"name" yaml:"name"`
	Status  GenerationValidationCheckStatus `json:"status" yaml:"status"`
	Message string                          `json:"message,omitempty" yaml:"message,omitempty"`
}

// GenerationValidationReport records the validator identity and exact outcome
// that made a candidate eligible (or ineligible) for publication.
type GenerationValidationReport struct {
	ValidatorVersion string                      `json:"validator_version" yaml:"validator_version"`
	ValidatedAt      time.Time                   `json:"validated_at" yaml:"validated_at"`
	Status           GenerationValidationStatus  `json:"status" yaml:"status"`
	ErrorCount       int                         `json:"error_count" yaml:"error_count"`
	WarningCount     int                         `json:"warning_count" yaml:"warning_count"`
	Checks           []GenerationValidationCheck `json:"checks" yaml:"checks"`
}

// SourceObservationLink binds a generation to one immutable source
// observation. The observation schema and retention policy are defined by the
// source pipeline; this link is deliberately small and replay-oriented.
type SourceObservationLink struct {
	Source           types.SourceID                `json:"source" yaml:"source"`
	ObservationID    string                        `json:"observation_id" yaml:"observation_id"`
	ObservedAt       time.Time                     `json:"observed_at" yaml:"observed_at"`
	Revision         types.ObservationRevision     `json:"revision" yaml:"revision"`
	Completeness     types.ObservationCompleteness `json:"completeness" yaml:"completeness"`
	Status           types.ObservationStatus       `json:"status" yaml:"status"`
	EvidenceChecksum string                        `json:"evidence_checksum" yaml:"evidence_checksum"`
}

// Validate verifies one complete source-observation link.
func (o SourceObservationLink) Validate() error {
	return validateObservationLinks([]SourceObservationLink{o})
}

// ConsumerCompatibility declares the catalog schema versions that can consume
// this generation. It never refers to a Starmap or Starport binary version.
type ConsumerCompatibility struct {
	MinSchemaVersion uint64 `json:"min_schema_version" yaml:"min_schema_version"`
	MaxSchemaVersion uint64 `json:"max_schema_version" yaml:"max_schema_version"`
}

// SupportsSchema reports whether a consumer catalog schema is compatible.
func (c ConsumerCompatibility) SupportsSchema(schemaVersion uint64) bool {
	return schemaVersion >= c.MinSchemaVersion && schemaVersion <= c.MaxSchemaVersion
}

// GenerationManifest describes one immutable, validated catalog generation.
// It is shared by local stores and distribution transports; transport-specific
// URLs, release tags, and binary versions do not belong in this domain record.
type GenerationManifest struct {
	ManifestVersion       uint64                     `json:"manifest_version" yaml:"manifest_version"`
	SchemaVersion         uint64                     `json:"schema_version" yaml:"schema_version"`
	GenerationID          string                     `json:"generation_id" yaml:"generation_id"`
	GeneratedAt           time.Time                  `json:"generated_at" yaml:"generated_at"`
	Payload               PayloadDescriptor          `json:"payload" yaml:"payload"`
	Validation            GenerationValidationReport `json:"validation" yaml:"validation"`
	SyncRunID             string                     `json:"sync_run_id" yaml:"sync_run_id"`
	SourceObservations    []SourceObservationLink    `json:"source_observations" yaml:"source_observations"`
	Completeness          GenerationCompleteness     `json:"completeness" yaml:"completeness"`
	Degraded              bool                       `json:"degraded" yaml:"degraded"`
	DegradationReasons    []string                   `json:"degradation_reasons,omitempty" yaml:"degradation_reasons,omitempty"`
	ConsumerCompatibility ConsumerCompatibility      `json:"consumer_compatibility" yaml:"consumer_compatibility"`
}

// ParseGenerationManifestJSON strictly parses and validates a JSON manifest.
// Unknown members, missing required members (including false/zero-valued
// members), malformed JSON, and trailing documents are rejected with typed
// validation errors.
func ParseGenerationManifestJSON(data []byte) (GenerationManifest, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return GenerationManifest{}, validationError("manifest", string(data), fmt.Sprintf("invalid JSON: %v", err))
	}
	if err := requireJSONFields(raw, "", requiredManifestJSONFields); err != nil {
		return GenerationManifest{}, err
	}
	if _, err := requireJSONObject(raw["payload"], "payload", requiredPayloadJSONFields); err != nil {
		return GenerationManifest{}, err
	}
	validation, err := requireJSONObject(raw["validation"], "validation", requiredValidationJSONFields)
	if err != nil {
		return GenerationManifest{}, err
	}
	if err := requireJSONArrayObjects(validation["checks"], "validation.checks", requiredValidationCheckJSONFields); err != nil {
		return GenerationManifest{}, err
	}
	if err := requireJSONArrayObjects(raw["source_observations"], "source_observations", requiredObservationJSONFields); err != nil {
		return GenerationManifest{}, err
	}
	var observationObjects []map[string]json.RawMessage
	if err := json.Unmarshal(raw["source_observations"], &observationObjects); err != nil {
		return GenerationManifest{}, validationError("source_observations", nil, fmt.Sprintf("must be an array of objects: %v", err))
	}
	for index, observation := range observationObjects {
		if _, err := requireJSONObject(observation["revision"], fmt.Sprintf("source_observations[%d].revision", index), requiredObservationRevisionFields); err != nil {
			return GenerationManifest{}, err
		}
	}
	if _, err := requireJSONObject(raw["consumer_compatibility"], "consumer_compatibility", requiredCompatibilityJSONFields); err != nil {
		return GenerationManifest{}, err
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest GenerationManifest
	if err := decoder.Decode(&manifest); err != nil {
		return GenerationManifest{}, validationError("manifest", nil, fmt.Sprintf("invalid manifest: %v", err))
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON documents")
		}
		return GenerationManifest{}, validationError("manifest", nil, fmt.Sprintf("invalid trailing data: %v", err))
	}
	if err := manifest.Validate(); err != nil {
		return GenerationManifest{}, err
	}
	return manifest, nil
}

// Copy returns a value whose slices do not alias the original manifest.
func (m GenerationManifest) Copy() GenerationManifest {
	copyManifest := m
	copyManifest.SourceObservations = append([]SourceObservationLink(nil), m.SourceObservations...)
	copyManifest.DegradationReasons = append([]string(nil), m.DegradationReasons...)
	copyManifest.Validation.Checks = append([]GenerationValidationCheck(nil), m.Validation.Checks...)
	return copyManifest
}

// Validate verifies that a manifest is complete and eligible for publication.
func (m GenerationManifest) Validate() error {
	if m.ManifestVersion != CurrentGenerationManifestVersion {
		return validationError("manifest_version", m.ManifestVersion, fmt.Sprintf("must be %d", CurrentGenerationManifestVersion))
	}
	if m.SchemaVersion == 0 {
		return validationError("schema_version", m.SchemaVersion, "must be greater than zero")
	}
	if strings.TrimSpace(m.GenerationID) == "" {
		return validationError("generation_id", m.GenerationID, "is required")
	}
	if err := validateUTCTime("generated_at", m.GeneratedAt); err != nil {
		return err
	}
	if err := validateChecksum("payload.checksum", m.Payload.Checksum); err != nil {
		return err
	}
	if m.Payload.SizeBytes <= 0 {
		return validationError("payload.size_bytes", m.Payload.SizeBytes, "must be greater than zero")
	}
	if strings.TrimSpace(m.Payload.MediaType) == "" {
		return validationError("payload.media_type", m.Payload.MediaType, "is required")
	}
	if err := m.Validation.validate(); err != nil {
		return err
	}
	if strings.TrimSpace(m.SyncRunID) == "" {
		return validationError("sync_run_id", m.SyncRunID, "is required")
	}
	if err := validateObservationLinks(m.SourceObservations); err != nil {
		return err
	}
	for _, observation := range m.SourceObservations {
		if observation.Completeness == types.ObservationCompletenessPartial && m.Completeness != GenerationCompletenessPartial {
			return validationError("completeness", m.Completeness, "must be partial when a linked observation is partial")
		}
		if observation.Status == types.ObservationStatusDegraded && !m.Degraded {
			return validationError("degraded", m.Degraded, "must be true when a linked observation is degraded")
		}
	}
	if m.Completeness != GenerationCompletenessComplete && m.Completeness != GenerationCompletenessPartial {
		return validationError("completeness", m.Completeness, "must be complete or partial")
	}
	if m.Completeness == GenerationCompletenessPartial && !m.Degraded {
		return validationError("degraded", m.Degraded, "partial generations must be degraded")
	}
	if m.Degraded && len(m.DegradationReasons) == 0 {
		return validationError("degradation_reasons", m.DegradationReasons, "at least one reason is required for a degraded generation")
	}
	if !m.Degraded && len(m.DegradationReasons) > 0 {
		return validationError("degradation_reasons", m.DegradationReasons, "reasons require degraded=true")
	}
	if m.ConsumerCompatibility.MinSchemaVersion == 0 {
		return validationError("consumer_compatibility.min_schema_version", m.ConsumerCompatibility.MinSchemaVersion, "must be greater than zero")
	}
	if m.ConsumerCompatibility.MaxSchemaVersion == 0 {
		return validationError("consumer_compatibility.max_schema_version", m.ConsumerCompatibility.MaxSchemaVersion, "must be greater than zero")
	}
	if m.ConsumerCompatibility.MaxSchemaVersion < m.ConsumerCompatibility.MinSchemaVersion {
		return validationError("consumer_compatibility.max_schema_version", m.ConsumerCompatibility.MaxSchemaVersion, "must not be less than min_schema_version")
	}
	if !m.ConsumerCompatibility.SupportsSchema(m.SchemaVersion) {
		return validationError("schema_version", m.SchemaVersion, "is outside the declared consumer compatibility range")
	}
	return nil
}

func (r GenerationValidationReport) validate() error {
	if strings.TrimSpace(r.ValidatorVersion) == "" {
		return validationError("validation.validator_version", r.ValidatorVersion, "is required")
	}
	if err := validateUTCTime("validation.validated_at", r.ValidatedAt); err != nil {
		return err
	}
	if r.Status != GenerationValidationPassed {
		return validationError("validation.status", r.Status, "must be passed before publication")
	}
	if r.ErrorCount < 0 {
		return validationError("validation.error_count", r.ErrorCount, "must not be negative")
	}
	if r.WarningCount < 0 {
		return validationError("validation.warning_count", r.WarningCount, "must not be negative")
	}
	if len(r.Checks) == 0 {
		return validationError("validation.checks", r.Checks, "at least one check is required")
	}

	var errorsCount, warningsCount int
	seen := make(map[string]struct{}, len(r.Checks))
	for index, check := range r.Checks {
		prefix := fmt.Sprintf("validation.checks[%d]", index)
		if strings.TrimSpace(check.Name) == "" {
			return validationError(prefix+".name", check.Name, "is required")
		}
		if _, found := seen[check.Name]; found {
			return validationError(prefix+".name", check.Name, "must be unique")
		}
		seen[check.Name] = struct{}{}
		switch check.Status {
		case GenerationValidationCheckPassed:
		case GenerationValidationCheckWarning:
			warningsCount++
		case GenerationValidationCheckFailed:
			errorsCount++
		default:
			return validationError(prefix+".status", check.Status, "must be passed, warning, or failed")
		}
	}
	if errorsCount != r.ErrorCount {
		return validationError("validation.error_count", r.ErrorCount, fmt.Sprintf("must equal failed check count %d", errorsCount))
	}
	if warningsCount != r.WarningCount {
		return validationError("validation.warning_count", r.WarningCount, fmt.Sprintf("must equal warning check count %d", warningsCount))
	}
	if errorsCount != 0 {
		return validationError("validation.status", r.Status, "passed report cannot contain failed checks")
	}
	return nil
}

func validateObservationLinks(observations []SourceObservationLink) error {
	if len(observations) == 0 {
		return validationError("source_observations", observations, "at least one observation is required")
	}
	seen := make(map[string]struct{}, len(observations))
	for index, observation := range observations {
		prefix := fmt.Sprintf("source_observations[%d]", index)
		if strings.TrimSpace(observation.Source.String()) == "" {
			return validationError(prefix+".source", observation.Source, "is required")
		}
		if strings.TrimSpace(observation.ObservationID) == "" {
			return validationError(prefix+".observation_id", observation.ObservationID, "is required")
		}
		if err := validateUTCTime(prefix+".observed_at", observation.ObservedAt); err != nil {
			return err
		}
		switch observation.Revision.Kind {
		case types.ObservationRevisionKindUnknown:
			if strings.TrimSpace(observation.Revision.Value) != "" {
				return validationError(prefix+".revision.value", observation.Revision.Value, "must be empty when revision kind is unknown")
			}
		case types.ObservationRevisionKindGitCommit:
			if strings.TrimSpace(observation.Revision.Value) == "" {
				return validationError(prefix+".revision.value", observation.Revision.Value, "is required")
			}
			if (len(observation.Revision.Value) != 40 && len(observation.Revision.Value) != 64) || !isHexChecksumValue(observation.Revision.Value) {
				return validationError(prefix+".revision.value", observation.Revision.Value, "must be an exact hexadecimal Git commit")
			}
			if observation.Revision.InputName == "" || observation.Revision.InputChecksum == "" {
				return validationError(prefix+".revision.input", observation.Revision, "Git revisions require a lockfile name and checksum")
			}
		case types.ObservationRevisionKindETag,
			types.ObservationRevisionKindLastModified,
			types.ObservationRevisionKindSourceVersion,
			types.ObservationRevisionKindContentDigest:
			if strings.TrimSpace(observation.Revision.Value) == "" {
				return validationError(prefix+".revision.value", observation.Revision.Value, "is required")
			}
		default:
			return validationError(prefix+".revision.kind", observation.Revision.Kind, "is not supported")
		}
		if (observation.Revision.InputName == "") != (observation.Revision.InputChecksum == "") {
			return validationError(prefix+".revision.input", observation.Revision, "name and checksum must be supplied together")
		}
		if observation.Revision.InputName != "" {
			if observation.Revision.Kind != types.ObservationRevisionKindGitCommit {
				return validationError(prefix+".revision.input", observation.Revision, "content-addressed build input is only supported for Git revisions")
			}
			if err := validateChecksum(prefix+".revision.input_checksum", observation.Revision.InputChecksum); err != nil {
				return err
			}
		}
		if observation.Completeness != types.ObservationCompletenessComplete && observation.Completeness != types.ObservationCompletenessPartial {
			return validationError(prefix+".completeness", observation.Completeness, "must be complete or partial")
		}
		if observation.Status != types.ObservationStatusSucceeded && observation.Status != types.ObservationStatusDegraded {
			return validationError(prefix+".status", observation.Status, "must be succeeded or degraded")
		}
		if observation.Completeness == types.ObservationCompletenessPartial && observation.Status != types.ObservationStatusDegraded {
			return validationError(prefix+".status", observation.Status, "partial observations must be degraded")
		}
		if err := validateChecksum(prefix+".evidence_checksum", observation.EvidenceChecksum); err != nil {
			return err
		}
		key := observation.Source.String() + "\x00" + observation.ObservationID
		if _, found := seen[key]; found {
			return validationError(prefix+".observation_id", observation.ObservationID, "source and observation ID must be unique")
		}
		seen[key] = struct{}{}
	}
	return nil
}

func isHexChecksumValue(value string) bool {
	_, err := hex.DecodeString(value)
	return err == nil
}

func validateChecksum(field, checksum string) error {
	digest, found := strings.CutPrefix(checksum, checksumAlgorithmPrefix)
	if !found || len(digest) != sha256.Size*2 {
		return validationError(field, checksum, "must use sha256:<64 lowercase hexadecimal characters>")
	}
	decoded, err := hex.DecodeString(digest)
	if err != nil || hex.EncodeToString(decoded) != digest {
		return validationError(field, checksum, "must use sha256:<64 lowercase hexadecimal characters>")
	}
	return nil
}

func validateUTCTime(field string, value time.Time) error {
	if value.IsZero() {
		return validationError(field, value, "is required")
	}
	_, offset := value.Zone()
	if offset != 0 {
		return validationError(field, value, "must use UTC")
	}
	return nil
}

func validationError(field string, value any, message string) *errors.ValidationError {
	return &errors.ValidationError{Field: field, Value: value, Message: message}
}

func requireJSONObject(data json.RawMessage, path string, fields []string) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil || object == nil {
		return nil, validationError(path, string(data), "must be an object")
	}
	if err := requireJSONFields(object, path, fields); err != nil {
		return nil, err
	}
	return object, nil
}

func requireJSONArrayObjects(data json.RawMessage, path string, fields []string) error {
	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil || items == nil {
		return validationError(path, string(data), "must be an array")
	}
	for index, item := range items {
		if _, err := requireJSONObject(item, fmt.Sprintf("%s[%d]", path, index), fields); err != nil {
			return err
		}
	}
	return nil
}

func requireJSONFields(object map[string]json.RawMessage, path string, fields []string) error {
	for _, field := range fields {
		if _, found := object[field]; found {
			continue
		}
		fieldPath := field
		if path != "" {
			fieldPath = path + "." + field
		}
		return validationError(fieldPath, nil, "is required")
	}
	return nil
}
