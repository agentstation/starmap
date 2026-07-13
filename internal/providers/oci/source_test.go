package oci

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

type fakeAPI struct {
	mu            sync.Mutex
	modelPages    map[string]sources.Page[Model]
	endpointPages map[string]sources.Page[Endpoint]
	modelErr      error
	endpointErr   error
	modelCursors  []string
	endpointCalls int
}

func (f *fakeAPI) ListModels(_ context.Context, cursor string) (sources.Page[Model], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.modelCursors = append(f.modelCursors, cursor)
	return f.modelPages[cursor], f.modelErr
}

func (f *fakeAPI) ListEndpoints(_ context.Context, cursor string) (sources.Page[Endpoint], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.endpointCalls++
	return f.endpointPages[cursor], f.endpointErr
}

func TestFetchSeparatesRegionalBaseModelsAndCustomerEndpoints(t *testing.T) {
	fixed := time.Date(2026, 7, 12, 20, 45, 0, 0, time.UTC)
	api := &fakeAPI{
		modelPages: map[string]sources.Page[Model]{
			"":       {Records: []Model{{ID: "cohere.command-a", DisplayName: "Command A", Vendor: "Cohere", Version: "1", Capabilities: []string{"CHAT", "TEXT_EMBEDDINGS"}, LifecycleState: "ACTIVE", Type: "BASE"}}, NextCursor: "page-2"},
			"page-2": {Records: []Model{{ID: "ocid1.generativeaimodel.oc1..private", DisplayName: "Fine Tune", Vendor: "Cohere", Capabilities: []string{"CHAT"}, LifecycleState: "ACTIVE", Type: "CUSTOM", BaseModelID: "cohere.command-a"}}},
		},
		endpointPages: map[string]sources.Page[Endpoint]{"": {Records: []Endpoint{{ID: "ocid1.generativeaiendpoint.oc1..private", ModelID: "ocid1.generativeaimodel.oc1..private", DedicatedAIClusterID: "ocid1.generativeaidedicatedaicluster.oc1..private", DisplayName: "customer-command", LifecycleState: "ACTIVE", PrivateEndpointID: "ocid1.generativeaiprivateendpoint.oc1..private"}}}},
	}
	source, err := NewSource(Config{Region: "us-chicago-1", Realm: "oc1", CompartmentID: "ocid1.compartment.oc1..private"}, api)
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	source.now = func() time.Time { return fixed }
	result, err := source.Fetch(context.Background(), true)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(result.Definitions) != 1 || len(result.Offerings) != 1 || len(result.CustomerInventory) != 1 {
		t.Fatalf("result = %#v", result)
	}
	offering := result.Offerings[0]
	if offering.DefinitionID != "cohere/command-a" || offering.Regions[0].ID != "us-chicago-1" || offering.Regions[0].Realm != "oc1" || offering.Deployment.Type != "on-demand" {
		t.Fatalf("offering = %#v", offering)
	}
	if !containsAPI(offering.Access.APIs, catalogs.InvocationAPIOCIInference) || !containsAPI(offering.Access.APIs, catalogs.InvocationAPIEmbeddings) || offering.Endpoint.BaseURL != "https://inference.generativeai.us-chicago-1.oci.oraclecloud.com" {
		t.Fatalf("invocation = %#v/%#v", offering.Access, offering.Endpoint)
	}
	inventory := result.CustomerInventory[0]
	if inventory.Scope.AccountID != "ocid1.compartment.oc1..private" || inventory.Deployments[0].Deployment.Type != "dedicated-ai-cluster" || inventory.Deployments[0].DefinitionID != "oci-private/ocid1-generativeaimodel-oc1--private" {
		t.Fatalf("inventory = %#v", inventory)
	}
	public, err := result.PublicCatalog()
	if err != nil {
		t.Fatalf("PublicCatalog: %v", err)
	}
	payload, err := catalogs.EncodeCatalogPayload(public)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	for _, private := range []string{"ocid1.compartment.oc1..private", "ocid1.generativeaiendpoint.oc1..private", "customer-command"} {
		if strings.Contains(string(payload), private) {
			t.Fatalf("private customer identity leaked into public catalog: %s", payload)
		}
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.modelCursors) != 2 || api.modelCursors[0] != "" || api.modelCursors[1] != "page-2" || api.endpointCalls != 1 {
		t.Fatalf("pagination/calls = %#v/%d", api.modelCursors, api.endpointCalls)
	}
}

