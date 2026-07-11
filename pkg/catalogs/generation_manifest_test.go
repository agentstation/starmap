package catalogs

import (
	"encoding/json"
	stderrors "errors"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/google/go-cmp/cmp"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestGenerationManifestFixtureRoundTrip(t *testing.T) {
	manifest := loadGenerationManifestFixture(t)
	if err := manifest.Validate(); err != nil {
		t.Fatalf("Validate fixture: %v", err)
	}
	payload, err := os.ReadFile("testdata/generation/catalog.json")
	if err != nil {
		t.Fatalf("Read payload fixture: %v", err)
	}
	if err := manifest.Payload.Verify(payload); err != nil {
		t.Fatalf("Verify fixture payload: %v", err)
	}

	jsonData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal JSON: %v", err)
	}
	fromJSON, err := ParseGenerationManifestJSON(jsonData)
	if err != nil {
		t.Fatalf("Parse JSON: %v", err)
	}
	if diff := cmp.Diff(manifest, fromJSON); diff != "" {
		t.Fatalf("JSON round trip mismatch (-want +got):\n%s", diff)
	}

	yamlData, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal YAML: %v", err)
	}
	var fromYAML GenerationManifest
	if err := yaml.Unmarshal(yamlData, &fromYAML); err != nil {
		t.Fatalf("Unmarshal YAML: %v", err)
	}
	if diff := cmp.Diff(manifest, fromYAML); diff != "" {
		t.Fatalf("YAML round trip mismatch (-want +got):\n%s", diff)
	}
}

func TestGenerationManifestRetainsPinnedGitLockfileInput(t *testing.T) {
	manifest := loadGenerationManifestFixture(t)
	manifest.SourceObservations[0].Revision = catalogmeta.ObservationRevision{
		Kind: catalogmeta.ObservationRevisionKindGitCommit, Value: strings.Repeat("a", 40),
		InputName: "bun.lock", InputChecksum: "sha256:" + strings.Repeat("b", 64),
	}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	restored, err := ParseGenerationManifestJSON(data)
	if err != nil {
		t.Fatalf("ParseGenerationManifestJSON: %v", err)
	}
	if restored.SourceObservations[0].Revision != manifest.SourceObservations[0].Revision {
		t.Fatalf("revision = %#v, want %#v", restored.SourceObservations[0].Revision, manifest.SourceObservations[0].Revision)
	}
}

func TestGenerationManifestParserRejectsMissingAndUnknownMembers(t *testing.T) {
	fixture, err := os.ReadFile("testdata/generation/manifest.json")
	if err != nil {
		t.Fatalf("Read fixture: %v", err)
	}

	tests := []struct {
		name   string
		field  string
		mutate func(map[string]any)
	}{
		{name: "missing false-valued degraded", field: "degraded", mutate: func(value map[string]any) { delete(value, "degraded") }},
		{name: "missing payload", field: "payload", mutate: func(value map[string]any) { delete(value, "payload") }},
		{name: "missing zero-valued error count", field: "validation.error_count", mutate: func(value map[string]any) {
			delete(value["validation"].(map[string]any), "error_count")
		}},
		{name: "missing zero-valued warning count", field: "validation.warning_count", mutate: func(value map[string]any) {
			delete(value["validation"].(map[string]any), "warning_count")
		}},
		{name: "missing check status", field: "validation.checks[0].status", mutate: func(value map[string]any) {
			checks := value["validation"].(map[string]any)["checks"].([]any)
			delete(checks[0].(map[string]any), "status")
		}},
		{name: "missing observation checksum", field: "source_observations[0].evidence_checksum", mutate: func(value map[string]any) {
			observations := value["source_observations"].([]any)
			delete(observations[0].(map[string]any), "evidence_checksum")
		}},
		{name: "missing observation time", field: "source_observations[0].observed_at", mutate: func(value map[string]any) {
			observations := value["source_observations"].([]any)
			delete(observations[0].(map[string]any), "observed_at")
		}},
		{name: "missing observation revision kind", field: "source_observations[0].revision.kind", mutate: func(value map[string]any) {
			observations := value["source_observations"].([]any)
			delete(observations[0].(map[string]any)["revision"].(map[string]any), "kind")
		}},
		{name: "missing compatibility maximum", field: "consumer_compatibility.max_schema_version", mutate: func(value map[string]any) {
			delete(value["consumer_compatibility"].(map[string]any), "max_schema_version")
		}},
		{name: "unknown member", field: "manifest", mutate: func(value map[string]any) { value["binary_version"] = "1.2.3" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var value map[string]any
			if err := json.Unmarshal(fixture, &value); err != nil {
				t.Fatalf("Unmarshal fixture: %v", err)
			}
			tt.mutate(value)
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatalf("Marshal mutation: %v", err)
			}
			_, err = ParseGenerationManifestJSON(data)
			if err == nil {
				t.Fatal("ParseGenerationManifestJSON returned nil error")
			}
			var validationErr *pkgerrors.ValidationError
			if !stderrors.As(err, &validationErr) {
				t.Fatalf("error type = %T, want *errors.ValidationError", err)
			}
			if validationErr.Field != tt.field {
				t.Fatalf("field = %q, want %q", validationErr.Field, tt.field)
			}
		})
	}

	if _, err := ParseGenerationManifestJSON(append(fixture, fixture...)); err == nil {
		t.Fatal("parser accepted trailing JSON document")
	}
}

