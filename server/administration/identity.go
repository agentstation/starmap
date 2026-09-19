// Package administration owns standalone server identities and administrative state.
package administration

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/agentstation/starmap/pkg/errors"
)

// Role separates catalog subscription from server administration.
type Role string

const (
	// Subscriber can read the catalog within its configured audience.
	Subscriber Role = "subscriber"
	// Administrator can authorize server mutations and read configuration reports.
	Administrator Role = "administrator"

	identitySchemaVersion = 1
	maximumIdentities     = 4096
	maximumIdentityBytes  = 128
	maximumAudienceBytes  = 256
	maximumCredentialKeys = 2
	maximumTokenBytes     = 256
	credentialBytes       = 32
)

// Principal identifies an authenticated caller. Callers cannot construct its authority.
type Principal struct {
	id       string
	role     Role
	audience string
	key      [sha256.Size]byte
}

// ID returns the stable identity name without credential material.
func (p Principal) ID() string { return p.id }

// Role returns the permissions assigned to this identity.
func (p Principal) Role() Role { return p.role }

// Audience returns the catalog authority namespace assigned to this identity.
func (p Principal) Audience() string { return p.audience }

type identitiesRecord struct {
	SchemaVersion int              `json:"schema_version"`
	Audience      string           `json:"audience"`
	Revision      uint64           `json:"revision"`
	Identities    []identityRecord `json:"identities"`
}

type identityRecord struct {
	ID      string             `json:"id"`
	Role    Role               `json:"role"`
	Revoked bool               `json:"revoked"`
	Keys    []credentialRecord `json:"keys"`
}

type credentialRecord struct {
	Digest    string    `json:"digest"`
	NotBefore time.Time `json:"not_before"`
	NotAfter  time.Time `json:"not_after"`
}

func validIdentity(value string) bool {
	if value == "" || len(value) > maximumIdentityBytes {
		return false
	}
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("._-", char) {
			continue
		}
		return false
	}
	return value != "." && value != ".."
}

func validRole(role Role) bool { return role == Subscriber || role == Administrator }

func validAudience(value string) bool {
	return value != "" && len(value) <= maximumAudienceBytes && utf8.ValidString(value) &&
		strings.TrimSpace(value) == value && strings.IndexFunc(value, unicode.IsControl) < 0
}

func newCredential(role Role, now time.Time) (string, credentialRecord, error) {
	if !validRole(role) || now.IsZero() {
		return "", credentialRecord{}, invalidIdentityRecord()
	}
	var random [credentialBytes]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", credentialRecord{}, err
	}
	prefix := "sms_"
	if role == Administrator {
		prefix = "sma_"
	}
	token := prefix + hex.EncodeToString(random[:])
	digest := sha256.Sum256([]byte(token))
	return token, credentialRecord{Digest: hex.EncodeToString(digest[:]), NotBefore: now.UTC()}, nil
}

func invalidIdentityRecord() error {
	return &errors.ValidationError{Field: "server.identities", Message: "requires valid, distinct identities and bounded credentials for one audience"}
}
