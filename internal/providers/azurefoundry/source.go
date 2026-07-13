// Package azurefoundry discovers Microsoft Foundry and Azure OpenAI account
// model availability while isolating deployments in customer inventory.
package azurefoundry

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// ProviderID is the canonical Microsoft Foundry service-channel identity.
const ProviderID catalogs.ProviderID = "microsoft-foundry"

const (
	defaultMaxPages   = 32
	defaultMaxRecords = 10000
	modelsAPIVersion  = "2024-10-01"
)

// Realm defines one Azure control-plane and identity boundary.
type Realm struct {
	ID              string
	ARMEndpoint     string
	AuthorityHost   string
	ManagementScope string
}

var (
	commercialRealm = Realm{ID: "azure-public", ARMEndpoint: "https://management.azure.com", AuthorityHost: "https://login.microsoftonline.com/", ManagementScope: "https://management.azure.com/.default"}
	governmentRealm = Realm{ID: "azure-us-government", ARMEndpoint: "https://management.usgovcloudapi.net", AuthorityHost: "https://login.microsoftonline.us/", ManagementScope: "https://management.usgovcloudapi.net/.default"}
)

// CommercialRealm returns the public Azure realm configuration.
func CommercialRealm() Realm { return commercialRealm }

// GovernmentRealm returns the physically isolated Azure Government realm.
func GovernmentRealm() Realm { return governmentRealm }

// Account identifies the credential-scoped ARM resource used for discovery.
type Account struct {
	SubscriptionID string
	ResourceGroup  string
	Name           string
	Location       string
	Endpoint       string
}

// Model is the account model-list subset used by Starmap.
type Model struct {
	Name             string
	Format           string
	Publisher        string
	Version          string
	IsDefaultVersion bool
	LifecycleStatus  string
	SKUs             []ModelSKU
}

// ModelSKU is one Azure deployment SKU advertised for a model.
type ModelSKU struct {
	Name        string
	UsageName   string
	MaxCapacity int64
}

// Deployment is the private account deployment-list subset used by Starmap.
type Deployment struct {
	Name              string
	ModelName         string
	ModelFormat       string
	ModelVersion      string
	SKUName           string
	ScaleType         string
	ProvisioningState string
}

type modelReference struct {
	DefinitionID    catalogs.ModelDefinitionID
	ProviderModelID catalogs.ProviderModelID
}

// API is the bounded native Azure resource-manager surface used by Source.
type API interface {
	ListModels(context.Context, string) (sources.Page[Model], error)
	ListDeployments(context.Context, string) (sources.Page[Deployment], error)
}

// Result separates sanitized service offerings from private account state.
type Result struct {
	Definitions       []catalogs.ModelDefinition
	Offerings         []catalogs.ProviderOffering
	CustomerInventory []catalogs.CustomerInventory
	PricingObservedAt *time.Time
	PricingMatched    int
	PricingIgnored    int
}

// Source observes one explicitly configured Foundry account.
type Source struct {
	realm      Realm
	account    Account
	client     API
	clientFunc clientFactory
	retry      sources.ProviderRetryPolicy
	pagination sources.PaginationPolicy
	pricing    pricingFetcher
	now        func() time.Time
}

var _ sources.Source = (*Source)(nil)

// NewSource constructs an injected account source.
func NewSource(realm Realm, account Account, client API) (*Source, error) {
	if err := validateConfig(realm, account); err != nil {
		return nil, err
	}
	if client == nil {
		return nil, &errors.ValidationError{Field: "azure_foundry.client", Message: "is required"}
	}
	return newSource(realm, account, client), nil
}

func newSource(realm Realm, account Account, client API) *Source {
	return &Source{realm: realm, account: account, client: client, retry: sources.DefaultProviderRetryPolicy(), pagination: sources.PaginationPolicy{MaxPages: defaultMaxPages, MaxRecords: defaultMaxRecords}, now: func() time.Time { return time.Now().UTC() }}
}

// NewCommercialSource constructs the optional default source from AZURE_* environment configuration.
func NewCommercialSource() *Source {
	source := newSource(CommercialRealm(), Account{
		SubscriptionID: os.Getenv("AZURE_SUBSCRIPTION_ID"), ResourceGroup: os.Getenv("AZURE_RESOURCE_GROUP"),
		Name: os.Getenv("AZURE_FOUNDRY_ACCOUNT"), Location: os.Getenv("AZURE_FOUNDRY_LOCATION"), Endpoint: os.Getenv("AZURE_FOUNDRY_ENDPOINT"),
	}, nil)
	source.clientFunc = newDefaultClient
	source.pricing = newHTTPPricingFetcher()
	return source
}

