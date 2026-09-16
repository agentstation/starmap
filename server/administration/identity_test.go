package administration

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"
)

func identityFixture() (identitiesRecord, time.Time) {
	now := time.Date(2026, time.September, 16, 0, 0, 0, 0, time.UTC)
	key := func(token string) credentialRecord {
		digest := sha256.Sum256([]byte(token))
		return credentialRecord{Digest: hex.EncodeToString(digest[:]), NotBefore: now}
	}
	return identitiesRecord{
		SchemaVersion: identitySchemaVersion, Audience: "enterprise/catalog", Revision: 1,
		Identities: []identityRecord{
			{ID: "operator", Role: Administrator, Keys: []credentialRecord{key("admin-fixture")}},
			{ID: "gateway", Role: Subscriber, Keys: []credentialRecord{key("reader-fixture")}},
		},
	}, now
}

func TestCredentialAudienceAndRoleIsolation(t *testing.T) {
	t.Parallel()
	record, now := identityFixture()
	snapshot, err := buildIdentitySnapshot(record, record.Audience)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, token, audience string
		role                  Role
		id                    string
		accepted              bool
	}{
		{"administrator", "admin-fixture", record.Audience, Administrator, "operator", true},
		{"subscriber", "reader-fixture", record.Audience, Subscriber, "gateway", true},
		{"different audience", "admin-fixture", "other/catalog", "", "", false},
		{"unknown token", "unknown-fixture", record.Audience, "", "", false},
		{"empty token", "", record.Audience, "", "", false},
		{"oversized token", strings.Repeat("x", maximumTokenBytes+1), record.Audience, "", "", false},
		{"stored digest is not a token", record.Identities[0].Keys[0].Digest, record.Audience, "", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			principal, ok := snapshot.authenticate(test.token, test.audience, now)
			if ok != test.accepted || principal.Role() != test.role || principal.ID() != test.id {
				t.Fatalf("authentication = (%q, %q, %v)", principal.ID(), principal.Role(), ok)
			}
			if ok && principal.Audience() != test.audience {
				t.Fatalf("audience = %q", principal.Audience())
			}
		})
	}
}

func TestCredentialRotationOverlapBoundary(t *testing.T) {
	t.Parallel()
	record, now := identityFixture()
	end := now.Add(time.Minute)
	old := &record.Identities[0].Keys[0]
	old.NotAfter = end
	digest := sha256.Sum256([]byte("replacement-fixture"))
	record.Identities[0].Keys = append(record.Identities[0].Keys, credentialRecord{
		Digest: hex.EncodeToString(digest[:]), NotBefore: now.Add(30 * time.Second),
	})
	snapshot, err := buildIdentitySnapshot(record, record.Audience)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, token string
		at          time.Time
		accepted    bool
	}{
		{"before first key", "admin-fixture", now.Add(-time.Nanosecond), false},
		{"first key begins", "admin-fixture", now, true},
		{"replacement not active", "replacement-fixture", now, false},
		{"old key in overlap", "admin-fixture", end.Add(-time.Nanosecond), true},
		{"replacement in overlap", "replacement-fixture", end.Add(-time.Nanosecond), true},
		{"old key expires", "admin-fixture", end, false},
		{"replacement remains", "replacement-fixture", end, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, ok := snapshot.authenticate(test.token, record.Audience, test.at); ok != test.accepted {
				t.Fatalf("accepted = %v, want %v", ok, test.accepted)
			}
		})
	}
	record.Identities[0].Revoked = true
	revoked, err := buildIdentitySnapshot(record, record.Audience)
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"admin-fixture", "replacement-fixture"} {
		if _, ok := revoked.authenticate(token, record.Audience, now.Add(45*time.Second)); ok {
			t.Fatal("revocation preserved an accepted key")
		}
	}
}

