package catalogs

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs/internal/resourcepolicy"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/provenance"
	sourcepayload "github.com/agentstation/starmap/pkg/sources/payload"
)

func TestCatalogPayloadEncodingUsesDecodeByteLimit(t *testing.T) {
	for _, delta := range []int{-1, 0, 1} {
		t.Run(strconv.Itoa(delta), func(t *testing.T) {
			builder := NewEmpty()
			description := ""
			author := Author{ID: "author", Name: "Author", Description: &description}
			if err := builder.SetAuthor(author); err != nil {
				t.Fatal(err)
			}
			empty, err := EncodeCatalogPayload(builder)
			if err != nil {
				t.Fatal(err)
			}
			description = strings.Repeat("x", resourcepolicy.MaxPayloadBytes-len(empty)+delta)
			if err := builder.SetAuthor(author); err != nil {
				t.Fatal(err)
			}
			payload, err := EncodeCatalogPayload(builder)
			if delta > 0 {
				assertPayloadEncodingRefused(t, payload, err)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(payload) != resourcepolicy.MaxPayloadBytes+delta {
				t.Fatalf("payload bytes=%d, want %d", len(payload), resourcepolicy.MaxPayloadBytes+delta)
			}
			if _, err := DecodeCatalogPayload(payload); err != nil {
				t.Fatalf("encoded catalog cannot reopen: %v", err)
			}
		})
	}
}

func TestCatalogPayloadEncodingUsesDecodeNestingLimit(t *testing.T) {
	// The envelope, provenance map, entry array, and entry object add four levels.
	const envelopeDepth = 4
	for _, delta := range []int{-1, 0, 1} {
		t.Run(strconv.Itoa(delta), func(t *testing.T) {
			builder := NewEmpty()
			if err := builder.SetAuthor(Author{ID: "author", Name: "Author"}); err != nil {
				t.Fatal(err)
			}
			var value any = true
			for range sourcepayload.MaxJSONNestingDepth - envelopeDepth + delta {
				value = map[string]any{"child": value}
			}
			builder.SetProvenance(provenance.Map{"author:author:note": {{Field: "note", Value: value}}})
			payload, err := EncodeCatalogPayload(builder)
			if delta > 0 {
				assertPayloadEncodingRefused(t, payload, err)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeCatalogPayload(payload); err != nil {
				t.Fatalf("encoded catalog cannot reopen: %v", err)
			}
		})
	}
}

func assertPayloadEncodingRefused(t *testing.T, payload []byte, err error) {
	t.Helper()
	var validation *pkgerrors.ValidationError
	if !errors.As(err, &validation) || validation.Field != "payload" || payload != nil {
		t.Fatalf("payload bytes=%d error=%v, want typed refusal without output", len(payload), err)
	}
}
