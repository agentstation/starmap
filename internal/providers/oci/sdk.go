package oci

import (
	"context"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/generativeai"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

type sdkClient struct {
	client        generativeai.GenerativeAiClient
	compartmentID string
}

func newSDKClient(config Config, provider common.ConfigurationProvider) (API, error) {
	if provider == nil {
		return nil, &errors.AuthenticationError{Provider: string(ProviderID), Method: authMethodCloudChain, Message: "resolved OCI configuration provider is required", Err: errors.ErrAPIKeyRequired}
	}
	client, err := generativeai.NewGenerativeAiClientWithConfigurationProvider(provider)
	if err != nil {
		return nil, &errors.AuthenticationError{Provider: string(ProviderID), Method: authMethodCloudChain, Message: "OCI SDK configuration is unavailable", Err: err}
	}
	client.SetRegion(config.Region)
	return &sdkClient{client: client, compartmentID: config.CompartmentID}, nil
}

func (c *sdkClient) ListModels(ctx context.Context, cursor string) (sources.Page[Model], error) {
	request := generativeai.ListModelsRequest{CompartmentId: common.String(c.compartmentID), Limit: common.Int(1000)}
	if cursor != "" {
		request.Page = common.String(cursor)
	}
	response, err := c.client.ListModels(ctx, request)
	if err != nil {
		return sources.Page[Model]{}, &errors.APIError{Provider: string(ProviderID), Endpoint: "ListModels", Message: "OCI SDK request failed", Err: err}
	}
	models := make([]Model, 0, len(response.Items))
	for _, item := range response.Items {
		capabilities := make([]string, len(item.Capabilities))
		for index, capability := range item.Capabilities {
			capabilities[index] = string(capability)
		}
		model := Model{
			ID: stringValue(item.Id), DisplayName: stringValue(item.DisplayName), Vendor: stringValue(item.Vendor),
			Version: stringValue(item.Version), Capabilities: capabilities, LifecycleState: string(item.LifecycleState),
			Type: string(item.Type), BaseModelID: stringValue(item.BaseModelId),
		}
		if item.TimeDeprecated != nil {
			value := item.TimeDeprecated.Time
			model.TimeDeprecated = &value
		}
		if item.TimeOnDemandRetired != nil {
			value := item.TimeOnDemandRetired.Time
			model.TimeOnDemandRetired = &value
		}
		if item.TimeDedicatedRetired != nil {
			value := item.TimeDedicatedRetired.Time
			model.TimeDedicatedRetired = &value
		}
		models = append(models, model)
	}
	return sources.Page[Model]{Records: models, NextCursor: stringValue(response.OpcNextPage)}, nil
}

func (c *sdkClient) ListEndpoints(ctx context.Context, cursor string) (sources.Page[Endpoint], error) {
	request := generativeai.ListEndpointsRequest{CompartmentId: common.String(c.compartmentID), Limit: common.Int(1000)}
	if cursor != "" {
		request.Page = common.String(cursor)
	}
	response, err := c.client.ListEndpoints(ctx, request)
	if err != nil {
		return sources.Page[Endpoint]{}, &errors.APIError{Provider: string(ProviderID), Endpoint: "ListEndpoints", Message: "OCI SDK request failed", Err: err}
	}
	endpoints := make([]Endpoint, 0, len(response.Items))
	for _, item := range response.Items {
		endpoints = append(endpoints, Endpoint{
			ID: stringValue(item.Id), ModelID: stringValue(item.ModelId), DedicatedAIClusterID: stringValue(item.DedicatedAiClusterId),
			DisplayName: stringValue(item.DisplayName), LifecycleState: string(item.LifecycleState), PrivateEndpointID: stringValue(item.GenerativeAiPrivateEndpointId),
		})
	}
	return sources.Page[Endpoint]{Records: endpoints, NextCursor: stringValue(response.OpcNextPage)}, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
