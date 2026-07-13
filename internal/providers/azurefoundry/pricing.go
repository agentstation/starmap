package azurefoundry

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sourcepayload"
)

const (
	// RetailPricesURL is Microsoft's unauthenticated commercial-cloud price API.
	RetailPricesURL      = "https://prices.azure.com/api/retail/prices"
	maxRetailPricePages  = 32
	foundryModelsService = "Foundry Models"
	priceComponentInput  = "input"
	priceComponentOutput = "output"
	priceComponentCache  = "cache_read"
)

var nonPriceWord = regexp.MustCompile(`[^a-z0-9]+`)

type pricingFetcher interface {
	Fetch(context.Context) (pricingCatalog, error)
}

type httpPricingFetcher struct {
	client *http.Client
	now    func() time.Time
}

type pricingCatalog struct {
	ObservedAt time.Time
	Meters     []retailMeter
}

type retailMeter struct {
	ProductName        string    `json:"productName"`
	SKUName            string    `json:"skuName"`
	MeterName          string    `json:"meterName"`
	ServiceName        string    `json:"serviceName"`
	Region             string    `json:"armRegionName"`
	Unit               string    `json:"unitOfMeasure"`
	Type               string    `json:"type"`
	Currency           string    `json:"currencyCode"`
	RetailPrice        float64   `json:"retailPrice"`
	EffectiveStartDate time.Time `json:"effectiveStartDate"`
}

type retailPage struct {
	Items        []retailMeter `json:"Items"`
	NextPageLink string        `json:"NextPageLink"`
}

func newHTTPPricingFetcher() pricingFetcher {
	return &httpPricingFetcher{client: &http.Client{Timeout: constants.DefaultHTTPTimeout}, now: func() time.Time { return time.Now().UTC() }}
}

func (f *httpPricingFetcher) Fetch(ctx context.Context) (pricingCatalog, error) {
	endpoint, err := url.Parse(RetailPricesURL)
	if err != nil {
		return pricingCatalog{}, errors.WrapParse("URL", "Azure Retail Prices endpoint", err)
	}
	query := endpoint.Query()
	query.Set("$filter", "serviceName eq '"+foundryModelsService+"'")
	endpoint.RawQuery = query.Encode()
	result := pricingCatalog{ObservedAt: f.now()}
	next := endpoint.String()
	seen := make(map[string]struct{})
	for pageNumber := 0; pageNumber < maxRetailPricePages && next != ""; pageNumber++ {
		if _, found := seen[next]; found {
			return pricingCatalog{}, &errors.ConflictError{Resource: "Azure price pagination", Actual: next, Message: "next page repeated"}
		}
		seen[next] = struct{}{}
		page, fetchErr := f.fetchPage(ctx, next)
		if fetchErr != nil {
			return pricingCatalog{}, fetchErr
		}
		result.Meters = append(result.Meters, page.Items...)
		next = page.NextPageLink
	}
	if next != "" {
		return pricingCatalog{}, &errors.ValidationError{Field: "azure_foundry.pricing.pages", Value: maxRetailPricePages, Message: "price feed did not terminate within page limit"}
	}
	return result, nil
}

func (f *httpPricingFetcher) fetchPage(ctx context.Context, endpoint string) (retailPage, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() != "prices.azure.com" || parsed.Path != "/api/retail/prices" {
		return retailPage{}, &errors.ValidationError{Field: "azure_foundry.pricing.next_page", Value: endpoint, Message: "must remain inside the Azure Retail Prices endpoint"}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return retailPage{}, errors.WrapResource("create", "Azure Retail Prices request", endpoint, err)
	}
	response, err := f.client.Do(request)
	if err != nil {
		return retailPage{}, errors.WrapResource("fetch", "Azure Retail Prices", endpoint, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return retailPage{}, &errors.APIError{Provider: string(ProviderID), Endpoint: endpoint, StatusCode: response.StatusCode, Message: "Azure Retail Prices request failed"}
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, constants.MaxSourcePayloadBytes+1))
	if err != nil {
		return retailPage{}, errors.WrapIO("read", "Azure Retail Prices response", err)
	}
	if err := sourcepayload.ValidateJSON(payload); err != nil {
		return retailPage{}, err
	}
	var page retailPage
	if err := json.Unmarshal(payload, &page); err != nil {
		return retailPage{}, errors.WrapParse("json", "Azure Retail Prices response", err)
	}
	return page, nil
}

