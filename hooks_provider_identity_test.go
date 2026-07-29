package starmap

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestReaderProviderModelsPreservesProviderScopedIdentity(t *testing.T) {
	builder := catalogs.NewEmpty()
	for _, provider := range []catalogs.Provider{
		{
			ID: "b-provider",
			Models: map[string]*catalogs.Model{
				"shared-model": {ID: "shared-model", Name: "Provider B"},
			},
		},
		{
			ID: "a-provider",
			Models: map[string]*catalogs.Model{
				"shared-model": {ID: "shared-model", Name: "Provider A"},
			},
		},
	} {
		if err := builder.SetProvider(provider); err != nil {
			t.Fatalf("SetProvider(%s): %v", provider.ID, err)
		}
	}

	records := readerProviderModels(mustTestCatalog(t, builder))
	if len(records) != 2 {
		t.Fatalf("readerProviderModels returned %d records, want 2", len(records))
	}
	if records[0].key.providerID != "a-provider" || records[0].model.Name != "Provider A" {
		t.Fatalf("first provider record = %#v, want deterministic a-provider record", records[0])
	}
	if records[1].key.providerID != "b-provider" || records[1].model.Name != "Provider B" {
		t.Fatalf("second provider record = %#v, want deterministic b-provider record", records[1])
	}
}

func TestTriggerUpdateDiffsSameModelIDPerProvider(t *testing.T) {
	oldBuilder := catalogs.NewEmpty()
	newBuilder := catalogs.NewEmpty()
	for _, definition := range []struct {
		providerID catalogs.ProviderID
		oldName    string
		newName    string
	}{
		{providerID: "a-provider", oldName: "Provider A", newName: "Provider A"},
		{providerID: "b-provider", oldName: "Provider B", newName: "Provider B updated"},
	} {
		oldModel := catalogs.Model{ID: "shared-model", Name: definition.oldName}
		if err := oldBuilder.SetProvider(catalogs.Provider{
			ID:     definition.providerID,
			Models: map[string]*catalogs.Model{oldModel.ID: &oldModel},
		}); err != nil {
			t.Fatalf("old SetProvider(%s): %v", definition.providerID, err)
		}
		newModel := catalogs.Model{ID: "shared-model", Name: definition.newName}
		if err := newBuilder.SetProvider(catalogs.Provider{
			ID:     definition.providerID,
			Models: map[string]*catalogs.Model{newModel.ID: &newModel},
		}); err != nil {
			t.Fatalf("new SetProvider(%s): %v", definition.providerID, err)
		}
	}

	var updates [][2]string
	hooks := newHooks()
	hooks.OnModelUpdated(func(old, updated catalogs.Model) {
		updates = append(updates, [2]string{old.Name, updated.Name})
	})
	hooks.triggerUpdate(
		mustTestCatalog(t, oldBuilder),
		mustTestCatalog(t, newBuilder),
	)

	if len(updates) != 1 {
		t.Fatalf("model update callbacks = %#v, want one provider-scoped change", updates)
	}
	if updates[0] != [2]string{"Provider B", "Provider B updated"} {
		t.Fatalf("model update = %#v, want b-provider change", updates[0])
	}
}