func TestGenerationManifestRequiredFields(t *testing.T) {
	valid := loadGenerationManifestFixture(t)

	tests := []struct {
		name   string
		field  string
		mutate func(*GenerationManifest)
	}{
		{name: "manifest version", field: "manifest_version", mutate: func(m *GenerationManifest) { m.ManifestVersion = 0 }},
		{name: "schema version", field: "schema_version", mutate: func(m *GenerationManifest) { m.SchemaVersion = 0 }},
		{name: "generation ID", field: "generation_id", mutate: func(m *GenerationManifest) { m.GenerationID = "" }},
		{name: "generated time", field: "generated_at", mutate: func(m *GenerationManifest) { m.GeneratedAt = time.Time{} }},
		{name: "payload checksum", field: "payload.checksum", mutate: func(m *GenerationManifest) { m.Payload.Checksum = "" }},
		{name: "payload size", field: "payload.size_bytes", mutate: func(m *GenerationManifest) { m.Payload.SizeBytes = 0 }},
		{name: "payload media type", field: "payload.media_type", mutate: func(m *GenerationManifest) { m.Payload.MediaType = "" }},
		{name: "validator version", field: "validation.validator_version", mutate: func(m *GenerationManifest) { m.Validation.ValidatorVersion = "" }},
		{name: "validation time", field: "validation.validated_at", mutate: func(m *GenerationManifest) { m.Validation.ValidatedAt = time.Time{} }},
		{name: "validation status", field: "validation.status", mutate: func(m *GenerationManifest) { m.Validation.Status = "" }},
		{name: "sync run ID", field: "sync_run_id", mutate: func(m *GenerationManifest) { m.SyncRunID = "" }},
		{name: "source observations", field: "source_observations", mutate: func(m *GenerationManifest) { m.SourceObservations = nil }},
		{name: "observation source", field: "source_observations[0].source", mutate: func(m *GenerationManifest) { m.SourceObservations[0].Source = "" }},
		{name: "observation ID", field: "source_observations[0].observation_id", mutate: func(m *GenerationManifest) { m.SourceObservations[0].ObservationID = "" }},
		{name: "observation time", field: "source_observations[0].observed_at", mutate: func(m *GenerationManifest) { m.SourceObservations[0].ObservedAt = time.Time{} }},
		{name: "observation revision", field: "source_observations[0].revision.kind", mutate: func(m *GenerationManifest) { m.SourceObservations[0].Revision.Kind = "" }},
		{name: "observation completeness", field: "source_observations[0].completeness", mutate: func(m *GenerationManifest) { m.SourceObservations[0].Completeness = "" }},
		{name: "observation status", field: "source_observations[0].status", mutate: func(m *GenerationManifest) { m.SourceObservations[0].Status = "" }},
		{name: "observation checksum", field: "source_observations[0].evidence_checksum", mutate: func(m *GenerationManifest) { m.SourceObservations[0].EvidenceChecksum = "" }},
		{name: "completeness", field: "completeness", mutate: func(m *GenerationManifest) { m.Completeness = "" }},
		{name: "compatibility minimum", field: "consumer_compatibility.min_schema_version", mutate: func(m *GenerationManifest) { m.ConsumerCompatibility.MinSchemaVersion = 0 }},
		{name: "compatibility maximum", field: "consumer_compatibility.max_schema_version", mutate: func(m *GenerationManifest) { m.ConsumerCompatibility.MaxSchemaVersion = 0 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := valid.Copy()
			tt.mutate(&manifest)
			err := manifest.Validate()
			if err == nil {
				t.Fatal("Validate returned nil")
			}
			var validationErr *pkgerrors.ValidationError
			if !stderrors.As(err, &validationErr) {
				t.Fatalf("error type = %T, want *errors.ValidationError", err)
			}
			if validationErr.Field != tt.field {
				t.Fatalf("field = %q, want %q", validationErr.Field, tt.field)
			}
		})
	}
}

