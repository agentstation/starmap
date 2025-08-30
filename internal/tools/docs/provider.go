package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
)

// generateProviderDocs generates documentation for all providers
func (g *Generator) generateProviderDocs(dir string, catalog catalogs.Reader) error {
	providers := catalog.Providers().List()

	// First generate the provider index
	if err := g.generateProviderIndex(dir, providers); err != nil {
		return fmt.Errorf("generating provider index: %w", err)
	}

	// Then generate individual provider pages
	for _, provider := range providers {
		providerDir := filepath.Join(dir, string(provider.ID))
		if err := os.MkdirAll(providerDir, constants.DirPermissions); err != nil {
			return fmt.Errorf("creating provider directory: %w", err)
		}

		// Create models subdirectory for this provider
		modelsDir := filepath.Join(providerDir, "models")
		if err := os.MkdirAll(modelsDir, constants.DirPermissions); err != nil {
			return fmt.Errorf("creating provider models directory: %w", err)
		}

		if err := g.generateProviderReadme(providerDir, provider, catalog); err != nil {
			return fmt.Errorf("generating provider %s README: %w", provider.ID, err)
		}

		// Generate model pages for this provider
		if err := g.generateProviderModelPages(modelsDir, provider, catalog); err != nil {
			return fmt.Errorf("generating provider %s model pages: %w", provider.ID, err)
		}
	}

	return nil
}

// generateProviderIndex generates the main provider listing page
func (g *Generator) generateProviderIndex(dir string, providers []*catalogs.Provider) error {
	// Ensure the directory exists
	if err := os.MkdirAll(dir, constants.DirPermissions); err != nil {
		return fmt.Errorf("creating provider directory: %w", err)
	}

	indexFile := filepath.Join(dir, "README.md")
	f, err := os.Create(indexFile)
	if err != nil {
		return fmt.Errorf("creating provider index: %w", err)
	}
	defer f.Close()

	fmt.Fprintln(f, "# 🏢 AI Model Providers")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "Organizations that host and serve AI models through APIs.")
	fmt.Fprintln(f)

	// Provider comparison table using the comparison function
	fmt.Fprintln(f, "## Provider Comparison")
	fmt.Fprintln(f)

	// Sort providers by model count for better presentation
	sort.Slice(providers, func(i, j int) bool {
		return len(providers[i].Models) > len(providers[j].Models)
	})

	writeProviderComparisonTable(f, providers)
	fmt.Fprintln(f)

	// Add top models overview across all providers
	fmt.Fprintln(f, "## 🌟 Top Models Across Providers")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "Overview of popular models available across different providers:")
	fmt.Fprintln(f)

	// Collect all models from all providers
	var allModels []*catalogs.Model
	for _, provider := range providers {
		for _, model := range provider.Models {
			modelCopy := model
			allModels = append(allModels, &modelCopy)
		}
	}

	// Show top 20 models
	if len(allModels) > 0 {
		writeModelsOverviewTable(f, allModels, providers)
	}
	fmt.Fprintln(f)

	// Provider details section
	fmt.Fprintln(f, "## Provider Details")
	fmt.Fprintln(f)

	for _, provider := range providers {
		fmt.Fprintf(f, "### %s %s\n\n", getProviderBadge(provider.Name), provider.Name)

		// Provider description (if we had it)
		desc := getProviderDescription(provider)
		if desc != "" {
			fmt.Fprintf(f, "%s\n\n", desc)
		}

		// Quick stats
		fmt.Fprintf(f, "- **Models**: %d available\n", len(provider.Models))
		if provider.APIKey != nil {
			fmt.Fprintf(f, "- **API Key**: Required (`%s`)\n", provider.APIKey.Name)
		}
		if provider.StatusPageURL != nil {
			fmt.Fprintf(f, "- **Status**: [Check current status](%s)\n", *provider.StatusPageURL)
		}

		// Top models preview
		if len(provider.Models) > 0 {
			fmt.Fprintln(f, "- **Featured Models**:")
			count := 0
			for _, model := range provider.Models {
				if count >= 3 {
					break
				}
				fmt.Fprintf(f, "  - %s\n", model.Name)
				count++
			}
			if len(provider.Models) > 3 {
				fmt.Fprintf(f, "  - [View all %d models →](%s/)\n", len(provider.Models), string(provider.ID))
			}
		}
		fmt.Fprintln(f)
	}

	// Footer
	g.writeFooter(f, Breadcrumb{Label: "Back to Catalog", Path: "../"})

	return nil
}

