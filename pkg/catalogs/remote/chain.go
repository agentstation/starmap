package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"time"
	"unicode"

	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// SourceChainPath returns the source-chain manifest of the serving node.
	SourceChainPath = CatalogPath + "/source-chain"

	// SourceChainMediaType identifies strict source-chain JSON.
	SourceChainMediaType = "application/vnd.agentstation.starmap.source-chain+json"

	// SourceChainSchemaVersion is the current source-chain document version.
	SourceChainSchemaVersion uint64 = 1

	// MaxSourceChainHops bounds the hops one document may disclose. A chain
	// discloses topology, so the bound keeps the disclosure small and keeps a
	// forged document from growing without limit.
	MaxSourceChainHops = 16

	// maxSourceIdentityBytes bounds one safe source identity.
	maxSourceIdentityBytes = 128
)

// Source-chain health codes. The set is closed, so a document discloses a
// grade and never free-form text.
const (
	// SourceChainHealthUnknown means the node reported no grade yet.
	SourceChainHealthUnknown = "unknown"
	// SourceChainHealthOK means the node reached its last objective.
	SourceChainHealthOK = "ok"
	// SourceChainHealthDegraded means the node works with reduced evidence.
	SourceChainHealthDegraded = "degraded"
	// SourceChainHealthUnavailable means the node cannot reach its dependency.
	SourceChainHealthUnavailable = "unavailable"
)

// sourceChainHealthCodes is the closed set of accepted grades.
var sourceChainHealthCodes = []string{
	SourceChainHealthUnknown,
	SourceChainHealthOK,
	SourceChainHealthDegraded,
	SourceChainHealthUnavailable,
}

// SourceChainHop is one sanitized upstream node of a served chain. A hop names
// a safe identity and a grade. It never names an address, a host, or a token.
type SourceChainHop struct {
	Identity    string    `json:"identity"`
	Health      string    `json:"health"`
	PublishedAt time.Time `json:"published_at,omitempty"`
	ObservedAt  time.Time `json:"observed_at,omitempty"`
}

// SourceChain is what one Starmap discloses about itself and the hops above
// it. The serving node is Identity, and Hops lists its upstream nodes with the
// nearest hop first. A downstream reads the document to reject a cycle and to
// evaluate the propagated channel freshness of the whole chain.
type SourceChain struct {
	// SchemaVersion is the document version. A reader rejects another version.
	SchemaVersion uint64 `json:"schema_version"`

	// Identity is the safe identity of the serving node.
	Identity string `json:"identity"`

	// Health is what the serving node observed while it read its own source.
	Health string `json:"health"`

	// UpstreamHealth is the health the serving node's upstream reported about
	// itself. It stays independent of Health.
	UpstreamHealth string `json:"upstream_health"`

	// SourceIdentity is the safe identity of the source the node reads.
	SourceIdentity string `json:"source_identity,omitempty"`

	// GenerationID identifies the catalog generation the node serves.
	GenerationID string `json:"generation_id,omitempty"`

	// ChannelUpdatedAt is the propagated time the origin channel last moved.
	// Every hop passes the value through unchanged, so a downstream grades the
	// whole chain instead of its own last check.
	ChannelUpdatedAt time.Time `json:"channel_updated_at"`

	// ObservedAt is when the serving node built the document.
	ObservedAt time.Time `json:"observed_at"`

	// Hops lists the upstream nodes, nearest hop first.
	Hops []SourceChainHop `json:"hops,omitempty"`
}

// Identities returns the serving identity followed by every hop identity, in
// chain order. A caller uses the list to reject a self reference and a cycle.
func (c SourceChain) Identities() []string {
	identities := make([]string, 0, len(c.Hops)+1)
	identities = append(identities, c.Identity)
	for _, hop := range c.Hops {
		identities = append(identities, hop.Identity)
	}
	return identities
}

// Validate reports whether the document is a usable source chain. It bounds
// the hop count and every identity, and it accepts only the closed health set.
func (c SourceChain) Validate() error {
	if c.SchemaVersion != SourceChainSchemaVersion {
		return chainValidation("schema_version", c.SchemaVersion,
			"does not match the supported source-chain version")
	}
	if err := validateSourceIdentity("identity", c.Identity); err != nil {
		return err
	}
	if err := validateChainHealth("health", c.Health); err != nil {
		return err
	}
	if err := validateChainHealth("upstream_health", c.UpstreamHealth); err != nil {
		return err
	}
	if c.SourceIdentity != "" {
		if err := validateSourceIdentity("source_identity", c.SourceIdentity); err != nil {
			return err
		}
	}
	if len(c.Hops) > MaxSourceChainHops {
		return chainValidation("hops", len(c.Hops), "exceeds the maximum hop count")
	}
	for _, hop := range c.Hops {
		if err := validateSourceIdentity("hops.identity", hop.Identity); err != nil {
			return err
		}
		if err := validateChainHealth("hops.health", hop.Health); err != nil {
			return err
		}
	}
	return nil
}

// MarshalSourceChain returns strict JSON bytes for the server route.
func MarshalSourceChain(chain SourceChain) ([]byte, error) {
	if err := chain.Validate(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(chain)
	if err != nil {
		return nil, errors.WrapResource("encode", "source chain", chain.Identity, err)
	}
	return data, nil
}

// UnmarshalSourceChain decodes and validates one source-chain document.
func UnmarshalSourceChain(data []byte) (SourceChain, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var chain SourceChain
	if err := decoder.Decode(&chain); err != nil {
		return SourceChain{}, errors.WrapParse("json", "source chain", err)
	}
	if err := chain.Validate(); err != nil {
		return SourceChain{}, err
	}
	return chain, nil
}

// FetchSourceChain returns the source-chain manifest of the configured
// publisher. A publisher that serves no chain answers with a not-found status,
// so a caller treats that upstream as an origin without a disclosed chain.
func (c *Client) FetchSourceChain(ctx context.Context) (SourceChain, error) {
	data, err := c.fetch(ctx, SourceChainPath, SourceChainMediaType)
	if err != nil {
		return SourceChain{}, err
	}
	return UnmarshalSourceChain(data)
}

func validateChainHealth(field, value string) error {
	if !slices.Contains(sourceChainHealthCodes, value) {
		return chainValidation(field, value, "is not a supported health code")
	}
	return nil
}

// validateSourceIdentity accepts a bounded printable identity. A control
// character or an oversized value is unsafe to log and to serve onward.
func validateSourceIdentity(field, value string) error {
	if value == "" {
		return chainValidation(field, value, "is required")
	}
	if len(value) > maxSourceIdentityBytes {
		return chainValidation(field, len(value), "exceeds the maximum identity size")
	}
	for _, symbol := range value {
		if !unicode.IsPrint(symbol) {
			return chainValidation(field, field, "must contain printable characters only")
		}
	}
	return nil
}

func chainValidation(field string, value any, message string) error {
	return &errors.ValidationError{
		Field:   "catalog_remote.source_chain." + field,
		Value:   value,
		Message: message,
	}
}
