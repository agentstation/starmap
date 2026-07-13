package bedrock

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sourcepayload"
)

const (
	// PriceListURL is AWS's public bulk price list for Bedrock foundation models.
	PriceListURL        = "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonBedrockFoundationModels/current/index.json"
	componentCacheWrite = "cache_write"
)

type pricingFetcher interface {
	Fetch(context.Context) (pricingCatalog, error)
}

type httpPricingFetcher struct{ client *http.Client }

type pricingCatalog struct {
	Version      string
	PublishedAt  time.Time
	ETag         string
	Prices       map[pricingKey]*catalogs.ModelPricing
	AcceptedSKUs int
	IgnoredSKUs  int
}

type pricingKey struct {
	ServiceName string
	Region      string
	Mode        string
}

type priceIndex struct {
	FormatVersion   string                                     `json:"formatVersion"`
	OfferCode       string                                     `json:"offerCode"`
	Version         string                                     `json:"version"`
	PublicationDate time.Time                                  `json:"publicationDate"`
	Products        map[string]priceProduct                    `json:"products"`
	Terms           map[string]map[string]map[string]priceTerm `json:"terms"`
}

type priceProduct struct {
	SKU        string          `json:"sku"`
	Attributes priceAttributes `json:"attributes"`
}

type priceAttributes struct {
	ServiceName string `json:"servicename"`
	RegionCode  string `json:"regionCode"`
	UsageType   string `json:"usagetype"`
}

type priceTerm struct {
	PriceDimensions map[string]priceDimension `json:"priceDimensions"`
}

type priceDimension struct {
	BeginRange   string            `json:"beginRange"`
	EndRange     string            `json:"endRange"`
	Unit         string            `json:"unit"`
	Description  string            `json:"description"`
	PricePerUnit map[string]string `json:"pricePerUnit"`
}

func newHTTPPricingFetcher() pricingFetcher {
	return &httpPricingFetcher{client: &http.Client{Timeout: constants.DefaultHTTPTimeout}}
}

func (f *httpPricingFetcher) Fetch(ctx context.Context) (pricingCatalog, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, PriceListURL, nil)
	if err != nil {
		return pricingCatalog{}, errors.WrapResource("create", "Bedrock price-list request", PriceListURL, err)
	}
	response, err := f.client.Do(request)
	if err != nil {
		return pricingCatalog{}, errors.WrapResource("fetch", "Bedrock price list", PriceListURL, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return pricingCatalog{}, &errors.APIError{Provider: string(ProviderID), Endpoint: PriceListURL, StatusCode: response.StatusCode, Message: "price-list request failed"}
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, constants.MaxSourcePayloadBytes+1))
	if err != nil {
		return pricingCatalog{}, errors.WrapIO("read", "Bedrock price list", err)
	}
	if err := sourcepayload.ValidateJSON(payload); err != nil {
		return pricingCatalog{}, err
	}
	result, err := parsePricing(payload)
	if err != nil {
		return pricingCatalog{}, err
	}
	result.ETag = response.Header.Get("ETag")
	return result, nil
}

func parsePricing(payload []byte) (pricingCatalog, error) {
	if err := sourcepayload.ValidateJSON(payload); err != nil {
		return pricingCatalog{}, err
	}
	var index priceIndex
	if err := json.Unmarshal(payload, &index); err != nil {
		return pricingCatalog{}, errors.WrapParse("json", "Amazon Bedrock price list", err)
	}
	if index.FormatVersion != "v1.0" || index.OfferCode != "AmazonBedrockFoundationModels" || index.Version == "" || index.PublicationDate.IsZero() {
		return pricingCatalog{}, &errors.ValidationError{Field: "bedrock.pricing.identity", Value: index.OfferCode, Message: "format, offer code, version, and publication date are required"}
	}
	result := pricingCatalog{Version: index.Version, PublishedAt: index.PublicationDate.UTC(), Prices: make(map[pricingKey]*catalogs.ModelPricing)}
	onDemand := index.Terms["OnDemand"]
	for sku, product := range index.Products {
		component, mode, ok := classifyTokenUsage(product.Attributes.UsageType)
		if !ok {
			result.IgnoredSKUs++
			continue
		}
		key := pricingKey{ServiceName: normalizePriceServiceName(product.Attributes.ServiceName), Region: product.Attributes.RegionCode, Mode: mode}
		if key.ServiceName == "" || key.Region == "" {
			return pricingCatalog{}, &errors.ValidationError{Field: "bedrock.pricing.product", Value: sku, Message: "service name and region are required"}
		}
		price, found, err := tokenSKUPrice(onDemand[sku])
		if err != nil {
			return pricingCatalog{}, errors.WrapResource("parse", "Bedrock price SKU", sku, err)
		}
		if !found {
			result.IgnoredSKUs++
			continue
		}
		if err := setTokenComponent(result.Prices, key, component, price); err != nil {
			return pricingCatalog{}, errors.WrapResource("merge", "Bedrock price SKU", sku, err)
		}
		result.AcceptedSKUs++
	}
	for key, price := range result.Prices {
		if err := price.Validate(); err != nil {
			return pricingCatalog{}, errors.WrapResource("validate", "Bedrock price", key.ServiceName+"/"+key.Region+"/"+key.Mode, err)
		}
	}
	return result, nil
}

