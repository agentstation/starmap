package azurefoundry

import (
	"context"
	stderrors "errors"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

type fakeAPI struct {
	modelPages      map[string]sources.Page[Model]
	deploymentPages map[string]sources.Page[Deployment]
	modelCalls      int
	deploymentCalls int
	failModels      int
}

func (f *fakeAPI) ListModels(_ context.Context, cursor string) (sources.Page[Model], error) {
	f.modelCalls++
	if f.failModels > 0 {
		f.failModels--
		return sources.Page[Model]{}, &errors.APIError{Provider: string(ProviderID), StatusCode: 429, Message: "bounded fixture"}
	}
	return f.modelPages[cursor], nil
}

func (f *fakeAPI) ListDeployments(_ context.Context, cursor string) (sources.Page[Deployment], error) {
	f.deploymentCalls++
	return f.deploymentPages[cursor], nil
}

func testAccount() Account {
	return Account{SubscriptionID: "private-subscription", ResourceGroup: "private-rg", Name: "private-account", Location: "eastus2", Endpoint: "https://private-account.openai.azure.com"}
}

func testModel() Model {
	return Model{Name: "gpt-4.1", Format: "OpenAI", Publisher: "OpenAI", Version: "2025-04-14", IsDefaultVersion: true, LifecycleStatus: "GenerallyAvailable", SKUs: []ModelSKU{{Name: "GlobalStandard", MaxCapacity: 100}}}
}

func TestFetchNormalizesContextualDeploymentsIntoCanonicalOfferings(t *testing.T) {
	api := &fakeAPI{
		modelPages:      map[string]sources.Page[Model]{"": {Records: []Model{testModel()}, NextCursor: "models-2"}, "models-2": {}},
		deploymentPages: map[string]sources.Page[Deployment]{"": {Records: []Deployment{{Name: "customer-chat", ModelName: "gpt-4.1", ModelFormat: "OpenAI", ModelVersion: "2025-04-14", SKUName: "GlobalStandard", ProvisioningState: "Succeeded"}}}},
	}
	source, err := NewSource(CommercialRealm(), testAccount(), api)
	if err != nil {
		t.Fatal(err)
	}
	source.now = func() time.Time { return time.Date(2026, 7, 12, 18, 0, 0, 0, time.UTC) }

	withContext, err := source.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if api.modelCalls != 2 || api.deploymentCalls != 1 {
		t.Fatalf("calls models=%d deployments=%d", api.modelCalls, api.deploymentCalls)
	}
	if got := withContext.Offerings[0]; got.IsRoutable() || got.Access.Routability != catalogs.OfferingRoutabilityDiscoverable || got.Regions[0].Realm != "azure-public" {
		t.Fatalf("unsafe public offering: %#v", got)
	}
	if len(withContext.Offerings) != 2 {
		t.Fatalf("contextual offerings = %#v", withContext.Offerings)
	}
	deployment := withContext.Offerings[1]
	if deployment.Aliases[0] != "customer-chat" || deployment.DeploymentID != "customer-chat" || deployment.Endpoint.BaseURL != testAccount().Endpoint {
		t.Fatalf("deployment = %#v", deployment)
	}

	catalog, err := withContext.Catalog()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatal(err)
	}
	for _, privateValue := range []string{"private-subscription", "private-rg"} {
		if strings.Contains(string(payload), privateValue) {
			t.Fatalf("public catalog contains %q", privateValue)
		}
	}
}

func TestFetchUsesBoundedRetryAndRejectsRepeatedCursor(t *testing.T) {
	api := &fakeAPI{failModels: 1, modelPages: map[string]sources.Page[Model]{"": {Records: []Model{testModel()}}}}
	source, err := NewSource(CommercialRealm(), testAccount(), api)
	if err != nil {
		t.Fatal(err)
	}
	source.retry = sources.ProviderRetryPolicy{MaxAttempts: 2, BaseDelay: time.Nanosecond, MaxDelay: time.Nanosecond}
	if _, err := source.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if api.modelCalls != 2 {
		t.Fatalf("model calls = %d", api.modelCalls)
	}

	api = &fakeAPI{modelPages: map[string]sources.Page[Model]{"": {NextCursor: "repeat"}, "repeat": {NextCursor: "repeat"}}}
	source, err = NewSource(CommercialRealm(), testAccount(), api)
	if err != nil {
		t.Fatal(err)
	}
	_, err = source.Fetch(context.Background())
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("expected typed cursor conflict, got %v", err)
	}
}

func TestGovernmentRealmRemainsDistinct(t *testing.T) {
	api := &fakeAPI{modelPages: map[string]sources.Page[Model]{"": {Records: []Model{testModel()}}}}
	source, err := NewSource(GovernmentRealm(), testAccount(), api)
	if err != nil {
		t.Fatal(err)
	}
	result, err := source.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Offerings[0].Regions[0].Realm; got != "azure-us-government" {
		t.Fatalf("realm = %q", got)
	}
}

func TestMissingResolvedConfigurationDegradesWithoutClientCall(t *testing.T) {
	source := newSource(CommercialRealm(), Account{}, nil)
	observation, err := source.Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if observation.Status != sources.ObservationStatusDegraded || observation.Issues[0].Code != sources.ObservationIssueCodeMissingCredentials {
		t.Fatalf("observation=%#v", observation)
	}
}

func TestRecordsRejectDuplicateOfferingIdentity(t *testing.T) {
	_, _, err := recordsFromModels(CommercialRealm(), testAccount(), []Model{testModel(), testModel()})
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestPublisherOwnsDefinitionAndDefaultVersionResolvesDeployment(t *testing.T) {
	model := Model{Name: "mistral-large", Format: "OpenAI", Publisher: "Mistral AI", Version: "2502", IsDefaultVersion: true, LifecycleStatus: "GenerallyAvailable"}
	result, index, err := recordsFromModels(CommercialRealm(), testAccount(), []Model{model})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Definitions[0]; got.ID != "mistral-ai/mistral-large" || got.AuthorIDs[0] != "mistral-ai" {
		t.Fatalf("definition = %#v", got)
	}
	offerings, err := deploymentOfferings(CommercialRealm(), testAccount(), []Deployment{{Name: "alias", ModelName: "mistral-large", ModelFormat: "OpenAI", ProvisioningState: "Succeeded"}}, index)
	if err != nil {
		t.Fatal(err)
	}
	if got := offerings[0]; got.ProviderModelID != "mistral-large@2502" || got.DefinitionID != "mistral-ai/mistral-large" {
		t.Fatalf("deployment = %#v", got)
	}
}

func TestRecordsRejectMultipleDefaultVersions(t *testing.T) {
	first := testModel()
	second := first
	second.Version = "2026-01-01"
	_, _, err := recordsFromModels(CommercialRealm(), testAccount(), []Model{first, second})
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("expected default-version conflict, got %v", err)
	}
}
