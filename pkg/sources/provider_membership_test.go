package sources

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestProviderMembershipAuthorityWireContract(t *testing.T) {
	for _, test := range []struct {
		name      string
		version   int
		authority string
		public    bool
		valid     bool
	}{
		{"legacy public evidence", 1, "", true, true},
		{"legacy cannot grant authority", 1, "provider", true, false},
		{"evidence only", 2, "", true, true},
		{"public scoped membership", 2, "scope", true, true},
		{"account scoped membership", 2, "scope", false, true},
		{"public provider membership", 2, "provider", true, true},
		{"account cannot remove provider membership", 2, "provider", false, false},
		{"unknown authority", 2, "all", true, false},
		{"unknown version", 3, "provider", true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			wire := providerBindingWire()
			wire["schema_version"] = test.version
			if test.authority != "" {
				wire["membership_authority"] = test.authority
			}
			if test.public {
				delete(wire, "account_id")
				delete(wire, "project_id")
				wire["public"] = true
			}
			raw, err := json.Marshal(wire)
			if err != nil {
				t.Fatal(err)
			}
			var binding ProviderAcquisitionBinding
			err = json.Unmarshal(raw, &binding)
			if (err == nil) != test.valid {
				t.Fatalf("binding accepted=%t, want %t: %v", err == nil, test.valid, err)
			}
		})
	}
}

func TestProviderMembershipAuthorityBoundToReceipt(t *testing.T) {
	ids := make(map[string]bool)
	for _, authority := range []string{"", "scope", "provider"} {
		wire := providerBindingWire()
		wire["schema_version"] = 2
		wire["public"] = true
		delete(wire, "account_id")
		delete(wire, "project_id")
		if authority != "" {
			wire["membership_authority"] = authority
		}
		raw, err := json.Marshal(wire)
		if err != nil {
			t.Fatal(err)
		}
		var binding ProviderAcquisitionBinding
		if err := json.Unmarshal(raw, &binding); err != nil {
			t.Fatal(err)
		}
		observation := scopedProviderObservation(t, &binding)
		if !strings.HasPrefix(observation.ID, "observation:v4:") {
			t.Fatalf("membership receipt format: %s", observation.ID)
		}
		if ids[observation.ID] {
			t.Fatal("changed membership authority retained the same receipt identity")
		}
		ids[observation.ID] = true
		receipt, err := observation.Receipt()
		if err != nil {
			t.Fatal(err)
		}
		raw, err = json.Marshal(receipt)
		if err != nil {
			t.Fatal(err)
		}
		var restored ObservationReceipt
		if err := json.Unmarshal(raw, &restored); err != nil {
			t.Fatal(err)
		}
		if _, err := restored.Restore(observation.Catalog); err != nil {
			t.Fatal(err)
		}
		if restored.ProviderBinding.MembershipAuthority == ProviderMembershipProvider {
			restored.ProviderBinding.MembershipAuthority = ProviderMembershipScope
		} else {
			restored.ProviderBinding.MembershipAuthority = ProviderMembershipProvider
		}
		if _, err := restored.Restore(observation.Catalog); err == nil {
			t.Fatal("changed membership authority accepted the original receipt identity")
		}
	}
}

func TestLegacyProviderMembershipReceiptIdentity(t *testing.T) {
	binding := validProviderBinding()
	binding.SchemaVersion = 1
	current := scopedProviderObservation(t, &binding)
	raw, err := catalogs.EncodeCatalogPayload(current.Catalog)
	if err != nil {
		t.Fatal(err)
	}
	var legacy map[string]json.RawMessage
	if err := json.Unmarshal(raw, &legacy); err != nil {
		t.Fatal(err)
	}
	legacy["schema_version"] = json.RawMessage("6")
	raw, err = json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := catalogs.DecodeSourceObservationPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	observation, err := NewObservation(ProvidersID, catalog, ObservationMetadata{ProviderBinding: &binding, ObservedAt: current.ObservedAt, Revision: Revision{Kind: RevisionKindContentDigest}, Completeness: current.Completeness, Status: current.Status})
	if err != nil {
		t.Fatal(err)
	}
	if observation.ID != "observation:v3:49c1b5d73c175f8454156b2d2bbae6f3d4064d6df6f53049cae3348c63ac17f4" {
		t.Fatalf("legacy receipt identity changed: %s", observation.ID)
	}
	receipt, err := observation.Receipt()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := receipt.Restore(observation.Catalog); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyProviderMembershipCannotGainAuthority(t *testing.T) {
	binding := validProviderBinding()
	binding.SchemaVersion = 1
	binding.AccountID = ""
	binding.Public = true
	observation := scopedProviderObservation(t, &binding)
	receipt, err := observation.Receipt()
	if err != nil {
		t.Fatal(err)
	}
	receipt.ProviderBinding.SchemaVersion = 2
	receipt.ProviderBinding.MembershipAuthority = ProviderMembershipProvider
	if _, err := receipt.Restore(observation.Catalog); err == nil {
		t.Fatal("legacy receipt gained replacement authority without a new identity")
	}
}
