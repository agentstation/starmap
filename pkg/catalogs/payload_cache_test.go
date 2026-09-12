package catalogs

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
)

func TestImmutablePayloadEncodingAllocationBound(t *testing.T) {
	builder := NewEmpty()
	if err := builder.SetAuthor(Author{ID: "author", Name: "Original"}); err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	want, err := EncodeCatalogPayload(builder)
	if err != nil {
		t.Fatal(err)
	}
	var data []byte
	allocations := testing.AllocsPerRun(100, func() { data, err = EncodeCatalogPayload(catalog) })
	if err != nil || !bytes.Equal(data, want) {
		t.Fatalf("immutable payload differs from construction records: %v", err)
	}
	if allocations > 1 {
		t.Fatalf("warm immutable encoding allocated %.0f objects; want one owned byte slice", allocations)
	}
	data[0] = '!'
	again, err := EncodeCatalogPayload(catalog)
	if err != nil || !bytes.Equal(again, want) {
		t.Fatalf("caller changed retained payload bytes: %v", err)
	}
	if err := builder.SetAuthor(Author{ID: "author", Name: "Changed"}); err != nil {
		t.Fatal(err)
	}
	changed, err := EncodeCatalogPayload(builder)
	if err != nil || bytes.Equal(changed, want) {
		t.Fatalf("mutable builder reused obsolete payload: %v", err)
	}
	again, err = EncodeCatalogPayload(catalog)
	if err != nil || !bytes.Equal(again, want) {
		t.Fatalf("builder mutation changed immutable payload: %v", err)
	}
}

func TestImmutablePayloadCachePreservesDecodedSchema(t *testing.T) {
	builder := NewEmpty()
	if err := builder.SetAuthor(Author{ID: "author", Name: "Author"}); err != nil {
		t.Fatal(err)
	}
	current, err := EncodeCatalogPayload(builder)
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []uint64{legacyCatalogSchemaVersion, membershipCatalogSchemaVersion, CatalogRemovalSchemaVersion, CurrentCatalogSchemaVersion} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			before := []byte(fmt.Sprintf(`"schema_version":%d`, CurrentCatalogSchemaVersion))
			after := []byte(fmt.Sprintf(`"schema_version":%d`, version))
			want := bytes.Replace(current, before, after, 1)
			catalog, err := DecodeCatalogPayload(want)
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				encoded, err := EncodeCatalogPayload(catalog)
				if err != nil || !bytes.Equal(encoded, want) {
					t.Fatalf("decoded schema changed after encoding: %v", err)
				}
			}
		})
	}
}

func TestImmutablePayloadConcurrentEncodingOwnsBytes(t *testing.T) {
	for _, observation := range []bool{false, true} {
		name := "catalog"
		if observation {
			name = "observation"
		}
		t.Run(name, func(t *testing.T) {
			builder := NewEmpty()
			if err := builder.SetAuthor(Author{ID: "author", Name: "Original"}); err != nil {
				t.Fatal(err)
			}
			var catalog *Catalog
			var err error
			if observation {
				catalog, err = NewObservationCatalog(builder)
			} else {
				catalog, err = builder.Build()
			}
			if err != nil {
				t.Fatal(err)
			}
			want, err := EncodeCatalogPayload(builder)
			if err != nil {
				t.Fatal(err)
			}
			start := make(chan struct{})
			var workers sync.WaitGroup
			for range 32 {
				workers.Go(func() {
					<-start
					for range 3 {
						data, err := EncodeCatalogPayload(catalog)
						if err != nil || !bytes.Equal(data, want) {
							t.Errorf("concurrent payload changed: %v", err)
							return
						}
						data[0] = '!'
					}
				})
			}
			close(start)
			workers.Wait()
		})
	}
}