func classifyTokenUsage(usage string) (component, mode string, ok bool) {
	value := strings.ToLower(usage)
	if strings.Contains(value, "reserved_") || strings.Contains(value, "provisionedthroughput") || strings.Contains(value, "customization") {
		return "", "", false
	}
	switch {
	case strings.Contains(value, "cacheread") || strings.Contains(value, "cache_read"):
		component = "cache_read"
	case strings.Contains(value, "cachewrite") || strings.Contains(value, "cache_write"):
		component = componentCacheWrite
	case strings.Contains(value, "outputtoken") || strings.Contains(value, "output_token"):
		component = "output"
	case strings.Contains(value, "inputtoken") || strings.Contains(value, "input_token"):
		component = "input"
	default:
		return "", "", false
	}
	global := strings.Contains(value, "global")
	batch := strings.Contains(value, "batch")
	switch {
	case strings.Contains(value, "latencyoptimized"):
		mode = "latency_optimized"
	case global && batch:
		mode = "global_batch"
	case batch:
		mode = "batch"
	case global:
		mode = "global"
	default:
		mode = "regional"
	}
	if component == componentCacheWrite && (strings.Contains(value, "cachewrite1h") || strings.Contains(value, "cache_write_tokens_1h")) {
		mode += "_cache_1h"
	}
	return component, mode, true
}

func tokenSKUPrice(terms map[string]priceTerm) (float64, bool, error) {
	for _, term := range terms {
		for _, dimension := range term.PriceDimensions {
			if dimension.BeginRange != "0" || dimension.Unit != "Units" || !strings.Contains(strings.ToLower(dimension.Description), "token") {
				continue
			}
			value, found := dimension.PricePerUnit["USD"]
			if !found {
				continue
			}
			price, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return 0, false, errors.WrapParse("decimal", "Bedrock USD price", err)
			}
			return price, true, nil
		}
	}
	return 0, false, nil
}

func setTokenComponent(prices map[pricingKey]*catalogs.ModelPricing, key pricingKey, component string, per1M float64) error {
	price := prices[key]
	if price == nil {
		price = &catalogs.ModelPricing{Currency: catalogs.ModelPricingCurrencyUSD, Tokens: &catalogs.ModelTokenPricing{}}
		prices[key] = price
	}
	cost, err := catalogs.NormalizeProviderTokenPrice(per1M, catalogs.ProviderNormalizationUnitPerMillionTokens)
	if err != nil {
		return err
	}
	var target **catalogs.ModelTokenCost
	switch component {
	case "input":
		target = &price.Tokens.Input
	case "output":
		target = &price.Tokens.Output
	case "cache_read":
		target = &price.Tokens.CacheRead
	case componentCacheWrite:
		target = &price.Tokens.CacheWrite
	default:
		return &errors.ValidationError{Field: "bedrock.pricing.component", Value: component, Message: "is not supported"}
	}
	if *target != nil && (*target).Per1M != per1M {
		return &errors.ConflictError{Resource: "Bedrock token price", Expected: strconv.FormatFloat((*target).Per1M, 'g', -1, 64), Actual: strconv.FormatFloat(per1M, 'g', -1, 64)}
	}
	*target = &cost
	return nil
}

func normalizePriceServiceName(value string) string {
	value = strings.TrimSpace(strings.TrimSuffix(value, "(Amazon Bedrock Edition)"))
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func applyPricing(result *Result, prices pricingCatalog) int {
	definitionNames := make(map[catalogs.ModelDefinitionID]string, len(result.Definitions))
	for _, definition := range result.Definitions {
		definitionNames[definition.ID] = normalizePriceServiceName(definition.Name)
	}
	matched := 0
	for index := range result.Offerings {
		offering := &result.Offerings[index]
		serviceName := definitionNames[offering.DefinitionID]
		for _, region := range offering.Regions {
			keys := make([]pricingKey, 0)
			for key := range prices.Prices {
				if key.ServiceName == serviceName && key.Region == region.ID && priceModeAllowed(*offering, key.Mode) {
					keys = append(keys, key)
				}
			}
			slices.SortFunc(keys, func(left, right pricingKey) int { return strings.Compare(left.Mode, right.Mode) })
			for _, key := range keys {
				price := prices.Prices[key]
				if offering.Modes == nil {
					offering.Modes = make(map[string]catalogs.ProviderOfferingMode)
				}
				name := key.Mode + "/" + region.ID
				offering.Modes[name] = catalogs.ProviderOfferingMode{Pricing: price}
				matched++
			}
		}
	}
	return matched
}

func priceModeAllowed(offering catalogs.ProviderOffering, mode string) bool {
	global := strings.HasPrefix(mode, "global")
	if offering.InferenceProfile == nil {
		return !global
	}
	if strings.EqualFold(offering.InferenceProfile.Scope, "GLOBAL") {
		return global
	}
	return !global && !strings.HasPrefix(mode, "latency_optimized")
}