// ID returns the stable Microsoft source identity.
func (s *Source) ID() sources.ID { return sources.MicrosoftFoundryID }

// Name returns the operator-facing source name.
func (s *Source) Name() string { return "Microsoft Foundry and Azure OpenAI" }

// Observe emits only sanitized public-catalog records.
func (s *Source) Observe(ctx context.Context, _ ...sources.Option) (sources.Observation, error) {
	result, fetchErr := s.Fetch(ctx, false)
	if fetchErr != nil {
		catalog, err := emptyPublicCatalog()
		if err != nil {
			return sources.Observation{}, err
		}
		code := sources.ObservationIssueCodeFetchFailed
		var authenticationErr *errors.AuthenticationError
		if stderrors.As(fetchErr, &authenticationErr) {
			code = sources.ObservationIssueCodeMissingCredentials
		}
		return sources.NewObservation(s.ID(), catalog, sources.ObservationMetadata{
			ObservedAt: s.now(), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
			Completeness: sources.ObservationCompletenessPartial, Status: sources.ObservationStatusDegraded,
			Scope: catalogmeta.ObservationScopeRegionalPublic, Kind: catalogmeta.SourceKindRegionalSweep,
			Coverage: catalogmeta.ProviderCoverage{Expected: 1},
			Issues:   []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeSource, Code: code, Subject: string(ProviderID), Message: fetchErr.Error()}},
		})
	}
	catalog, err := result.PublicCatalog()
	if err != nil {
		return sources.Observation{}, err
	}
	return sources.NewObservation(s.ID(), catalog, sources.ObservationMetadata{
		ObservedAt: s.now(), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
		Records: sources.ObservationRecordCounts{Accepted: len(result.Offerings)}, Scope: catalogmeta.ObservationScopeRegionalPublic,
		Kind: catalogmeta.SourceKindRegionalSweep, Coverage: catalogmeta.ProviderCoverage{Expected: 1, Observed: 1}, PricingObservedAt: result.PricingObservedAt,
	})
}

// Cleanup releases source resources. HTTP clients are request safe and reusable.
func (s *Source) Cleanup() error { return nil }

// Dependencies reports no external executable dependency.
func (s *Source) Dependencies() []sources.Dependency { return nil }

// IsOptional keeps credential-free public generation operational.
func (s *Source) IsOptional() bool { return true }

// Fetch reads account model availability and optionally private deployments.
func (s *Source) Fetch(ctx context.Context, includeCustomerInventory bool) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateConfig(s.realm, s.account); err != nil {
		return Result{}, &errors.AuthenticationError{Provider: string(ProviderID), Method: "azure_account_configuration", Message: "Azure account discovery is not configured", Err: err}
	}
	if includeCustomerInventory {
		if err := validateCustomerAccount(s.account); err != nil {
			return Result{}, err
		}
	}
	client := s.client
	if client == nil {
		var err error
		client, err = s.clientFunc(ctx, s.realm, s.account)
		if err != nil {
			return Result{}, err
		}
	}
	models, err := s.collectModels(ctx, client)
	if err != nil {
		return Result{}, err
	}
	result, index, err := recordsFromModels(s.realm, s.account, models)
	if err != nil {
		return Result{}, err
	}
	if s.pricing != nil && s.realm.ID == commercialRealm.ID {
		prices, priceErr := s.pricing.Fetch(ctx)
		if priceErr != nil {
			return Result{}, priceErr
		}
		result.PricingObservedAt = &prices.ObservedAt
		result.PricingMatched, result.PricingIgnored, err = applyPricing(result.Offerings, prices)
		if err != nil {
			return Result{}, err
		}
	}
	if includeCustomerInventory {
		deployments, listErr := s.collectDeployments(ctx, client)
		if listErr != nil {
			return Result{}, listErr
		}
		inventory, convertErr := customerInventory(s.realm, s.account, s.now(), deployments, index)
		if convertErr != nil {
			return Result{}, convertErr
		}
		result.CustomerInventory = []catalogs.CustomerInventory{inventory}
	}
	return result, nil
}

