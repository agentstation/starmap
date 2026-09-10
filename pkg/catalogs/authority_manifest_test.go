package catalogs

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func authorityManifestFixture(t *testing.T) ([]byte, CatalogAuthorityHead) {
	t.Helper()
	manifest := loadGenerationManifestFixture(t)
	head := permissionEnvelopeFixture().Head
	head.GenerationID = manifest.GenerationID
	head.PayloadChecksum = manifest.Payload.Checksum
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	object["manifest_version"], err = json.Marshal(AuthorityGenerationManifestVersion)
	if err != nil {
		t.Fatal(err)
	}
	object["authority_head"], err = json.Marshal(head)
	if err != nil {
		t.Fatal(err)
	}
	data, err = json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	return data, head
}

func TestAuthorityManifestBindsPublicationAndPayload(t *testing.T) {
	data, head := authorityManifestFixture(t)
	manifest, err := ParseGenerationManifestJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(manifest.Copy())
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatal(err)
	}
	var retained CatalogAuthorityHead
	if err := json.Unmarshal(object["authority_head"], &retained); err != nil {
		t.Fatal(err)
	}
	if retained != head {
		t.Fatal("manifest copy lost the committed authority binding")
	}
}

func TestAuthorityManifestRejectsInvalidBindings(t *testing.T) {
	data, _ := authorityManifestFixture(t)
	for name, mutate := range map[string]func([]byte) []byte{
		"ordinary version with authority": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"manifest_version":3`), []byte(`"manifest_version":2`), 1)
		},
		"missing head": func(b []byte) []byte {
			var m map[string]json.RawMessage
			_ = json.Unmarshal(b, &m)
			delete(m, "authority_head")
			out, _ := json.Marshal(m)
			return out
		},
		"null head": func(b []byte) []byte {
			var m map[string]json.RawMessage
			_ = json.Unmarshal(b, &m)
			m["authority_head"] = json.RawMessage("null")
			out, _ := json.Marshal(m)
			return out
		},
		"generation mismatch": func(b []byte) []byte {
			return changeManifestHead(t, b, func(h *CatalogAuthorityHead) { h.GenerationID = "other-generation" })
		},
		"payload mismatch": func(b []byte) []byte {
			return changeManifestHead(t, b, func(h *CatalogAuthorityHead) { h.PayloadChecksum = "sha256:" + strings.Repeat("c", 64) })
		},
		"invalid checksum": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"required_permission_revision":"sha256:`+strings.Repeat("b", 64)+`"`), []byte(`"required_permission_revision":"invalid"`), 1)
		},
		"zero sequence": func(b []byte) []byte { return bytes.Replace(b, []byte(`"sequence":1`), []byte(`"sequence":0`), 1) },
		"case variant head": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"authority_head"`), []byte(`"AUTHORITY_HEAD"`), 1)
		},
		"duplicate head": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"authority_head":`), []byte(`"authority_head":{},"authority_head":`), 1)
		},
		"case variant nested member": func(b []byte) []byte { return bytes.Replace(b, []byte(`"policy_id"`), []byte(`"POLICY_ID"`), 1) },
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseGenerationManifestJSON(mutate(bytes.Clone(data))); err == nil {
				t.Fatal("accepted an invalid authority manifest")
			}
		})
	}
}

func TestOrdinaryManifestOmitsAuthorityBinding(t *testing.T) {
	manifest := loadGenerationManifestFixture(t)
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("authority_head")) {
		t.Fatal("ordinary manifest gained authority fields")
	}
	if _, err := ParseGenerationManifestJSON(encoded); err != nil {
		t.Fatal(err)
	}
}

func changeManifestHead(t *testing.T, data []byte, change func(*CatalogAuthorityHead)) []byte {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	var head CatalogAuthorityHead
	if err := json.Unmarshal(object["authority_head"], &head); err != nil {
		t.Fatal(err)
	}
	change(&head)
	var err error
	object["authority_head"], err = json.Marshal(head)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
