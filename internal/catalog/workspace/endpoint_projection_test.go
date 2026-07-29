package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/save"
)

func TestEndpointProjectionDeterministicallyJoinsCanonicalModelToOfferings(t *testing.T) {
	t.Parallel()

	catalog, identity := endpointProjectionCatalog(t)
	first, err := EncodeEndpointProjection(catalog, identity)
	if err != nil {
		t.Fatalf("encode first: %v", err)
	}
	second, err := EncodeEndpointProjection(catalog, identity)
	if err != nil {
		t.Fatalf("encode second: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("identical catalog and generation produced different endpoint bytes")
	}
	if !bytes.Contains(first, []byte("service_tier: priority")) ||
		bytes.Contains(first, []byte("- 34")) {
		t.Fatalf("request body is not native readable YAML:\n%s", first)
	}
	for _, forbidden := range [][]byte{
		[]byte("latency"), []byte("throughput"), []byte("uptime"),
	} {
		if bytes.Contains(bytes.ToLower(first), forbidden) {
			t.Fatalf("endpoint projection invents runtime telemetry %q", forbidden)
		}
	}

	var document endpointProjection
	if err := yaml.Unmarshal(first, &document); err != nil {
		t.Fatalf("decode projection: %v", err)
	}
	if document.SchemaVersion != endpointProjectionSchemaVersion ||
		document.GenerationID != identity.GenerationID ||
		document.CatalogDigest != identity.PayloadChecksum {
		t.Fatalf("projection identity = %#v, want %#v", document, identity)
	}
	if len(document.Models) != 1 || document.Models[0].Model != "moonshot-ai/kimi-k2.5" {
		t.Fatalf("projection models = %#v, want one Kimi model", document.Models)
	}
	rows := document.Models[0].Endpoints
	if len(rows) != 2 ||
		rows[0].ProviderID != "alibaba" ||
		rows[1].ProviderID != "deepinfra" {
		t.Fatalf("endpoint ordering = %#v, want Alibaba then DeepInfra", rows)
	}
	if rows[0].Pricing.Tokens.Input.Per1M != 1 ||
		rows[1].Pricing.Tokens.Input.Per1M != 2 {
		t.Fatalf("endpoint prices = %#v, want exact provider prices", rows)
	}
	var reasoning map[string]string
	if err := json.Unmarshal(rows[0].Modes["pro"].Request.Body["reasoning"], &reasoning); err != nil {
		t.Fatalf("decode nested reasoning request: %v", err)
	}
	if reasoning["mode"] != "pro" {
		t.Fatalf("nested reasoning request = %#v, want mode pro", reasoning)
	}
}

func TestEndpointProjectionIsStableAcrossWorkspaceRoundTrip(t *testing.T) {
	t.Parallel()

	catalog, identity := endpointProjectionCatalog(t)
	before, err := EncodeEndpointProjection(catalog, identity)
	if err != nil {
		t.Fatalf("EncodeEndpointProjection before: %v", err)
	}
	path := filepath.Join(t.TempDir(), "catalog")
	builder, err := catalogs.NewBuilderFrom(catalog)
	if err != nil {
		t.Fatalf("NewBuilderFrom: %v", err)
	}
	if err := builder.Save(save.WithPath(path)); err != nil {
		t.Fatalf("Save: %v", err)
	}
	reloadedBuilder, err := catalogs.NewFromPath(path)
	if err != nil {
		t.Fatalf("NewFromPath: %v", err)
	}
	reloaded, err := reloadedBuilder.Build()
	if err != nil {
		t.Fatalf("Build reloaded: %v", err)
	}
	after, err := EncodeEndpointProjection(reloaded, identity)
	if err != nil {
		t.Fatalf("EncodeEndpointProjection after: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("endpoint projection changed across workspace round trip\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestEndpointProjectionDriftIsDetectedWithoutOverwrite(t *testing.T) {
	t.Parallel()

	catalog, identity := endpointProjectionCatalog(t)
	path := filepath.Join(t.TempDir(), "catalog")
	receipt, err := Project(context.Background(), path, catalog, identity)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if receipt.EndpointChecksum == "" {
		t.Fatal("Project returned an empty endpoint checksum")
	}
	endpointsPath := filepath.Join(path, endpointProjectionFilename)
	original, err := os.ReadFile(endpointsPath)
	if err != nil {
		t.Fatalf("read endpoints: %v", err)
	}
	edited := append(append([]byte(nil), original...), []byte("# operator edit\n")...)
	if err := os.WriteFile(endpointsPath, edited, 0o644); err != nil {
		t.Fatalf("edit endpoints: %v", err)
	}

	result, err := Repair(context.Background(), path, catalog, identity)
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}
	if result.Status != RepairStatusSkippedDirty || result.IssueCode != IssueDirty {
		t.Fatalf("Repair result = %#v, want dirty skip", result)
	}
	after, err := os.ReadFile(endpointsPath)
	if err != nil {
		t.Fatalf("reread endpoints: %v", err)
	}
	if !bytes.Equal(after, edited) {
		t.Fatal("dirty endpoint projection was silently overwritten")
	}
}

func TestEndpointProjectionEditBeforeProjectExpectedIsPreserved(t *testing.T) {
	t.Parallel()

	catalog, identity := endpointProjectionCatalog(t)
	path := filepath.Join(t.TempDir(), "catalog")
	if _, err := Project(context.Background(), path, catalog, identity); err != nil {
		t.Fatalf("Project initial: %v", err)
	}
	input, err := ObserveInput(path)
	if err != nil {
		t.Fatalf("ObserveInput: %v", err)
	}
	input, err = BindInputCatalog(input, catalog)
	if err != nil {
		t.Fatalf("BindInputCatalog: %v", err)
	}

	endpointsPath := filepath.Join(path, endpointProjectionFilename)
	original, err := os.ReadFile(endpointsPath)
	if err != nil {
		t.Fatalf("read endpoints: %v", err)
	}
	edited := append(append([]byte(nil), original...), []byte("# operator edit\n")...)
	if err := os.WriteFile(endpointsPath, edited, 0o644); err != nil {
		t.Fatalf("edit endpoints: %v", err)
	}

	if _, err := ProjectExpected(
		context.Background(),
		path,
		catalog,
		identity,
		input,
	); err == nil {
		t.Fatal("ProjectExpected accepted endpoint projection drift")
	}
	after, err := os.ReadFile(endpointsPath)
	if err != nil {
		t.Fatalf("reread endpoints: %v", err)
	}
	if !bytes.Equal(after, edited) {
		t.Fatal("edited endpoint projection was overwritten")
	}
}

func endpointProjectionCatalog(t testing.TB) (*catalogs.Catalog, Identity) {
	t.Helper()
	builder := catalogs.NewEmpty()
	for _, author := range []catalogs.Author{
		{ID: "moonshot-ai", Name: "Moonshot AI"},
		{ID: "alibaba", Name: "Alibaba"},
	} {
		if err := builder.SetAuthor(author); err != nil {
			t.Fatalf("SetAuthor(%s): %v", author.ID, err)
		}
	}
	if err := builder.SetAuthorModel("moonshot-ai", catalogs.Model{
		ID: "kimi-k2.5", Name: "Kimi K2.5",
		Authors: []catalogs.Author{{ID: "moonshot-ai", Name: "Moonshot AI"}},
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}
	for _, provider := range []catalogs.Provider{
		{
			ID: "deepinfra", Name: "DeepInfra",
			Models: map[string]*catalogs.Model{
				"moonshotai/Kimi-K2.5": endpointProjectionProviderModel(
					"moonshotai/Kimi-K2.5", 2,
				),
			},
		},
		{
			ID: "alibaba", Name: "Alibaba Cloud",
			Models: map[string]*catalogs.Model{
				"kimi-k2.5": endpointProjectionProviderModel("kimi-k2.5", 1),
			},
		},
	} {
		if err := builder.SetProvider(provider); err != nil {
			t.Fatalf("SetProvider(%s): %v", provider.ID, err)
		}
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	return catalog, Identity{
		GenerationID:    "generation-endpoint-test",
		PayloadChecksum: catalogs.DescribeCatalogPayload(payload).Checksum,
	}
}

func endpointProjectionProviderModel(id string, price float64) *catalogs.Model {
	return &catalogs.Model{
		ID: id, ModelRef: "moonshot-ai/kimi-k2.5", Name: "Kimi K2.5",
		Pricing: &catalogs.ModelPricing{
			Currency: catalogs.ModelPricingCurrencyUSD,
			Tokens: &catalogs.ModelTokenPricing{
				Input: &catalogs.ModelTokenCost{Per1M: price},
			},
		},
		Modes: map[string]catalogs.ModelMode{
			"fast": {
				Provider: &catalogs.ModelProviderMode{
					Body: map[string]any{
						"service_tier": json.RawMessage(`"priority"`),
					},
				},
			},
			"pro": {
				Provider: &catalogs.ModelProviderMode{
					Body: map[string]any{
						"reasoning": json.RawMessage(`{"mode":"pro"}`),
					},
				},
			},
		},
	}
}
