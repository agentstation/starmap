package reconciler

import (
	stderrors "errors"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestFilterCatalogToBaselineProvidersReturnsSetProviderError(t *testing.T) {
	baseline := catalogs.NewEmpty()
	if err := baseline.SetProvider(catalogs.Provider{
		ID:      "openai",
		Name:    "OpenAI",
		Aliases: []catalogs.ProviderID{"open-ai"},
	}); err != nil {
		t.Fatalf("SetProvider baseline: %v", err)
	}

	sourceCatalog := catalogs.NewEmpty()
	if err := sourceCatalog.SetProvider(catalogs.Provider{
		ID:   "open-ai",
		Name: "OpenAI from models.dev",
	}); err != nil {
		t.Fatalf("SetProvider source: %v", err)
	}

	setErr := stderrors.New("set provider failed")
	err := setBaselineProviders(
		&setProviderFailingCatalog{
			Builder: catalogs.NewEmpty(),
			err:     setErr,
		},
		sourceCatalog,
		baseline,
		nil,
	)
	if !stderrors.Is(err, setErr) {
		t.Fatalf("expected wrapped SetProvider error, got %v", err)
	}
	if !strings.Contains(err.Error(), "openai") {
		t.Fatalf("expected error to include baseline provider ID, got %v", err)
	}
}

type setProviderFailingCatalog struct {
	*catalogs.Builder
	err error
}

func (c *setProviderFailingCatalog) SetProvider(catalogs.Provider) error {
	return c.err
}
