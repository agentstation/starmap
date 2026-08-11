package pipeline

import (
	stderrors "errors"
	"os"
	"strings"

	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

type catalogInputs struct {
	workspace       *catalogs.Catalog
	embedded        *catalogs.Catalog
	providerConfig  *catalogs.Catalog
	workspaceReport catalogs.LoadReport
	workspaceInput  workspace.InputExpectation
}

func (p *Pipeline) loadCatalogInputs(
	path string,
	input workspace.InputExpectation,
) (catalogInputs, error) {
	human, err := p.loadWorkspace(path)
	if err != nil {
		return catalogInputs{}, errors.WrapResource("load", "catalog", "human workspace", err)
	}
	embedded, err := p.loadEmbedded()
	if err != nil {
		return catalogInputs{}, errors.WrapResource("load", "catalog", "embedded", err)
	}
	humanCatalog, err := human.Build()
	if err != nil {
		return catalogInputs{}, errors.WrapResource("publish", "human workspace catalog", "", err)
	}
	input, err = workspace.BindInputCatalog(input, humanCatalog)
	if err != nil {
		return catalogInputs{}, err
	}
	embeddedCatalog, err := embedded.Build()
	if err != nil {
		return catalogInputs{}, errors.WrapResource("publish", "embedded catalog", "", err)
	}
	providerCatalog, err := composeProviderCatalog(embeddedCatalog, humanCatalog, input.Exists)
	if err != nil {
		return catalogInputs{}, err
	}
	return catalogInputs{
		workspace:       humanCatalog,
		embedded:        embeddedCatalog,
		providerConfig:  providerCatalog,
		workspaceReport: human.LoadReport(),
		workspaceInput:  input,
	}, nil
}

func loadHumanWorkspace(path string) (*catalogs.Builder, error) {
	if strings.TrimSpace(path) == "" {
		return catalogs.NewEmpty(), nil
	}
	builder, err := catalogs.NewFromPath(path)
	if err == nil {
		return builder, nil
	}
	if stderrors.Is(err, os.ErrNotExist) {
		return catalogs.NewEmpty(), nil
	}
	return nil, err
}

func composeProviderCatalog(
	embedded, human *catalogs.Catalog,
	humanExists bool,
) (*catalogs.Catalog, error) {
	if embedded == nil {
		return nil, &errors.ValidationError{
			Field:   "embedded_catalog",
			Message: "verified embedded catalog is required",
		}
	}
	builder, err := catalogs.NewBuilderFrom(embedded)
	if err != nil {
		return nil, errors.WrapResource("create", "provider configuration catalog", "", err)
	}
	if humanExists {
		if human == nil {
			return nil, &errors.ValidationError{
				Field:   "human_catalog",
				Message: "existing human workspace catalog is required",
			}
		}
		for _, author := range human.Authors().List() {
			if err := builder.SetAuthor(author); err != nil {
				return nil, errors.WrapResource(
					"set",
					"provider configuration author",
					author.ID.String(),
					err,
				)
			}
		}
		for _, authoredModel := range human.AuthoredModels() {
			if err := builder.SetAuthorModel(authoredModel.AuthorID, authoredModel.Model); err != nil {
				return nil, errors.WrapResource(
					"set",
					"provider configuration authored model",
					string(authoredModel.ID()),
					err,
				)
			}
		}
		for _, provider := range human.Providers().List() {
			if err := builder.SetProvider(provider); err != nil {
				return nil, errors.WrapResource(
					"set",
					"provider configuration",
					provider.ID.String(),
					err,
				)
			}
		}
	}
	catalog, err := builder.Build()
	if err != nil {
		return nil, errors.WrapResource("publish", "provider configuration catalog", "", err)
	}
	return catalog, nil
}
