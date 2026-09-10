package pipeline

import (
	"context"
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
	baseline        *catalogs.Catalog
	providerConfig  *catalogs.Catalog
	workspaceReport catalogs.LoadReport
	workspaceInput  workspace.InputExpectation
}

func (p *Pipeline) loadCatalogInputs(ctx context.Context, path string, baseline *catalogs.Catalog) (catalogInputs, error) {
	var inputs catalogInputs
	err := workspace.Read(ctx, path, func(input workspace.InputExpectation) error {
		human, err := p.loadWorkspace(path)
		if err != nil {
			return errors.WrapResource("load", "catalog", "human workspace", err)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		inputs.embedded, err = p.loadEmbedded()
		if err != nil {
			return errors.WrapResource("load", "catalog", "embedded", err)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		humanCatalog, err := human.Build()
		if err != nil {
			return errors.WrapResource("publish", "human workspace catalog", "", err)
		}
		input, err = workspace.BindInputCatalog(input, humanCatalog)
		if err != nil {
			return err
		}
		inputs.workspace = humanCatalog
		inputs.workspaceReport = human.LoadReport()
		inputs.workspaceInput = input
		return nil
	})
	if err != nil {
		return catalogInputs{}, err
	}
	if err := ctx.Err(); err != nil {
		return catalogInputs{}, err
	}
	if baseline == nil {
		baseline = inputs.embedded
	}
	inputs.baseline = baseline
	inputs.providerConfig, err = composeProviderCatalog(baseline, inputs.workspace, inputs.workspaceInput.Exists)
	if err != nil {
		return catalogInputs{}, err
	}
	return inputs, nil
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
	baseline, human *catalogs.Catalog,
	humanExists bool,
) (*catalogs.Catalog, error) {
	if baseline == nil {
		return nil, &errors.ValidationError{
			Field:   "acquisition_baseline",
			Message: "accepted catalog is required",
		}
	}
	if !humanExists {
		return baseline, nil
	}
	builder, err := catalogs.NewBuilderFrom(baseline)
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

// metadataProviderRegistry resolves metadata filters without enabling provider acquisition.
func metadataProviderRegistry(inputs catalogInputs, filter *catalogs.ProviderID) catalogs.ProvidersReader {
	configured := inputs.providerConfig.Providers()
	if filter != nil && !configured.Exists(*filter) && inputs.embedded != nil {
		return inputs.embedded.Providers()
	}
	return configured
}
