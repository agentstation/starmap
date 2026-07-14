package azurefoundry

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"

	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sourcepayload"
	"github.com/agentstation/starmap/pkg/sources"
)

type armClient struct {
	realm      Realm
	modelsURL  string
	accountURL string
	credential azcore.TokenCredential
	http       *http.Client
}

type modelListResponse struct {
	Value    []armModel `json:"value"`
	NextLink string     `json:"nextLink"`
}

type armModel struct {
	Model struct {
		Name             string `json:"name"`
		Format           string `json:"format"`
		Publisher        string `json:"publisher"`
		Version          string `json:"version"`
		IsDefaultVersion bool   `json:"isDefaultVersion"`
		Lifecycle        string `json:"lifecycleStatus"`
		SKUs             []struct {
			Name      string `json:"name"`
			UsageName string `json:"usageName"`
			Capacity  struct {
				Maximum int64 `json:"maximum"`
			} `json:"capacity"`
		} `json:"skus"`
	} `json:"model"`
}

type deploymentListResponse struct {
	Value    []armDeployment `json:"value"`
	NextLink string          `json:"nextLink"`
}

type armDeployment struct {
	Name string `json:"name"`
	SKU  struct {
		Name string `json:"name"`
	} `json:"sku"`
	Properties struct {
		ProvisioningState string `json:"provisioningState"`
		Model             struct {
			Name    string `json:"name"`
			Format  string `json:"format"`
			Version string `json:"version"`
		} `json:"model"`
		ScaleSettings struct {
			ScaleType string `json:"scaleType"`
		} `json:"scaleSettings"`
	} `json:"properties"`
}

func newClientWithCredential(realm Realm, account Account, credential azcore.TokenCredential) (API, error) {
	if credential == nil {
		return nil, &errors.AuthenticationError{Provider: string(ProviderID), Method: authMethodCloudChain, Message: "resolved Microsoft Entra credential is required", Err: errors.ErrAPIKeyRequired}
	}
	var err error
	accountURL := ""
	if validateCustomerAccount(account) == nil {
		accountURL, err = accountResourceURL(realm, account)
		if err != nil {
			return nil, err
		}
	}
	modelsURL, err := locationModelsURL(realm, account)
	if err != nil {
		return nil, err
	}
	return &armClient{realm: realm, modelsURL: modelsURL, accountURL: accountURL, credential: credential, http: &http.Client{Timeout: constants.DefaultHTTPTimeout}}, nil
}

func accountResourceURL(realm Realm, account Account) (string, error) {
	base, err := url.Parse(realm.ARMEndpoint)
	if err != nil {
		return "", errors.WrapParse("URL", "Azure ARM endpoint", err)
	}
	base.Path = path.Join(base.Path, "subscriptions", account.SubscriptionID, "resourceGroups", account.ResourceGroup, "providers", "Microsoft.CognitiveServices", "accounts", account.Name)
	return strings.TrimRight(base.String(), "/"), nil
}

func locationModelsURL(realm Realm, account Account) (string, error) {
	base, err := url.Parse(realm.ARMEndpoint)
	if err != nil {
		return "", errors.WrapParse("URL", "Azure ARM endpoint", err)
	}
	base.Path = path.Join(base.Path, "subscriptions", account.SubscriptionID, "providers", "Microsoft.CognitiveServices", "locations", account.Location, "models")
	query := base.Query()
	query.Set("api-version", modelsAPIVersion)
	base.RawQuery = query.Encode()
	return base.String(), nil
}

func (c *armClient) ListModels(ctx context.Context, cursor string) (sources.Page[Model], error) {
	var response modelListResponse
	if err := c.get(ctx, c.pageURL(cursor, "models"), &response); err != nil {
		return sources.Page[Model]{}, err
	}
	records := make([]Model, 0, len(response.Value))
	for _, value := range response.Value {
		model := Model{Name: value.Model.Name, Format: value.Model.Format, Publisher: value.Model.Publisher, Version: value.Model.Version, IsDefaultVersion: value.Model.IsDefaultVersion, LifecycleStatus: value.Model.Lifecycle}
		for _, sku := range value.Model.SKUs {
			model.SKUs = append(model.SKUs, ModelSKU{Name: sku.Name, UsageName: sku.UsageName, MaxCapacity: sku.Capacity.Maximum})
		}
		records = append(records, model)
	}
	return sources.Page[Model]{Records: records, NextCursor: response.NextLink}, nil
}

func (c *armClient) ListDeployments(ctx context.Context, cursor string) (sources.Page[Deployment], error) {
	var response deploymentListResponse
	if err := c.get(ctx, c.pageURL(cursor, "deployments"), &response); err != nil {
		return sources.Page[Deployment]{}, err
	}
	records := make([]Deployment, 0, len(response.Value))
	for _, value := range response.Value {
		records = append(records, Deployment{Name: value.Name, ModelName: value.Properties.Model.Name, ModelFormat: value.Properties.Model.Format, ModelVersion: value.Properties.Model.Version, SKUName: value.SKU.Name, ScaleType: value.Properties.ScaleSettings.ScaleType, ProvisioningState: value.Properties.ProvisioningState})
	}
	return sources.Page[Deployment]{Records: records, NextCursor: response.NextLink}, nil
}

func (c *armClient) pageURL(cursor, collection string) string {
	if cursor != "" {
		return cursor
	}
	if collection == "models" {
		return c.modelsURL
	}
	return c.accountURL + "/" + collection + "?api-version=" + modelsAPIVersion
}

func (c *armClient) get(ctx context.Context, endpoint string, target any) error {
	if !strings.HasPrefix(endpoint, strings.TrimRight(c.realm.ARMEndpoint, "/")+"/") {
		return &errors.ValidationError{Field: "azure_foundry.next_link", Value: endpoint, Message: "must remain inside the configured ARM realm"}
	}
	token, err := c.credential.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{c.realm.ManagementScope}})
	if err != nil {
		return &errors.AuthenticationError{Provider: string(ProviderID), Method: "default_azure_credential", Message: "Microsoft Entra token is unavailable", Err: err}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return errors.WrapResource("create", "Azure ARM request", endpoint, err)
	}
	request.Header.Set("Authorization", "Bearer "+token.Token)
	response, err := c.http.Do(request)
	if err != nil {
		return errors.WrapResource("fetch", "Azure ARM resource", endpoint, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return &errors.APIError{Provider: string(ProviderID), Endpoint: endpoint, StatusCode: response.StatusCode, Message: "Azure ARM request failed"}
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, constants.MaxSourcePayloadBytes+1))
	if err != nil {
		return errors.WrapIO("read", "Azure ARM response", err)
	}
	if err := sourcepayload.ValidateJSON(payload); err != nil {
		return err
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return errors.WrapParse("json", "Azure ARM response", err)
	}
	return nil
}