// generateProviderReadme generates documentation for a single provider
func (g *Generator) generateProviderReadme(dir string, provider *catalogs.Provider, catalog catalogs.Reader) error {
	readmeFile := filepath.Join(dir, "README.md")
	f, err := os.Create(readmeFile)
	if err != nil {
		return fmt.Errorf("creating README: %w", err)
	}
	defer f.Close()

	// Header with logo if available
	logoPath := fmt.Sprintf("https://raw.githubusercontent.com/agentstation/starmap/master/internal/embedded/logos/%s.svg", provider.ID)
	fmt.Fprintf(f, "# <img src=\"%s\" alt=\"%s\" width=\"32\" height=\"32\" style=\"vertical-align: middle;\"> %s\n\n",
		logoPath, provider.Name, provider.Name)

	// Provider description
	desc := getProviderDescription(provider)
	if desc != "" {
		fmt.Fprintf(f, "%s\n\n", desc)
	}

	// Provider Information section
	fmt.Fprintln(f, "## Provider Information")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "| Field | Value |")
	fmt.Fprintln(f, "|-------|-------|")
	fmt.Fprintf(f, "| **Provider ID** | `%s` |\n", provider.ID)
	fmt.Fprintf(f, "| **Total Models** | %d |\n", len(provider.Models))

	if provider.APIKey != nil {
		fmt.Fprintf(f, "| **Authentication** | API Key Required |\n")
		fmt.Fprintf(f, "| **Environment Variable** | `%s` |\n", provider.APIKey.Name)
	} else {
		fmt.Fprintln(f, "| **Authentication** | None |")
	}

	if provider.StatusPageURL != nil {
		fmt.Fprintf(f, "| **Status Page** | [%s](%s) |\n", *provider.StatusPageURL, *provider.StatusPageURL)
	}

	// Add endpoints if available
	endpoints := catalog.Endpoints().List()
	if len(endpoints) > 0 {
		fmt.Fprintf(f, "| **Total Endpoints** | %d available |\n", len(endpoints))
	}

	fmt.Fprintln(f)

	// API Endpoints section
	if provider.Catalog != nil || provider.ChatCompletions != nil {
		hasEndpoints := false
		if provider.Catalog != nil && (provider.Catalog.DocsURL != nil || provider.Catalog.APIURL != nil) {
			hasEndpoints = true
		}
		if provider.ChatCompletions != nil && (provider.ChatCompletions.URL != nil || provider.ChatCompletions.HealthAPIURL != nil) {
			hasEndpoints = true
		}

		if hasEndpoints {
			fmt.Fprintln(f, "## 🔗 API Endpoints")
			fmt.Fprintln(f)

			if provider.Catalog != nil {
				if provider.Catalog.DocsURL != nil {
					fmt.Fprintf(f, "**Documentation**: [%s](%s)  \n", *provider.Catalog.DocsURL, *provider.Catalog.DocsURL)
				}
				if provider.Catalog.APIURL != nil {
					fmt.Fprintf(f, "**Models API**: [%s](%s)  \n", *provider.Catalog.APIURL, *provider.Catalog.APIURL)
				}
			}

			if provider.ChatCompletions != nil {
				if provider.ChatCompletions.URL != nil {
					fmt.Fprintf(f, "**Chat Completions**: [%s](%s)  \n", *provider.ChatCompletions.URL, *provider.ChatCompletions.URL)
				}
				if provider.ChatCompletions.HealthAPIURL != nil {
					fmt.Fprintf(f, "**Health API**: [%s](%s)  \n", *provider.ChatCompletions.HealthAPIURL, *provider.ChatCompletions.HealthAPIURL)
				}
			}

			fmt.Fprintln(f)
		}
	}

	// Privacy & Data Handling section
	if provider.PrivacyPolicy != nil {
		fmt.Fprintln(f, "## 🔒 Privacy & Data Handling")
		fmt.Fprintln(f)

		if provider.PrivacyPolicy.PrivacyPolicyURL != nil {
			fmt.Fprintf(f, "**Privacy Policy**: [%s](%s)  \n", *provider.PrivacyPolicy.PrivacyPolicyURL, *provider.PrivacyPolicy.PrivacyPolicyURL)
		}
		if provider.PrivacyPolicy.TermsOfServiceURL != nil {
			fmt.Fprintf(f, "**Terms of Service**: [%s](%s)  \n", *provider.PrivacyPolicy.TermsOfServiceURL, *provider.PrivacyPolicy.TermsOfServiceURL)
		}

		if provider.PrivacyPolicy.RetainsData != nil {
			retainsData := "No"
			if *provider.PrivacyPolicy.RetainsData {
				retainsData = "Yes"
			}
			fmt.Fprintf(f, "**Retains User Data**: %s  \n", retainsData)
		}

		if provider.PrivacyPolicy.TrainsOnData != nil {
			trainsOnData := "No"
			if *provider.PrivacyPolicy.TrainsOnData {
				trainsOnData = "Yes"
			}
			fmt.Fprintf(f, "**Trains on User Data**: %s  \n", trainsOnData)
		}

		fmt.Fprintln(f)
	}

	// Data Retention Policy section
	if provider.RetentionPolicy != nil {
		fmt.Fprintln(f, "## ⏱️ Data Retention Policy")
		fmt.Fprintln(f)

		// Policy type with capitalization
		policyType := string(provider.RetentionPolicy.Type)
		switch policyType {
		case "fixed":
			policyType = "Fixed Duration"
		case "none":
			policyType = "No Retention"
		case "indefinite":
			policyType = "Indefinite"
		case "conditional":
			policyType = "Conditional"
		}
		fmt.Fprintf(f, "**Policy Type**: %s  \n", policyType)

		// Duration
		duration := formatDuration(provider.RetentionPolicy.Duration)
		fmt.Fprintf(f, "**Retention Duration**: %s  \n", duration)

		// Details if available
		if provider.RetentionPolicy.Details != nil && *provider.RetentionPolicy.Details != "" {
			fmt.Fprintf(f, "**Details**: %s  \n", *provider.RetentionPolicy.Details)
		}

		fmt.Fprintln(f)
	}

	// Content Moderation section
	if provider.GovernancePolicy != nil {
		fmt.Fprintln(f, "## 🛡️ Content Moderation")
		fmt.Fprintln(f)

		if provider.GovernancePolicy.ModerationRequired != nil {
			requiresModeration := "No"
			if *provider.GovernancePolicy.ModerationRequired {
				requiresModeration = "Yes"
			}
			fmt.Fprintf(f, "**Requires Moderation**: %s  \n", requiresModeration)
		}

		if provider.GovernancePolicy.Moderated != nil {
			moderated := "No"
			if *provider.GovernancePolicy.Moderated {
				moderated = "Yes"
			}
			fmt.Fprintf(f, "**Content Moderated**: %s  \n", moderated)
		}

		if provider.GovernancePolicy.Moderator != nil && *provider.GovernancePolicy.Moderator != "" {
			// Capitalize the moderator name
			moderator := *provider.GovernancePolicy.Moderator
			if len(moderator) > 0 {
				moderator = strings.Title(moderator)
			}
			fmt.Fprintf(f, "**Moderated by**: %s  \n", moderator)
		}

		fmt.Fprintln(f)
	}

	// Headquarters info
	if provider.Headquarters != nil {
		fmt.Fprintln(f, "## 🏢 Headquarters")
		fmt.Fprintln(f)
		fmt.Fprintf(f, "%s\n\n", *provider.Headquarters)
	}

	// Available Models section
	fmt.Fprintln(f, "## Available Models")
	fmt.Fprintln(f)

	if len(provider.Models) == 0 {
		fmt.Fprintln(f, "*No models currently available from this provider.*")
	} else {
		// Convert map to slice
		var modelList []*catalogs.Model
		for _, model := range provider.Models {
			m := model // Copy to avoid reference issues
			modelList = append(modelList, &m)
		}

		// Group models by category/family
		modelGroups := groupModelsByFamily(modelList)

		for family, models := range modelGroups {
			if family != "" {
				fmt.Fprintf(f, "### %s\n\n", family)
			}

			fmt.Fprintln(f, "| Model | Context | Input | Output | Features |")
			fmt.Fprintln(f, "|-------|---------|-------|--------|----------|")

			// Sort models within family
			sort.Slice(models, func(i, j int) bool {
				return models[i].Name < models[j].Name
			})

			for _, model := range models {
				// Context window
				contextStr := "N/A"
				if model.Limits != nil && model.Limits.ContextWindow > 0 {
					contextStr = formatContext(model.Limits.ContextWindow)
				}

				// Pricing
				inputStr := "N/A"
				outputStr := "N/A"
				if model.Pricing != nil && model.Pricing.Tokens != nil {
					if model.Pricing.Tokens.Input != nil {
						if model.Pricing.Tokens.Input.Per1M == 0 {
							inputStr = "Free"
						} else {
							inputStr = fmt.Sprintf("$%.2f", model.Pricing.Tokens.Input.Per1M)
						}
					}
					if model.Pricing.Tokens.Output != nil {
						if model.Pricing.Tokens.Output.Per1M == 0 {
							outputStr = "Free"
						} else {
							outputStr = fmt.Sprintf("$%.2f", model.Pricing.Tokens.Output.Per1M)
						}
					}
				}

				// Features
				features := compactFeatures(*model)

				// Generate model link to local models subdirectory
				modelLink := fmt.Sprintf("[%s](./models/%s.md)", model.Name, formatModelID(model.ID))

				fmt.Fprintf(f, "| %s | %s | %s | %s | %s |\n",
					modelLink, contextStr, inputStr, outputStr, features)
			}

			fmt.Fprintln(f)
		}
	}

	// Configuration section
	fmt.Fprintln(f, "## Configuration")
	fmt.Fprintln(f)

	if provider.APIKey != nil {
		fmt.Fprintln(f, "### Authentication")
		fmt.Fprintln(f)
		fmt.Fprintln(f, "This provider requires an API key. Set it as an environment variable:")
		fmt.Fprintln(f)
		fmt.Fprintln(f, "```bash")
		fmt.Fprintf(f, "export %s=\"your-api-key-here\"\n", provider.APIKey.Name)
		fmt.Fprintln(f, "```")
		fmt.Fprintln(f)
	}

	fmt.Fprintln(f, "### Using with Starmap")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "```bash")
	fmt.Fprintln(f, "# List all models from this provider")
	fmt.Fprintf(f, "starmap list models --provider %s\n", provider.ID)
	fmt.Fprintln(f)
	fmt.Fprintln(f, "# Fetch latest models from provider API")
	fmt.Fprintf(f, "starmap fetch --provider %s\n", provider.ID)
	fmt.Fprintln(f)
	fmt.Fprintln(f, "# Sync provider data")
	fmt.Fprintf(f, "starmap sync --provider %s\n", provider.ID)
	fmt.Fprintln(f, "```")
	fmt.Fprintln(f)

	// Add cross-reference navigation
	crossRefs := g.buildProviderCrossReferences(string(provider.ID))
	if len(crossRefs) > 0 {
		g.writeNavigationSection(f, "See Also", crossRefs)
		fmt.Fprintln(f)
	}

	// Footer
	g.writeFooter(f,
		Breadcrumb{Label: "Back to Providers", Path: "../"},
		Breadcrumb{Label: "Back to Catalog", Path: "../../"})

	return nil
}

