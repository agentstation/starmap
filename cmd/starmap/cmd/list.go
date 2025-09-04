package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/cmd/common"
	"github.com/agentstation/starmap/internal/cmd/filter"
	"github.com/agentstation/starmap/internal/cmd/output"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/spf13/cobra"
)

// newListCommand creates the list command with subcommands
func newListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list [resource]",
		GroupID: "core",
		Short:   "List resources from local catalog",
		Long: `List displays resources from the local starmap catalog.

Available resources:
  models      - AI models and their capabilities
  providers   - Model providers and API endpoints  
  authors     - Model creators and organizations`,
		Example: `  starmap list models                      # List all models
  starmap list models claude-3-5-sonnet    # Show specific model details
  starmap list providers                   # List all providers
  starmap list authors                     # List all authors`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default to models if no subcommand
			if len(args) == 0 {
				return cmd.Help()
			}
			return fmt.Errorf("unknown resource: %s", args[0])
		},
	}

	// Add subcommands for each resource
	cmd.AddCommand(newListModelsCommand())
	cmd.AddCommand(newListProvidersCommand())
	cmd.AddCommand(newListAuthorsCommand())

	return cmd
}

// newListModelsCommand creates the list models subcommand
func newListModelsCommand() *cobra.Command {
	var (
		resourceFlags *common.ResourceFlags
		showDetails   bool
		capability    string
		minContext    int64
		maxPrice      float64
	)

	cmd := &cobra.Command{
		Use:     "models [model-id]",
		Short:   "List models from catalog",
		Aliases: []string{"model"},
		Args:    cobra.MaximumNArgs(1),
		Example: `  starmap list models                          # List all models
  starmap list models claude-3-5-sonnet        # Show specific model details
  starmap list models --provider openai        # List OpenAI models only
  starmap list models --search claude          # Search for models by name`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Single model detail view
			if len(args) == 1 {
				return showModelDetails(ctx, args[0])
			}

			// List view with filters
			return listModels(ctx, resourceFlags, capability, minContext, maxPrice, showDetails)
		},
	}

	// Add resource-specific flags
	resourceFlags = common.AddResourceFlags(cmd)
	cmd.Flags().BoolVar(&showDetails, "details", false,
		"Show detailed information for each model")
	cmd.Flags().StringVar(&capability, "capability", "",
		"Filter by capability (e.g., tool_calls, reasoning, vision)")
	cmd.Flags().Int64Var(&minContext, "min-context", 0,
		"Minimum context window size")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0,
		"Maximum price per 1M input tokens")

	return cmd
}

// newListProvidersCommand creates the list providers subcommand
func newListProvidersCommand() *cobra.Command {
	var (
		resourceFlags *common.ResourceFlags
		showKeys      bool
	)

	cmd := &cobra.Command{
		Use:     "providers [provider-id]",
		Short:   "List providers from catalog",
		Aliases: []string{"provider"},
		Args:    cobra.MaximumNArgs(1),
		Example: `  starmap list providers                   # List all providers
  starmap list providers anthropic         # Show specific provider details
  starmap list providers --keys            # Show API key configuration status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Single provider detail view
			if len(args) == 1 {
				return showProviderDetails(ctx, args[0], showKeys)
			}

			// List view with filters
			return listProviders(ctx, resourceFlags, showKeys)
		},
	}

	// Add resource-specific flags
	resourceFlags = common.AddResourceFlags(cmd)
	cmd.Flags().BoolVar(&showKeys, "keys", false,
		"Show if API keys are configured (keys are not displayed)")

	return cmd
}

// newListAuthorsCommand creates the list authors subcommand
func newListAuthorsCommand() *cobra.Command {
	var resourceFlags *common.ResourceFlags

	cmd := &cobra.Command{
		Use:     "authors [author-id]",
		Short:   "List authors from catalog",
		Aliases: []string{"author"},
		Args:    cobra.MaximumNArgs(1),
		Example: `  starmap list authors                     # List all authors
  starmap list authors openai              # Show specific author details
  starmap list authors --search meta       # Search for authors by name`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Single author detail view
			if len(args) == 1 {
				return showAuthorDetails(ctx, args[0])
			}

			// List view with filters
			return listAuthors(ctx, resourceFlags)
		},
	}

	// Add resource-specific flags
	resourceFlags = common.AddResourceFlags(cmd)

	return cmd
}

