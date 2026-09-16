package server

import (
	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/runtime/status"
)

func validateAdministration(client *starmap.Client, configured options) error {
	authority := client.CurrentCatalogState().AuthorityHead.AuthorityID
	required := authority != ""
	if configured.runtime != nil {
		report := configured.runtime.Status()
		required = required || report.AuthorityRequired || report.SourceKind == status.SourceStarmap
	}
	if required && configured.administration == nil {
		return &errors.ConfigError{Component: "server administration", Message: "internal catalogs require managed subscriber and administrator identities"}
	}
	if authority != "" && configured.audience != authority {
		return &errors.ConfigError{Component: "server administration", Message: "reader audience must match the accepted catalog authority"}
	}
	return nil
}
