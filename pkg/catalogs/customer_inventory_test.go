package catalogs

import (
	"reflect"
	"testing"
	"time"
)

func TestCustomerInventoryIsStructurallySeparateFromPublicCatalog(t *testing.T) {
	typeOfCatalog := reflect.TypeFor[Catalog]()
	for _, forbidden := range []string{"CustomerInventory", "CustomerDeployments", "Deployments", "Accounts", "Projects", "Workspaces"} {
		if _, found := typeOfCatalog.FieldByName(forbidden); found {
			t.Fatalf("Catalog exposes private field %s", forbidden)
		}
	}
	inventory := CustomerInventory{
		ProviderID: "azure", Scope: CustomerScope{SubscriptionID: "subscription"}, ObservedAt: time.Now().UTC(),
		Deployments: []CustomerDeployment{{
			ID: "deployment", DefinitionID: "model", ProviderModelID: "model@2026-01-01",
			Deployment: ProviderDeployment{Type: "provisioned"}, Endpoint: "https://private.example",
		}},
	}
	if err := inventory.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestCustomerInventoryValidation(t *testing.T) {
	valid := CustomerInventory{
		ProviderID: "aws", Scope: CustomerScope{AccountID: "account"}, ObservedAt: time.Now().UTC(),
		Deployments: []CustomerDeployment{{ID: "profile", DefinitionID: "model", ProviderModelID: "model", Deployment: ProviderDeployment{Type: "application_profile"}}},
	}
	tests := []struct {
		name string
		edit func(*CustomerInventory)
	}{
		{name: "provider", edit: func(i *CustomerInventory) { i.ProviderID = "" }},
		{name: "scope", edit: func(i *CustomerInventory) { i.Scope = CustomerScope{} }},
		{name: "time", edit: func(i *CustomerInventory) { i.ObservedAt = time.Time{} }},
		{name: "deployment ID", edit: func(i *CustomerInventory) { i.Deployments[0].ID = "" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			candidate.Deployments = append([]CustomerDeployment(nil), valid.Deployments...)
			test.edit(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatal("Validate returned nil")
			}
		})
	}
}