// listModels lists all models with optional filters
func listModels(ctx context.Context, flags *common.ResourceFlags, capability string, minContext int64, maxPrice float64, showDetails bool) error {
	// Get starmap instance
	sm, err := starmap.New()
	if err != nil {
		return errors.WrapResource("create", "starmap", "", err)
	}

	// Get catalog
	catalog, err := sm.Catalog()
	if err != nil {
		return errors.WrapResource("get", "catalog", "", err)
	}

	// Get all models
	allModels := catalog.GetAllModels()

	// Apply filters
	modelFilter := &filter.ModelFilter{
		Provider:   flags.Provider,
		Author:     flags.Author,
		Capability: capability,
		MinContext: minContext,
		MaxPrice:   maxPrice,
		Search:     flags.Search,
	}
	filtered := modelFilter.Apply(allModels)

	// Sort models
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].ID < filtered[j].ID
	})

	// Apply limit
	if flags.Limit > 0 && len(filtered) > flags.Limit {
		filtered = filtered[:flags.Limit]
	}

	// Format output
	formatter := output.NewFormatter(output.Format(globalFlags.Output))

	// Transform to output format
	var outputData any
	switch globalFlags.Output {
	case "table", "wide", "":
		outputData = modelsToTableData(filtered, showDetails)
	default:
		outputData = filtered
	}

	if !globalFlags.Quiet {
		fmt.Fprintf(os.Stderr, "Found %d models\n", len(filtered))
	}

	return formatter.Format(os.Stdout, outputData)
}

// showModelDetails shows detailed information about a specific model
func showModelDetails(ctx context.Context, modelID string) error {
	sm, err := starmap.New()
	if err != nil {
		return errors.WrapResource("create", "starmap", "", err)
	}

	catalog, err := sm.Catalog()
	if err != nil {
		return errors.WrapResource("get", "catalog", "", err)
	}

	// Find specific model across all providers
	providers := catalog.Providers().List()
	for _, provider := range providers {
		if model, exists := provider.Models[modelID]; exists {
			formatter := output.NewFormatter(output.Format(globalFlags.Output))

			// For table output, show detailed view
			if globalFlags.Output == "table" || globalFlags.Output == "" {
				printModelDetails(model, *provider)
				return nil
			}

			// For structured output, return the model
			return formatter.Format(os.Stdout, model)
		}
	}

	return &errors.NotFoundError{
		Resource: "model",
		ID:       modelID,
	}
}

// listProviders lists all providers with optional filters
func listProviders(ctx context.Context, flags *common.ResourceFlags, showKeys bool) error {
	sm, err := starmap.New()
	if err != nil {
		return errors.WrapResource("create", "starmap", "", err)
	}

	catalog, err := sm.Catalog()
	if err != nil {
		return errors.WrapResource("get", "catalog", "", err)
	}

	// Get all providers
	providers := catalog.Providers().List()

	// Apply filters
	providerFilter := &filter.ProviderFilter{
		Search: flags.Search,
	}
	// Convert to value slice for filter
	providerValues := make([]catalogs.Provider, len(providers))
	for i, p := range providers {
		providerValues[i] = *p
	}
	filtered := providerFilter.Apply(providerValues)

	// Sort providers
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].ID < filtered[j].ID
	})

	// Apply limit
	if flags.Limit > 0 && len(filtered) > flags.Limit {
		filtered = filtered[:flags.Limit]
	}

	// Format output
	formatter := output.NewFormatter(output.Format(globalFlags.Output))

	// Transform to output format
	var outputData any
	switch globalFlags.Output {
	case "table", "wide", "":
		outputData = providersToTableData(filtered, showKeys)
	default:
		outputData = filtered
	}

	if !globalFlags.Quiet {
		fmt.Fprintf(os.Stderr, "Found %d providers\n", len(filtered))
	}

	return formatter.Format(os.Stdout, outputData)
}

