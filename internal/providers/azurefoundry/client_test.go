package azurefoundry

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

type staticCredential struct{}

func (staticCredential) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: "fixture-token", ExpiresOn: time.Now().Add(time.Hour)}, nil
}

func TestARMClientUsesLocationModelsWireContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Errorf("authorization = %q", request.Header.Get("Authorization"))
		}
		if got, want := request.URL.Path, "/subscriptions/sub/providers/Microsoft.CognitiveServices/locations/eastus2/models"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		response.Header().Set("Content-Type", "application/json")
		fmt.Fprint(response, `{"value":[{"kind":"OpenAI","skuName":"S0","model":{"format":"OpenAI","publisher":"OpenAI","name":"gpt-4.1","version":"2025-04-14","isDefaultVersion":true,"lifecycleStatus":"GenerallyAvailable","skus":[{"name":"GlobalStandard","usageName":"OpenAI.GlobalStandard","capacity":{"maximum":1000}}]}}]}`)
	}))
	defer server.Close()

	realm := Realm{ID: "fixture", ARMEndpoint: server.URL, AuthorityHost: server.URL, ManagementScope: server.URL + "/.default"}
	account := Account{SubscriptionID: "sub", Location: "eastus2"}
	modelsURL, err := locationModelsURL(realm, account)
	if err != nil {
		t.Fatal(err)
	}
	client := &armClient{realm: realm, modelsURL: modelsURL, credential: staticCredential{}, http: server.Client()}
	page, err := client.ListModels(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Records) != 1 {
		t.Fatalf("records = %#v", page.Records)
	}
	model := page.Records[0]
	if model.Name != "gpt-4.1" || model.Format != "OpenAI" || model.Publisher != "OpenAI" || model.Version != "2025-04-14" || !model.IsDefaultVersion || model.LifecycleStatus != "GenerallyAvailable" || len(model.SKUs) != 1 || model.SKUs[0].MaxCapacity != 1000 {
		t.Fatalf("model = %#v", model)
	}
}

func TestLocationModelsURLDoesNotContainCustomerAccountIdentity(t *testing.T) {
	endpoint, err := locationModelsURL(CommercialRealm(), testAccount())
	if err != nil {
		t.Fatal(err)
	}
	want := "https://management.azure.com/subscriptions/private-subscription/providers/Microsoft.CognitiveServices/locations/eastus2/models?api-version=2024-10-01"
	if endpoint != want {
		t.Fatalf("endpoint = %q, want %q", endpoint, want)
	}
}

func TestARMClientUsesAccountDeploymentWireContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if got, want := request.URL.Path, "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.CognitiveServices/accounts/account/deployments"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		response.Header().Set("Content-Type", "application/json")
		fmt.Fprint(response, `{"value":[{"name":"chat-alias","sku":{"name":"DataZoneStandard"},"properties":{"provisioningState":"Succeeded","model":{"format":"OpenAI","name":"gpt-4.1","version":"2025-04-14"},"scaleSettings":{"scaleType":"Standard"}}}]}`)
	}))
	defer server.Close()

	realm := Realm{ID: "fixture", ARMEndpoint: server.URL, AuthorityHost: server.URL, ManagementScope: server.URL + "/.default"}
	account := Account{SubscriptionID: "sub", ResourceGroup: "rg", Name: "account", Location: "eastus2"}
	accountURL, err := accountResourceURL(realm, account)
	if err != nil {
		t.Fatal(err)
	}
	client := &armClient{realm: realm, accountURL: accountURL, credential: staticCredential{}, http: server.Client()}
	page, err := client.ListDeployments(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Records) != 1 {
		t.Fatalf("records = %#v", page.Records)
	}
	deployment := page.Records[0]
	if deployment.Name != "chat-alias" || deployment.ModelName != "gpt-4.1" || deployment.ModelVersion != "2025-04-14" || deployment.SKUName != "DataZoneStandard" || deployment.ProvisioningState != "Succeeded" {
		t.Fatalf("deployment = %#v", deployment)
	}
}

func TestGovernmentRealmUsesSeparateHostsAndAudience(t *testing.T) {
	realm := GovernmentRealm()
	if realm.ARMEndpoint != "https://management.usgovcloudapi.net" || realm.AuthorityHost != "https://login.microsoftonline.us/" || realm.ManagementScope != "https://management.usgovcloudapi.net/.default" {
		t.Fatalf("government realm = %#v", realm)
	}
}