func TestGenerationManifestDegradedCompletenessRules(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*GenerationManifest)
		valid  bool
	}{
		{name: "complete healthy", valid: true},
		{name: "complete degraded", valid: true, mutate: func(m *GenerationManifest) {
			m.Degraded = true
			m.DegradationReasons = []string{"provider observation used a last-known-good fallback"}
		}},
		{name: "partial must be degraded", mutate: func(m *GenerationManifest) {
			m.Completeness = GenerationCompletenessPartial
		}},
		{name: "degraded needs reason", mutate: func(m *GenerationManifest) {
			m.Degraded = true
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := loadGenerationManifestFixture(t)
			if tt.mutate != nil {
				tt.mutate(&manifest)
			}
			err := manifest.Validate()
			if tt.valid && err != nil {
				t.Fatalf("Validate: %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatal("Validate returned nil")
			}
		})
	}
}

func TestGenerationManifestCopyOwnership(t *testing.T) {
	original := loadGenerationManifestFixture(t)
	copyManifest := original.Copy()
	copyManifest.SourceObservations[0].ObservationID = "changed"
	copyManifest.Validation.Checks[0].Name = "changed"
	copyManifest.DegradationReasons = append(copyManifest.DegradationReasons, "changed")

	if original.SourceObservations[0].ObservationID == "changed" {
		t.Fatal("source observations alias the original")
	}
	if original.Validation.Checks[0].Name == "changed" {
		t.Fatal("validation checks alias the original")
	}
	if len(original.DegradationReasons) != 0 {
		t.Fatal("degradation reasons alias the original")
	}
}

func TestGenerationManifestValidationReportConsistency(t *testing.T) {
	tests := []struct {
		name   string
		field  string
		mutate func(*GenerationManifest)
	}{
		{name: "warning count", field: "validation.warning_count", mutate: func(m *GenerationManifest) {
			m.Validation.WarningCount = 1
		}},
		{name: "failed check", field: "validation.error_count", mutate: func(m *GenerationManifest) {
			m.Validation.Checks[0].Status = GenerationValidationCheckFailed
		}},
		{name: "duplicate check", field: "validation.checks[1].name", mutate: func(m *GenerationManifest) {
			m.Validation.Checks[1].Name = m.Validation.Checks[0].Name
		}},
		{name: "duplicate observation", field: "source_observations[1].observation_id", mutate: func(m *GenerationManifest) {
			m.SourceObservations[1] = m.SourceObservations[0]
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := loadGenerationManifestFixture(t)
			tt.mutate(&manifest)
			err := manifest.Validate()
			var validationErr *pkgerrors.ValidationError
			if !stderrors.As(err, &validationErr) {
				t.Fatalf("error = %v, want *errors.ValidationError", err)
			}
			if validationErr.Field != tt.field {
				t.Fatalf("field = %q, want %q", validationErr.Field, tt.field)
			}
		})
	}
}

