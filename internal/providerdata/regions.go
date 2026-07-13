package providerdata

import (
	"fmt"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/internal/embedded"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// RegionCatalog is a provider-scoped, evidence-bound acquisition inventory.
type RegionCatalog struct {
	ProviderID catalogs.ProviderID `json:"provider_id" yaml:"provider_id"`
	Evidence   Evidence            `json:"evidence" yaml:"evidence"`
	Commercial []string            `json:"commercial" yaml:"commercial"`
	GovCloud   []string            `json:"govcloud,omitempty" yaml:"govcloud,omitempty"`
}

// LoadRegionCatalog reads and validates one provider's embedded region list.
func LoadRegionCatalog(providerID catalogs.ProviderID) (RegionCatalog, error) {
	path := fmt.Sprintf("catalog/providers/%s/regions.yaml", providerID)
	payload, err := embedded.FS.ReadFile(path)
	if err != nil {
		return RegionCatalog{}, errors.WrapIO("read", path, err)
	}
	var catalog RegionCatalog
	if err := yaml.Unmarshal(payload, &catalog); err != nil {
		return RegionCatalog{}, errors.WrapParse("yaml", "provider region catalog", err)
	}
	if catalog.ProviderID != providerID {
		return RegionCatalog{}, &errors.ValidationError{Field: "provider_id", Value: catalog.ProviderID, Message: "must match the requested provider"}
	}
	if err := validateEvidence(catalog.Evidence); err != nil {
		return RegionCatalog{}, err
	}
	seen := make(map[string]struct{}, len(catalog.Commercial)+len(catalog.GovCloud))
	for group, regions := range map[string][]string{"commercial": catalog.Commercial, "govcloud": catalog.GovCloud} {
		if group == "commercial" && len(regions) == 0 {
			return RegionCatalog{}, &errors.ValidationError{Field: group, Message: "must not be empty"}
		}
		for index, region := range regions {
			if strings.TrimSpace(region) == "" {
				return RegionCatalog{}, &errors.ValidationError{Field: fmt.Sprintf("%s[%d]", group, index), Message: "must not be empty"}
			}
			if _, found := seen[region]; found {
				return RegionCatalog{}, &errors.ValidationError{Field: group, Value: region, Message: "must be unique across region groups"}
			}
			seen[region] = struct{}{}
		}
	}
	slices.Sort(catalog.Commercial)
	slices.Sort(catalog.GovCloud)
	return catalog, nil
}
