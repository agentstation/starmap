package docs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	md "github.com/nao1215/markdown"

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

	// Write to both README.md (for GitHub) and _index.md (for Hugo)
	for _, filename := range []string{"README.md", "_index.md"} {
		indexFile := filepath.Join(dir, filename)
		f, err := os.Create(indexFile)
		if err != nil {
			return fmt.Errorf("creating provider index %s: %w", filename, err)
		}
		if err := g.writeProviderIndex(f, providers); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}

	return nil
}

// writeProviderIndex writes the provider index content using markdown builder
func (g *Generator) writeProviderIndex(w io.Writer, providers []*catalogs.Provider) error {
	markdown := NewMarkdown(w)

	// Add Hugo front matter
	markdown.HugoFrontMatter("Provider Overview", 1)

	markdown.H1("🏢 AI Model Providers").
		LF().
		PlainText("Organizations that host and serve AI models through APIs.").
		LF().LF()

	// Provider comparison table
	markdown.H2("Provider Comparison").LF()

	// Sort providers by model count for better presentation
	sort.Slice(providers, func(i, j int) bool {
		return len(providers[i].Models) > len(providers[j].Models)
	})

	g.writeProviderComparisonTableToMarkdown(markdown, providers)
	markdown.LF()

	// Add top models overview across all providers
	markdown.H2("🌟 Top Models Across Providers").
		LF().
		PlainText("Overview of popular models available across different providers:").
		LF().LF()

	// Collect all models from all providers
	var allModels []*catalogs.Model
	for _, provider := range providers {
		// Use sorted models for deterministic iteration
		sortedModels := SortedModels(provider.Models)
		for _, model := range sortedModels {
			allModels = append(allModels, model)
		}
	}

	// Show top 20 models
	if len(allModels) > 0 {
		g.writeModelsOverviewTableToMarkdown(markdown, allModels, providers)
	}
	markdown.LF()

	// Provider details section
	markdown.H2("Provider Details").LF()

	// Sort providers alphabetically for the detailed list
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].ID < providers[j].ID
	})

	for _, provider := range providers {
		// Provider heading with logo and link - using GitHub-compatible directory paths
		// Works on both GitHub (auto-finds README.md) and Hugo (with permalink config)
		logoPath := fmt.Sprintf("./%s/logo.svg", provider.ID)
		// Use just the provider ID without ./ prefix
		providerLink := fmt.Sprintf("%s/", provider.ID)

		// Use markdown builder to create the heading with embedded image and link
		// This ensures proper ordering and avoids buffering issues
		heading := fmt.Sprintf(`<img src="%s" alt="" width="16" height="16" style="vertical-align: middle"> [%s](%s)`,
			logoPath, provider.Name, providerLink)
		markdown.H3(heading).LF()

		// Provider description (if we had it)
		desc := getProviderDescription(provider)
		if desc != "" {
			markdown.PlainText(desc).LF().LF()
		}

		// Quick stats using bullet lists
		markdown.BulletList(
			fmt.Sprintf("**Models**: %d available", len(provider.Models)),
		)

		if provider.APIKey != nil {
			markdown.BulletList(
				fmt.Sprintf("**API Key**: Required (`%s`)", provider.APIKey.Name),
			)
		}
		if provider.StatusPageURL != nil {
			markdown.BulletList(
				fmt.Sprintf("**Status**: [Check current status](%s)", *provider.StatusPageURL),
			)
		}

		// Top models preview
		if len(provider.Models) > 0 {
			markdown.BulletList("**Featured Models**:")
			// Use sorted models for deterministic iteration
			sortedModels := SortedModels(provider.Models)
			count := 0
			for _, model := range sortedModels {
				if count >= 3 {
					break
				}
				modelName := model.Name
				if modelName == "" {
					modelName = model.ID
				}
				markdown.BulletList(fmt.Sprintf("  - %s", modelName))
				count++
			}
			if len(provider.Models) > 3 {
				// Use Hugo-compatible link to provider page
				markdown.BulletList(fmt.Sprintf("  - [View all %d models →](%s)", len(provider.Models), providerLink))
			}
		}
		markdown.LF()
	}

	// Footer
	g.writeFooterToMarkdown(markdown, Breadcrumb{Label: "Back to Catalog", Path: "../"})

	return markdown.Build()
}

