package artifact

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/errors"
)

// Validate checks receipt identity, admission, times, and the declared artifact binding.
func (r PublicationReceipt) Validate() error {
	if r.SchemaVersion != PublicationReceiptSchemaVersion {
		return publicationReceiptError("schema_version", "is not supported")
	}
	if !publicationIdentifier(r.RunID, maxChannelIDBytes) || !publicationIdentifier(r.PolicyVersion, 256) {
		return publicationReceiptError("identity", "requires bounded run and policy identifiers")
	}
	if !publicationTime(r.StartedAt) || !publicationTime(r.CompletedAt) || r.CompletedAt.Before(r.StartedAt) {
		return publicationReceiptError("time", "requires ordered, nonzero UTC times")
	}
	if !publicationIdentifier(r.Artifact.GenerationID, maxChannelIDBytes) ||
		!publicationChecksum(r.Artifact.CatalogChecksum) || !publicationChecksum(r.Artifact.PayloadChecksum) || !publicationChecksum(r.Artifact.ArchiveChecksum) {
		return publicationReceiptError("artifact", "requires an exact generation identity and canonical SHA-256 checksums")
	}
	if len(r.Sources) == 0 || len(r.Sources) > maxPublicationScopes {
		return publicationReceiptError("sources", "requires between one and 4096 declared scopes")
	}
	type scopeKey struct{ source, binding string }
	seen := make(map[scopeKey]bool, len(r.Sources))
	fresh := false
	for _, source := range r.Sources {
		if err := source.validate(r.StartedAt, r.CompletedAt); err != nil {
			return err
		}
		key := scopeKey{source: string(source.Policy.Source)}
		if source.Policy.Binding != nil {
			key.binding = source.Policy.Binding.ID
		}
		if seen[key] {
			return publicationReceiptError("sources", "cannot repeat a source and binding identity")
		}
		seen[key] = true
		if source.EvidenceKind == "fresh" && source.Policy.Source != evidence.EmbeddedCatalogID && source.Policy.Source != evidence.ReleaseArtifactID {
			fresh = true
		}
	}
	if r.FreshAcquisition != fresh {
		return publicationReceiptError("fresh_acquisition", "must agree with the admitted source observations")
	}
	return r.validateReviews()
}

func (r PublicationReceipt) validateReviews() error {
	if len(r.Reviews) == 0 && len(r.ReviewObservations) == 0 {
		return nil
	}
	if len(r.Reviews) > maxPublicationScopes || len(r.ReviewObservations) > maxPublicationScopes {
		return publicationReceiptError("reviews", "exceeds the review evidence count limit")
	}
	referenced := make(map[string]bool, len(r.Reviews))
	for _, review := range r.Reviews {
		referenced[review.SourceObservationID] = true
	}
	seen := make(map[string]bool, len(r.ReviewObservations))
	for _, observation := range r.ReviewObservations {
		if err := observation.Validate(); err != nil {
			return err
		}
		if !referenced[observation.ObservationID] || seen[observation.ObservationID] || observation.ObservedAt.After(r.CompletedAt) {
			return publicationReceiptError("review_observations", "requires unique referenced evidence no later than the run completion")
		}
		seen[observation.ObservationID] = true
	}
	return catalogs.ValidateReviewCandidates(r.Reviews, r.ReviewObservations)
}

func (p PublicationScopePolicy) validate() error {
	if !p.Source.IsValid() || p.MaxRetainedAge < 0 || (p.Required && p.AllowMissing) {
		return publicationReceiptError("source.policy", "requires a supported source and consistent evidence requirements")
	}
	if p.DisabledAction != "preserve" && p.DisabledAction != "remove" {
		return publicationReceiptError("source.disabled_action", "requires an explicit preserve or remove policy")
	}
	if p.Source != evidence.ProvidersID {
		if p.Binding != nil {
			return publicationReceiptError("source.binding", "only provider sources can declare a binding")
		}
		return nil
	}
	if p.Binding == nil || !publicationIdentifier(p.Binding.ID, maxPublicationBindingID) ||
		!publicationIdentifier(p.Binding.Revision, maxPublicationBindingID) ||
		!publicationIdentifier(string(p.Binding.ProviderID), maxPublicationBindingID) || !publicationChecksum(p.Binding.Checksum) {
		return publicationReceiptError("source.binding", "provider scopes require a complete binding identity and checksum")
	}
	return nil
}