// groupModelsByFamily groups models by their family/category
func groupModelsByFamily(models []*catalogs.Model) map[string][]*catalogs.Model {
	groups := make(map[string][]*catalogs.Model)

	for _, model := range models {
		family := detectModelFamily(model.Name)
		groups[family] = append(groups[family], model)
	}

	return groups
}

// generateCompactFeatures generates a compact feature list for table display
func compactFeatures(model catalogs.Model) string {
	var features []string

	if model.Features != nil {
		if hasText(model.Features) {
			features = append(features, "📝")
		}
		if hasVision(model.Features) {
			features = append(features, "👁️")
		}
		if hasAudio(model.Features) {
			features = append(features, "🎵")
		}
		if hasVideo(model.Features) {
			features = append(features, "🎬")
		}
		if hasToolSupport(model.Features) {
			features = append(features, "🔧")
		}
		if model.Features.Streaming {
			features = append(features, "⚡")
		}
	}

	if len(features) == 0 {
		return "—"
	}
	return strings.Join(features, " ")
}

// generateProviderModelPages generates model documentation pages for a provider
func (g *Generator) generateProviderModelPages(dir string, provider *catalogs.Provider, catalog catalogs.Reader) error {
	// Skip if provider has no models
	if len(provider.Models) == 0 {
		return nil
	}

	// Ensure the directory exists
	if err := os.MkdirAll(dir, constants.DirPermissions); err != nil {
		return fmt.Errorf("creating models directory: %w", err)
	}

	// Get author map for cross-references
	authorMap := make(map[catalogs.AuthorID]*catalogs.Author)
	for _, author := range catalog.Authors().List() {
		authorMap[author.ID] = author
	}

	// Generate a page for each model
	for modelID, model := range provider.Models {
		// Use getModelFilePath to preserve subdirectory structure
		modelFile, err := getModelFilePath(dir, modelID)
		if err != nil {
			return fmt.Errorf("getting file path for model %s: %w", modelID, err)
		}
		modelCopy := model // Create a copy to avoid pointer issues
		if err := g.generateProviderModelPage(modelFile, &modelCopy, provider, authorMap); err != nil {
			return fmt.Errorf("generating model %s: %w", modelID, err)
		}
	}

	return nil
}

