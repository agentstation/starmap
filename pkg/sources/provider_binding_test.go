package sources

import (
	"encoding/json"
	stderrors "errors"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func providerBindingCatalog(t *testing.T) *catalogs.Catalog {
	t.Helper()
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{ID: "provider", Name: "Provider"}); err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func unboundProviderObservation(t *testing.T) Observation {
	t.Helper()
	observation, err := NewObservation(ProvidersID, providerBindingCatalog(t), ObservationMetadata{
		ObservedAt: time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC), Revision: Revision{Kind: RevisionKindContentDigest},
		Completeness: ObservationCompletenessComplete, Status: ObservationStatusSucceeded,
	})
	if err != nil {
		t.Fatal(err)
	}
	return observation
}

func providerBindingWire() map[string]any {
	return map[string]any{
		"schema_version": 1, "id": "developer-account", "revision": "1", "provider_id": "provider",
		"account_id": "account-a", "region": "global", "api_surface": "models.list",
		"credential_role": "catalog_acquisition", "credential_profile_id": "catalog-key",
	}
}

func TestObservationRejectsUnboundProviderBinding(t *testing.T) {
	observation := unboundProviderObservation(t)
	raw, err := json.Marshal(observation)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	wire["provider_binding"] = providerBindingWire()
	raw, err = json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	var restored Observation
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	restored.Catalog = observation.Catalog
	if err := restored.Validate(); err == nil {
		t.Fatal("accepted provider binding without binding its observation identity")
	}
}

func TestReceiptRejectsUnboundProviderBinding(t *testing.T) {
	observation := unboundProviderObservation(t)
	receipt, err := observation.Receipt()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	wire["provider_binding"] = providerBindingWire()
	raw, err = json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	var restored ObservationReceipt
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if _, err := restored.Restore(observation.Catalog); err == nil {
		t.Fatal("receipt discarded provider binding and accepted unbound evidence")
	}
}

func validProviderBinding() ProviderAcquisitionBinding {
	return ProviderAcquisitionBinding{SchemaVersion: ProviderAcquisitionBindingSchemaVersion, ID: "developer-account", Revision: "1", ProviderID: "provider", AccountID: "account-a", Region: "global", APISurface: "models.list", CredentialRole: ProviderBindingCatalogAcquisition, CredentialProfileID: "catalog-key"}
}

func scopedProviderObservation(t *testing.T, binding *ProviderAcquisitionBinding) Observation {
	t.Helper()
	unbound := unboundProviderObservation(t)
	observation, err := NewObservation(ProvidersID, unbound.Catalog, ObservationMetadata{ProviderBinding: binding, ObservedAt: unbound.ObservedAt, Revision: unbound.Revision, Completeness: unbound.Completeness, Status: unbound.Status})
	if err != nil {
		t.Fatal(err)
	}
	return observation
}