func applyPricing(offerings []catalogs.ProviderOffering, catalog pricingCatalog) (matched, ignored int, err error) {
	for _, meter := range catalog.Meters {
		component, per1M, ok := classifyRetailMeter(meter)
		if !ok {
			ignored++
			continue
		}
		index := matchingOffering(offerings, meter)
		if index < 0 {
			ignored++
			continue
		}
		modeName := "retail/" + meter.Region + "/" + priceMode(meter.SKUName)
		mode := offerings[index].Modes[modeName]
		if mode.Pricing == nil {
			mode.Pricing = &catalogs.ModelPricing{Currency: catalogs.ModelPricingCurrencyUSD, Tokens: &catalogs.ModelTokenPricing{}}
		}
		cost, normalizeErr := catalogs.NormalizeProviderTokenPrice(per1M, catalogs.ProviderNormalizationUnitPerMillionTokens)
		if normalizeErr != nil {
			return matched, ignored, normalizeErr
		}
		switch component {
		case priceComponentInput:
			mode.Pricing.Tokens.Input = &cost
		case priceComponentOutput:
			mode.Pricing.Tokens.Output = &cost
		case priceComponentCache:
			mode.Pricing.Tokens.CacheRead = &cost
		}
		offerings[index].Modes[modeName] = mode
		matched++
	}
	return matched, ignored, nil
}

func classifyRetailMeter(meter retailMeter) (string, float64, bool) {
	if meter.ServiceName != foundryModelsService || meter.Type != "Consumption" || meter.Currency != "USD" || meter.RetailPrice < 0 {
		return "", 0, false
	}
	var multiplier float64
	switch strings.ToUpper(meter.Unit) {
	case "1K":
		multiplier = 1000
	case "1M":
		multiplier = 1
	default:
		return "", 0, false
	}
	name := " " + strings.ToLower(meter.SKUName+" "+meter.MeterName) + " "
	component := ""
	switch {
	case containsAny(name, " cached ", " cchd ", " cache ") && containsAny(name, " inp ", " input ", " inpt "):
		component = priceComponentCache
	case containsAny(name, " inp ", " input ", " inpt "):
		component = priceComponentInput
	case containsAny(name, " out ", " outp ", " output ", " outpt ", " opt "):
		component = priceComponentOutput
	default:
		return "", 0, false
	}
	return component, meter.RetailPrice * multiplier, true
}

func matchingOffering(offerings []catalogs.ProviderOffering, meter retailMeter) int {
	meterKey := compactPriceName(meter.SKUName + " " + meter.MeterName)
	candidates := make([]int, 0, 1)
	longest := 0
	for index, offering := range offerings {
		name := strings.SplitN(string(offering.ProviderModelID), "@", 2)[0]
		key := compactPriceName(name)
		if len(key) >= 2 && strings.Contains(meterKey, key) {
			if len(key) > longest {
				candidates = candidates[:0]
				longest = len(key)
			}
			if len(key) == longest {
				candidates = append(candidates, index)
			}
		}
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	return -1
}

func priceMode(value string) string {
	words := strings.Fields(nonPriceWord.ReplaceAllString(strings.ToLower(value), " "))
	words = slices.DeleteFunc(words, func(word string) bool {
		switch word {
		case "inp", "input", "inpt", "out", "outp", "output", "outpt", "opt", "token", "tokens", "1k", "1m":
			return true
		default:
			return false
		}
	})
	slices.Sort(words)
	return strings.Join(words, "-")
}

func compactPriceName(value string) string {
	return nonPriceWord.ReplaceAllString(strings.ToLower(value), "")
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}