func (s *Source) collectModels(ctx context.Context, client API) ([]Model, error) {
	return sources.CollectPages(ctx, s.pagination, func(ctx context.Context, cursor string) (page sources.Page[Model], err error) {
		err = sources.RetryProviderCall(ctx, s.retry, func(ctx context.Context) (sources.RetryHint, error) {
			page, err = client.ListModels(ctx, cursor)
			return sources.RetryHint{}, err
		})
		return page, err
	})
}

func (s *Source) collectDeployments(ctx context.Context, client API) ([]Deployment, error) {
	return sources.CollectPages(ctx, s.pagination, func(ctx context.Context, cursor string) (page sources.Page[Deployment], err error) {
		err = sources.RetryProviderCall(ctx, s.retry, func(ctx context.Context) (sources.RetryHint, error) {
			page, err = client.ListDeployments(ctx, cursor)
			return sources.RetryHint{}, err
		})
		return page, err
	})
}

func recordsFromModels(realm Realm, account Account, models []Model) (Result, map[string]modelReference, error) {
	result := Result{Definitions: make([]catalogs.ModelDefinition, 0, len(models)), Offerings: make([]catalogs.ProviderOffering, 0, len(models))}
	index := make(map[string]modelReference, len(models)*2)
	seen := make(map[catalogs.OfferingKey]struct{}, len(models))
	for _, model := range models {
		if strings.TrimSpace(model.Name) == "" || strings.TrimSpace(model.Version) == "" {
			return Result{}, nil, &errors.ValidationError{Field: "azure_foundry.model", Value: model, Message: "name and version are required"}
		}
		author := canonicalAuthor(firstNonempty(model.Publisher, model.Format))
		definitionID := catalogs.ModelDefinitionID(string(author) + "/" + model.Name)
		providerModelID := catalogs.ProviderModelID(model.Name + "@" + model.Version)
		definition := catalogs.ModelDefinition{ID: definitionID, Name: model.Name, AuthorIDs: []catalogs.AuthorID{author}}
		lifecycle := lifecycle(model.LifecycleStatus)
		modes := make(map[string]catalogs.ProviderOfferingMode, len(model.SKUs))
		for _, sku := range model.SKUs {
			if name := strings.TrimSpace(sku.Name); name != "" {
				modes[name] = catalogs.ProviderOfferingMode{}
			}
		}
		offering := catalogs.ProviderOffering{
			ProviderID: ProviderID, ProviderModelID: providerModelID, DefinitionID: definitionID,
			Availability: catalogs.OfferingAvailabilityRestricted,
			Access:       catalogs.OfferingAccess{Channel: catalogs.OfferingAccessChannelServerToServer, Routability: catalogs.OfferingRoutabilityDiscoverable},
			Regions:      []catalogs.CloudRegion{{ID: account.Location, Realm: realm.ID}},
			Deployment:   catalogs.ProviderDeployment{Type: "customer_deployment"}, Lifecycle: lifecycle, Modes: modes,
		}
		if err := definition.Validate(); err != nil {
			return Result{}, nil, err
		}
		if err := offering.Validate(); err != nil {
			return Result{}, nil, err
		}
		if _, found := seen[offering.Key()]; found {
			return Result{}, nil, &errors.ConflictError{Resource: "Azure model offering", Actual: fmt.Sprint(offering.Key()), Message: "duplicate model name and version"}
		}
		seen[offering.Key()] = struct{}{}
		result.Definitions = append(result.Definitions, definition)
		result.Offerings = append(result.Offerings, offering)
		reference := modelReference{DefinitionID: definitionID, ProviderModelID: providerModelID}
		index[model.Name+"@"+model.Version] = reference
		if model.IsDefaultVersion {
			defaultKey := model.Name + "@"
			if existing, found := index[defaultKey]; found && existing != reference {
				return Result{}, nil, &errors.ConflictError{Resource: "Azure default model version", Expected: string(existing.ProviderModelID), Actual: string(providerModelID), Message: "multiple versions claim to be the default"}
			}
			index[defaultKey] = reference
		}
	}
	sort.Slice(result.Definitions, func(i, j int) bool { return result.Definitions[i].ID < result.Definitions[j].ID })
	sort.Slice(result.Offerings, func(i, j int) bool { return result.Offerings[i].ProviderModelID < result.Offerings[j].ProviderModelID })
	return result, index, nil
}