func TestProviderBindingRequiresDeclaredScope(t *testing.T) {
	valid := map[string]func(*ProviderAcquisitionBinding){
		"account":             func(*ProviderAcquisitionBinding) {},
		"project":             func(b *ProviderAcquisitionBinding) { b.AccountID = ""; b.ProjectID = "project-a" },
		"account and project": func(b *ProviderAcquisitionBinding) { b.ProjectID = "project-a" },
		"public":              func(b *ProviderAcquisitionBinding) { b.AccountID = ""; b.Public = true },
		"unicode":             func(b *ProviderAcquisitionBinding) { b.ID = "équipe" },
		"field limit":         func(b *ProviderAcquisitionBinding) { b.ID = strings.Repeat("a", MaxProviderBindingFieldBytes) },
	}
	for name, change := range valid {
		t.Run(name, func(t *testing.T) {
			binding := validProviderBinding()
			change(&binding)
			if err := binding.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
	invalid := map[string]func(*ProviderAcquisitionBinding){
		"missing schema":   func(b *ProviderAcquisitionBinding) { b.SchemaVersion = 0 },
		"unknown schema":   func(b *ProviderAcquisitionBinding) { b.SchemaVersion++ },
		"missing id":       func(b *ProviderAcquisitionBinding) { b.ID = "" },
		"missing revision": func(b *ProviderAcquisitionBinding) { b.Revision = "" },
		"missing provider": func(b *ProviderAcquisitionBinding) { b.ProviderID = "" },
		"missing region":   func(b *ProviderAcquisitionBinding) { b.Region = "" },
		"missing surface":  func(b *ProviderAcquisitionBinding) { b.APISurface = "" },
		"missing profile":  func(b *ProviderAcquisitionBinding) { b.CredentialProfileID = "" },
		"inference role":   func(b *ProviderAcquisitionBinding) { b.CredentialRole = "inference" },
		"unknown scope":    func(b *ProviderAcquisitionBinding) { b.AccountID = "" },
		"public account":   func(b *ProviderAcquisitionBinding) { b.Public = true },
		"public project":   func(b *ProviderAcquisitionBinding) { b.AccountID = ""; b.ProjectID = "project-a"; b.Public = true },
		"whitespace":       func(b *ProviderAcquisitionBinding) { b.Region = " global" },
		"control":          func(b *ProviderAcquisitionBinding) { b.AccountID = "private-sentinel\n" },
		"invalid UTF8":     func(b *ProviderAcquisitionBinding) { b.ProjectID = string([]byte{255}) },
		"oversized":        func(b *ProviderAcquisitionBinding) { b.ID = strings.Repeat("a", MaxProviderBindingFieldBytes+1) },
	}
	for name, change := range invalid {
		t.Run(name, func(t *testing.T) {
			binding := validProviderBinding()
			change(&binding)
			err := binding.Validate()
			var validation *pkgerrors.ValidationError
			if !stderrors.As(err, &validation) || !strings.HasPrefix(validation.Field, "provider_binding.") {
				t.Fatalf("want typed binding error, got %v", err)
			}
			if validation.Value != nil || strings.Contains(err.Error(), "private-sentinel") {
				t.Fatal("binding error exposed a field value")
			}
		})
	}
}

func TestProviderBindingReceiptPreservesIdentityAndOwnership(t *testing.T) {
	binding := validProviderBinding()
	expected := binding
	observation := scopedProviderObservation(t, &binding)
	if !strings.HasPrefix(observation.ID, "observation:v4:") {
		t.Fatalf("scoped identity=%s", observation.ID)
	}
	binding.AccountID = "caller-change"
	if *observation.ProviderBinding != expected {
		t.Fatal("constructor retained caller binding pointer")
	}
	receipt, err := observation.Receipt()
	if err != nil {
		t.Fatal(err)
	}
	observation.ProviderBinding.AccountID = "observation-change"
	if *receipt.ProviderBinding != expected {
		t.Fatal("receipt retained observation binding pointer")
	}
	raw, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ObservationReceipt
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	restored, err := decoded.Restore(observation.Catalog)
	if err != nil {
		t.Fatal(err)
	}
	if *restored.ProviderBinding != expected || restored.ID != receipt.Link.ObservationID {
		t.Fatal("receipt restoration lost binding or identity")
	}
	restored.ProviderBinding.AccountID = "restored-change"
	clone := receipt.Clone()
	clone.ProviderBinding.AccountID = "clone-change"
	if *receipt.ProviderBinding != expected || *decoded.ProviderBinding != expected {
		t.Fatal("restored or cloned binding shares mutable ownership")
	}
}

func TestProviderBindingReceiptRejectsChangedOrRemovedScope(t *testing.T) {
	changes := map[string]func(*ProviderAcquisitionBinding){
		"id":       func(b *ProviderAcquisitionBinding) { b.ID = "other" },
		"revision": func(b *ProviderAcquisitionBinding) { b.Revision = "2" },
		"account":  func(b *ProviderAcquisitionBinding) { b.AccountID = "account-b" },
		"project":  func(b *ProviderAcquisitionBinding) { b.ProjectID = "project-b" },
		"region":   func(b *ProviderAcquisitionBinding) { b.Region = "regional" },
		"surface":  func(b *ProviderAcquisitionBinding) { b.APISurface = "models.other" },
		"profile":  func(b *ProviderAcquisitionBinding) { b.CredentialProfileID = "other-profile" },
		"public":   func(b *ProviderAcquisitionBinding) { b.AccountID = ""; b.Public = true },
	}
	binding := validProviderBinding()
	observation := scopedProviderObservation(t, &binding)
	receipt, err := observation.Receipt()
	if err != nil {
		t.Fatal(err)
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			altered := receipt.Clone()
			change(altered.ProviderBinding)
			if err := altered.ProviderBinding.Validate(); err != nil {
				t.Fatal(err)
			}
			if _, err := altered.Restore(observation.Catalog); err == nil {
				t.Fatal("accepted changed binding with original identity")
			}
		})
	}
	receipt.ProviderBinding = nil
	if _, err := receipt.Restore(observation.Catalog); err == nil {
		t.Fatal("accepted scoped identity without binding")
	}
	legacy := observation
	legacy.ID = legacyObservationID(legacy)
	if err := legacy.Validate(); err == nil {
		t.Fatal("accepted legacy identity for scoped observation")
	}
	unbound := observation
	unbound.ProviderBinding = nil
	observation.ID = observationID(unbound)
	if err := observation.Validate(); err == nil {
		t.Fatal("accepted unscoped v2 identity with binding")
	}
}

func TestProviderBindingMatchesObservedProvider(t *testing.T) {
	binding := validProviderBinding()
	observation := scopedProviderObservation(t, &binding)
	for _, sourceID := range []ID{LocalCatalogID, ModelsDevGitID} {
		observation.SourceID = sourceID
		observation.ID = observationID(observation)
		if err := observation.Validate(); err == nil {
			t.Fatal("accepted provider binding for another source")
		}
	}
	observation.SourceID = ProvidersID
	observation.ProviderBinding.ProviderID = "other"
	observation.ID = observationID(observation)
	if err := observation.Validate(); err == nil {
		t.Fatal("accepted binding for another provider")
	}
	observation.ProviderBinding.ProviderID = "provider"
	builder := catalogs.NewEmpty()
	for _, id := range []catalogs.ProviderID{"provider", "other"} {
		if err := builder.SetProvider(catalogs.Provider{ID: id, Name: string(id)}); err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewObservation(ProvidersID, catalog, ObservationMetadata{ProviderBinding: &binding, ObservedAt: observation.ObservedAt, Revision: Revision{Kind: RevisionKindContentDigest}, Completeness: ObservationCompletenessComplete, Status: ObservationStatusSucceeded}); err == nil {
		t.Fatal("accepted multiple providers under one binding")
	}
}

func TestProviderBindingJSONRejectsUnknownFieldsAndSchemas(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(map[string]any)
	}{
		{"unknown field", func(w map[string]any) { w["organization_id"] = "unrecognized" }},
		{"unknown schema", func(w map[string]any) { w["schema_version"] = 3 }},
		{"invalid field type", func(w map[string]any) { w["public"] = "true" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			wire := providerBindingWire()
			test.change(wire)
			raw, err := json.Marshal(wire)
			if err != nil {
				t.Fatal(err)
			}
			binding := validProviderBinding()
			original := binding
			if err := json.Unmarshal(raw, &binding); err == nil {
				t.Fatal("accepted unsupported binding wire data")
			}
			if binding != original {
				t.Fatal("invalid binding changed receiver")
			}
		})
	}
	binding := validProviderBinding()
	raw, err := json.Marshal(binding)
	if err != nil {
		t.Fatal(err)
	}
	if err := binding.UnmarshalJSON(append(raw, []byte(" {}")...)); err == nil {
		t.Fatal("accepted trailing binding data")
	}
}
