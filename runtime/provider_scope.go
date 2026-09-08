package runtime

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"strconv"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// providerEvidenceKey separates retained scopes and their declared revisions.
// Empty binding fields identify legacy unscoped evidence, without declaring public coverage.
type providerEvidenceKey struct {
	providerID catalogs.ProviderID
	bindingID  string
	revision   string
}

func (layer ProviderLayer) evidenceKey() providerEvidenceKey {
	key := providerEvidenceKey{providerID: layer.ProviderID}
	if binding := layer.Receipt.ProviderBinding; binding != nil {
		key.bindingID = binding.ID
		key.revision = binding.Revision
	}
	return key
}

func compareProviderEvidenceKeys(left, right providerEvidenceKey) int {
	if order := cmp.Compare(left.providerID, right.providerID); order != 0 {
		return order
	}
	if order := cmp.Compare(left.bindingID, right.bindingID); order != 0 {
		return order
	}
	return cmp.Compare(left.revision, right.revision)
}

func (key providerEvidenceKey) scoped() bool { return key.bindingID != "" }

// filename hashes scoped identities so selectors never become filesystem paths.
func (key providerEvidenceKey) filename() string {
	if !key.scoped() {
		return string(key.providerID) + ".json"
	}
	encoded := "provider-evidence:v1:"
	for _, field := range []string{string(key.providerID), key.bindingID, key.revision} {
		encoded += strconv.Itoa(len(field)) + ":" + field
	}
	digest := sha256.Sum256([]byte(encoded))
	return hex.EncodeToString(digest[:]) + ".json"
}

// providerBindingRevision identifies one deployment declaration across providers.
type providerBindingRevision struct {
	id       string
	revision string
}

func validateProviderScopes(prepared []ProviderLayer, retained map[providerEvidenceKey]ProviderLayer) error {
	bindings := make(map[providerBindingRevision]sources.ProviderAcquisitionBinding, len(prepared)+len(retained))
	record := func(layer ProviderLayer) error {
		binding := layer.Receipt.ProviderBinding
		if binding == nil {
			return nil
		}
		key := providerBindingRevision{id: binding.ID, revision: binding.Revision}
		if prior, exists := bindings[key]; exists && prior != *binding {
			return &errors.ConflictError{Resource: "provider binding revision", Message: "scope selectors changed without a new binding revision"}
		}
		bindings[key] = *binding
		return nil
	}
	for _, layer := range retained {
		if err := record(layer); err != nil {
			return err
		}
	}
	for _, layer := range prepared {
		if err := record(layer); err != nil {
			return err
		}
	}
	return nil
}