// showProviderDetails shows detailed information about a specific provider
func showProviderDetails(ctx context.Context, providerID string, showKeys bool) error {
	sm, err := starmap.New()
	if err != nil {
		return errors.WrapResource("create", "starmap", "", err)
	}

	catalog, err := sm.Catalog()
	if err != nil {
		return errors.WrapResource("get", "catalog", "", err)
	}

	provider, found := catalog.Providers().Get(catalogs.ProviderID(providerID))
	if !found {
		return &errors.NotFoundError{
			Resource: "provider",
			ID:       providerID,
		}
	}

	formatter := output.NewFormatter(output.Format(globalFlags.Output))

	// For table output, show detailed view
	if globalFlags.Output == "table" || globalFlags.Output == "" {
		printProviderDetails(provider, showKeys)
		return nil
	}

	// For structured output, return the provider
	return formatter.Format(os.Stdout, provider)
}

// listAuthors lists all authors with optional filters
func listAuthors(ctx context.Context, flags *common.ResourceFlags) error {
	sm, err := starmap.New()
	if err != nil {
		return errors.WrapResource("create", "starmap", "", err)
	}

	catalog, err := sm.Catalog()
	if err != nil {
		return errors.WrapResource("get", "catalog", "", err)
	}

	// Get all authors
	authors := catalog.Authors().List()

	// Apply filters
	authorFilter := &filter.AuthorFilter{
		Search: flags.Search,
	}
	// Convert to value slice for filter
	authorValues := make([]catalogs.Author, len(authors))
	for i, a := range authors {
		authorValues[i] = *a
	}
	filtered := authorFilter.Apply(authorValues)

	// Sort authors
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].ID < filtered[j].ID
	})

	// Apply limit
	if flags.Limit > 0 && len(filtered) > flags.Limit {
		filtered = filtered[:flags.Limit]
	}

	// Format output
	formatter := output.NewFormatter(output.Format(globalFlags.Output))

	// Transform to output format
	var outputData any
	switch globalFlags.Output {
	case "table", "wide", "":
		outputData = authorsToTableData(filtered)
	default:
		outputData = filtered
	}

	if !globalFlags.Quiet {
		fmt.Fprintf(os.Stderr, "Found %d authors\n", len(filtered))
	}

	return formatter.Format(os.Stdout, outputData)
}

// showAuthorDetails shows detailed information about a specific author
func showAuthorDetails(ctx context.Context, authorID string) error {
	sm, err := starmap.New()
	if err != nil {
		return errors.WrapResource("create", "starmap", "", err)
	}

	catalog, err := sm.Catalog()
	if err != nil {
		return errors.WrapResource("get", "catalog", "", err)
	}

	author, found := catalog.Authors().Get(catalogs.AuthorID(authorID))
	if !found {
		return &errors.NotFoundError{
			Resource: "author",
			ID:       authorID,
		}
	}

	formatter := output.NewFormatter(output.Format(globalFlags.Output))

	// For table output, show detailed view
	if globalFlags.Output == "table" || globalFlags.Output == "" {
		printAuthorDetails(author)
		return nil
	}

	// For structured output, return the author
	return formatter.Format(os.Stdout, author)
}

// Helper functions to convert data to table format

func modelsToTableData(models []catalogs.Model, showDetails bool) output.TableData {
	headers := []string{"ID", "NAME", "CONTEXT", "OUTPUT"}
	if showDetails {
		headers = append(headers, "PRICE (IN)", "PRICE (OUT)")
	}

	var rows [][]string
	for _, model := range models {
		row := []string{
			model.ID,
			model.Name,
			formatTokens(model.Limits),
			formatOutput(model.Limits),
		}

		if showDetails {
			row = append(row,
				formatPrice(model.Pricing, true),
				formatPrice(model.Pricing, false),
			)
		}

		rows = append(rows, row)
	}

	return output.TableData{
		Headers: headers,
		Rows:    rows,
	}
}

