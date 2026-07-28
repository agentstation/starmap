package provenance

import "testing"

func TestModelResourceIDRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		providerID string
		modelID    string
		want       string
	}{
		{name: "readable", providerID: "provider-a", modelID: "shared", want: "provider-a/shared"},
		{name: "opaque slashes", providerID: "provider/a", modelID: "org/model", want: "provider%2Fa/org%2Fmodel"},
		{name: "key delimiters", providerID: "provider:a", modelID: "model:1", want: "provider%3Aa/model%3A1"},
		{name: "percent", providerID: "provider", modelID: "model%2Fvariant", want: "provider/model%252Fvariant"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			resourceID := ModelResourceID(test.providerID, test.modelID)
			if resourceID != test.want {
				t.Fatalf("ModelResourceID(%q, %q) = %q, want %q", test.providerID, test.modelID, resourceID, test.want)
			}
			providerID, modelID, ok := ParseModelResourceID(resourceID)
			if !ok || providerID != test.providerID || modelID != test.modelID {
				t.Fatalf("ParseModelResourceID(%q) = (%q, %q, %t)", resourceID, providerID, modelID, ok)
			}
		})
	}
}

func TestParseModelResourceIDRejectsMalformedOrNonCanonicalIdentity(t *testing.T) {
	t.Parallel()

	for _, resourceID := range []string{
		"",
		"provider",
		"/model",
		"provider/",
		"provider/model/variant",
		"provider/%zz",
		"provider/model%2fvariant",
	} {
		t.Run(resourceID, func(t *testing.T) {
			t.Parallel()
			if _, _, ok := ParseModelResourceID(resourceID); ok {
				t.Fatalf("ParseModelResourceID(%q) succeeded", resourceID)
			}
		})
	}
}
