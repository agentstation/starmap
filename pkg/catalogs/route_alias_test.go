package catalogs

import (
	"reflect"
	"testing"
)

func TestRouteAliasMaterializesEligibleProjectedOfferings(t *testing.T) {
	builder := NewEmpty()
	providers := []Provider{
		{ID: "available", Name: "Available", Models: map[string]*Model{"shared": sourceProjectionTestModel("shared", 1, "standard")}},
		{ID: "unavailable", Name: "Unavailable", Models: map[string]*Model{"shared": sourceProjectionTestModel("shared", 2, "standard")}},
		{ID: "retired", Name: "Retired", Models: map[string]*Model{"shared": sourceProjectionTestModel("shared", 3, "standard")}},
	}
	for _, provider := range providers {
		if err := builder.SetProvider(provider); err != nil {
			t.Fatalf("SetProvider: %v", err)
		}
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Eligibility is evaluated from the current offering facts. The test seam
	// replaces only the immutable derived values, not ingestion configuration.
	catalog.offerings[OfferingKey{ProviderID: "unavailable", ProviderModelID: "shared"}] = ProviderOffering{
		ProviderID: "unavailable", ProviderModelID: "shared", DefinitionID: "shared",
		Availability: OfferingAvailabilityUnavailable, Lifecycle: OfferingLifecycleActive,
	}
	catalog.offerings[OfferingKey{ProviderID: "retired", ProviderModelID: "shared"}] = ProviderOffering{
		ProviderID: "retired", ProviderModelID: "shared", DefinitionID: "shared",
		Availability: OfferingAvailabilityAvailable, Lifecycle: OfferingLifecycleRetired,
	}

	alias := RouteAlias{
		ID: "starport/shared",
		Targets: []OfferingKey{
			{ProviderID: "available", ProviderModelID: "shared"},
			{ProviderID: "unavailable", ProviderModelID: "shared"},
			{ProviderID: "retired", ProviderModelID: "shared"},
			{ProviderID: "missing", ProviderModelID: "shared"},
		},
	}
	resolution, err := catalog.MaterializeRouteAlias(alias)
	if err != nil {
		t.Fatalf("MaterializeRouteAlias: %v", err)
	}
	if resolution.AliasID != alias.ID || len(resolution.Eligible) != 1 {
		t.Fatalf("resolution = %#v", resolution)
	}
	if resolution.Eligible[0].ProviderID != "available" {
		t.Fatalf("eligible = %#v", resolution.Eligible)
	}
	wantReasons := []RouteAliasRejectionReason{
		RouteAliasRejectedUnavailable,
		RouteAliasRejectedRetired,
		RouteAliasRejectedMissing,
	}
	if len(resolution.Rejected) != len(wantReasons) {
		t.Fatalf("rejections = %#v", resolution.Rejected)
	}
	for index, reason := range wantReasons {
		if resolution.Rejected[index].Reason != reason {
			t.Fatalf("rejection %d = %q, want %q", index, resolution.Rejected[index].Reason, reason)
		}
	}
}

func TestRouteAliasContainsIdentityNotRoutingPolicy(t *testing.T) {
	typeOfAlias := reflect.TypeFor[RouteAlias]()
	for _, forbidden := range []string{"Weights", "Fallback", "Tenant", "Policy", "Strategy"} {
		if _, found := typeOfAlias.FieldByName(forbidden); found {
			t.Fatalf("RouteAlias exposes routing policy field %s", forbidden)
		}
	}
	valid := RouteAlias{ID: "route", Targets: []OfferingKey{{ProviderID: "provider", ProviderModelID: "model"}}}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	for name, alias := range map[string]RouteAlias{
		"missing ID":      {Targets: valid.Targets},
		"missing targets": {ID: "route"},
		"duplicate target": {ID: "route", Targets: []OfferingKey{
			{ProviderID: "provider", ProviderModelID: "model"},
			{ProviderID: "provider", ProviderModelID: "model"},
		}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := alias.Validate(); err == nil {
				t.Fatal("Validate returned nil error")
			}
		})
	}
}

func TestRouteAliasRejectsApplicationOnlyOffering(t *testing.T) {
	catalog := &Catalog{offerings: map[OfferingKey]ProviderOffering{
		{ProviderID: "cursor", ProviderModelID: "composer-2.5"}: {
			ProviderID: "cursor", ProviderModelID: "composer-2.5", DefinitionID: "composer-2.5",
			Availability: OfferingAvailabilityAvailable, Lifecycle: OfferingLifecycleActive,
			Access:     OfferingAccess{Channel: OfferingAccessChannelApplication, Routability: OfferingRoutabilityDiscoverable},
			Deployment: ProviderDeployment{Type: "application"},
		},
	}}
	resolution, err := catalog.MaterializeRouteAlias(RouteAlias{ID: "composer", Targets: []OfferingKey{{ProviderID: "cursor", ProviderModelID: "composer-2.5"}}})
	if err != nil {
		t.Fatalf("MaterializeRouteAlias: %v", err)
	}
	if len(resolution.Eligible) != 0 || len(resolution.Rejected) != 1 || resolution.Rejected[0].Reason != RouteAliasRejectedNotRoutable {
		t.Fatalf("resolution = %#v", resolution)
	}
}
