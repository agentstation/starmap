package evidence

import (
	"reflect"
	"testing"
)

func TestSourceIDsAreStableAndIndependent(t *testing.T) {
	t.Parallel()

	want := []SourceID{
		ProvidersID,
		ModelsDevGitID,
		ModelsDevHTTPID,
		LocalCatalogID,
		ReleaseArtifactID,
		EmbeddedCatalogID,
	}
	got := SourceIDs()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SourceIDs() = %#v, want %#v", got, want)
	}
	got[0] = "changed"
	if next := SourceIDs(); !reflect.DeepEqual(next, want) {
		t.Fatalf("SourceIDs() retained caller mutation: %#v", next)
	}
	for _, id := range want {
		if !id.IsValid() || id.String() != string(id) {
			t.Fatalf("source identity %q is not valid and stable", id)
		}
	}
	if SourceID("unknown").IsValid() {
		t.Fatal("unknown source identity is valid")
	}
}

func TestResourceTypeValuesAreStable(t *testing.T) {
	t.Parallel()

	tests := map[ResourceType]string{
		ResourceTypeModel:            "model",
		ResourceTypeProvider:         "provider",
		ResourceTypeAuthor:           "author",
		ResourceTypeModelDefinition:  "model_definition",
		ResourceTypeProviderOffering: "provider_offering",
	}
	for resourceType, want := range tests {
		if got := resourceType.String(); got != want {
			t.Errorf("ResourceType.String() = %q, want %q", got, want)
		}
	}
}

func TestCompareReviewCandidatesUsesStableIdentityOrder(t *testing.T) {
	t.Parallel()

	base := ReviewCandidate{
		ProviderID:          "provider-a",
		ProviderModelID:     "model-a",
		Code:                ReviewCandidateUnresolvedModelReference,
		SourceID:            ProvidersID,
		SourceObservationID: "observation-a",
	}
	later := base
	later.ProviderModelID = "model-b"
	if got := CompareReviewCandidates(base, later); got >= 0 {
		t.Fatalf("CompareReviewCandidates(base, later) = %d, want negative", got)
	}
	if got := CompareReviewCandidates(later, base); got <= 0 {
		t.Fatalf("CompareReviewCandidates(later, base) = %d, want positive", got)
	}
	if got := CompareReviewCandidates(base, base); got != 0 {
		t.Fatalf("CompareReviewCandidates(base, base) = %d, want zero", got)
	}
}

func TestEvidenceWireTagsRemainStable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		typeOf reflect.Type
		field  string
		json   string
		yaml   string
	}{
		{reflect.TypeFor[ObservationRevision](), "InputChecksum", "input_checksum,omitempty", "input_checksum,omitempty"},
		{reflect.TypeFor[ObservationIssue](), "Subject", "subject,omitempty", "subject,omitempty"},
		{reflect.TypeFor[ReviewCandidate](), "ProviderModelID", "provider_model_id", "provider_model_id"},
		{reflect.TypeFor[ReviewCandidate](), "PriorReviewedModelLink", "prior_reviewed_model_link", "prior_reviewed_model_link"},
	}
	for _, test := range tests {
		field, found := test.typeOf.FieldByName(test.field)
		if !found {
			t.Fatalf("%s.%s is absent", test.typeOf.Name(), test.field)
		}
		if got := field.Tag.Get("json"); got != test.json {
			t.Errorf("%s.%s json tag = %q, want %q", test.typeOf.Name(), test.field, got, test.json)
		}
		if got := field.Tag.Get("yaml"); got != test.yaml {
			t.Errorf("%s.%s yaml tag = %q, want %q", test.typeOf.Name(), test.field, got, test.yaml)
		}
	}
}
