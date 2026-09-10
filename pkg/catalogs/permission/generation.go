package permission

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	permissionRevisionDomain  = "starmap/catalog-permission-revision/v1"
	authorityGenerationDomain = "starmap/catalog-authority-generation/v1"
)

// GenerationConfig identifies one complete permitted catalog and its publication sequence.
// The origin selects Sequence from its durable predecessor. Publisher checks the final atomic commit.
type GenerationConfig struct {
	AuthorityID string
	PolicyID    string
	Sequence    uint64
}

// PrepareGeneration binds an ordinary catalog to an explicitly selected origin authority.
// The entire input catalog defines the permitted catalog for this policy.
// The caller must apply its catalog policy before preparation and authorize the origin separately.
// An existing authority generation must retain its original identity through the subscriber or relay path.
// Preparation starts no I/O and returns independent copies of mutable data.
// Publication still requires Publisher and its exact durable predecessor.
//
// The required revision binds authority, policy, permission schema, and every semantic catalog fact.
// It excludes provenance and manifest observation metadata, but includes catalog scope evidence and rename history.
// This conservative revision changes even for metadata-only catalog changes.
// Identical semantics under a new publication sequence keep the revision, while the generation identity changes.
func PrepareGeneration(input catalogs.Generation, config GenerationConfig) (catalogs.Generation, error) {
	if err := catalogs.ValidateCatalogAuthorityIdentity(config.AuthorityID, config.PolicyID); err != nil {
		return catalogs.Generation{}, err
	}
	if config.Sequence == 0 {
		return catalogs.Generation{}, generationError("publication sequence must be positive")
	}
	if input.Manifest.ManifestVersion != catalogs.CurrentGenerationManifestVersion || input.Manifest.AuthorityHead != (catalogs.CatalogAuthorityHead{}) {
		return catalogs.Generation{}, generationError("origin preparation requires an ordinary catalog generation")
	}
	catalog, err := catalogs.DecodeCatalogGeneration(input)
	if err != nil {
		return catalogs.Generation{}, err
	}
	semantic, err := catalogs.CatalogSemanticChecksum(catalog)
	if err != nil {
		return catalogs.Generation{}, err
	}
	revision, err := originDigest(permissionRevisionDomain, struct {
		AuthorityID             string `json:"authority_id"`
		PolicyID                string `json:"policy_id"`
		PermissionSchemaVersion uint64 `json:"permission_schema_version"`
		CatalogChecksum         string `json:"catalog_checksum"`
	}{config.AuthorityID, config.PolicyID, catalogs.CatalogPermissionSchemaVersion, semantic})
	if err != nil {
		return catalogs.Generation{}, err
	}
	return prepareAuthorityManifest(input, config, revision)
}

func prepareAuthorityManifest(input catalogs.Generation, config GenerationConfig, revision string) (catalogs.Generation, error) {
	identity, err := originDigest(authorityGenerationDomain, struct {
		AuthorityID                string                      `json:"authority_id"`
		PolicyID                   string                      `json:"policy_id"`
		Sequence                   uint64                      `json:"sequence"`
		RequiredPermissionRevision string                      `json:"required_permission_revision"`
		SourceManifest             catalogs.GenerationManifest `json:"source_manifest"`
	}{config.AuthorityID, config.PolicyID, config.Sequence, revision, input.Manifest})
	if err != nil {
		return catalogs.Generation{}, err
	}
	output := input.Copy()
	output.Manifest.ManifestVersion = catalogs.AuthorityGenerationManifestVersion
	output.Manifest.GenerationID = "authority-" + identity[len("sha256:"):]
	output.Manifest.AuthorityHead = catalogs.CatalogAuthorityHead{
		AuthorityID: config.AuthorityID, PolicyID: config.PolicyID, Sequence: config.Sequence,
		GenerationID: output.Manifest.GenerationID, PayloadChecksum: output.Manifest.Payload.Checksum,
		RequiredPermissionRevision: revision, PermissionSchemaVersion: catalogs.CatalogPermissionSchemaVersion,
	}
	if err := output.Validate(); err != nil {
		return catalogs.Generation{}, err
	}
	return output, nil
}

func originDigest(domain string, value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", &errors.ValidationError{Field: "authority_generation", Message: fmt.Sprintf("cannot encode identity: %v", err)}
	}
	digest := sha256.Sum256(append(append([]byte(domain), 0), data...))
	return fmt.Sprintf("sha256:%x", digest), nil
}

func generationError(message string) error {
	return &errors.ValidationError{Field: "authority_generation", Message: message}
}
