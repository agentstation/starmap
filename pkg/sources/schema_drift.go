package sources

import "slices"

// SchemaRecord identifies one independently validated source record shape.
type SchemaRecord string

const (
	// SchemaRecordObservation is the source observation envelope.
	SchemaRecordObservation SchemaRecord = "observation"
	// SchemaRecordCatalog is a complete decoded source catalog.
	SchemaRecordCatalog SchemaRecord = "catalog"
	// SchemaRecordProvider is one provider record inside a source catalog.
	SchemaRecordProvider SchemaRecord = "provider"
	// SchemaRecordModel is one source model record before canonical promotion.
	SchemaRecordModel SchemaRecord = "model"
	// SchemaRecordModelDefinition is one canonical provider-independent definition.
	SchemaRecordModelDefinition SchemaRecord = "model_definition"
	// SchemaRecordProviderOffering is one canonical provider-scoped offering.
	SchemaRecordProviderOffering SchemaRecord = "provider_offering"
)

// SchemaFieldClass explains why a field boundary is strict or tolerant.
type SchemaFieldClass string

const (
	// SchemaFieldIdentity is required identity whose absence or type drift rejects the record.
	SchemaFieldIdentity SchemaFieldClass = "strict_identity"
	// SchemaFieldContainer is an object/array boundary whose type drift rejects its scope.
	SchemaFieldContainer SchemaFieldClass = "strict_container"
	// SchemaFieldValue is a known scalar value validated before canonical promotion.
	SchemaFieldValue SchemaFieldClass = "validated_value"
	// SchemaFieldExtension is an explicitly lossless source-extension boundary.
	SchemaFieldExtension SchemaFieldClass = "tolerant_extension"
)

// SchemaDriftDisposition defines how a mismatch or unknown member is handled.
type SchemaDriftDisposition string

const (
	// SchemaDriftRejectSource rejects a structurally unusable source observation.
	SchemaDriftRejectSource SchemaDriftDisposition = "reject_source"
	// SchemaDriftRejectRecord quarantines one malformed record and preserves valid siblings.
	SchemaDriftRejectRecord SchemaDriftDisposition = "reject_record"
	// SchemaDriftClassify preserves a fingerprint/evidence record for review without promotion.
	SchemaDriftClassify SchemaDriftDisposition = "classify"
	// SchemaDriftPreserve retains the exact value inside the source extension boundary.
	SchemaDriftPreserve SchemaDriftDisposition = "preserve"
	// SchemaDriftNotApplicable means the disposition does not apply at this path.
	SchemaDriftNotApplicable SchemaDriftDisposition = "n/a"
)

// SchemaDriftPolicy is the executable strict/tolerant contract for one path.
type SchemaDriftPolicy struct {
	Record       SchemaRecord
	Path         string
	Class        SchemaFieldClass
	Required     bool
	Mismatch     SchemaDriftDisposition
	UnknownField SchemaDriftDisposition
	Rationale    string
}

// SchemaDriftPolicies returns caller-owned policies for a source record shape.
func SchemaDriftPolicies(record SchemaRecord) []SchemaDriftPolicy {
	policies := schemaDriftPolicies(record)
	return slices.Clone(policies)
}

