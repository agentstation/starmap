package catalogs

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestAuthorityRecordStrictIndependentFormat(t *testing.T) {
	record := CatalogAuthorityRecord{Version: CatalogAuthorityRecordVersion, Head: permissionEnvelopeFixture().Head}
	record.Head.PermissionSchemaVersion++
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseCatalogAuthorityRecord(data)
	if err != nil || got != record {
		t.Fatalf("record=%+v error=%v", got, err)
	}
	if got.Head.SupportsPermissions() {
		t.Fatal("future permission semantics became compatible")
	}
	for name, invalid := range map[string][]byte{
		"empty":                     nil,
		"null":                      []byte("null"),
		"array":                     []byte("[]"),
		"utf8":                      append(bytes.Clone(data), 0xff),
		"oversized":                 []byte(strings.Repeat(" ", MaxCatalogAuthorityRecordBytes+1)),
		"unknown version":           bytes.Replace(data, []byte(`"version":1`), []byte(`"version":2`), 1),
		"duplicate version":         bytes.Replace(data, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1),
		"duplicate escaped version": bytes.Replace(data, []byte(`"version":1`), []byte(`"version":1,"\u0076ersion":1`), 1),
		"unknown field":             bytes.Replace(data, []byte(`"version":1`), []byte(`"version":1,"unexpected":true`), 1),
		"unknown head field":        bytes.Replace(data, []byte(`"authority_id":`), []byte(`"unexpected":true,"authority_id":`), 1),
		"duplicate head field":      bytes.Replace(data, []byte(`"authority_id":`), []byte(`"authority_id":"other","authority_id":`), 1),
		"trailing document":         append(bytes.Clone(data), data...),
		"missing head":              []byte(`{"version":1}`),
		"null head":                 []byte(`{"version":1,"head":null}`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseCatalogAuthorityRecord(invalid); err == nil {
				t.Fatal("accepted invalid authority record")
			}
		})
	}
}
