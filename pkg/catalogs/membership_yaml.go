package catalogs

import (
	"encoding/json"

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/pkg/errors"
)

const membershipScopeFileName = "membership-scopes.yaml"

func (b *Builder) loadMembershipScopesYAML() error {
	records, err := readWorkspaceRecords[ProviderMembershipScope](b.config.readFilesystem(), membershipScopeFileName, "membership_scope.workspace")
	if err != nil {
		return err
	}
	return b.SetMembershipScopes(records)
}

func (b *Builder) saveMembershipScopesYAML(writeFile catalogFileWriter) error {
	scopes := b.MembershipScopes()
	if len(scopes) == 0 {
		return nil
	}
	// JSON preserves the distinction between null and empty membership arrays.
	data, err := json.Marshal(scopes)
	if err != nil {
		return errors.WrapResource("encode", "membership scopes", "", err)
	}
	data, err = yaml.JSONToYAML(data)
	if err != nil {
		return errors.WrapResource("encode", "membership scopes", "", err)
	}
	if err := writeFile(membershipScopeFileName, data); err != nil {
		return errors.WrapIO("write", membershipScopeFileName, err)
	}
	return nil
}