// writeProviderComparisonTableToMarkdown adds provider comparison table to markdown builder
func (g *Generator) writeProviderComparisonTableToMarkdown(markdown *Markdown, providers []*catalogs.Provider) {
	// Create a temporary writer to capture the table output
	var buf strings.Builder
	writeProviderComparisonTable(&buf, providers)
	markdown.PlainText(buf.String())
}

// writeModelsOverviewTableToMarkdown adds models overview table to markdown builder
func (g *Generator) writeModelsOverviewTableToMarkdown(markdown *Markdown, allModels []*catalogs.Model, providers []*catalogs.Provider) {
	// Create a temporary writer to capture the table output
	var buf strings.Builder
	writeModelsOverviewTable(&buf, allModels, providers)
	markdown.PlainText(buf.String())
}

// generateProviderReadme generates documentation for a single provider
func (g *Generator) generateProviderReadme(dir string, provider *catalogs.Provider, catalog catalogs.Reader) error {
	// Write to both README.md (for GitHub) and _index.md (for Hugo)
	for _, filename := range []string{"README.md", "_index.md"} {
		readmeFile := filepath.Join(dir, filename)
		f, err := os.Create(readmeFile)
		if err != nil {
			return fmt.Errorf("creating %s: %w", filename, err)
		}
		if err := g.writeProviderReadme(f, provider, catalog); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}

	return nil
}

