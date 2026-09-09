package catalogs

import (
	"fmt"
	"testing"
)

func TestSourceObservationProviderLimitIsSeparateFromCanonicalLimit(t *testing.T) {
	for _, count := range []int{101, 512, 513} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			builder := NewEmpty()
			for index := range count {
				if err := builder.SetProvider(Provider{ID: ProviderID(fmt.Sprintf("provider-%03d", index)), Name: "Provider"}); err != nil {
					t.Fatal(err)
				}
			}
			catalog, err := NewObservationCatalog(builder)
			if err != nil {
				t.Fatal(err)
			}
			payload, err := EncodeCatalogPayload(catalog)
			if err != nil {
				t.Fatal(err)
			}
			restored, err := DecodeSourceObservationPayload(payload)
			if count <= 512 {
				if err != nil {
					t.Fatalf("bounded source observation did not round-trip: %v", err)
				}
				if len(restored.Providers().List()) != count {
					t.Fatal("source providers were discarded")
				}
			} else if err == nil {
				t.Fatal("oversized source observation was accepted")
			}
			if _, err := DecodeCatalogPayload(payload); err == nil {
				t.Fatal("source limit relaxed canonical generation validation")
			}
		})
	}
}
