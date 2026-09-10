package catalogs

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func permissionEnvelopeFixture() CatalogPermissionEnvelope {
	at := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	return CatalogPermissionEnvelope{
		Version: CatalogPermissionEnvelopeVersion,
		Head: CatalogAuthorityHead{
			AuthorityID: "enterprise", PolicyID: "production", Sequence: 1,
			GenerationID: "catalog-one", PayloadChecksum: "sha256:" + strings.Repeat("a", 64),
			RequiredPermissionRevision: "sha256:" + strings.Repeat("b", 64),
			PermissionSchemaVersion:    CatalogPermissionSchemaVersion,
		},
		IssuedAt: at, ValidUntil: at.Add(5 * time.Minute),
	}
}

func TestPermissionEnvelopeRejectsInvalidPublication(t *testing.T) {
	for name, change := range map[string]func(*CatalogPermissionEnvelope){
		"missing envelope version":     func(e *CatalogPermissionEnvelope) { e.Version = 0 },
		"future envelope version":      func(e *CatalogPermissionEnvelope) { e.Version++ },
		"missing authority":            func(e *CatalogPermissionEnvelope) { e.Head.AuthorityID = "" },
		"authority whitespace":         func(e *CatalogPermissionEnvelope) { e.Head.AuthorityID = " enterprise" },
		"policy control character":     func(e *CatalogPermissionEnvelope) { e.Head.PolicyID = "production\nprivate" },
		"oversized identity":           func(e *CatalogPermissionEnvelope) { e.Head.AuthorityID = strings.Repeat("x", 257) },
		"missing sequence":             func(e *CatalogPermissionEnvelope) { e.Head.Sequence = 0 },
		"missing generation":           func(e *CatalogPermissionEnvelope) { e.Head.GenerationID = "" },
		"invalid payload digest":       func(e *CatalogPermissionEnvelope) { e.Head.PayloadChecksum = "sha256:wrong" },
		"invalid permission digest":    func(e *CatalogPermissionEnvelope) { e.Head.RequiredPermissionRevision = "revision-one" },
		"missing permission semantics": func(e *CatalogPermissionEnvelope) { e.Head.PermissionSchemaVersion = 0 },
		"missing issue time":           func(e *CatalogPermissionEnvelope) { e.IssuedAt = time.Time{} },
		"missing expiry":               func(e *CatalogPermissionEnvelope) { e.ValidUntil = time.Time{} },
		"zero lifetime":                func(e *CatalogPermissionEnvelope) { e.ValidUntil = e.IssuedAt },
		"negative lifetime":            func(e *CatalogPermissionEnvelope) { e.ValidUntil = e.IssuedAt.Add(-time.Second) },
		"excessive lifetime":           func(e *CatalogPermissionEnvelope) { e.ValidUntil = e.ValidUntil.Add(time.Nanosecond) },
	} {
		t.Run(name, func(t *testing.T) {
			envelope := permissionEnvelopeFixture()
			change(&envelope)
			if err := envelope.Validate(); err == nil {
				t.Fatal("accepted invalid publication")
			}
		})
	}
}

func TestPermissionEnvelopeDecodesWithoutCatalogCompatibility(t *testing.T) {
	envelope := permissionEnvelopeFixture()
	envelope.Head.PermissionSchemaVersion++
	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ParseCatalogPermissionEnvelope(data)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Head != envelope.Head || decoded.Head.SupportsPermissions() {
		t.Fatal("unknown permission semantics lost their required revision or became supported")
	}
	for name, data := range map[string][]byte{
		"empty": nil, "null": []byte("null"), "missing": []byte("{}"),
		"case variant version":      bytes.Replace(data, []byte(`"version":1`), []byte(`"version":1,"Version":1`), 1),
		"case variant authority":    bytes.Replace(data, []byte(`"authority_id":"enterprise"`), []byte(`"authority_id":"other","AUTHORITY_ID":"enterprise"`), 1),
		"case variant field":        bytes.Replace(data, []byte(`"issued_at"`), []byte(`"ISSUED_AT"`), 1),
		"escaped duplicate version": bytes.Replace(data, []byte(`"version":1`), []byte(`"version":1,"versi\u006fn":1`), 1),
		"invalid UTF-8":             bytes.Replace(data, []byte("enterprise"), []byte{0xff}, 1),
		"duplicate version":         bytes.Replace(data, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1),
		"duplicate authority":       bytes.Replace(data, []byte(`"authority_id":"enterprise"`), []byte(`"authority_id":"other","authority_id":"enterprise"`), 1),
		"oversized":                 bytes.Repeat([]byte(" "), MaxCatalogPermissionEnvelopeBytes+1),
		"trailing document":         append(bytes.Clone(data), []byte(" {}")...),
		"unknown member":            bytes.Replace(data, []byte(`"version":1`), []byte(`"unexpected":true,"version":1`), 1),
		"unknown head member":       bytes.Replace(data, []byte(`"authority_id"`), []byte(`"unknown":true,"authority_id"`), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseCatalogPermissionEnvelope(data); err == nil {
				t.Fatal("accepted invalid envelope JSON")
			}
		})
	}
}