// writeProviderReadme writes the provider readme content using markdown builder
func (g *Generator) writeProviderReadme(w io.Writer, provider *catalogs.Provider, catalog catalogs.Reader) error {
	markdown := NewMarkdown(w)

	// Header with logo if available
	logoPath := fmt.Sprintf("https://raw.githubusercontent.com/agentstation/starmap/master/internal/embedded/catalog/providers/%s/logo.svg", provider.ID)
	headerText := fmt.Sprintf("<img src=\"%s\" alt=\"%s logo\" width=\"48\" height=\"48\" style=\"vertical-align: middle;\"> %s",
		logoPath, provider.Name, provider.Name)
	markdown.H1(headerText).LF()

	// Provider description
	desc := getProviderDescription(provider)
	if desc != "" {
		markdown.PlainText(desc).LF().LF()
	}

	// Provider Information section
	markdown.H2("Provider Information").LF()

	// Build provider info table
	headers := []string{"Field", "Value"}
	rows := [][]string{
		{"**Provider ID**", fmt.Sprintf("`%s`", provider.ID)},
		{"**Total Models**", fmt.Sprintf("%d", len(provider.Models))},
	}

	if provider.APIKey != nil {
		rows = append(rows, []string{"**Authentication**", "API Key Required"})
		rows = append(rows, []string{"**Environment Variable**", fmt.Sprintf("`%s`", provider.APIKey.Name)})
	} else {
		rows = append(rows, []string{"**Authentication**", "None"})
	}

	if provider.StatusPageURL != nil {
		rows = append(rows, []string{"**Status Page**", fmt.Sprintf("[%s](%s)", *provider.StatusPageURL, *provider.StatusPageURL)})
	}

	// Add endpoints if available
	endpoints := catalog.Endpoints().List()
	if len(endpoints) > 0 {
		rows = append(rows, []string{"**Total Endpoints**", fmt.Sprintf("%d available", len(endpoints))})
	}

	markdown.Table(md.TableSet{Header: headers, Rows: rows}).LF()

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
			markdown.H2("🔗 API Endpoints").LF()

			if provider.Catalog != nil {
				if provider.Catalog.DocsURL != nil {
					markdown.PlainText(fmt.Sprintf("**Documentation**: [%s](%s)  ", *provider.Catalog.DocsURL, *provider.Catalog.DocsURL)).LF()
				}
				if provider.Catalog.APIURL != nil {
					markdown.PlainText(fmt.Sprintf("**Models API**: [%s](%s)  ", *provider.Catalog.APIURL, *provider.Catalog.APIURL)).LF()
				}
			}

			if provider.ChatCompletions != nil {
				if provider.ChatCompletions.URL != nil {
					markdown.PlainText(fmt.Sprintf("**Chat Completions**: [%s](%s)  ", *provider.ChatCompletions.URL, *provider.ChatCompletions.URL)).LF()
				}
				if provider.ChatCompletions.HealthAPIURL != nil {
					markdown.PlainText(fmt.Sprintf("**Health API**: [%s](%s)  ", *provider.ChatCompletions.HealthAPIURL, *provider.ChatCompletions.HealthAPIURL)).LF()
				}
			}

			markdown.LF()
		}
	}

	// Privacy & Data Handling section
	if provider.PrivacyPolicy != nil {
		markdown.H2("🔒 Privacy & Data Handling").LF()

		if provider.PrivacyPolicy.PrivacyPolicyURL != nil {
			markdown.PlainText(fmt.Sprintf("**Privacy Policy**: [%s](%s)  ", *provider.PrivacyPolicy.PrivacyPolicyURL, *provider.PrivacyPolicy.PrivacyPolicyURL)).LF()
		}
		if provider.PrivacyPolicy.TermsOfServiceURL != nil {
			markdown.PlainText(fmt.Sprintf("**Terms of Service**: [%s](%s)  ", *provider.PrivacyPolicy.TermsOfServiceURL, *provider.PrivacyPolicy.TermsOfServiceURL)).LF()
		}

		if provider.PrivacyPolicy.RetainsData != nil {
			retainsData := "No"
			if *provider.PrivacyPolicy.RetainsData {
				retainsData = "Yes"
			}
			markdown.PlainText(fmt.Sprintf("**Retains User Data**: %s  ", retainsData)).LF()
		}

		if provider.PrivacyPolicy.TrainsOnData != nil {
			trainsOnData := "No"
			if *provider.PrivacyPolicy.TrainsOnData {
				trainsOnData = "Yes"
			}
			markdown.PlainText(fmt.Sprintf("**Trains on User Data**: %s  ", trainsOnData)).LF()
		}

		markdown.LF()
	}

	// Data Retention Policy section
	if provider.RetentionPolicy != nil {
		markdown.H2("⏱️ Data Retention Policy").LF()

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
		markdown.PlainText(fmt.Sprintf("**Policy Type**: %s  ", policyType)).LF()

		// Duration
		duration := formatDuration(provider.RetentionPolicy.Duration)
		markdown.PlainText(fmt.Sprintf("**Retention Duration**: %s  ", duration)).LF()

		// Details if available
		if provider.RetentionPolicy.Details != nil && *provider.RetentionPolicy.Details != "" {
			markdown.PlainText(fmt.Sprintf("**Details**: %s  ", *provider.RetentionPolicy.Details)).LF()
		}

		markdown.LF()
	}

	// Content Moderation section
	if provider.GovernancePolicy != nil {
		markdown.H2("🛡️ Content Moderation").LF()

		if provider.GovernancePolicy.ModerationRequired != nil {
			requiresModeration := "No"
			if *provider.GovernancePolicy.ModerationRequired {
				requiresModeration = "Yes"
			}
			markdown.PlainText(fmt.Sprintf("**Requires Moderation**: %s  ", requiresModeration)).LF()
		}

		if provider.GovernancePolicy.Moderated != nil {
			moderated := "No"
			if *provider.GovernancePolicy.Moderated {
				moderated = "Yes"
			}
			markdown.PlainText(fmt.Sprintf("**Content Moderated**: %s  ", moderated)).LF()
		}

		if provider.GovernancePolicy.Moderator != nil && *provider.GovernancePolicy.Moderator != "" {
			// Capitalize the moderator name
			moderator := *provider.GovernancePolicy.Moderator
			if len(moderator) > 0 {
				moderator = strings.Title(moderator)
			}
			markdown.PlainText(fmt.Sprintf("**Moderated by**: %s  ", moderator)).LF()
		}

		markdown.LF()
	}

	// Headquarters info
	if provider.Headquarters != nil {
		markdown.H2("🏢 Headquarters").LF().
			PlainText(*provider.Headquarters).LF().LF()
	}

	// Available Models section
	markdown.H2("Available Models").LF()

	if len(provider.Models) == 0 {
		markdown.Italic("No models currently available from this provider.").LF()
	} else {
		// Use sorted models for deterministic iteration
		modelList := SortedModels(provider.Models)

		// Group models by category/family
		modelGroups := groupModelsByFamily(modelList)

		// Sort families for deterministic ordering
		var families []string
		for family := range modelGroups {
			families = append(families, family)
		}
		sort.Strings(families)

		for _, family := range families {
			models := modelGroups[family]
			if family != "" {
				markdown.H3(family).LF()
			}

			// Build model table
			headers := []string{"Model", "Context", "Input", "Output", "Features"}
			var rows [][]string

			// Sort models within family (make a copy to avoid modifying the original)
			modelsCopy := make([]*catalogs.Model, len(models))
			copy(modelsCopy, models)
			sort.Slice(modelsCopy, func(i, j int) bool {
				return modelsCopy[i].Name < modelsCopy[j].Name
			})

			for _, model := range modelsCopy {
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
				modelName := model.Name
				if modelName == "" {
					modelName = model.ID
				}
				modelLink := fmt.Sprintf("[%s](./models/%s.md)", modelName, formatModelID(model.ID))

				rows = append(rows, []string{
					modelLink, contextStr, inputStr, outputStr, features,
				})
			}

			markdown.Table(md.TableSet{Header: headers, Rows: rows}).LF()
		}
	}

	// Configuration section
	markdown.H2("Configuration").LF()

	if provider.APIKey != nil {
		markdown.H3("Authentication").LF().
			PlainText("This provider requires an API key. Set it as an environment variable:").
			LF().LF().
			CodeBlock("bash", fmt.Sprintf("export %s=\"your-api-key-here\"", provider.APIKey.Name)).
			LF()
	}

	markdown.H3("Using with Starmap").LF().
		CodeBlock("bash", fmt.Sprintf(`# List all models from this provider
starmap list models --provider %s

# Fetch latest models from provider API
starmap fetch --provider %s

# Sync provider data
starmap sync --provider %s`, provider.ID, provider.ID, provider.ID)).
		LF()

	// Add cross-reference navigation
	crossRefs := g.buildProviderCrossReferences(string(provider.ID))
	if len(crossRefs) > 0 {
		g.writeNavigationSectionToMarkdown(markdown, "See Also", crossRefs)
		markdown.LF()
	}

	// Footer
	g.writeFooterToMarkdown(markdown,
		Breadcrumb{Label: "Back to Providers", Path: "../"},
		Breadcrumb{Label: "Back to Catalog", Path: "../../"})

	return markdown.Build()
}