func providersToTableData(providers []catalogs.Provider, showKeys bool) output.TableData {
	headers := []string{"ID", "NAME", "LOCATION", "STATUS"}

	var rows [][]string
	for _, provider := range providers {
		status := "✓"
		if showKeys && provider.APIKey != nil {
			if provider.IsAPIKeyRequired() {
				// Check if key is configured
				if os.Getenv(provider.APIKey.Name) == "" {
					status = "✗"
				}
			}
		}

		location := ""
		if provider.Headquarters != nil {
			location = *provider.Headquarters
		}

		row := []string{
			string(provider.ID),
			provider.Name,
			location,
			status,
		}
		rows = append(rows, row)
	}

	return output.TableData{
		Headers: headers,
		Rows:    rows,
	}
}

func authorsToTableData(authors []catalogs.Author) output.TableData {
	headers := []string{"ID", "NAME", "MODELS", "WEBSITE"}

	var rows [][]string
	for _, author := range authors {
		website := ""
		if author.Website != nil {
			website = *author.Website
		}

		row := []string{
			string(author.ID),
			author.Name,
			fmt.Sprintf("%d", len(author.Models)),
			website,
		}
		rows = append(rows, row)
	}

	return output.TableData{
		Headers: headers,
		Rows:    rows,
	}
}

// Helper functions for formatting

func formatTokens(limits *catalogs.ModelLimits) string {
	if limits == nil || limits.ContextWindow == 0 {
		return "-"
	}
	if limits.ContextWindow >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(limits.ContextWindow)/1000000)
	}
	if limits.ContextWindow >= 1000 {
		return fmt.Sprintf("%dk", limits.ContextWindow/1000)
	}
	return fmt.Sprintf("%d", limits.ContextWindow)
}

func formatOutput(limits *catalogs.ModelLimits) string {
	if limits == nil || limits.OutputTokens == 0 {
		return "-"
	}
	if limits.OutputTokens >= 1000 {
		return fmt.Sprintf("%dk", limits.OutputTokens/1000)
	}
	return fmt.Sprintf("%d", limits.OutputTokens)
}

func formatPrice(pricing *catalogs.ModelPricing, input bool) string {
	if pricing == nil || pricing.Tokens == nil {
		return "-"
	}

	var price float64
	if input && pricing.Tokens.Input != nil {
		price = pricing.Tokens.Input.Per1M
	} else if !input && pricing.Tokens.Output != nil {
		price = pricing.Tokens.Output.Per1M
	} else {
		return "-"
	}

	if price == 0 {
		return "free"
	}
	return fmt.Sprintf("$%.2f", price)
}

// Detailed print functions for single resource views

func printModelDetails(model catalogs.Model, provider catalogs.Provider) {
	fmt.Printf("Model: %s\n", model.ID)
	fmt.Printf("Name: %s\n", model.Name)

	if model.Description != "" {
		fmt.Printf("Description: %s\n", model.Description)
	}

	fmt.Printf("Provider: %s\n", provider.Name)

	if len(model.Authors) > 0 {
		authors := make([]string, 0, len(model.Authors))
		for _, a := range model.Authors {
			authors = append(authors, a.Name)
		}
		fmt.Printf("Authors: %s\n", strings.Join(authors, ", "))
	}

	if model.Metadata != nil {
		fmt.Printf("\nMetadata:\n")
		if !model.Metadata.ReleaseDate.IsZero() {
			fmt.Printf("  Release Date: %s\n", model.Metadata.ReleaseDate.Format("2006-01-02"))
		}
		if model.Metadata.KnowledgeCutoff != nil && !model.Metadata.KnowledgeCutoff.IsZero() {
			fmt.Printf("  Knowledge Cutoff: %s\n", model.Metadata.KnowledgeCutoff.Format("2006-01-02"))
		}
		fmt.Printf("  Open Weights: %v\n", model.Metadata.OpenWeights)
	}

	if model.Limits != nil {
		fmt.Printf("\nLimits:\n")
		fmt.Printf("  Context Window: %s tokens\n", formatTokens(model.Limits))
		fmt.Printf("  Output Tokens: %s tokens\n", formatOutput(model.Limits))
	}

	if model.Pricing != nil && model.Pricing.Tokens != nil {
		fmt.Printf("\nPricing:\n")
		fmt.Printf("  Input: %s per 1M tokens\n", formatPrice(model.Pricing, true))
		fmt.Printf("  Output: %s per 1M tokens\n", formatPrice(model.Pricing, false))
	}

	if model.Features != nil {
		fmt.Printf("\nFeatures:\n")
		if model.Features.ToolCalls {
			fmt.Printf("  ✓ Tool Calls\n")
		}
		if model.Features.Reasoning {
			fmt.Printf("  ✓ Reasoning\n")
		}
		if model.Features.Streaming {
			fmt.Printf("  ✓ Streaming\n")
		}
		if model.Features.StructuredOutputs {
			fmt.Printf("  ✓ Structured Outputs\n")
		}
	}
}