func TestPermissionEnvelopeClockBound(t *testing.T) {
	envelope := permissionEnvelopeFixture()
	if err := envelope.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name         string
		now          time.Time
		uncertainty  time.Duration
		known, valid bool
	}{
		{"fresh", envelope.IssuedAt, 30 * time.Second, true, true},
		{"last valid instant", envelope.ValidUntil.Add(-30*time.Second - time.Nanosecond), 30 * time.Second, true, true},
		{"conservative expiry", envelope.ValidUntil.Add(-30 * time.Second), 30 * time.Second, true, false},
		{"expired", envelope.ValidUntil, 0, true, false},
		{"future beyond bound", envelope.IssuedAt.Add(-30*time.Second - time.Nanosecond), 30 * time.Second, true, false},
		{"future within bound", envelope.IssuedAt.Add(-30 * time.Second), 30 * time.Second, true, true},
		{"unknown clock", envelope.IssuedAt, 0, false, false},
		{"excessive uncertainty", envelope.IssuedAt, 30*time.Second + time.Nanosecond, true, false},
		{"negative uncertainty", envelope.IssuedAt, -time.Nanosecond, true, false},
		{"zero clock", time.Time{}, 0, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := envelope.ValidAt(test.now, test.uncertainty, test.known); got != test.valid {
				t.Fatalf("valid=%v, want %v", got, test.valid)
			}
		})
	}
	var valid bool
	allocations := testing.AllocsPerRun(100, func() {
		valid = envelope.ValidAt(envelope.IssuedAt, 30*time.Second, true)
	})
	if !valid || allocations != 0 {
		t.Fatalf("permission time check: valid=%v, allocations=%v", valid, allocations)
	}
}

func TestPermissionEnvelopeSuccessorPreservesPublicationIdentity(t *testing.T) {
	first := permissionEnvelopeFixture()
	for _, test := range []struct {
		name   string
		change func(*CatalogPermissionEnvelope)
		valid  bool
	}{
		{"identical retry", func(*CatalogPermissionEnvelope) {}, true},
		{"renew unchanged head", func(e *CatalogPermissionEnvelope) {
			e.IssuedAt = e.IssuedAt.Add(time.Minute)
			e.ValidUntil = e.ValidUntil.Add(time.Minute)
		}, true},
		{"shorter renewed validity", func(e *CatalogPermissionEnvelope) {
			e.IssuedAt = e.IssuedAt.Add(time.Minute)
			e.ValidUntil = e.ValidUntil.Add(-time.Minute)
		}, true},
		{"later publication", func(e *CatalogPermissionEnvelope) {
			e.Head.Sequence++
			e.Head.GenerationID = "catalog-two"
			e.Head.RequiredPermissionRevision = "sha256:" + strings.Repeat("c", 64)
		}, true},
		{"different authority", func(e *CatalogPermissionEnvelope) { e.Head.AuthorityID = "another"; e.Head.Sequence++ }, false},
		{"different policy", func(e *CatalogPermissionEnvelope) { e.Head.PolicyID = "another"; e.Head.Sequence++ }, false},
		{"reassigned sequence", func(e *CatalogPermissionEnvelope) { e.Head.GenerationID = "catalog-two" }, false},
		{"reassigned permission", func(e *CatalogPermissionEnvelope) {
			e.Head.RequiredPermissionRevision = "sha256:" + strings.Repeat("c", 64)
		}, false},
		{"reassigned receipt", func(e *CatalogPermissionEnvelope) { e.ValidUntil = e.ValidUntil.Add(-time.Minute) }, false},
		{"older receipt", func(e *CatalogPermissionEnvelope) {
			e.IssuedAt = e.IssuedAt.Add(-time.Minute)
			e.ValidUntil = e.ValidUntil.Add(-time.Minute)
		}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			next := first
			test.change(&next)
			if err := first.ValidateSuccessor(next); (err == nil) != test.valid {
				t.Fatalf("successor error=%v, valid=%v", err, test.valid)
			}
		})
	}
	later := first
	later.Head.Sequence++
	if err := later.ValidateSuccessor(first); err == nil {
		t.Fatal("accepted an earlier publication sequence")
	}
}
