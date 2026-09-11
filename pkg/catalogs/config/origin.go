package config

import (
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// AuthorityOrigin selects one complete origin declaration across configuration layers.
const AuthorityOrigin = Prefix + "CATALOG_AUTHORITY_ORIGIN"

// OriginSettings authorizes a host to publish one catalog authority.
// Configuration replaces the complete declaration, including explicit disablement.
// Disabling issuance does not remove authority from an existing catalog store.
type OriginSettings struct {
	Enabled            bool
	AuthorityID        string
	PolicyID           string
	Bootstrap          bool
	PermissionLifetime time.Duration
}

const maxOriginDeclarationBytes = 4096

func captureAuthorityOrigin(value string, c *Config) error {
	invalid := func() error {
		return &errors.ValidationError{Field: AuthorityOrigin, Message: "must declare enabled and valid origin fields in one JSON object"}
	}
	if len(value) > maxOriginDeclarationBytes {
		return invalid()
	}
	// Decode each key once. Duplicate keys, unknown fields, and null values fail.
	decoder := json.NewDecoder(strings.NewReader(value))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return invalid()
	}
	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return invalid()
		}
		name, ok := key.(string)
		if !ok || fields[name] != nil {
			return invalid()
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil || string(raw) == "null" {
			return invalid()
		}
		fields[name] = raw
	}
	if end, err := decoder.Token(); err != nil || end != json.Delim('}') {
		return invalid()
	}
	if _, err := decoder.Token(); err != io.EOF {
		return invalid()
	}
	var origin OriginSettings
	for name, raw := range fields {
		var target any
		var duration string
		switch name {
		case "enabled":
			target = &origin.Enabled
		case "authority_id":
			target = &origin.AuthorityID
		case "policy_id":
			target = &origin.PolicyID
		case "bootstrap":
			target = &origin.Bootstrap
		case "permission_lifetime":
			target = &duration
		default:
			return invalid()
		}
		if err := json.Unmarshal(raw, target); err != nil {
			return invalid()
		}
		if name == "permission_lifetime" {
			parsed, err := time.ParseDuration(duration)
			if err != nil || parsed <= 0 || parsed > catalogs.MaxCatalogPermissionValidity {
				return invalid()
			}
			origin.PermissionLifetime = parsed
		}
	}
	if fields["enabled"] == nil || !origin.Enabled && len(fields) != 1 {
		return invalid()
	}
	if origin.Enabled {
		if err := catalogs.ValidateCatalogAuthorityIdentity(origin.AuthorityID, origin.PolicyID); err != nil {
			return invalid()
		}
	}
	c.AuthorityOrigin = origin
	return nil
}