func printProviderDetails(provider *catalogs.Provider, showKeys bool) {
	fmt.Printf("Provider: %s\n", provider.ID)
	fmt.Printf("Name: %s\n", provider.Name)

	if provider.Headquarters != nil {
		fmt.Printf("Location: %s\n", *provider.Headquarters)
	}

	if provider.APIKey != nil {
		fmt.Printf("\nAPI Configuration:\n")
		fmt.Printf("  Key Variable: %s", provider.APIKey.Name)
		if provider.IsAPIKeyRequired() {
			fmt.Printf(" (required)")
		} else {
			fmt.Printf(" (optional)")
		}
		fmt.Println()

		if showKeys {
			if os.Getenv(provider.APIKey.Name) != "" {
				fmt.Printf("  Status: ✓ Configured\n")
			} else {
				fmt.Printf("  Status: ✗ Not configured\n")
			}
		}
	}

	if len(provider.EnvVars) > 0 {
		fmt.Printf("\nEnvironment Variables:\n")
		for _, envVar := range provider.EnvVars {
			status := "optional"
			if envVar.Required {
				status = "required"
			}
			fmt.Printf("  %s (%s)", envVar.Name, status)
			if envVar.Description != "" {
				fmt.Printf(" - %s", envVar.Description)
			}
			fmt.Println()
		}
	}

	if provider.Catalog != nil && provider.Catalog.DocsURL != nil {
		fmt.Printf("\nDocumentation: %s\n", *provider.Catalog.DocsURL)
	}

	if provider.ChatCompletions != nil && provider.ChatCompletions.URL != nil {
		fmt.Printf("API Endpoint: %s\n", *provider.ChatCompletions.URL)
	}

	fmt.Printf("\nModels: %d\n", len(provider.Models))
}

func printAuthorDetails(author *catalogs.Author) {
	fmt.Printf("Author: %s\n", author.ID)
	fmt.Printf("Name: %s\n", author.Name)

	if author.Description != nil && *author.Description != "" {
		fmt.Printf("Description: %s\n", *author.Description)
	}

	fmt.Printf("Models: %d\n", len(author.Models))

	if author.Website != nil && *author.Website != "" {
		fmt.Printf("\nLinks:\n")
		fmt.Printf("  Website: %s\n", *author.Website)
	}
	if author.GitHub != nil && *author.GitHub != "" {
		fmt.Printf("  GitHub: %s\n", *author.GitHub)
	}
	if author.HuggingFace != nil && *author.HuggingFace != "" {
		fmt.Printf("  HuggingFace: %s\n", *author.HuggingFace)
	}
	if author.Twitter != nil && *author.Twitter != "" {
		fmt.Printf("  Twitter: %s\n", *author.Twitter)
	}

	if len(author.Models) > 0 && len(author.Models) <= 20 {
		fmt.Printf("\nModels:\n")
		// Sort model IDs for consistent output
		modelIDs := make([]string, 0, len(author.Models))
		for id := range author.Models {
			modelIDs = append(modelIDs, id)
		}
		sort.Strings(modelIDs)

		for _, id := range modelIDs {
			model := author.Models[id]
			fmt.Printf("  • %s - %s\n", model.ID, model.Name)
		}
	}
}

func init() {
	rootCmd.AddCommand(newListCommand())
}