// groupModelsByFamily groups models by their family/category
func groupModelsByFamily(models []*catalogs.Model) map[string][]*catalogs.Model {
	groups := make(map[string][]*catalogs.Model)

	for _, model := range models {
		modelName := model.Name
		if modelName == "" {
			modelName = model.ID
		}
		family := detectModelFamily(modelName)
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
	// Use sorted models for deterministic iteration
	sortedModels := SortedModels(provider.Models)
	for _, model := range sortedModels {
		// Use getModelFilePath to preserve subdirectory structure
		modelFile, err := getModelFilePath(dir, model.ID)
		if err != nil {
			return fmt.Errorf("getting file path for model %s: %w", model.ID, err)
		}
		if err := g.generateProviderModelPage(modelFile, model, provider, authorMap); err != nil {
			return fmt.Errorf("generating model %s: %w", model.ID, err)
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

	return g.writeProviderModelPage(f, model, provider, authorMap)
}

// writeProviderModelPage writes the provider model page content using markdown builder
func (g *Generator) writeProviderModelPage(w io.Writer, model *catalogs.Model, provider *catalogs.Provider, authorMap map[catalogs.AuthorID]*catalogs.Author) error {
	markdown := NewMarkdown(w)

	// Header with model name - use ID as fallback if name is empty
	modelName := model.Name
	if modelName == "" {
		modelName = model.ID
	}
	markdown.H1(modelName).LF()

	// Breadcrumb navigation
	// Calculate depth based on model ID (for subdirectories)
	breadcrumbs := g.providerModelBreadcrumb(provider.Name, modelName, model.ID)
	g.writeBreadcrumbsToMarkdown(markdown, breadcrumbs...)

	// Description
	if model.Description != "" {
		markdown.PlainText(model.Description).LF().LF()
	}

	// 📋 Overview Section
	markdown.H2("📋 Overview").LF()

	overviewItems := []string{
		fmt.Sprintf("**ID**: `%s`", model.ID),
		fmt.Sprintf("**Provider**: [%s](../)", provider.Name),
	}

	// Authors with links
	if len(model.Authors) > 0 {
		authorLinks := []string{}
		for _, author := range model.Authors {
			if a, ok := authorMap[author.ID]; ok {
				authorLinks = append(authorLinks,
					fmt.Sprintf("[%s](../../../authors/%s/)", a.Name, string(a.ID)))
			}
		}
		overviewItems = append(overviewItems, fmt.Sprintf("**Authors**: %s", strings.Join(authorLinks, ", ")))
	}

	// Quick stats from metadata and limits
	if model.Metadata != nil {
		if !model.Metadata.ReleaseDate.IsZero() {
			overviewItems = append(overviewItems, fmt.Sprintf("**Release Date**: %s", model.Metadata.ReleaseDate.Format("2006-01-02")))
		}
		if model.Metadata.KnowledgeCutoff != nil {
			overviewItems = append(overviewItems, fmt.Sprintf("**Knowledge Cutoff**: %s", model.Metadata.KnowledgeCutoff.Format("2006-01-02")))
		}
		overviewItems = append(overviewItems, fmt.Sprintf("**Open Weights**: %t", model.Metadata.OpenWeights))
	}

	if model.Limits != nil {
		if model.Limits.ContextWindow > 0 {
			overviewItems = append(overviewItems, fmt.Sprintf("**Context Window**: %s tokens", formatNumber(int(model.Limits.ContextWindow))))
		}
		if model.Limits.OutputTokens > 0 {
			overviewItems = append(overviewItems, fmt.Sprintf("**Max Output**: %s tokens", formatNumber(int(model.Limits.OutputTokens))))
		}
	}

	// Architecture info if available
	if model.Metadata != nil && model.Metadata.Architecture != nil {
		if model.Metadata.Architecture.ParameterCount != "" {
			overviewItems = append(overviewItems, fmt.Sprintf("**Parameters**: %s", model.Metadata.Architecture.ParameterCount))
		}
	}

	for _, item := range overviewItems {
		markdown.BulletList(item)
	}
	markdown.LF()

	// 🔬 Technical Specifications Section with shields.io badges
	if model.Features != nil {
		badges := technicalSpecBadges(model)
		if badges != "" {
			markdown.H2("🔬 Technical Specifications").LF().
				PlainText(badges).LF().LF()
		}
	}

	// 🎯 Capabilities Section
	markdown.H2("🎯 Capabilities").LF()

	// Feature Overview with comprehensive badges
	if model.Features != nil {
		badges := featureBadges(model)
		if badges != "" {
			markdown.H3("Feature Overview").LF().
				PlainText(badges).LF().LF()
		}
	}

	// Input/Output Modalities Table
	markdown.H3("Input/Output Modalities").LF()
	g.writeModalityTableToMarkdown(markdown, model)

	// Core Features Table
	markdown.H3("Core Features").LF()
	g.writeCoreFeatureTableToMarkdown(markdown, model)

	// Response Delivery Table
	markdown.H3("Response Delivery").LF()
	g.writeResponseDeliveryTableToMarkdown(markdown, model)

	// Advanced Reasoning Table (only if applicable)
	g.writeAdvancedReasoningTableToMarkdown(markdown, model)

	// 🎛️ Generation Controls section
	markdown.H2("🎛️ Generation Controls").LF()

	// Architecture table (if available)
	g.writeArchitectureTableToMarkdown(markdown, model)

	// Model Tags table (if available)
	g.writeTagsTableToMarkdown(markdown, model)

	// Generation Controls tables
	g.writeControlsTablesToMarkdown(markdown, model)

	// 💰 Pricing section (Provider-specific pricing)
	markdown.H2("💰 Pricing").LF().
		Italic(fmt.Sprintf("Pricing shown for %s", provider.Name)).LF().LF()

	// Token Pricing Table
	g.writeTokenPricingTableToMarkdown(markdown, model)

	// Operation Pricing Table (if applicable)
	g.writeOperationPricingTableToMarkdown(markdown, model)

	// Cost Calculator and Example Costs
	g.writeCostCalculatorToMarkdown(markdown, model)
	g.writeExampleCostsToMarkdown(markdown, model)

	// Advanced Features section
	hasAdvancedFeatures := false
	markdownBuffer := NewMarkdownBuffer()
	markdownBuffer.H2("🚀 Advanced Features").LF()

	// Tool configuration
	if model.Tools != nil && len(model.Tools.ToolChoices) > 0 {
		markdownBuffer.H3("Tool Configuration").LF()
		var choices []string
		for _, choice := range model.Tools.ToolChoices {
			choices = append(choices, string(choice))
		}
		markdownBuffer.PlainText(fmt.Sprintf("**Supported Tool Choices**: %s", strings.Join(choices, ", "))).LF().LF()
		hasAdvancedFeatures = true
	}

	// Attachments support
	if model.Attachments != nil {
		markdownBuffer.H3("File Attachments").LF()
		if len(model.Attachments.MimeTypes) > 0 {
			markdownBuffer.PlainText(fmt.Sprintf("**Supported Types**: %s", strings.Join(model.Attachments.MimeTypes, ", "))).LF()
		}
		if model.Attachments.MaxFileSize != nil {
			markdownBuffer.PlainText(fmt.Sprintf("**Max File Size**: %s bytes", formatNumber(int(*model.Attachments.MaxFileSize)))).LF()
		}
		if model.Attachments.MaxFiles != nil {
			markdownBuffer.PlainText(fmt.Sprintf("**Max Files**: %d per request", *model.Attachments.MaxFiles)).LF()
		}
		markdownBuffer.LF()
		hasAdvancedFeatures = true
	}

	// Delivery options
	if model.Delivery != nil && (len(model.Delivery.Formats) > 0 || len(model.Delivery.Streaming) > 0 || len(model.Delivery.Protocols) > 0) {
		markdownBuffer.H3("Response Delivery Options").LF()
		if len(model.Delivery.Formats) > 0 {
			var formats []string
			for _, format := range model.Delivery.Formats {
				formats = append(formats, string(format))
			}
			markdownBuffer.PlainText(fmt.Sprintf("**Response Formats**: %s", strings.Join(formats, ", "))).LF()
		}
		if len(model.Delivery.Streaming) > 0 {
			var streaming []string
			for _, stream := range model.Delivery.Streaming {
				streaming = append(streaming, string(stream))
			}
			markdownBuffer.PlainText(fmt.Sprintf("**Streaming Modes**: %s", strings.Join(streaming, ", "))).LF()
		}
		if len(model.Delivery.Protocols) > 0 {
			var protocols []string
			for _, protocol := range model.Delivery.Protocols {
				protocols = append(protocols, string(protocol))
			}
			markdownBuffer.PlainText(fmt.Sprintf("**Protocols**: %s", strings.Join(protocols, ", "))).LF()
		}
		markdownBuffer.LF()
		hasAdvancedFeatures = true
	}

	if hasAdvancedFeatures {
		markdownBuffer.Build()
		markdown.PlainText(markdownBuffer.String())
	}

	// Metadata section
	markdown.H2("📋 Metadata").LF().
		PlainText(fmt.Sprintf("**Created**: %s", model.CreatedAt.Format("2006-01-02 15:04:05 UTC"))).LF().
		PlainText(fmt.Sprintf("**Last Updated**: %s", model.UpdatedAt.Format("2006-01-02 15:04:05 UTC"))).LF().LF().
		PlainText("---").LF().LF()

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
	g.writeNavigationSectionToMarkdown(markdown, "Navigation", navLinks)

	// Use timestamped footer helper
	g.writeTimestampedFooterToMarkdown(markdown)

	return markdown.Build()
}

// Helper methods to write tables and other content to builder
// These methods would need to be implemented to capture the table output
// and add it to the builder. For now, they'll use temporary string builders.

func (g *Generator) writeModalityTableToMarkdown(markdown *Markdown, model *catalogs.Model) {
	var buf strings.Builder
	writeModalityTable(&buf, model)
	markdown.PlainText(buf.String())
}

func (g *Generator) writeCoreFeatureTableToMarkdown(markdown *Markdown, model *catalogs.Model) {
	var buf strings.Builder
	writeCoreFeatureTable(&buf, model)
	markdown.PlainText(buf.String())
}

func (g *Generator) writeResponseDeliveryTableToMarkdown(markdown *Markdown, model *catalogs.Model) {
	var buf strings.Builder
	writeResponseDeliveryTable(&buf, model)
	markdown.PlainText(buf.String())
}

func (g *Generator) writeAdvancedReasoningTableToMarkdown(markdown *Markdown, model *catalogs.Model) {
	var buf strings.Builder
	writeAdvancedReasoningTable(&buf, model)
	if buf.Len() > 0 {
		markdown.PlainText(buf.String())
	}
}

func (g *Generator) writeArchitectureTableToMarkdown(markdown *Markdown, model *catalogs.Model) {
	var buf strings.Builder
	writeArchitectureTable(&buf, model)
	if buf.Len() > 0 {
		markdown.PlainText(buf.String())
	}
}

func (g *Generator) writeTagsTableToMarkdown(markdown *Markdown, model *catalogs.Model) {
	var buf strings.Builder
	writeTagsTable(&buf, model)
	if buf.Len() > 0 {
		markdown.PlainText(buf.String())
	}
}

func (g *Generator) writeControlsTablesToMarkdown(markdown *Markdown, model *catalogs.Model) {
	var buf strings.Builder
	writeControlsTables(&buf, model)
	if buf.Len() > 0 {
		markdown.PlainText(buf.String())
	}
}

func (g *Generator) writeTokenPricingTableToMarkdown(markdown *Markdown, model *catalogs.Model) {
	var buf strings.Builder
	writeTokenPricingTable(&buf, model)
	markdown.PlainText(buf.String())
}

func (g *Generator) writeOperationPricingTableToMarkdown(markdown *Markdown, model *catalogs.Model) {
	var buf strings.Builder
	writeOperationPricingTable(&buf, model)
	if buf.Len() > 0 {
		markdown.PlainText(buf.String())
	}
}

func (g *Generator) writeCostCalculatorToMarkdown(markdown *Markdown, model *catalogs.Model) {
	var buf strings.Builder
	writeCostCalculator(&buf, model)
	if buf.Len() > 0 {
		markdown.PlainText(buf.String())
	}
}

func (g *Generator) writeExampleCostsToMarkdown(markdown *Markdown, model *catalogs.Model) {
	var buf strings.Builder
	writeExampleCosts(&buf, model)
	if buf.Len() > 0 {
		markdown.PlainText(buf.String())
	}
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