func TestIdentitySnapshotRejectsAmbiguousRecords(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		change func(*identitiesRecord)
	}{
		{"unknown schema", func(r *identitiesRecord) { r.SchemaVersion++ }},
		{"zero revision", func(r *identitiesRecord) { r.Revision = 0 }},
		{"different audience", func(r *identitiesRecord) { r.Audience = "another" }},
		{"empty identities", func(r *identitiesRecord) { r.Identities = nil }},
		{"duplicate identity", func(r *identitiesRecord) { r.Identities[1].ID = r.Identities[0].ID }},
		{"invalid identity", func(r *identitiesRecord) { r.Identities[0].ID = "../operator" }},
		{"unknown role", func(r *identitiesRecord) { r.Identities[0].Role = "owner" }},
		{"missing key", func(r *identitiesRecord) { r.Identities[0].Keys = nil }},
		{"duplicate key across roles", func(r *identitiesRecord) { r.Identities[1].Keys = slices.Clone(r.Identities[0].Keys) }},
		{"three keys", func(r *identitiesRecord) { r.Identities[0].Keys = slices.Repeat(r.Identities[0].Keys, 3) }},
		{"invalid digest", func(r *identitiesRecord) { r.Identities[0].Keys[0].Digest = "secret-fixture" }},
		{"uppercase digest", func(r *identitiesRecord) {
			r.Identities[0].Keys[0].Digest = strings.ToUpper(r.Identities[0].Keys[0].Digest)
		}},
		{"missing start", func(r *identitiesRecord) { r.Identities[0].Keys[0].NotBefore = time.Time{} }},
		{"invalid interval", func(r *identitiesRecord) { r.Identities[0].Keys[0].NotAfter = r.Identities[0].Keys[0].NotBefore }},
	} {
		t.Run(test.name, func(t *testing.T) {
			record, _ := identityFixture()
			test.change(&record)
			if snapshot, err := buildIdentitySnapshot(record, "enterprise/catalog"); err == nil || snapshot != nil {
				t.Fatal("invalid identities produced an accepted snapshot")
			}
		})
	}
}

func TestIdentitySnapshotDoesNotAliasRecords(t *testing.T) {
	t.Parallel()
	record, now := identityFixture()
	snapshot, err := buildIdentitySnapshot(record, record.Audience)
	if err != nil {
		t.Fatal(err)
	}
	record.Identities[1].Role = Administrator
	record.Identities[1].ID = "replacement"
	record.Identities[1].Keys[0].NotAfter = now
	principal, ok := snapshot.authenticate("reader-fixture", record.Audience, now)
	if !ok || principal.ID() != "gateway" || principal.Role() != Subscriber {
		t.Fatal("caller mutation changed the published identity")
	}
}

func TestIdentityCodecRejectsNonCanonicalRecords(t *testing.T) {
	t.Parallel()
	record, _ := identityFixture()
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := decodeIdentities(append(slices.Clone(encoded), '\n'), record.Audience); err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{
		nil,
		append(slices.Clone(encoded), encoded...),
		bytes.Replace(encoded, []byte(`"revision":1`), []byte(`"revision":1,"revision":1`), 1),
		bytes.Replace(encoded, []byte(`"revision":1`), []byte(`"revision":1,"unknown":true`), 1),
		bytes.Replace(encoded, []byte(`"role":"subscriber"`), []byte(`"role":"administrator","role":"subscriber"`), 1),
	} {
		if _, snapshot, err := decodeIdentities(data, record.Audience); err == nil || snapshot != nil {
			t.Fatal("ambiguous identity document was accepted")
		}
	}
}

func TestCredentialGenerationKeepsSecretsOutOfRecords(t *testing.T) {
	t.Parallel()
	for _, role := range []Role{Subscriber, Administrator} {
		token, credential, err := newCredential(role, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(credential)
		if err != nil {
			t.Fatal(err)
		}
		if len(token) != 4+2*credentialBytes || bytes.Contains(encoded, []byte(token)) {
			t.Fatal("credential record retained the authentication token")
		}
		if _, err := checkedCredentialDigest(credential); err != nil {
			t.Fatal(err)
		}
	}
}