func (s PublicationSourceReceipt) validate(startedAt, completedAt time.Time) error {
	if err := s.Policy.validate(); err != nil {
		return err
	}
	switch s.Attempt {
	case "succeeded", "partial", "failed", "missing_credentials", "not_attempted", "disabled":
	default:
		return publicationReceiptError("source.attempt", "must name a supported source outcome")
	}
	if s.Policy.Enabled == (s.Attempt == "disabled") {
		return publicationReceiptError("source.attempt", "must agree with the configured enabled state")
	}
	if s.EvidenceKind == "none" {
		if s.Observation != nil || s.Quarantine != nil || s.Attempt == "succeeded" || (s.Policy.Enabled && (s.Policy.Required || !s.Policy.AllowMissing)) {
			return publicationReceiptError("source.observation", "cannot omit required or successful source evidence")
		}
		return nil
	}
	return s.validateObservation(startedAt, completedAt)
}

func (s PublicationSourceReceipt) validateObservation(startedAt, completedAt time.Time) error {
	if s.Attempt == "disabled" || s.Observation == nil {
		return publicationReceiptError("source.observation", "requires admitted evidence from an enabled scope")
	}
	observation := *s.Observation
	if err := observation.Validate(); err != nil {
		return publicationReceiptError("source.observation", "must contain a valid observation receipt")
	}
	complete := observation.Completeness == evidence.ObservationCompletenessComplete && observation.Status == evidence.ObservationStatusSucceeded
	quarantined := s.Policy.AllowRecordQuarantine && s.Quarantine != nil && s.Quarantine.Valid() &&
		observation.Completeness == evidence.ObservationCompletenessPartial && observation.Status == evidence.ObservationStatusDegraded
	if observation.Source != s.Policy.Source || (!complete && !quarantined) || (complete && s.Quarantine != nil) {
		return publicationReceiptError("source.observation", "requires complete evidence or explicitly permitted record quarantine for the declared source")
	}
	switch s.EvidenceKind {
	case "fresh":
		if (complete && s.Attempt != "succeeded") || (quarantined && s.Attempt != "partial") || observation.ObservedAt.Before(startedAt) || observation.ObservedAt.After(completedAt) {
			return publicationReceiptError("source.observed_at", "fresh evidence requires an eligible observation within the run interval")
		}
	case "retained":
		if s.Attempt == "succeeded" || observation.ObservedAt.After(startedAt) || s.Policy.MaxRetainedAge == 0 || completedAt.Sub(observation.ObservedAt) > s.Policy.MaxRetainedAge {
			return publicationReceiptError("source.observed_at", "retained evidence must precede the run and remain within its age limit")
		}
	default:
		return publicationReceiptError("source.evidence_kind", "must name fresh, retained, or absent evidence")
	}
	return nil
}

func publicationIdentifier(value string, limit int) bool {
	return value != "" && len(value) <= limit && utf8.ValidString(value) && strings.TrimSpace(value) == value && !strings.ContainsFunc(value, unicode.IsControl)
}

func publicationTime(value time.Time) bool {
	_, offset := value.Zone()
	return !value.IsZero() && offset == 0
}

func publicationChecksum(value string) bool {
	return strings.HasPrefix(value, ChecksumPrefix) && isDigestHex(strings.TrimPrefix(value, ChecksumPrefix))
}

func publicationReceiptError(field, message string) error {
	return &errors.ValidationError{Field: "publication_receipt." + field, Message: message}
}
