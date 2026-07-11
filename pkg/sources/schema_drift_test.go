package sources

import "testing"

func TestSchemaDriftPolicyClassifiesStrictAndTolerantFields(t *testing.T) {
	records := []SchemaRecord{
		SchemaRecordObservation,
		SchemaRecordCatalog,
		SchemaRecordProvider,
		SchemaRecordModel,
		SchemaRecordModelDefinition,
		SchemaRecordProviderOffering,
	}
	for _, record := range records {
		t.Run(string(record), func(t *testing.T) {
			policies := SchemaDriftPolicies(record)
			if len(policies) == 0 {
				t.Fatal("policy inventory is empty")
			}
			seen := make(map[string]struct{}, len(policies))
			hasStrict := false
			hasUnknownDisposition := false
			for _, policy := range policies {
				if policy.Record != record || policy.Path == "" || policy.Class == "" || policy.Mismatch == "" || policy.UnknownField == "" || policy.Rationale == "" {
					t.Fatalf("incomplete policy: %#v", policy)
				}
				if _, exists := seen[policy.Path]; exists {
					t.Fatalf("duplicate path %q", policy.Path)
				}
				seen[policy.Path] = struct{}{}
				if policy.Class == SchemaFieldIdentity || policy.Class == SchemaFieldContainer {
					hasStrict = true
				}
				if policy.UnknownField == SchemaDriftClassify || policy.UnknownField == SchemaDriftPreserve {
					hasUnknownDisposition = true
				}
			}
			if !hasStrict || !hasUnknownDisposition {
				t.Fatalf("record lacks strict or tolerant policy: %#v", policies)
			}
		})
	}
}

func TestSchemaDriftIdentityAndExtensionInvariants(t *testing.T) {
	for _, record := range []SchemaRecord{SchemaRecordProvider, SchemaRecordModel, SchemaRecordModelDefinition, SchemaRecordProviderOffering} {
		for _, policy := range SchemaDriftPolicies(record) {
			switch policy.Class {
			case SchemaFieldIdentity:
				if !policy.Required || policy.Mismatch != SchemaDriftRejectRecord {
					t.Fatalf("identity policy is not strict: %#v", policy)
				}
			case SchemaFieldExtension:
				if policy.UnknownField != SchemaDriftPreserve {
					t.Fatalf("extension policy is not lossless: %#v", policy)
				}
			}
		}
	}
}

func TestSchemaDriftPoliciesReturnCallerOwnedSlices(t *testing.T) {
	policies := SchemaDriftPolicies(SchemaRecordModel)
	policies[0].Path = "mutated"
	if got := SchemaDriftPolicies(SchemaRecordModel)[0].Path; got == "mutated" {
		t.Fatal("mutating returned policies changed canonical inventory")
	}
}
