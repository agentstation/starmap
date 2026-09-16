package app

import (
	"encoding/json"
	"net/url"
	"slices"

	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/productpaths"
	"github.com/agentstation/starmap/runtime"
	"github.com/agentstation/starmap/server/administration"
)

type reportedSetting struct {
	ID       string              `json:"id"`
	Name     string              `json:"name"`
	Value    string              `json:"value"`
	Present  bool                `json:"present"`
	Presence string              `json:"presence"`
	Origin   string              `json:"origin"`
	Scope    catalogconfig.Scope `json:"scope"`
	Redacted bool                `json:"redacted"`
}

// AdministrationConfig selects private server state and its catalog authority audience.
func (a *App) AdministrationConfig() (administration.Config, error) {
	paths, err := a.ResolvedPaths()
	if err != nil {
		return administration.Config{}, err
	}
	audience := paths.DeploymentID
	if a.catalogSettings.AuthorityOrigin.Enabled {
		audience = a.catalogSettings.AuthorityOrigin.AuthorityID
	} else if selected, _ := a.catalogSettings.Value(catalogconfig.SourceAuthorityID); a.catalogSettings.SourceKind == runtime.SourceStarmap && selected != "" {
		audience = selected
	}
	return administration.Config{StateDirectory: paths.Roots[productpaths.State].Path, Audience: audience}, nil
}

// ConfigurationReports describes resolved configuration without opening the catalog runtime.
func (a *App) ConfigurationReports() (*administration.Reports, error) {
	paths, err := a.FileManifest()
	if err != nil {
		return nil, err
	}
	descriptors := catalogconfig.Descriptors()
	schema, err := json.Marshal(struct {
		SchemaVersion int                        `json:"schema_version"`
		Settings      []catalogconfig.Descriptor `json:"settings"`
	}{1, descriptors})
	if err != nil {
		return nil, err
	}
	values := make([]reportedSetting, 0, len(descriptors))
	for _, descriptor := range descriptors {
		value, present := a.catalogSettings.Value(descriptor.Name)
		item := reportedSetting{ID: descriptor.ID, Name: descriptor.Name, Scope: descriptor.Scope, Present: present, Presence: "omitted", Origin: "default", Value: descriptor.Default}
		if present {
			item.Value, item.Presence = value, "value"
			item.Origin = a.config.CatalogOrigins[descriptor.Name]
			if item.Origin == "" {
				item.Origin = "explicit"
			}
			if value == "" {
				item.Presence = "empty"
			}
		}
		if descriptor.Name == catalogconfig.SourceURL && present {
			parsed, parseErr := url.Parse(value)
			if parseErr != nil {
				item.Value = ""
				item.Redacted = true
			} else if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
				parsed.User = nil
				parsed.RawQuery = ""
				parsed.Fragment = ""
				item.Value = parsed.String()
				item.Redacted = true
			}
		}
		if descriptor.Sensitive {
			item.Value = ""
			item.Redacted = true
		}
		values = append(values, item)
	}
	effective, err := json.Marshal(struct {
		SchemaVersion int                       `json:"schema_version"`
		Settings      []reportedSetting         `json:"settings"`
		Ignored       []catalogconfig.Ignored   `json:"ignored"`
		Paths         productpaths.FileManifest `json:"paths"`
	}{1, values, slices.Clone(a.config.CatalogIgnored), paths})
	if err != nil {
		return nil, err
	}
	return administration.NewReports(schema, effective)
}
