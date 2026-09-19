package administration

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

type authenticatedKey struct {
	principal Principal
	notBefore time.Time
	notAfter  time.Time
}

// identitySnapshot contains only accepted credentials. Published snapshots never change.
type identitySnapshot struct {
	audience string
	revision uint64
	keys     map[[sha256.Size]byte]authenticatedKey
}

func buildIdentitySnapshot(record identitiesRecord, audience string) (*identitySnapshot, error) {
	if record.SchemaVersion != identitySchemaVersion || !validAudience(audience) || record.Audience != audience || record.Revision == 0 ||
		len(record.Identities) == 0 || len(record.Identities) > maximumIdentities {
		return nil, invalidIdentityRecord()
	}
	snapshot := &identitySnapshot{audience: audience, revision: record.Revision, keys: make(map[[sha256.Size]byte]authenticatedKey)}
	identities := make(map[string]bool, len(record.Identities))
	digests := make(map[[sha256.Size]byte]bool)
	for _, identity := range record.Identities {
		if !validIdentity(identity.ID) || !validRole(identity.Role) || identities[identity.ID] ||
			len(identity.Keys) == 0 || len(identity.Keys) > maximumCredentialKeys {
			return nil, invalidIdentityRecord()
		}
		identities[identity.ID] = true
		for _, credential := range identity.Keys {
			digest, err := checkedCredentialDigest(credential)
			if err != nil || digests[digest] {
				return nil, invalidIdentityRecord()
			}
			digests[digest] = true
			if !identity.Revoked {
				snapshot.keys[digest] = authenticatedKey{
					principal: Principal{id: identity.ID, role: identity.Role, audience: audience, key: digest},
					notBefore: credential.NotBefore, notAfter: credential.NotAfter,
				}
			}
		}
	}
	return snapshot, nil
}

func checkedCredentialDigest(credential credentialRecord) ([sha256.Size]byte, error) {
	var digest [sha256.Size]byte
	if len(credential.Digest) != hex.EncodedLen(len(digest)) || credential.NotBefore.IsZero() ||
		!credential.NotAfter.IsZero() && !credential.NotAfter.After(credential.NotBefore) {
		return digest, invalidIdentityRecord()
	}
	if _, err := hex.Decode(digest[:], []byte(credential.Digest)); err != nil || hex.EncodeToString(digest[:]) != credential.Digest {
		return [sha256.Size]byte{}, invalidIdentityRecord()
	}
	return digest, nil
}

func (s *identitySnapshot) authenticate(token, audience string, now time.Time) (Principal, bool) {
	if s == nil || audience != s.audience || token == "" || len(token) > maximumTokenBytes || now.IsZero() {
		return Principal{}, false
	}
	var encoded [maximumTokenBytes]byte
	copy(encoded[:], token)
	digest := sha256.Sum256(encoded[:len(token)])
	return s.authenticateDigest(digest, now)
}

func (s *identitySnapshot) authenticateDigest(digest [sha256.Size]byte, now time.Time) (Principal, bool) {
	credential, exists := s.keys[digest]
	if !exists || now.Before(credential.notBefore) || !credential.notAfter.IsZero() && !now.Before(credential.notAfter) {
		return Principal{}, false
	}
	return credential.principal, true
}