func schemaDriftPolicies(record SchemaRecord) []SchemaDriftPolicy {
	switch record {
	case SchemaRecordObservation:
		return []SchemaDriftPolicy{
			strictSourceContainer(record, "$", true, SchemaDriftClassify, "The observation envelope must be an object; additive members are evidence until reviewed."),
			strictSourceIdentity(record, "id", "Observation identity is required for replay and audit."),
			strictSourceIdentity(record, "source", "Source identity selects authority and evidence ownership."),
			strictSourceIdentity(record, "observed_at", "Observation time is required for freshness and effective-price selection."),
			strictSourceContainer(record, "revision", true, SchemaDriftClassify, "Revision kind/value must retain a typed container."),
			strictSourceIdentity(record, "completeness", "Completeness controls publication policy."),
			strictSourceIdentity(record, "status", "Status distinguishes usable degradation from success."),
			strictSourceIdentity(record, "evidence_checksum", "The checksum binds normalized observation evidence."),
			strictSourceContainer(record, "issues", false, SchemaDriftClassify, "Issues must remain a typed array when present."),
		}
	case SchemaRecordCatalog:
		return []SchemaDriftPolicy{
			strictSourceContainer(record, "$", true, SchemaDriftClassify, "A non-object catalog is unusable as a source observation."),
			strictSourceContainer(record, "providers", true, SchemaDriftClassify, "Provider collection type drift invalidates source structure."),
			strictSourceContainer(record, "authors", false, SchemaDriftClassify, "Author collection is optional but typed when present."),
		}
	case SchemaRecordProvider:
		return []SchemaDriftPolicy{
			strictRecordContainer(record, "$", true, SchemaDriftClassify, "One malformed provider must not discard valid sibling providers."),
			strictRecordIdentity(record, "id", "Provider identity is required and unique within the observation."),
			strictRecordContainer(record, "models", false, SchemaDriftClassify, "The model collection must retain its declared container type."),
			extensionPolicy(record, "extensions", "Unknown provider data is lossless only inside the typed extension boundary."),
		}
	case SchemaRecordModel:
		return []SchemaDriftPolicy{
			strictRecordContainer(record, "$", true, SchemaDriftClassify, "One malformed model must be quarantined without erasing valid siblings."),
			strictRecordIdentity(record, "id", "The exact provider model ID is required and cannot be synthesized."),
			strictRecordIdentity(record, "name", "A promoted source model requires a non-empty display name or an explicit adapter default."),
			strictRecordContainer(record, "pricing", false, SchemaDriftClassify, "Pricing type drift rejects only pricing promotion and is retained as drift evidence."),
			strictRecordContainer(record, "limits", false, SchemaDriftClassify, "Limits must be a typed object when present."),
			strictRecordContainer(record, "features", false, SchemaDriftClassify, "Feature presence carries known-false semantics and must remain typed."),
			strictRecordContainer(record, "metadata", false, SchemaDriftClassify, "Metadata must be a typed object when present."),
			extensionPolicy(record, "extensions", "Unknown model data is preserved losslessly only inside source extensions."),
		}
	case SchemaRecordModelDefinition:
		return []SchemaDriftPolicy{
			strictRecordContainer(record, "$", true, SchemaDriftClassify, "Canonical definitions require a typed object boundary."),
			strictRecordIdentity(record, "id", "Canonical definition identity is required."),
			strictRecordIdentity(record, "name", "Canonical definition name is required."),
			strictRecordContainer(record, "metadata", true, SchemaDriftClassify, "Definition metadata is a typed canonical container."),
			strictRecordContainer(record, "lineage", true, SchemaDriftClassify, "Definition lineage is a typed canonical container."),
			strictRecordContainer(record, "weights", true, SchemaDriftClassify, "Definition weight facts are a typed canonical container."),
			strictRecordContainer(record, "capabilities", true, SchemaDriftClassify, "Definition capabilities are a typed canonical container."),
		}
	case SchemaRecordProviderOffering:
		return []SchemaDriftPolicy{
			strictRecordContainer(record, "$", true, SchemaDriftClassify, "Canonical offerings require a typed object boundary."),
			strictRecordIdentity(record, "provider_id", "Offering identity requires its exact provider."),
			strictRecordIdentity(record, "provider_model_id", "Offering identity requires the provider's exact opaque model ID."),
			strictRecordIdentity(record, "definition_id", "Every offering must resolve to one canonical definition."),
			strictRecordContainer(record, "pricing", false, SchemaDriftClassify, "Invalid pricing is rejected at field scope so the offering remains usable."),
			strictRecordContainer(record, "limits", false, SchemaDriftClassify, "Offering limits are typed when present."),
			strictRecordContainer(record, "endpoint", true, SchemaDriftClassify, "Endpoint behavior is a typed offering fact."),
			strictRecordContainer(record, "access", true, SchemaDriftClassify, "Access channel and routability fail closed."),
			strictRecordContainer(record, "deployment", true, SchemaDriftClassify, "Deployment type and tier are provider facts."),
			strictRecordContainer(record, "modes", false, SchemaDriftClassify, "Named service modes require a typed map when present."),
			strictRecordContainer(record, "regions", false, SchemaDriftClassify, "Regions require a typed set-like array when present."),
			strictRecordContainer(record, "inference_profile", false, SchemaDriftClassify, "Cross-region profile identity and destinations remain typed."),
			strictRecordContainer(record, "aggregator_upstream", false, SchemaDriftClassify, "Aggregator offerings retain the underlying provider identity when known."),
		}
	default:
		return nil
	}
}

func strictSourceIdentity(record SchemaRecord, path, rationale string) SchemaDriftPolicy {
	return SchemaDriftPolicy{Record: record, Path: path, Class: SchemaFieldIdentity, Required: true, Mismatch: SchemaDriftRejectSource, UnknownField: SchemaDriftNotApplicable, Rationale: rationale}
}

func strictRecordIdentity(record SchemaRecord, path, rationale string) SchemaDriftPolicy {
	return SchemaDriftPolicy{Record: record, Path: path, Class: SchemaFieldIdentity, Required: true, Mismatch: SchemaDriftRejectRecord, UnknownField: SchemaDriftNotApplicable, Rationale: rationale}
}

func strictSourceContainer(record SchemaRecord, path string, required bool, unknown SchemaDriftDisposition, rationale string) SchemaDriftPolicy {
	return SchemaDriftPolicy{Record: record, Path: path, Class: SchemaFieldContainer, Required: required, Mismatch: SchemaDriftRejectSource, UnknownField: unknown, Rationale: rationale}
}

func strictRecordContainer(record SchemaRecord, path string, required bool, unknown SchemaDriftDisposition, rationale string) SchemaDriftPolicy {
	return SchemaDriftPolicy{Record: record, Path: path, Class: SchemaFieldContainer, Required: required, Mismatch: SchemaDriftRejectRecord, UnknownField: unknown, Rationale: rationale}
}

func extensionPolicy(record SchemaRecord, path, rationale string) SchemaDriftPolicy {
	return SchemaDriftPolicy{Record: record, Path: path, Class: SchemaFieldExtension, Required: false, Mismatch: SchemaDriftRejectRecord, UnknownField: SchemaDriftPreserve, Rationale: rationale}
}
