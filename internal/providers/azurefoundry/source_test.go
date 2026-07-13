package azurefoundry

import (
	"context"
	stderrors "errors"
	"os"
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

func TestFetchSeparatesPublicModelsAndCustomerDeployments(t *testing.T) {
	api := &fakeAPI{
		modelPages:      map[string]sources.Page[Model]{"": {Records: []Model{testModel()}, NextCursor: "models-2"}, "models-2": {}},
		deploymentPages: map[string]sources.Page[Deployment]{"": {Records: []Deployment{{Name: "customer-chat", ModelName: "gpt-4.1", ModelFormat: "OpenAI", ModelVersion: "2025-04-14", SKUName: "GlobalStandard", ProvisioningState: "Succeeded"}}}},
	}
	source, err := NewSource(CommercialRealm(), testAccount(), api)
	if err != nil {
		t.Fatal(err)
	}
	source.now = func() time.Time { return time.Date(2026, 7, 12, 18, 0, 0, 0, time.UTC) }

	publicOnly, err := source.Fetch(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if api.modelCalls != 2 || api.deploymentCalls != 0 {
		t.Fatalf("calls models=%d deployments=%d", api.modelCalls, api.deploymentCalls)
	}
	if got := publicOnly.Offerings[0]; got.IsRoutable() || got.Access.Routability != catalogs.OfferingRoutabilityDiscoverable || got.Regions[0].Realm != "azure-public" {
		t.Fatalf("unsafe public offering: %#v", got)
	}

	withCustomer, err := source.Fetch(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(withCustomer.CustomerInventory) != 1 || len(withCustomer.CustomerInventory[0].Deployments) != 1 {
		t.Fatalf("customer inventory = %#v", withCustomer.CustomerInventory)
	}
	deployment := withCustomer.CustomerInventory[0].Deployments[0]
	if deployment.Aliases[0] != "customer-chat" || deployment.Endpoint != testAccount().Endpoint {
		t.Fatalf("deployment = %#v", deployment)
	}

	catalog, err := withCustomer.PublicCatalog()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatal(err)
	}
	for _, privateValue := range []string{"private-subscription", "private-rg", "private-account", "customer-chat"} {
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
	if _, err := source.Fetch(context.Background(), false); err != nil {
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
	_, err = source.Fetch(context.Background(), false)
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
	result, err := source.Fetch(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Offerings[0].Regions[0].Realm; got != "azure-us-government" {
		t.Fatalf("realm = %q", got)
	}
}

func TestMissingDefaultConfigurationDegradesWithoutClientCall(t *testing.T) {
	t.Setenv("AZURE_SUBSCRIPTION_ID", "")
	t.Setenv("AZURE_RESOURCE_GROUP", "")
	t.Setenv("AZURE_FOUNDRY_ACCOUNT", "")
	t.Setenv("AZURE_FOUNDRY_LOCATION", "")
	source := NewCommercialSource()
	called := false
	source.clientFunc = func(context.Context, Realm, Account) (API, error) {
		called = true
		return nil, stderrors.New("must not be called")
	}
	observation, err := source.Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if called || observation.Status != sources.ObservationStatusDegraded || observation.Issues[0].Code != sources.ObservationIssueCodeMissingCredentials {
		t.Fatalf("observation=%#v called=%v", observation, called)
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
	inventory, err := customerInventory(CommercialRealm(), testAccount(), time.Now().UTC(), []Deployment{{Name: "alias", ModelName: "mistral-large", ModelFormat: "OpenAI"}}, index)
	if err != nil {
		t.Fatal(err)
	}
	if got := inventory.Deployments[0]; got.ProviderModelID != "mistral-large@2502" || got.DefinitionID != "mistral-ai/mistral-large" {
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

func TestLiveCommercialAccount(t *testing.T) {
	if os.Getenv("STARMAP_LIVE_AZURE") != "1" {
		t.Skip("set STARMAP_LIVE_AZURE=1 with AZURE_* account configuration to run the credential-aware fixture")
	}
	source := NewCommercialSource()
	if err := validateConfig(source.realm, source.account); err != nil {
		t.Skipf("Azure account configuration unavailable: %v", err)
	}
	result, err := source.Fetch(context.Background(), false)
	if err != nil {
		var authenticationErr *errors.AuthenticationError
		if stderrors.As(err, &authenticationErr) {
			t.Skipf("Microsoft Entra default credential chain unavailable: %v", authenticationErr)
		}
		t.Fatal(err)
	}
	if len(result.Offerings) == 0 {
		t.Fatal("live Azure model inventory returned no offerings")
	}
	t.Logf("definitions=%d offerings=%d pricing_matched=%d", len(result.Definitions), len(result.Offerings), result.PricingMatched)
}
