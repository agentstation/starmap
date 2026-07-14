package catalogs

import "testing"

func TestCatalogRetainsIsolatedScopedDeploymentsForOneProviderModel(t *testing.T) {
	builder := NewEmpty()
	if err := builder.SetProvider(Provider{ID: "provider", Name: "Provider"}); err != nil {
		t.Fatal(err)
	}
	if err := builder.SetDefinition(ModelDefinition{ID: "author/model", Name: "Model", AuthorIDs: []AuthorID{"author"}}); err != nil {
		t.Fatal(err)
	}
	if err := builder.SetAuthor(Author{ID: "author", Name: "Author"}); err != nil {
		t.Fatal(err)
	}
	for _, deploymentID := range []string{"team-a", "team-b"} {
		offering := ProviderOffering{
			ProviderID: "provider", ProviderModelID: "model", DeploymentID: deploymentID,
			DefinitionID: "author/model", Aliases: []string{deploymentID + "-alias"},
			Availability: OfferingAvailabilityRestricted,
			Access:       OfferingAccess{Channel: OfferingAccessChannelServerToServer, Routability: OfferingRoutabilityDiscoverable, APIs: []InvocationAPI{}},
			Deployment:   ProviderDeployment{Type: "dedicated"}, Lifecycle: OfferingLifecycleActive,
		}
		if err := builder.SetOffering(offering); err != nil {
			t.Fatalf("SetOffering(%s): %v", deploymentID, err)
		}
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(catalog.Offerings()); got != 2 {
		t.Fatalf("offering count = %d, want 2", got)
	}
	first, err := catalog.OfferingByKey(OfferingKey{ProviderID: "provider", ProviderModelID: "model", DeploymentID: "team-a"})
	if err != nil {
		t.Fatal(err)
	}
	first.Aliases[0] = "mutated"
	again, err := catalog.OfferingByKey(first.Key())
	if err != nil {
		t.Fatal(err)
	}
	if again.Aliases[0] != "team-a-alias" {
		t.Fatalf("caller mutation leaked into catalog: %#v", again.Aliases)
	}
}
