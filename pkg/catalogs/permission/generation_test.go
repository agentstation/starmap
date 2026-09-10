package permission

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func originInput(t *testing.T, includeModel bool) catalogs.Generation {
	t.Helper()
	builder := catalogs.NewEmpty()
	if includeModel {
		if err := builder.SetAuthor(catalogs.Author{ID: "author", Name: "Author"}); err != nil {
			t.Fatal(err)
		}
		if err := builder.SetAuthorModel("author", catalogs.Model{ID: "model", Name: "Model", Authors: []catalogs.Author{{ID: "author", Name: "Author"}}}); err != nil {
			t.Fatal(err)
		}
		if err := builder.SetProvider(catalogs.Provider{ID: "provider", Name: "Provider", Models: map[string]*catalogs.Model{
			"served-model": {ID: "served-model", Name: "Model", ModelRef: "author/model"},
		}}); err != nil {
			t.Fatal(err)
		}
	}
	input := ordinaryPublication(t, "source-generation")
	var err error
	input.Payload, err = catalogs.EncodeCatalogPayload(builder)
	if err != nil {
		t.Fatal(err)
	}
	input.Manifest.Payload = catalogs.DescribeCatalogPayload(input.Payload)
	return input
}

func TestPrepareGenerationBindsAuthority(t *testing.T) {
	input := originInput(t, true)
	before := input.Copy()
	config := GenerationConfig{AuthorityID: "enterprise", PolicyID: "production", Sequence: 1}
	prepared, err := PrepareGeneration(input, config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := catalogs.DecodeCatalogGeneration(prepared); err != nil {
		t.Fatal(err)
	}
	head := prepared.Manifest.AuthorityHead
	if head.AuthorityID != config.AuthorityID || head.PolicyID != config.PolicyID || head.Sequence != 1 || !head.SupportsPermissions() {
		t.Fatalf("unexpected authority head: %+v", head)
	}
	if head.GenerationID == input.Manifest.GenerationID || head.GenerationID != prepared.Manifest.GenerationID || head.PayloadChecksum != input.Manifest.Payload.Checksum {
		t.Fatal("authority identity did not bind the separate publication to its exact source payload")
	}
	if !reflect.DeepEqual(input, before) || !bytes.Equal(prepared.Payload, input.Payload) {
		t.Fatal("preparation changed the source generation or its catalog bytes")
	}
	publisher := newTestPublisher(t, storage.NewMemory())
	if err := publisher.Commit(t.Context(), prepared, ""); err != nil {
		t.Fatal(err)
	}
	retry, err := PrepareGeneration(input, config)
	if err != nil || !reflect.DeepEqual(prepared, retry) {
		t.Fatalf("preparation is not deterministic: %v", err)
	}
	prepared.Payload[0] = ' '
	prepared.Manifest.SourceObservations[0].ObservationID = "changed"
	prepared.Manifest.Validation.Checks[0].Name = "changed"
	if !reflect.DeepEqual(input, before) || !reflect.DeepEqual(retry, prepareOrigin(t, input, config)) {
		t.Fatal("prepared generation shares mutable state with the source or another result")
	}
}

func prepareOrigin(t *testing.T, input catalogs.Generation, config GenerationConfig) catalogs.Generation {
	t.Helper()
	result, err := PrepareGeneration(input, config)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestPrepareGenerationWithdrawalChangesRequiredRevision(t *testing.T) {
	config := GenerationConfig{AuthorityID: "enterprise", PolicyID: "production", Sequence: 1}
	first := prepareOrigin(t, originInput(t, true), config)
	config.Sequence++
	next := prepareOrigin(t, originInput(t, false), config)
	if first.Manifest.AuthorityHead.RequiredPermissionRevision == next.Manifest.AuthorityHead.RequiredPermissionRevision {
		t.Fatal("model withdrawal reused the prior required permission revision")
	}
	publisher := newTestPublisher(t, storage.NewMemory())
	if err := publisher.Commit(t.Context(), first, ""); err != nil {
		t.Fatal(err)
	}
	if err := publisher.Commit(t.Context(), next, first.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	head, err := publisher.CurrentAuthorityHead(t.Context())
	if err != nil || head != next.Manifest.AuthorityHead {
		t.Fatalf("current authority did not expose the withdrawal: %+v / %v", head, err)
	}
}
