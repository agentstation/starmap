package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	policyRecordLimit  = 4096
	policyRecordName   = "policy.json"
	policyRecordPrefix = ".policy-"
)

// PolicyStore retains the accepted selection policy without credential material.
type PolicyStore interface {
	Policy(context.Context, catalogs.ProviderID) (EnvironmentPolicy, error)
	Accept(context.Context, catalogs.ProviderID) error
}

// PolicyOwner binds a local policy store to one product deployment and instance.
type PolicyOwner struct {
	Product    string `json:"product"`
	Deployment string `json:"deployment"`
	Instance   string `json:"instance"`
}

type policyRecord struct {
	SchemaVersion int                 `json:"schema_version"`
	Owner         PolicyOwner         `json:"owner"`
	Provider      catalogs.ProviderID `json:"provider,omitempty"`
	Policy        EnvironmentPolicy   `json:"policy"`
}

// FilePolicyStore stores immutable default and per-provider policy decisions.
type FilePolicyStore struct {
	directory *privatefiles.Directory
	owner     PolicyOwner
}

// OpenFilePolicyStore initializes explicit private storage without reading provider credentials.
// Existing default policy takes precedence over the initial policy hint.
func OpenFilePolicyStore(ctx context.Context, path string, owner PolicyOwner, initial EnvironmentPolicy) (*FilePolicyStore, error) {
	if ctx == nil {
		return nil, policyStoreError("context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if owner.Product == "" || owner.Deployment == "" || owner.Instance == "" || !validEnvironmentPolicy(initial) {
		return nil, policyStoreError("requires a complete owner and supported initial policy")
	}
	directory, err := privatefiles.NewDirectory(path)
	if err != nil {
		return nil, err
	}
	store := &FilePolicyStore{directory: directory, owner: owner}
	if _, err := store.read(policyRecordName, ""); err == nil {
		return store, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	record := policyRecord{SchemaVersion: 1, Owner: owner, Policy: initial}
	if err := store.create(ctx, policyRecordName, record); err != nil && !errors.IsConflict(err) {
		return nil, err
	}
	if _, err := store.read(policyRecordName, ""); err != nil {
		return nil, err
	}
	return store, nil
}

// Policy reads the current default and any accepted provider migration.
func (s *FilePolicyStore) Policy(ctx context.Context, provider catalogs.ProviderID) (EnvironmentPolicy, error) {
	if ctx == nil || provider == "" {
		return "", policyStoreError("context and provider are required")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	base, err := s.read(policyRecordName, "")
	if err != nil {
		return "", err
	}
	if base.Policy == EnvironmentPolicyCurrent {
		return base.Policy, nil
	}
	record, err := s.read(providerPolicyName(provider), provider)
	if os.IsNotExist(err) {
		return base.Policy, nil
	}
	if err != nil {
		return "", err
	}
	if record.Policy != EnvironmentPolicyCurrent {
		return "", policyStoreError("provider migration requires the current policy")
	}
	return record.Policy, nil
}

// Accept records a provider migration after complete material comparison succeeds.
func (s *FilePolicyStore) Accept(ctx context.Context, provider catalogs.ProviderID) error {
	policy, err := s.Policy(ctx, provider)
	if err != nil || policy == EnvironmentPolicyCurrent {
		return err
	}
	record := policyRecord{SchemaVersion: 1, Owner: s.owner, Provider: provider, Policy: EnvironmentPolicyCurrent}
	if err := s.create(ctx, providerPolicyName(provider), record); err != nil && !errors.IsConflict(err) {
		return err
	}
	accepted, err := s.Policy(ctx, provider)
	if err != nil {
		return err
	}
	if accepted != EnvironmentPolicyCurrent {
		return policyStoreError("provider policy did not advance")
	}
	return nil
}

func (s *FilePolicyStore) read(name string, provider catalogs.ProviderID) (policyRecord, error) {
	data, err := s.directory.ReadFile(name, policyRecordLimit)
	if err != nil {
		return policyRecord{}, err
	}
	var record policyRecord
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return policyRecord{}, policyStoreError("policy record is invalid")
	}
	if err := rejectTrailingJSON(decoder); err != nil || record.SchemaVersion != 1 || record.Owner != s.owner ||
		record.Provider != provider || !validEnvironmentPolicy(record.Policy) {
		return policyRecord{}, policyStoreError("policy record does not match its owner, provider, or supported schema")
	}
	canonical, err := json.Marshal(record)
	if err != nil || !bytes.Equal(data, canonical) {
		return policyRecord{}, policyStoreError("policy record requires canonical encoding")
	}
	return record, nil
}

func (s *FilePolicyStore) create(ctx context.Context, name string, record policyRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return policyStoreError("cannot encode policy record")
	}
	if len(data) > policyRecordLimit {
		return policyStoreError("policy record exceeds its size limit")
	}
	return s.directory.WriteFileIfAbsentContext(ctx, name, data, policyRecordPrefix)
}

func providerPolicyName(provider catalogs.ProviderID) string {
	digest := sha256.Sum256([]byte(provider))
	return "provider-" + hex.EncodeToString(digest[:]) + ".json"
}

func validEnvironmentPolicy(policy EnvironmentPolicy) bool {
	return policy == EnvironmentPolicyLegacy || policy == EnvironmentPolicyCurrent
}

func policyStoreError(message string) error {
	return &errors.ConfigError{Component: "credential policy", Message: message}
}