func TestGenerationManifestPayloadDescriptor(t *testing.T) {
	payload := []byte(`{"providers":[],"authors":[],"endpoints":[]}`)
	descriptor := DescribeCatalogPayload(payload)
	if err := descriptor.Verify(payload); err != nil {
		t.Fatalf("Verify original payload: %v", err)
	}
	if err := descriptor.Verify(append(payload, '\n')); err == nil {
		t.Fatal("Verify accepted a changed payload")
	}
}

func TestGenerationManifestConsumerCompatibilityUsesSchemaVersions(t *testing.T) {
	compatibility := ConsumerCompatibility{MinSchemaVersion: 2, MaxSchemaVersion: 4}
	for schema, want := range map[uint64]bool{1: false, 2: true, 3: true, 4: true, 5: false} {
		if got := compatibility.SupportsSchema(schema); got != want {
			t.Fatalf("SupportsSchema(%d) = %v, want %v", schema, got, want)
		}
	}

	typ := reflect.TypeFor[GenerationManifest]()
	for _, forbidden := range []string{"BinaryVersion", "MinBinaryVersion", "MaxBinaryVersion"} {
		if _, found := typ.FieldByName(forbidden); found {
			t.Fatalf("manifest compatibility is coupled to %s", forbidden)
		}
	}
}

func TestGenerationManifestJSONSchemaRequiredFields(t *testing.T) {
	type schemaNode struct {
		Required    []string                   `json:"required"`
		Properties  map[string]json.RawMessage `json:"properties"`
		Definitions map[string]json.RawMessage `json:"$defs"`
	}

	data, err := os.ReadFile("testdata/generation/manifest.schema.json")
	if err != nil {
		t.Fatalf("Read schema: %v", err)
	}
	var schema schemaNode
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("Unmarshal schema: %v", err)
	}
	want := []string{
		"manifest_version", "schema_version", "generation_id", "generated_at",
		"payload", "validation", "sync_run_id", "source_observations",
		"completeness", "degraded", "consumer_compatibility",
	}
	slices.Sort(schema.Required)
	slices.Sort(want)
	if !slices.Equal(schema.Required, want) {
		t.Fatalf("schema required = %v, want %v", schema.Required, want)
	}
	for _, property := range want {
		if _, found := schema.Properties[property]; !found {
			t.Errorf("schema has no property %q", property)
		}
	}

	for definition, required := range map[string][]string{
		"payload":                {"checksum", "size_bytes", "media_type"},
		"validation":             {"validator_version", "validated_at", "status", "error_count", "warning_count", "checks"},
		"source_observation":     {"source", "observation_id", "observed_at", "revision", "completeness", "status", "evidence_checksum"},
		"consumer_compatibility": {"min_schema_version", "max_schema_version"},
	} {
		data, found := schema.Definitions[definition]
		if !found {
			t.Errorf("schema has no definition %q", definition)
			continue
		}
		var node schemaNode
		if err := json.Unmarshal(data, &node); err != nil {
			t.Fatalf("Unmarshal %s definition: %v", definition, err)
		}
		slices.Sort(node.Required)
		slices.Sort(required)
		if !slices.Equal(node.Required, required) {
			t.Errorf("%s required = %v, want %v", definition, node.Required, required)
		}
	}
}

func loadGenerationManifestFixture(t *testing.T) GenerationManifest {
	t.Helper()
	data, err := os.ReadFile("testdata/generation/manifest.json")
	if err != nil {
		t.Fatalf("Read fixture: %v", err)
	}
	manifest, err := ParseGenerationManifestJSON(data)
	if err != nil {
		t.Fatalf("Parse fixture: %v", err)
	}
	return manifest
}