// generateProviderModelPage generates a single model page with provider context
func (g *Generator) generateProviderModelPage(filepath string, model *catalogs.Model, provider *catalogs.Provider, authorMap map[catalogs.AuthorID]*catalogs.Author) error {
	f, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("creating model file: %w", err)
	}
	defer f.Close()

	// Header with model name
	fmt.Fprintf(f, "# %s\n\n", model.Name)

	// Breadcrumb navigation
	// Calculate depth based on model ID (for subdirectories)
	breadcrumbs := g.providerModelBreadcrumb(provider.Name, model.Name, model.ID)
	g.writeBreadcrumbs(f, breadcrumbs...)

	// Description
	if model.Description != "" {
		fmt.Fprintf(f, "%s\n\n", model.Description)
	}

	// 📋 Overview Section
	fmt.Fprintln(f, "## 📋 Overview")
	fmt.Fprintln(f)
	fmt.Fprintf(f, "- **ID**: `%s`\n", model.ID)
	fmt.Fprintf(f, "- **Provider**: [%s](../)\n", provider.Name)

	// Authors with links
	if len(model.Authors) > 0 {
		authorLinks := []string{}
		for _, author := range model.Authors {
			if a, ok := authorMap[author.ID]; ok {
				authorLinks = append(authorLinks,
					fmt.Sprintf("[%s](../../../authors/%s/)", a.Name, string(a.ID)))
			}
		}
		fmt.Fprintf(f, "- **Authors**: %s\n", strings.Join(authorLinks, ", "))
	}

	// Quick stats from metadata and limits
	if model.Metadata != nil {
		if !model.Metadata.ReleaseDate.IsZero() {
			fmt.Fprintf(f, "- **Release Date**: %s\n", model.Metadata.ReleaseDate.Format("2006-01-02"))
		}
		if model.Metadata.KnowledgeCutoff != nil {
			fmt.Fprintf(f, "- **Knowledge Cutoff**: %s\n", model.Metadata.KnowledgeCutoff.Format("2006-01-02"))
		}
		fmt.Fprintf(f, "- **Open Weights**: %t\n", model.Metadata.OpenWeights)
	}

	if model.Limits != nil {
		if model.Limits.ContextWindow > 0 {
			fmt.Fprintf(f, "- **Context Window**: %s tokens\n", formatNumber(int(model.Limits.ContextWindow)))
		}
		if model.Limits.OutputTokens > 0 {
			fmt.Fprintf(f, "- **Max Output**: %s tokens\n", formatNumber(int(model.Limits.OutputTokens)))
		}
	}

	// Architecture info if available
	if model.Metadata != nil && model.Metadata.Architecture != nil {
		if model.Metadata.Architecture.ParameterCount != "" {
			fmt.Fprintf(f, "- **Parameters**: %s\n", model.Metadata.Architecture.ParameterCount)
		}
	}

	fmt.Fprintln(f)

	// 🔬 Technical Specifications Section with shields.io badges
	if model.Features != nil {
		badges := technicalSpecBadges(model)
		if badges != "" {
			fmt.Fprintln(f, "## 🔬 Technical Specifications")
			fmt.Fprintln(f)
			fmt.Fprintln(f, badges)
			fmt.Fprintln(f)
		}
	}

	// 🎯 Capabilities Section
	fmt.Fprintln(f, "## 🎯 Capabilities")
	fmt.Fprintln(f)

	// Feature Overview with comprehensive badges
	if model.Features != nil {
		badges := featureBadges(model)
		if badges != "" {
			fmt.Fprintln(f, "### Feature Overview")
			fmt.Fprintln(f)
			fmt.Fprintln(f, badges)
			fmt.Fprintln(f)
		}
	}

	// Input/Output Modalities Table
	fmt.Fprintln(f, "### Input/Output Modalities")
	fmt.Fprintln(f)
	writeModalityTable(f, model)

	// Core Features Table
	fmt.Fprintln(f, "### Core Features")
	fmt.Fprintln(f)
	writeCoreFeatureTable(f, model)

	// Response Delivery Table
	fmt.Fprintln(f, "### Response Delivery")
	fmt.Fprintln(f)
	writeResponseDeliveryTable(f, model)

	// Advanced Reasoning Table (only if applicable)
	writeAdvancedReasoningTable(f, model)

	// 🎛️ Generation Controls section
	fmt.Fprintln(f, "## 🎛️ Generation Controls")
	fmt.Fprintln(f)

	// Architecture table (if available)
	writeArchitectureTable(f, model)

	// Model Tags table (if available)
	writeTagsTable(f, model)

	// Generation Controls tables
	writeControlsTables(f, model)

	// 💰 Pricing section (Provider-specific pricing)
	fmt.Fprintln(f, "## 💰 Pricing")
	fmt.Fprintln(f)
	fmt.Fprintf(f, "_Pricing shown for %s_\n\n", provider.Name)

	// Token Pricing Table
	writeTokenPricingTable(f, model)

	// Operation Pricing Table (if applicable)
	writeOperationPricingTable(f, model)

	// Cost Calculator and Example Costs
	writeCostCalculator(f, model)
	writeExampleCosts(f, model)

	// Advanced Features section
	hasAdvancedFeatures := false
	advancedSection := strings.Builder{}
	advancedSection.WriteString("## 🚀 Advanced Features\n\n")

	// Tool configuration
	if model.Tools != nil && len(model.Tools.ToolChoices) > 0 {
		advancedSection.WriteString("### Tool Configuration\n\n")
		var choices []string
		for _, choice := range model.Tools.ToolChoices {
			choices = append(choices, string(choice))
		}
		advancedSection.WriteString(fmt.Sprintf("**Supported Tool Choices**: %s\n\n", strings.Join(choices, ", ")))
		hasAdvancedFeatures = true
	}

	// Attachments support
	if model.Attachments != nil {
		advancedSection.WriteString("### File Attachments\n\n")
		if len(model.Attachments.MimeTypes) > 0 {
			advancedSection.WriteString(fmt.Sprintf("**Supported Types**: %s\n", strings.Join(model.Attachments.MimeTypes, ", ")))
		}
		if model.Attachments.MaxFileSize != nil {
			advancedSection.WriteString(fmt.Sprintf("**Max File Size**: %s bytes\n", formatNumber(int(*model.Attachments.MaxFileSize))))
		}
		if model.Attachments.MaxFiles != nil {
			advancedSection.WriteString(fmt.Sprintf("**Max Files**: %d per request\n", *model.Attachments.MaxFiles))
		}
		advancedSection.WriteString("\n")
		hasAdvancedFeatures = true
	}

	// Delivery options
	if model.Delivery != nil && (len(model.Delivery.Formats) > 0 || len(model.Delivery.Streaming) > 0 || len(model.Delivery.Protocols) > 0) {
		advancedSection.WriteString("### Response Delivery Options\n\n")
		if len(model.Delivery.Formats) > 0 {
			var formats []string
			for _, format := range model.Delivery.Formats {
				formats = append(formats, string(format))
			}
			advancedSection.WriteString(fmt.Sprintf("**Response Formats**: %s\n", strings.Join(formats, ", ")))
		}
		if len(model.Delivery.Streaming) > 0 {
			var streaming []string
			for _, stream := range model.Delivery.Streaming {
				streaming = append(streaming, string(stream))
			}
			advancedSection.WriteString(fmt.Sprintf("**Streaming Modes**: %s\n", strings.Join(streaming, ", ")))
		}
		if len(model.Delivery.Protocols) > 0 {
			var protocols []string
			for _, protocol := range model.Delivery.Protocols {
				protocols = append(protocols, string(protocol))
			}
			advancedSection.WriteString(fmt.Sprintf("**Protocols**: %s\n", strings.Join(protocols, ", ")))
		}
		advancedSection.WriteString("\n")
		hasAdvancedFeatures = true
	}

	if hasAdvancedFeatures {
		fmt.Fprint(f, advancedSection.String())
	}

	// Metadata section
	fmt.Fprintln(f, "## 📋 Metadata")
	fmt.Fprintln(f)
	fmt.Fprintf(f, "**Created**: %s\n", model.CreatedAt.Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(f, "**Last Updated**: %s\n", model.UpdatedAt.Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintln(f)

	// Navigation section
	fmt.Fprintln(f, "---")
	fmt.Fprintln(f)

	// Build navigation links
	navLinks := []NavigationLink{}
	navLinks = append(navLinks, NavigationLink{
		Label: fmt.Sprintf("More models by %s", provider.Name),
		Path:  "../",
	})

	// Add author links if available
	if len(model.Authors) > 0 {
		for _, author := range model.Authors {
			if a, ok := authorMap[author.ID]; ok {
				// Calculate depth for proper path
				depth := 4
				if strings.Contains(model.ID, "/") {
					depth += strings.Count(model.ID, "/")
				}
				authorsPath := getAuthorsPath(depth)
				navLinks = append(navLinks, NavigationLink{
					Label: fmt.Sprintf("More models by %s", a.Name),
					Path:  fmt.Sprintf("%s/%s/", authorsPath, string(a.ID)),
				})
			}
		}
	}

	// Add cross-references
	depth := 4
	if strings.Contains(model.ID, "/") {
		depth += strings.Count(model.ID, "/")
	}
	crossRefs := g.buildModelCrossReferences("provider", depth)
	navLinks = append(navLinks, crossRefs...)

	// Write navigation section
	g.writeNavigationSection(f, "Navigation", navLinks)

	// Use timestamped footer helper
	g.writeTimestampedFooter(f)

	return nil
}

// getProviderDescription returns a description for the provider
func getProviderDescription(provider *catalogs.Provider) string {
	descriptions := map[catalogs.ProviderID]string{
		"openai":           "Industry-leading AI models including GPT-4 and DALL-E, pioneering AGI research.",
		"anthropic":        "Creator of Claude, focusing on safe and beneficial AI with constitutional training.",
		"google-ai-studio": "Google's AI platform offering Gemini models with multimodal capabilities.",
		"google-vertex":    "Enterprise AI platform on Google Cloud with Gemini and PaLM models.",
		"groq":             "Ultra-fast inference with custom LPU hardware, offering low-latency model serving.",
		"deepseek":         "Chinese AI company specializing in efficient, high-performance language models.",
		"cerebras":         "Fastest inference speeds using revolutionary wafer-scale computing technology.",
		"xai":              "Elon Musk's AI company building truthful AI systems with Grok models.",
		"mistral":          "European AI leader offering efficient open-source and commercial models.",
		"cohere":           "Enterprise-focused AI with models optimized for business applications.",
		"ai21":             "Advanced language models with focus on controllability and reliability.",
	}

	if desc, ok := descriptions[provider.ID]; ok {
		return desc
	}
	return ""
}