func TestOpenAICompatibleModelsUseExactResponsesBaseURL(t *testing.T) {
	result, _, err := recordsFromModels(Config{Region: "us-chicago-1", Realm: "oc1", CompartmentID: "compartment"}, time.Now(), []Model{{ID: "openai.gpt-oss-120b", DisplayName: "GPT OSS", Vendor: "OpenAI", Capabilities: []string{"CHAT"}, LifecycleState: "ACTIVE", Type: "BASE"}})
	if err != nil {
		t.Fatalf("recordsFromModels: %v", err)
	}
	offering := result.Offerings[0]
	if !slices.Equal(offering.Access.APIs, []catalogs.InvocationAPI{catalogs.InvocationAPIResponses}) || offering.Endpoint.BaseURL != "https://inference.generativeai.us-chicago-1.oci.oraclecloud.com/openai/v1" {
		t.Fatalf("OpenAI-compatible offering = %#v", offering)
	}
}

func TestFetchWithoutCustomerInventoryNeverListsEndpoints(t *testing.T) {
	api := &fakeAPI{modelPages: map[string]sources.Page[Model]{"": {Records: []Model{}}}, endpointPages: map[string]sources.Page[Endpoint]{}}
	source, err := NewSource(Config{Region: "eu-frankfurt-1", Realm: "oc1", CompartmentID: "compartment"}, api)
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if _, err := source.Fetch(context.Background(), false); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if api.endpointCalls != 0 {
		t.Fatalf("private endpoints called %d times", api.endpointCalls)
	}
}

func TestFetchRejectsDuplicateAndUnmappedRecords(t *testing.T) {
	base := Model{ID: "cohere.command", DisplayName: "Command", Vendor: "Cohere", LifecycleState: "ACTIVE", Type: "BASE"}
	if _, _, err := recordsFromModels(Config{Region: "us-chicago-1", Realm: "oc1", CompartmentID: "compartment"}, time.Now(), []Model{base, base}); err == nil {
		t.Fatal("expected duplicate model failure")
	}
	_, err := customerInventory(Config{Region: "us-chicago-1", Realm: "oc1", CompartmentID: "compartment"}, time.Now(), []Endpoint{{ID: "endpoint", ModelID: "missing", DedicatedAIClusterID: "cluster"}}, map[string]catalogs.ModelDefinitionID{})
	if err == nil {
		t.Fatal("expected unmapped endpoint failure")
	}
}

func TestObserveMissingConfigurationIsDegradedAndSecretSafe(t *testing.T) {
	source := newSource(Config{Realm: "oc1"}, nil)
	observation, err := source.Observe(context.Background())
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if observation.Status != sources.ObservationStatusDegraded || len(observation.Issues) != 1 || observation.Issues[0].Code != sources.ObservationIssueCodeMissingCredentials {
		t.Fatalf("observation = %#v", observation)
	}
	payload, err := json.Marshal(observation)
	if err != nil {
		t.Fatalf("marshal observation: %v", err)
	}
	if strings.Contains(string(payload), "private-key") {
		t.Fatalf("secret leaked: %s", payload)
	}
}

func TestFetchPropagatesNativeFailures(t *testing.T) {
	source, err := NewSource(Config{Region: "us-chicago-1", Realm: "oc1", CompartmentID: "compartment"}, &fakeAPI{modelErr: errors.New("terminal OCI failure")})
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if _, err := source.Fetch(context.Background(), false); err == nil {
		t.Fatal("expected native failure")
	}
}

func containsAPI(values []catalogs.InvocationAPI, target catalogs.InvocationAPI) bool {
	return slices.Contains(values, target)
}
