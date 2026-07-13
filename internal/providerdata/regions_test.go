package providerdata

import (
	"slices"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestLoadBedrockRegionCatalog(t *testing.T) {
	catalog, err := LoadRegionCatalog(catalogs.ProviderID("amazon-bedrock"))
	if err != nil {
		t.Fatalf("LoadRegionCatalog: %v", err)
	}
	if len(catalog.Commercial) != 32 || len(catalog.GovCloud) != 2 {
		t.Fatalf("region counts = %d/%d", len(catalog.Commercial), len(catalog.GovCloud))
	}
	if !slices.Contains(catalog.Commercial, "us-east-1") || !slices.Contains(catalog.GovCloud, "us-gov-west-1") {
		t.Fatalf("region catalog = %#v", catalog)
	}
}