func customerInventory(realm Realm, account Account, observedAt time.Time, deployments []Deployment, definitions map[string]modelReference) (catalogs.CustomerInventory, error) {
	inventory := catalogs.CustomerInventory{ProviderID: ProviderID, Scope: catalogs.CustomerScope{SubscriptionID: account.SubscriptionID}, ObservedAt: observedAt}
	for _, deployment := range deployments {
		reference, found := definitions[deployment.ModelName+"@"+deployment.ModelVersion]
		if !found {
			return catalogs.CustomerInventory{}, &errors.NotFoundError{Resource: "Azure model definition", ID: deployment.ModelName + "@" + deployment.ModelVersion}
		}
		inventory.Deployments = append(inventory.Deployments, catalogs.CustomerDeployment{
			ID: deployment.Name, DefinitionID: reference.DefinitionID, ProviderModelID: reference.ProviderModelID,
			Region:     &catalogs.CloudRegion{ID: account.Location, Realm: realm.ID},
			Deployment: catalogs.ProviderDeployment{Type: "customer_deployment", Tier: firstNonempty(deployment.SKUName, deployment.ScaleType)},
			Endpoint:   account.Endpoint, Aliases: []string{deployment.Name},
		})
	}
	sort.Slice(inventory.Deployments, func(i, j int) bool { return inventory.Deployments[i].ID < inventory.Deployments[j].ID })
	return inventory, inventory.Validate()
}

// PublicCatalog materializes records that contain no account identity or endpoint.
func (r Result) PublicCatalog() (*catalogs.Catalog, error) {
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{ID: ProviderID, Name: "Microsoft Foundry"}); err != nil {
		return nil, err
	}
	authors := make(map[catalogs.AuthorID]struct{})
	for _, definition := range r.Definitions {
		if err := builder.SetDefinition(definition); err != nil {
			return nil, err
		}
		for _, author := range definition.AuthorIDs {
			authors[author] = struct{}{}
		}
	}
	for author := range authors {
		if err := builder.SetAuthor(catalogs.Author{ID: author, Name: string(author)}); err != nil {
			return nil, err
		}
	}
	for _, offering := range r.Offerings {
		if err := builder.SetOffering(offering); err != nil {
			return nil, err
		}
	}
	return builder.Build()
}

func validateConfig(realm Realm, account Account) error {
	if strings.TrimSpace(realm.ID) == "" || strings.TrimSpace(realm.ARMEndpoint) == "" || strings.TrimSpace(realm.AuthorityHost) == "" || strings.TrimSpace(realm.ManagementScope) == "" {
		return &errors.ValidationError{Field: "azure_foundry.realm", Value: realm, Message: "identity, ARM endpoint, authority host, and management scope are required"}
	}
	missing := make([]string, 0, 2)
	for field, value := range map[string]string{"subscription_id": account.SubscriptionID, "location": account.Location} {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, field)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return &errors.ValidationError{Field: "azure_foundry.account", Value: missing, Message: "required account configuration is missing"}
	}
	return nil
}

func validateCustomerAccount(account Account) error {
	missing := make([]string, 0, 2)
	for field, value := range map[string]string{"resource_group": account.ResourceGroup, "name": account.Name} {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, field)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return &errors.ValidationError{Field: "azure_foundry.customer_account", Value: missing, Message: "customer inventory configuration is missing"}
	}
	return nil
}

func lifecycle(value string) catalogs.OfferingLifecycle {
	switch strings.ToLower(value) {
	case "preview":
		return catalogs.OfferingLifecyclePreview
	case "deprecated", "deprecating":
		return catalogs.OfferingLifecycleDeprecated
	case "retired":
		return catalogs.OfferingLifecycleRetired
	default:
		return catalogs.OfferingLifecycleActive
	}
}

func canonicalAuthor(format string) catalogs.AuthorID {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "openai":
		return "openai"
	case "microsoft":
		return "microsoft"
	default:
		value := strings.ToLower(strings.TrimSpace(format))
		value = strings.NewReplacer(" ", "-", "_", "-", "/", "-").Replace(value)
		if value == "" {
			return "unknown"
		}
		return catalogs.AuthorID(value)
	}
}

func emptyPublicCatalog() (*catalogs.Catalog, error) { return catalogs.NewEmpty().Build() }

func firstNonempty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return "standard"
}
