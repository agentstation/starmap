package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/spf13/cobra"
)

var generateOutput string

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate markdown documentation for providers and models",
	Long: `Generate creates comprehensive markdown documentation for all providers and models
in the starmap catalog. The documentation includes:

• Main index with provider overview
• Individual provider pages with model listings  
• Detailed model specification pages
• Cross-referenced navigation links

The documentation is organized hierarchically and optimized for GitHub viewing.`,
	Example: `  starmap generate
  starmap generate --output ./documentation
  starmap generate -o ./my-docs`,
	RunE: runGenerate,
}

func init() {
	rootCmd.AddCommand(generateCmd)

	generateCmd.Flags().StringVarP(&generateOutput, "output", "o", "./docs", "Output directory for generated documentation")
}

// formatDuration converts a time.Duration to a human-readable string
func formatDuration(d *time.Duration) string {
	if d == nil {
		return "Not specified"
	}

	duration := *d
	if duration == 0 {
		return "Immediate deletion"
	}

	// Convert to days if it's a multiple of 24 hours
	if duration%(24*time.Hour) == 0 {
		days := int(duration / (24 * time.Hour))
		if days == 1 {
			return "1 day"
		}
		return fmt.Sprintf("%d days", days)
	}

	// Otherwise show hours
	hours := int(duration / time.Hour)
	if hours == 1 {
		return "1 hour"
	}
	return fmt.Sprintf("%d hours", hours)
}

func runGenerate(cmd *cobra.Command, args []string) error {
	fmt.Printf("🚀 Generating documentation...\n")

	// Create starmap instance
	sm, err := starmap.New()
	if err != nil {
		return fmt.Errorf("creating starmap: %w", err)
	}

	// Get catalog
	catalog, err := sm.Catalog()
	if err != nil {
		return fmt.Errorf("getting catalog: %w", err)
	}

	// Create output directory structure
	if err := os.MkdirAll(generateOutput, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	// Create catalog subdirectory
	catalogDir := filepath.Join(generateOutput, "catalog")
	if err := os.MkdirAll(catalogDir, 0755); err != nil {
		return fmt.Errorf("creating catalog directory: %w", err)
	}

	// Get data
	providers := catalog.Providers().List()
	authors := catalog.Authors().List()
	allModels := catalog.Models().List()

	fmt.Printf("📊 Found %d providers, %d authors, and %d models\n", len(providers), len(authors), len(allModels))

	// Generate main catalog index
	if err := generateMainIndex(providers, authors, allModels, catalogDir); err != nil {
		return fmt.Errorf("generating main catalog index: %w", err)
	}

	// Generate provider documentation
	if err := generateProviderDocs(providers, catalog, catalogDir); err != nil {
		return fmt.Errorf("generating provider docs: %w", err)
	}

	// Generate author documentation
	if err := generateAuthorDocs(authors, catalog, catalogDir); err != nil {
		return fmt.Errorf("generating author docs: %w", err)
	}

	fmt.Printf("✅ Documentation generated successfully in %s\n", generateOutput)
	return nil
}

func generateMainIndex(providers []*catalogs.Provider, authors []*catalogs.Author, allModels []*catalogs.Model, outputDir string) error {
	// Sort providers and authors by name
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Name < providers[j].Name
	})
	sort.Slice(authors, func(i, j int) bool {
		return authors[i].Name < authors[j].Name
	})

	var sb strings.Builder
	sb.WriteString("# Starmap Model Catalog\n\n")
	sb.WriteString("Comprehensive documentation for AI models, providers, and authors in the Starmap catalog.\n\n")

	// Statistics
	sb.WriteString("## Overview\n\n")
	sb.WriteString(fmt.Sprintf("- **Authors**: %d\n", len(authors)))
	sb.WriteString(fmt.Sprintf("- **Providers**: %d\n", len(providers)))
	sb.WriteString(fmt.Sprintf("- **Models**: %d\n", len(allModels)))
	sb.WriteString(fmt.Sprintf("- **Last Updated**: %s\n\n", time.Now().UTC().Format("2006-01-02 15:04:05 UTC")))

	// Navigation options
	sb.WriteString("## Browse Catalog\n\n")
	sb.WriteString("Choose how you'd like to explore the model catalog:\n\n")
	sb.WriteString("### 📋 [By Provider](./providers/README.md)\n")
	sb.WriteString("Browse models organized by AI service providers (OpenAI, Anthropic, Google, etc.)\n\n")
	sb.WriteString("### 👥 [By Author](./authors/README.md)\n")
	sb.WriteString("Browse models organized by the organizations that created them (Meta, Google, OpenAI, etc.)\n\n")

	// Provider summary table
	sb.WriteString("## Providers\n\n")
	sb.WriteString("| Provider | Models | Description |\n")
	sb.WriteString("|----------|--------|--------------|\n")

	for _, provider := range providers {
		modelCount := len(provider.Models)

		// Check if provider has a logo and create provider link with icon
		logoPath := filepath.Join("internal", "embedded", "catalog", "providers", string(provider.ID), "logo.svg")
		var providerLink string
		if _, err := os.Stat(logoPath); err == nil {
			providerLink = fmt.Sprintf("<img src=\"./providers/%s/logo.svg\" alt=\"\" width=\"16\" height=\"16\" style=\"vertical-align: middle\"> [%s](./providers/%s/README.md)", provider.ID, provider.Name, provider.ID)
		} else {
			providerLink = fmt.Sprintf("[%s](./providers/%s/README.md)", provider.Name, provider.ID)
		}

		// Get a brief description or use default
		description := "AI model provider"
		if provider.Name != "" {
			description = fmt.Sprintf("%s models", provider.Name)
		}

		sb.WriteString(fmt.Sprintf("| %s | %d | %s |\n", providerLink, modelCount, description))
	}

	// Author summary table
	sb.WriteString("\n## Authors\n\n")
	sb.WriteString("| Author | Models | Description |\n")
	sb.WriteString("|--------|--------|--------------|\n")

	for _, author := range authors {
		modelCount := len(author.Models)
		authorLink := fmt.Sprintf("[%s](./authors/%s/README.md)", author.Name, author.ID)

		// Get description or use default
		description := "AI model creator"
		if author.Description != nil {
			description = *author.Description
		} else if author.Name != "" {
			description = fmt.Sprintf("%s models", author.Name)
		}

		sb.WriteString(fmt.Sprintf("| %s | %d | %s |\n", authorLink, modelCount, description))
	}

	sb.WriteString("\n## Quick Links\n\n")

	// Add quick links to major providers
	majorProviders := []string{"anthropic", "openai", "google-ai-studio", "groq"}
	for _, providerID := range majorProviders {
		for _, provider := range providers {
			if string(provider.ID) == providerID {
				sb.WriteString(fmt.Sprintf("- 🔥 [%s Models](./providers/%s/README.md)\n", provider.Name, provider.ID))
				break
			}
		}
	}

	// Add quick links to major authors
	majorAuthors := []string{"openai", "anthropic", "google", "meta"}
	for _, authorID := range majorAuthors {
		for _, author := range authors {
			if string(author.ID) == authorID {
				sb.WriteString(fmt.Sprintf("- 👤 [%s Models](./authors/%s/README.md)\n", author.Name, author.ID))
				break
			}
		}
	}

	return os.WriteFile(filepath.Join(outputDir, "README.md"), []byte(sb.String()), 0644)
}

func generateProviderDocs(providers []*catalogs.Provider, catalog catalogs.Catalog, outputDir string) error {
	// Create providers directory
	providersDir := filepath.Join(outputDir, "providers")
	if err := os.MkdirAll(providersDir, 0755); err != nil {
		return fmt.Errorf("creating providers directory: %w", err)
	}

	// Generate provider overview page
	if err := generateProvidersOverview(providers, providersDir); err != nil {
		return err
	}

	// Generate individual provider pages
	for _, provider := range providers {
		if err := generateProviderPage(provider, catalog, providersDir); err != nil {
			return fmt.Errorf("generating page for %s: %w", provider.ID, err)
		}
		fmt.Printf("📝 Generated documentation for %s (%d models)\n", provider.Name, len(provider.Models))
	}

	return nil
}

func generateAuthorDocs(authors []*catalogs.Author, catalog catalogs.Catalog, outputDir string) error {
	// Create authors directory
	authorsDir := filepath.Join(outputDir, "authors")
	if err := os.MkdirAll(authorsDir, 0755); err != nil {
		return fmt.Errorf("creating authors directory: %w", err)
	}

	// Generate authors overview page
	if err := generateAuthorsOverview(authors, authorsDir); err != nil {
		return err
	}

	// Generate individual author pages
	for _, author := range authors {
		if err := generateAuthorPage(author, catalog, authorsDir); err != nil {
			return fmt.Errorf("generating page for %s: %w", author.ID, err)
		}
		fmt.Printf("👤 Generated documentation for %s (%d models)\n", author.Name, len(author.Models))
	}

	return nil
}

func generateAuthorsOverview(authors []*catalogs.Author, authorsDir string) error {
	// Sort authors by name
	sort.Slice(authors, func(i, j int) bool {
		return authors[i].Name < authors[j].Name
	})

	var sb strings.Builder
	sb.WriteString("# Authors Overview\n\n")
	sb.WriteString("Complete list of AI model authors and organizations in the Starmap catalog.\n\n")

	sb.WriteString("## Authors\n\n")

	for _, author := range authors {
		sb.WriteString(fmt.Sprintf("### [%s](./%s/README.md)\n\n", author.Name, author.ID))

		// Basic info
		if author.Description != nil {
			sb.WriteString(fmt.Sprintf("**Description**: %s  \n", *author.Description))
		}
		if author.Website != nil {
			sb.WriteString(fmt.Sprintf("**Website**: [%s](%s)  \n", *author.Website, *author.Website))
		}
		if author.GitHub != nil {
			sb.WriteString(fmt.Sprintf("**GitHub**: [%s](%s)  \n", *author.GitHub, *author.GitHub))
		}
		sb.WriteString(fmt.Sprintf("**Models**: %d  \n", len(author.Models)))

		sb.WriteString("\n")
	}

	// Navigation
	sb.WriteString("## Navigation\n\n")
	sb.WriteString("- [📋 Browse by Provider](../providers/README.md)\n")
	sb.WriteString("- [← Back to Main Catalog](../README.md)\n")

	return os.WriteFile(filepath.Join(authorsDir, "README.md"), []byte(sb.String()), 0644)
}

func generateAuthorPage(author *catalogs.Author, catalog catalogs.Catalog, authorsDir string) error {
	// Create author directory
	authorDir := filepath.Join(authorsDir, string(author.ID))
	if err := os.MkdirAll(authorDir, 0755); err != nil {
		return fmt.Errorf("creating author directory: %w", err)
	}

	// Create models directory
	modelsDir := filepath.Join(authorDir, "models")
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		return fmt.Errorf("creating models directory: %w", err)
	}

	// Get models for this author
	var models []*catalogs.Model
	for _, model := range author.Models {
		models = append(models, &model)
	}

	// Sort models by ID
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})

	// Generate author README
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", author.Name))

	// Author info
	if author.Description != nil {
		sb.WriteString(fmt.Sprintf("**Description**: %s  \n", *author.Description))
	}
	if author.Website != nil {
		sb.WriteString(fmt.Sprintf("**Website**: [%s](%s)  \n", *author.Website, *author.Website))
	}
	if author.GitHub != nil {
		sb.WriteString(fmt.Sprintf("**GitHub**: [%s](%s)  \n", *author.GitHub, *author.GitHub))
	}
	if author.HuggingFace != nil {
		sb.WriteString(fmt.Sprintf("**Hugging Face**: [%s](%s)  \n", *author.HuggingFace, *author.HuggingFace))
	}
	if author.Twitter != nil {
		sb.WriteString(fmt.Sprintf("**Twitter**: [%s](%s)  \n", *author.Twitter, *author.Twitter))
	}

	sb.WriteString(fmt.Sprintf("**Total Models**: %d\n\n", len(models)))

	// Models table
	sb.WriteString("## Models\n\n")
	if len(models) > 0 {
		sb.WriteString("| Model | Context Window | Available Via | Features |\n")
		sb.WriteString("|-------|----------------|---------------|----------|\n")

		for _, model := range models {
			modelLink := fmt.Sprintf("[%s](./models/%s.md)", model.ID, model.ID)

			// Context window
			contextWindow := "N/A"
			if model.Limits != nil && model.Limits.ContextWindow > 0 {
				contextWindow = fmt.Sprintf("%d", model.Limits.ContextWindow)
			}

			// Find which providers offer this model
			var availableVia []string
			allProviders := catalog.Providers().List()
			for _, provider := range allProviders {
				if _, found := provider.Models[model.ID]; found {
					availableVia = append(availableVia, string(provider.ID))
				}
			}
			providersText := "N/A"
			if len(availableVia) > 0 {
				providersText = strings.Join(availableVia, ", ")
			}

			// Features
			features := getModelFeatureBadges(model)

			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
				modelLink, contextWindow, providersText, features))
		}
	} else {
		sb.WriteString("No models available for this author.\n")
	}

	// Navigation
	sb.WriteString("\n## Navigation\n\n")
	sb.WriteString("- [← Back to Authors](../README.md)\n")
	sb.WriteString("- [📋 Browse by Provider](../../providers/README.md)\n")
	sb.WriteString("- [← Back to Main Catalog](../../README.md)\n")

	// Write author README
	if err := os.WriteFile(filepath.Join(authorDir, "README.md"), []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("writing author README: %w", err)
	}

	// Generate individual model pages (but with author context)
	for _, model := range models {
		if err := generateAuthorModelPage(model, author, modelsDir); err != nil {
			return fmt.Errorf("generating model page for %s: %w", model.ID, err)
		}
	}

	return nil
}

func generateProvidersOverview(providers []*catalogs.Provider, providersDir string) error {
	// Sort providers by name
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Name < providers[j].Name
	})

	var sb strings.Builder
	sb.WriteString("# Provider Overview\n\n")
	sb.WriteString("Complete list of AI model providers in the Starmap catalog.\n\n")

	sb.WriteString("## Providers\n\n")

	for _, provider := range providers {
		// Check if provider has a logo
		logoPath := filepath.Join("internal", "embedded", "catalog", "providers", string(provider.ID), "logo.svg")
		if _, err := os.Stat(logoPath); err == nil {
			sb.WriteString(fmt.Sprintf("### <img src=\"./%s/logo.svg\" alt=\"\" width=\"16\" height=\"16\" style=\"vertical-align: middle\"> [%s](./%s/README.md)\n\n", provider.ID, provider.Name, provider.ID))
		} else {
			sb.WriteString(fmt.Sprintf("### [%s](./%s/README.md)\n\n", provider.Name, provider.ID))
		}

		// Basic info
		if provider.Headquarters != nil {
			sb.WriteString(fmt.Sprintf("**Headquarters**: %s  \n", *provider.Headquarters))
		}
		sb.WriteString(fmt.Sprintf("**Models**: %d  \n", len(provider.Models)))

		// API info
		if provider.APIKey != nil {
			sb.WriteString(fmt.Sprintf("**API Key Required**: Yes  \n"))
			sb.WriteString(fmt.Sprintf("**API Key Name**: $%s  \n", provider.APIKey.Name))
		}

		sb.WriteString("\n")
	}

	return os.WriteFile(filepath.Join(providersDir, "README.md"), []byte(sb.String()), 0644)
}

// copyProviderLogo copies the provider's logo.svg file to the docs directory if it exists
func copyProviderLogo(providerID string, providerDir string) error {
	// Source logo path from embedded catalog
	sourcePath := filepath.Join("internal", "embedded", "catalog", "providers", providerID, "logo.svg")

	// Check if source logo exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		// Logo doesn't exist, skip quietly
		return nil
	}

	// Destination path in docs
	destPath := filepath.Join(providerDir, "logo.svg")

	// Copy the file
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("opening source logo: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating destination logo: %w", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("copying logo: %w", err)
	}

	return nil
}

func generateProviderPage(provider *catalogs.Provider, catalog catalogs.Catalog, providersDir string) error {
	// Create provider directory
	providerDir := filepath.Join(providersDir, string(provider.ID))
	if err := os.MkdirAll(providerDir, 0755); err != nil {
		return fmt.Errorf("creating provider directory: %w", err)
	}

	// Create models directory
	modelsDir := filepath.Join(providerDir, "models")
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		return fmt.Errorf("creating models directory: %w", err)
	}

	// Copy provider logo if available
	if err := copyProviderLogo(string(provider.ID), providerDir); err != nil {
		return fmt.Errorf("copying provider logo: %w", err)
	}

	// Get models for this provider
	var models []*catalogs.Model
	for _, model := range provider.Models {
		models = append(models, &model)
	}

	// Sort models by ID
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})

	// Generate provider README
	var sb strings.Builder

	// Check if logo was copied and add it inline with the title
	logoPath := filepath.Join(providerDir, "logo.svg")
	if _, err := os.Stat(logoPath); err == nil {
		sb.WriteString(fmt.Sprintf("# <img src=\"./logo.svg\" alt=\"%s Logo\" style=\"vertical-align: middle; height: 32px; width: auto; min-width: 32px\"> %s\n\n", provider.Name, provider.Name))
	} else {
		sb.WriteString(fmt.Sprintf("# %s\n\n", provider.Name))
	}

	// Provider info
	if provider.Headquarters != nil {
		sb.WriteString(fmt.Sprintf("**Headquarters**: %s  \n", *provider.Headquarters))
	}

	if provider.StatusPageURL != nil {
		sb.WriteString(fmt.Sprintf("**Status Page**: [%s](%s)  \n", *provider.StatusPageURL, *provider.StatusPageURL))
	}

	// API info
	if provider.APIKey != nil {
		sb.WriteString(fmt.Sprintf("**API Key Required**: Yes  \n"))
		sb.WriteString(fmt.Sprintf("**API Key Name**: $%s  \n", provider.APIKey.Name))
	}

	sb.WriteString(fmt.Sprintf("**Total Models**: %d\n\n", len(models)))

	// API Endpoints section
	if (provider.Catalog != nil && (provider.Catalog.DocsURL != nil || provider.Catalog.APIURL != nil)) ||
		(provider.ChatCompletions != nil && (provider.ChatCompletions.URL != nil || provider.ChatCompletions.HealthAPIURL != nil)) {
		sb.WriteString("## 🔗 API Endpoints\n\n")

		if provider.Catalog != nil {
			if provider.Catalog.DocsURL != nil {
				sb.WriteString(fmt.Sprintf("**Documentation**: [%s](%s)  \n", *provider.Catalog.DocsURL, *provider.Catalog.DocsURL))
			}
			if provider.Catalog.APIURL != nil {
				sb.WriteString(fmt.Sprintf("**Models API**: [%s](%s)  \n", *provider.Catalog.APIURL, *provider.Catalog.APIURL))
			}
		}

		if provider.ChatCompletions != nil {
			if provider.ChatCompletions.URL != nil {
				sb.WriteString(fmt.Sprintf("**Chat Completions**: [%s](%s)  \n", *provider.ChatCompletions.URL, *provider.ChatCompletions.URL))
			}
			if provider.ChatCompletions.HealthAPIURL != nil {
				sb.WriteString(fmt.Sprintf("**Health API**: [%s](%s)  \n", *provider.ChatCompletions.HealthAPIURL, *provider.ChatCompletions.HealthAPIURL))
			}
		}

		sb.WriteString("\n")
	}

	// Privacy & Data Handling section
	if provider.PrivacyPolicy != nil {
		sb.WriteString("## 🔒 Privacy & Data Handling\n\n")

		if provider.PrivacyPolicy.PrivacyPolicyURL != nil {
			sb.WriteString(fmt.Sprintf("**Privacy Policy**: [%s](%s)  \n", *provider.PrivacyPolicy.PrivacyPolicyURL, *provider.PrivacyPolicy.PrivacyPolicyURL))
		}
		if provider.PrivacyPolicy.TermsOfServiceURL != nil {
			sb.WriteString(fmt.Sprintf("**Terms of Service**: [%s](%s)  \n", *provider.PrivacyPolicy.TermsOfServiceURL, *provider.PrivacyPolicy.TermsOfServiceURL))
		}

		if provider.PrivacyPolicy.RetainsData != nil {
			retainsData := "No"
			if *provider.PrivacyPolicy.RetainsData {
				retainsData = "Yes"
			}
			sb.WriteString(fmt.Sprintf("**Retains User Data**: %s  \n", retainsData))
		}

		if provider.PrivacyPolicy.TrainsOnData != nil {
			trainsOnData := "No"
			if *provider.PrivacyPolicy.TrainsOnData {
				trainsOnData = "Yes"
			}
			sb.WriteString(fmt.Sprintf("**Trains on User Data**: %s  \n", trainsOnData))
		}

		sb.WriteString("\n")
	}

	// Data Retention Policy section
	if provider.RetentionPolicy != nil {
		sb.WriteString("## ⏱️ Data Retention Policy\n\n")

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
		sb.WriteString(fmt.Sprintf("**Policy Type**: %s  \n", policyType))

		// Duration
		duration := formatDuration(provider.RetentionPolicy.Duration)
		sb.WriteString(fmt.Sprintf("**Retention Duration**: %s  \n", duration))

		// Details if available
		if provider.RetentionPolicy.Details != nil && *provider.RetentionPolicy.Details != "" {
			sb.WriteString(fmt.Sprintf("**Details**: %s  \n", *provider.RetentionPolicy.Details))
		}

		sb.WriteString("\n")
	}

	// Content Moderation section
	if provider.GovernancePolicy != nil || provider.RequiresModeration != nil {
		sb.WriteString("## 🛡️ Content Moderation\n\n")

		if provider.RequiresModeration != nil {
			requiresModeration := "No"
			if *provider.RequiresModeration {
				requiresModeration = "Yes"
			}
			sb.WriteString(fmt.Sprintf("**Requires Moderation**: %s  \n", requiresModeration))
		}

		if provider.GovernancePolicy != nil {
			if provider.GovernancePolicy.Moderated != nil {
				moderated := "No"
				if *provider.GovernancePolicy.Moderated {
					moderated = "Yes"
				}
				sb.WriteString(fmt.Sprintf("**Content Moderated**: %s  \n", moderated))
			}

			if provider.GovernancePolicy.Moderator != nil && *provider.GovernancePolicy.Moderator != "" {
				// Capitalize the moderator name
				moderator := *provider.GovernancePolicy.Moderator
				if len(moderator) > 0 {
					moderator = strings.Title(moderator)
				}
				sb.WriteString(fmt.Sprintf("**Moderated by**: %s  \n", moderator))
			}
		}

		sb.WriteString("\n")
	}

	// Models table
	sb.WriteString("## Models\n\n")
	if len(models) > 0 {
		sb.WriteString("| Model | Context Window | Input Price | Output Price | Features |\n")
		sb.WriteString("|-------|----------------|-------------|--------------|----------|\n")

		for _, model := range models {
			modelLink := fmt.Sprintf("[%s](./models/%s.md)", model.ID, model.ID)

			// Context window
			contextWindow := "N/A"
			if model.Limits != nil && model.Limits.ContextWindow > 0 {
				contextWindow = fmt.Sprintf("%d", model.Limits.ContextWindow)
			}

			// Pricing
			inputPrice := "N/A"
			outputPrice := "N/A"
			if model.Pricing != nil && model.Pricing.Tokens != nil {
				if model.Pricing.Tokens.Input != nil && model.Pricing.Tokens.Input.Per1M > 0 {
					inputPrice = fmt.Sprintf("$%.2f/1M", model.Pricing.Tokens.Input.Per1M)
				}
				if model.Pricing.Tokens.Output != nil && model.Pricing.Tokens.Output.Per1M > 0 {
					outputPrice = fmt.Sprintf("$%.2f/1M", model.Pricing.Tokens.Output.Per1M)
				}
			}

			// Features
			features := getModelFeatureBadges(model)

			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
				modelLink, contextWindow, inputPrice, outputPrice, features))
		}
	} else {
		sb.WriteString("No models available for this provider.\n")
	}

	// Navigation
	sb.WriteString("\n## Navigation\n\n")
	sb.WriteString("- [← Back to Providers](../README.md)\n")
	sb.WriteString("- [← Back to Main Index](../../README.md)\n")

	// Write provider README
	if err := os.WriteFile(filepath.Join(providerDir, "README.md"), []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("writing provider README: %w", err)
	}

	// Generate individual model pages
	for _, model := range models {
		if err := generateModelPage(model, provider, modelsDir); err != nil {
			return fmt.Errorf("generating model page for %s: %w", model.ID, err)
		}
	}

	return nil
}

func generateModelPage(model *catalogs.Model, provider *catalogs.Provider, modelsDir string) error {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("# %s\n\n", model.Name))

	// Description if available
	if model.Description != "" {
		sb.WriteString(fmt.Sprintf("%s\n\n", model.Description))
	}

	// Overview section
	sb.WriteString("## 📋 Overview\n\n")
	sb.WriteString(fmt.Sprintf("- **ID**: `%s`\n", model.ID))
	// Check if provider logo exists and include it
	logoPath := filepath.Join("internal", "embedded", "catalog", "providers", string(provider.ID), "logo.svg")
	if _, err := os.Stat(logoPath); err == nil {
		sb.WriteString(fmt.Sprintf("- **Provider**: <img src=\"../logo.svg\" alt=\"\" width=\"20\" height=\"20\" style=\"vertical-align: middle\"> [%s](../README.md)\n", provider.Name))
	} else {
		sb.WriteString(fmt.Sprintf("- **Provider**: [%s](../README.md)\n", provider.Name))
	}

	// Authors with links
	if len(model.Authors) > 0 {
		var authors []string
		for _, author := range model.Authors {
			// Calculate relative path to author page from model depth
			modelDepth := strings.Count(model.ID, "/")
			authorPath := strings.Repeat("../", modelDepth+3) + fmt.Sprintf("authors/%s/README.md", author.ID)
			authors = append(authors, fmt.Sprintf("[%s](%s)", author.Name, authorPath))
		}
		sb.WriteString(fmt.Sprintf("- **Authors**: %s\n", strings.Join(authors, ", ")))
	}

	// Quick stats from metadata and limits
	if model.Metadata != nil {
		sb.WriteString(fmt.Sprintf("- **Release Date**: %s\n", model.Metadata.ReleaseDate.Format("2006-01-02")))
		if model.Metadata.KnowledgeCutoff != nil {
			sb.WriteString(fmt.Sprintf("- **Knowledge Cutoff**: %s\n", model.Metadata.KnowledgeCutoff.Format("2006-01-02")))
		}
		sb.WriteString(fmt.Sprintf("- **Open Weights**: %t\n", model.Metadata.OpenWeights))
	}

	if model.Limits != nil {
		if model.Limits.ContextWindow > 0 {
			sb.WriteString(fmt.Sprintf("- **Context Window**: %s tokens\n", formatNumber(int(model.Limits.ContextWindow))))
		}
		if model.Limits.OutputTokens > 0 {
			sb.WriteString(fmt.Sprintf("- **Max Output**: %s tokens\n", formatNumber(int(model.Limits.OutputTokens))))
		}
	}

	// Architecture info if available
	if model.Metadata != nil && model.Metadata.Architecture != nil {
		if model.Metadata.Architecture.ParameterCount != "" {
			sb.WriteString(fmt.Sprintf("- **Parameters**: %s\n", model.Metadata.Architecture.ParameterCount))
		}
	}

	sb.WriteString("\n")

	// Capabilities section with horizontal tables
	sb.WriteString("## 🎯 Capabilities\n\n")

	// Input/Output Modalities Table
	sb.WriteString("### Input/Output Modalities\n\n")
	sb.WriteString(generateModalityTable(model))

	// Core Features Table
	sb.WriteString("### Core Features\n\n")
	sb.WriteString(generateCoreFeatureTable(model))

	// Response Delivery Table
	sb.WriteString("### Response Delivery\n\n")
	sb.WriteString(generateResponseDeliveryTable(model))

	// Advanced Reasoning Table (only if applicable)
	reasoningTable := generateAdvancedReasoningTable(model)
	if reasoningTable != "" {
		sb.WriteString(reasoningTable)
	}

	// Generation Controls section with horizontal tables
	sb.WriteString("## Generation Controls\n\n")

	// Architecture table (if available)
	architectureTable := generateArchitectureTable(model)
	if architectureTable != "" {
		sb.WriteString(architectureTable)
	}

	// Model Tags table (if available)
	tagsTable := generateTagsTable(model)
	if tagsTable != "" {
		sb.WriteString(tagsTable)
	}

	// Generation Controls tables
	sb.WriteString(generateControlsTables(model))

	// Pricing section with horizontal tables
	sb.WriteString("## 💰 Pricing\n\n")

	// Token Pricing Table
	sb.WriteString(generateTokenPricingTable(model))

	// Operation Pricing Table (if applicable)
	operationTable := generateOperationPricingTable(model)
	if operationTable != "" {
		sb.WriteString(operationTable)
	}

	// Advanced Features section
	hasAdvancedFeatures := false
	advancedSection := strings.Builder{}
	advancedSection.WriteString("## Advanced Features 🚀\n\n")

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
		advancedSection.WriteString("### Response Delivery\n\n")
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
		sb.WriteString(advancedSection.String())
	}

	// Metadata section
	sb.WriteString("## 📋 Metadata\n\n")
	if model.Metadata != nil && len(model.Metadata.Tags) > 0 {
		var tags []string
		for _, tag := range model.Metadata.Tags {
			tags = append(tags, fmt.Sprintf("`%s`", string(tag)))
		}
		sb.WriteString(fmt.Sprintf("**Use Case Tags**: %s\n", strings.Join(tags, " ")))
	}
	sb.WriteString(fmt.Sprintf("**Created**: %s\n", model.CreatedAt.Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString(fmt.Sprintf("**Last Updated**: %s\n", model.UpdatedAt.Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString("\n")

	// Navigation - calculate relative path based on model ID depth
	modelDepth := strings.Count(model.ID, "/")
	providerPath := strings.Repeat("../", modelDepth+1) + "README.md"
	providersPath := strings.Repeat("../", modelDepth+2) + "README.md"
	mainPath := strings.Repeat("../", modelDepth+3) + "README.md"

	sb.WriteString("## Navigation\n\n")
	sb.WriteString(fmt.Sprintf("- [← Back to %s](%s)\n", provider.Name, providerPath))
	sb.WriteString(fmt.Sprintf("- [← Back to Providers](%s)\n", providersPath))
	sb.WriteString(fmt.Sprintf("- [← Back to Main Index](%s)\n", mainPath))

	// Create model file path and ensure directories exist
	modelFilePath := filepath.Join(modelsDir, fmt.Sprintf("%s.md", model.ID))
	modelFileDir := filepath.Dir(modelFilePath)

	// Create any necessary subdirectories
	if err := os.MkdirAll(modelFileDir, 0755); err != nil {
		return fmt.Errorf("creating model file directory: %w", err)
	}

	return os.WriteFile(modelFilePath, []byte(sb.String()), 0644)
}

// Helper function to format modalities with icons
func formatModalitiesWithIcons(modalities []catalogs.ModelModality) string {
	var formatted []string
	for _, modality := range modalities {
		switch modality {
		case catalogs.ModelModalityText:
			formatted = append(formatted, `<span title="Text">📝</span> text`)
		case catalogs.ModelModalityImage:
			formatted = append(formatted, `<span title="Image">🖼️</span> image`)
		case catalogs.ModelModalityAudio:
			formatted = append(formatted, `<span title="Audio">🔊</span> audio`)
		case catalogs.ModelModalityVideo:
			formatted = append(formatted, `<span title="Video">🎥</span> video`)
		case catalogs.ModelModalityPDF:
			formatted = append(formatted, `<span title="PDF">📄</span> pdf`)
		default:
			formatted = append(formatted, string(modality))
		}
	}
	return strings.Join(formatted, ", ")
}

// Helper function to format generation parameters
func formatGenerationParams(model *catalogs.Model) string {
	if model.Generation == nil {
		return ""
	}

	var params []string
	if model.Generation.Temperature != nil {
		params = append(params, fmt.Sprintf("**Temperature**: %.2f - %.2f (default: %.2f)",
			model.Generation.Temperature.Min, model.Generation.Temperature.Max, model.Generation.Temperature.Default))
	}
	if model.Generation.TopP != nil {
		params = append(params, fmt.Sprintf("**Top-P**: %.2f - %.2f (default: %.2f)",
			model.Generation.TopP.Min, model.Generation.TopP.Max, model.Generation.TopP.Default))
	}
	if model.Generation.TopK != nil {
		params = append(params, fmt.Sprintf("**Top-K**: %d - %d (default: %d)",
			model.Generation.TopK.Min, model.Generation.TopK.Max, model.Generation.TopK.Default))
	}
	if model.Generation.MaxTokens != nil {
		params = append(params, fmt.Sprintf("**Max Output Tokens**: %d", *model.Generation.MaxTokens))
	}

	if len(params) == 0 {
		return ""
	}

	return strings.Join(params, "  \n") + "  \n"
}

// Helper function to format comprehensive pricing
func formatComprehensivePricing(model *catalogs.Model) string {
	if model.Pricing == nil {
		return "Contact provider for pricing information."
	}

	var lines []string
	currency := "USD"
	if model.Pricing.Currency != "" {
		currency = model.Pricing.Currency
	}

	// Get currency symbol
	currencySymbol := "$"
	switch currency {
	case "USD":
		currencySymbol = "$"
	case "EUR":
		currencySymbol = "€"
	case "GBP":
		currencySymbol = "£"
	default:
		currencySymbol = currency + " "
	}

	// Token-based pricing
	if model.Pricing.Tokens != nil {
		tokenPricingLines := []string{}

		if model.Pricing.Tokens.Input != nil {
			if model.Pricing.Tokens.Input.Per1M > 0 {
				tokenPricingLines = append(tokenPricingLines, fmt.Sprintf("- **Input Tokens**: %s%.2f per 1M tokens", currencySymbol, model.Pricing.Tokens.Input.Per1M))
			} else if model.Pricing.Tokens.Input.PerToken > 0 {
				tokenPricingLines = append(tokenPricingLines, fmt.Sprintf("- **Input Tokens**: %s%.6f per token", currencySymbol, model.Pricing.Tokens.Input.PerToken))
			}
		}

		if model.Pricing.Tokens.Output != nil {
			if model.Pricing.Tokens.Output.Per1M > 0 {
				tokenPricingLines = append(tokenPricingLines, fmt.Sprintf("- **Output Tokens**: %s%.2f per 1M tokens", currencySymbol, model.Pricing.Tokens.Output.Per1M))
			} else if model.Pricing.Tokens.Output.PerToken > 0 {
				tokenPricingLines = append(tokenPricingLines, fmt.Sprintf("- **Output Tokens**: %s%.6f per token", currencySymbol, model.Pricing.Tokens.Output.PerToken))
			}
		}

		if model.Pricing.Tokens.Reasoning != nil {
			if model.Pricing.Tokens.Reasoning.Per1M > 0 {
				tokenPricingLines = append(tokenPricingLines, fmt.Sprintf("- **Reasoning Tokens**: %s%.2f per 1M tokens", currencySymbol, model.Pricing.Tokens.Reasoning.Per1M))
			} else if model.Pricing.Tokens.Reasoning.PerToken > 0 {
				tokenPricingLines = append(tokenPricingLines, fmt.Sprintf("- **Reasoning Tokens**: %s%.6f per token", currencySymbol, model.Pricing.Tokens.Reasoning.PerToken))
			}
		}

		// Cache pricing (nested structure)
		if model.Pricing.Tokens.Cache != nil {
			if model.Pricing.Tokens.Cache.Read != nil && model.Pricing.Tokens.Cache.Read.Per1M > 0 {
				tokenPricingLines = append(tokenPricingLines, fmt.Sprintf("- **Cache Read**: %s%.2f per 1M tokens", currencySymbol, model.Pricing.Tokens.Cache.Read.Per1M))
			}
			if model.Pricing.Tokens.Cache.Write != nil && model.Pricing.Tokens.Cache.Write.Per1M > 0 {
				tokenPricingLines = append(tokenPricingLines, fmt.Sprintf("- **Cache Write**: %s%.2f per 1M tokens", currencySymbol, model.Pricing.Tokens.Cache.Write.Per1M))
			}
		}

		// Cache pricing (flat structure - for backward compatibility)
		if model.Pricing.Tokens.CacheRead != nil && model.Pricing.Tokens.CacheRead.Per1M > 0 {
			tokenPricingLines = append(tokenPricingLines, fmt.Sprintf("- **Cache Read**: %s%.2f per 1M tokens", currencySymbol, model.Pricing.Tokens.CacheRead.Per1M))
		}
		if model.Pricing.Tokens.CacheWrite != nil && model.Pricing.Tokens.CacheWrite.Per1M > 0 {
			tokenPricingLines = append(tokenPricingLines, fmt.Sprintf("- **Cache Write**: %s%.2f per 1M tokens", currencySymbol, model.Pricing.Tokens.CacheWrite.Per1M))
		}

		// Only add the header if we have actual token pricing data
		if len(tokenPricingLines) > 0 {
			lines = append(lines, "### Token Pricing")
			lines = append(lines, tokenPricingLines...)
		}
	}

	// Operation-based pricing
	if model.Pricing.Operations != nil {
		hasOperations := false
		operationLines := []string{"### Operation Pricing"}

		if model.Pricing.Operations.Request != nil {
			operationLines = append(operationLines, fmt.Sprintf("- **Per Request**: %s%.6f", currencySymbol, *model.Pricing.Operations.Request))
			hasOperations = true
		}
		if model.Pricing.Operations.ImageInput != nil {
			operationLines = append(operationLines, fmt.Sprintf("- **Image Input**: %s%.4f per image", currencySymbol, *model.Pricing.Operations.ImageInput))
			hasOperations = true
		}
		if model.Pricing.Operations.ImageGen != nil {
			operationLines = append(operationLines, fmt.Sprintf("- **Image Generation**: %s%.4f per image", currencySymbol, *model.Pricing.Operations.ImageGen))
			hasOperations = true
		}
		if model.Pricing.Operations.AudioInput != nil {
			operationLines = append(operationLines, fmt.Sprintf("- **Audio Input**: %s%.4f per minute", currencySymbol, *model.Pricing.Operations.AudioInput))
			hasOperations = true
		}
		if model.Pricing.Operations.AudioGen != nil {
			operationLines = append(operationLines, fmt.Sprintf("- **Audio Generation**: %s%.4f per minute", currencySymbol, *model.Pricing.Operations.AudioGen))
			hasOperations = true
		}
		if model.Pricing.Operations.VideoInput != nil {
			operationLines = append(operationLines, fmt.Sprintf("- **Video Input**: %s%.4f per minute", currencySymbol, *model.Pricing.Operations.VideoInput))
			hasOperations = true
		}
		if model.Pricing.Operations.VideoGen != nil {
			operationLines = append(operationLines, fmt.Sprintf("- **Video Generation**: %s%.4f per minute", currencySymbol, *model.Pricing.Operations.VideoGen))
			hasOperations = true
		}

		if hasOperations {
			lines = append(lines, operationLines...)
		}
	}

	if len(lines) == 0 {
		return "Contact provider for pricing information."
	}

	return strings.Join(lines, "\n")
}

func getModelFeatureBadges(model *catalogs.Model) string {
	var badges []string

	if model.Features == nil {
		// If no features data, return empty (show no capabilities)
		return ""
	}

	// Check input modalities
	hasVision := false
	hasAudio := false
	hasText := false
	for _, modality := range model.Features.Modalities.Input {
		switch modality {
		case catalogs.ModelModalityText:
			hasText = true
		case catalogs.ModelModalityImage:
			hasVision = true
		case catalogs.ModelModalityAudio:
			hasAudio = true
		}
	}

	// Check output modalities
	hasImageGen := false
	hasAudioGen := false
	hasVideoGen := false
	for _, modality := range model.Features.Modalities.Output {
		switch modality {
		case catalogs.ModelModalityImage:
			hasImageGen = true
		case catalogs.ModelModalityAudio:
			hasAudioGen = true
		case catalogs.ModelModalityVideo:
			hasVideoGen = true
		}
	}

	// Only show features that are actually supported

	// Text Processing (basic capability)
	if hasText {
		badges = append(badges, `<span title="Text Processing">📝</span>`)
	}

	// Vision/Image Input
	if hasVision {
		badges = append(badges, `<span title="Vision/Image Input">👁️</span>`)
	}

	// Image Generation
	if hasImageGen {
		badges = append(badges, `<span title="Image Generation">🖼️</span>`)
	}

	// Audio Processing (input or output)
	if hasAudio || hasAudioGen {
		badges = append(badges, `<span title="Audio Processing">🔊</span>`)
	}

	// Video Generation
	if hasVideoGen {
		badges = append(badges, `<span title="Video Generation">🎥</span>`)
	}

	// Tool Calling
	if model.Features.Tools {
		badges = append(badges, `<span title="Tool Calling">🔧</span>`)
	}

	// Web Search
	if model.Features.WebSearch {
		badges = append(badges, `<span title="Web Search">🔍</span>`)
	}

	// Advanced Reasoning
	if model.Features.Reasoning {
		badges = append(badges, `<span title="Advanced Reasoning">🧠</span>`)
	}

	// File Attachments
	if model.Features.Attachments {
		badges = append(badges, `<span title="File Attachments">📎</span>`)
	}

	// Response Streaming
	if model.Features.Streaming {
		badges = append(badges, `<span title="Response Streaming">⚡</span>`)
	}

	// Structured Output
	if model.Features.StructuredOutputs {
		badges = append(badges, `<span title="Structured Output"></></span>`)
	}

	return strings.Join(badges, " ")
}

func generateAuthorModelPage(model *catalogs.Model, author *catalogs.Author, modelsDir string) error {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("# %s\n\n", model.Name))

	// Description if available
	if model.Description != "" {
		sb.WriteString(fmt.Sprintf("%s\n\n", model.Description))
	}

	// Overview section
	sb.WriteString("## 📋 Overview\n\n")
	sb.WriteString(fmt.Sprintf("- **ID**: `%s`\n", model.ID))

	// Authors with primary author emphasized and links to co-authors
	if len(model.Authors) > 1 {
		var authors []string
		for _, modelAuthor := range model.Authors {
			if modelAuthor.ID == author.ID {
				// Current author - link to current author page
				authors = append(authors, fmt.Sprintf("[%s](../README.md)", modelAuthor.Name))
			} else {
				// Co-author - link to their author page
				modelDepth := strings.Count(model.ID, "/")
				coAuthorPath := strings.Repeat("../", modelDepth+2) + fmt.Sprintf("%s/README.md", modelAuthor.ID)
				authors = append(authors, fmt.Sprintf("[%s](%s)", modelAuthor.Name, coAuthorPath))
			}
		}
		sb.WriteString(fmt.Sprintf("- **Authors**: %s\n", strings.Join(authors, ", ")))
	} else {
		// Single author
		sb.WriteString(fmt.Sprintf("- **Author**: [%s](../README.md)\n", author.Name))
	}

	// Quick stats from metadata and limits
	if model.Metadata != nil {
		sb.WriteString(fmt.Sprintf("- **Release Date**: %s\n", model.Metadata.ReleaseDate.Format("2006-01-02")))
		if model.Metadata.KnowledgeCutoff != nil {
			sb.WriteString(fmt.Sprintf("- **Knowledge Cutoff**: %s\n", model.Metadata.KnowledgeCutoff.Format("2006-01-02")))
		}
		sb.WriteString(fmt.Sprintf("- **Open Weights**: %t\n", model.Metadata.OpenWeights))
	}

	if model.Limits != nil {
		if model.Limits.ContextWindow > 0 {
			sb.WriteString(fmt.Sprintf("- **Context Window**: %s tokens\n", formatNumber(int(model.Limits.ContextWindow))))
		}
		if model.Limits.OutputTokens > 0 {
			sb.WriteString(fmt.Sprintf("- **Max Output**: %s tokens\n", formatNumber(int(model.Limits.OutputTokens))))
		}
	}

	// Architecture info if available
	if model.Metadata != nil && model.Metadata.Architecture != nil {
		if model.Metadata.Architecture.ParameterCount != "" {
			sb.WriteString(fmt.Sprintf("- **Parameters**: %s\n", model.Metadata.Architecture.ParameterCount))
		}
	}

	sb.WriteString("\n")

	// Capabilities section with horizontal tables
	sb.WriteString("## 🎯 Capabilities\n\n")

	// Input/Output Modalities Table
	sb.WriteString("### Input/Output Modalities\n\n")
	sb.WriteString(generateModalityTable(model))

	// Core Features Table
	sb.WriteString("### Core Features\n\n")
	sb.WriteString(generateCoreFeatureTable(model))

	// Response Delivery Table
	sb.WriteString("### Response Delivery\n\n")
	sb.WriteString(generateResponseDeliveryTable(model))

	// Advanced Reasoning Table (only if applicable)
	reasoningTable := generateAdvancedReasoningTable(model)
	if reasoningTable != "" {
		sb.WriteString(reasoningTable)
	}

	// Generation Controls section with horizontal tables
	sb.WriteString("## Generation Controls\n\n")

	// Architecture table (if available)
	architectureTable := generateArchitectureTable(model)
	if architectureTable != "" {
		sb.WriteString(architectureTable)
	}

	// Model Tags table (if available)
	tagsTable := generateTagsTable(model)
	if tagsTable != "" {
		sb.WriteString(tagsTable)
	}

	// Generation Controls tables
	sb.WriteString(generateControlsTables(model))

	// Pricing section with horizontal tables
	sb.WriteString("## 💰 Pricing\n\n")

	// Token Pricing Table
	sb.WriteString(generateTokenPricingTable(model))

	// Operation Pricing Table (if applicable)
	operationTable := generateOperationPricingTable(model)
	if operationTable != "" {
		sb.WriteString(operationTable)
	}

	// Advanced Features section
	hasAdvancedFeatures := false
	advancedSection := strings.Builder{}
	advancedSection.WriteString("## Advanced Features 🚀\n\n")

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
		advancedSection.WriteString("### Response Delivery\n\n")
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
		sb.WriteString(advancedSection.String())
	}

	// Metadata section
	sb.WriteString("## 📋 Metadata\n\n")
	if model.Metadata != nil && len(model.Metadata.Tags) > 0 {
		var tags []string
		for _, tag := range model.Metadata.Tags {
			tags = append(tags, fmt.Sprintf("`%s`", string(tag)))
		}
		sb.WriteString(fmt.Sprintf("**Use Case Tags**: %s\n", strings.Join(tags, " ")))
	}
	sb.WriteString(fmt.Sprintf("**Created**: %s\n", model.CreatedAt.Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString(fmt.Sprintf("**Last Updated**: %s\n", model.UpdatedAt.Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString("\n")

	// Navigation - calculate relative path based on model ID depth
	modelDepth := strings.Count(model.ID, "/")
	authorPath := strings.Repeat("../", modelDepth+1) + "README.md"
	authorsPath := strings.Repeat("../", modelDepth+2) + "README.md"
	providersPath := strings.Repeat("../", modelDepth+3) + "providers/README.md"
	mainPath := strings.Repeat("../", modelDepth+3) + "README.md"

	sb.WriteString("## Navigation\n\n")
	sb.WriteString(fmt.Sprintf("- [← Back to %s](%s)\n", author.Name, authorPath))
	sb.WriteString(fmt.Sprintf("- [← Back to Authors](%s)\n", authorsPath))
	sb.WriteString(fmt.Sprintf("- [📋 Browse by Provider](%s)\n", providersPath))
	sb.WriteString(fmt.Sprintf("- [← Back to Main Catalog](%s)\n", mainPath))

	// Create model file path and ensure directories exist
	modelFilePath := filepath.Join(modelsDir, fmt.Sprintf("%s.md", model.ID))
	modelFileDir := filepath.Dir(modelFilePath)

	// Create any necessary subdirectories
	if err := os.MkdirAll(modelFileDir, 0755); err != nil {
		return fmt.Errorf("creating model file directory: %w", err)
	}

	return os.WriteFile(modelFilePath, []byte(sb.String()), 0644)
}

// Helper functions for horizontal tables

func generateModalityTable(model *catalogs.Model) string {
	if model.Features == nil {
		return "No modality information available.\n\n"
	}

	var sb strings.Builder
	sb.WriteString("| Direction | Text | Image | Audio | Video | PDF |\n")
	sb.WriteString("|-----------|------|-------|-------|-------|-----|\n")

	// Input row
	sb.WriteString("| Input     |")
	allModalities := []catalogs.ModelModality{
		catalogs.ModelModalityText,
		catalogs.ModelModalityImage,
		catalogs.ModelModalityAudio,
		catalogs.ModelModalityVideo,
		catalogs.ModelModalityPDF,
	}

	for _, modality := range allModalities {
		hasModality := false
		for _, inputModality := range model.Features.Modalities.Input {
			if inputModality == modality {
				hasModality = true
				break
			}
		}
		if hasModality {
			sb.WriteString(" ✅   |")
		} else {
			sb.WriteString(" ❌   |")
		}
	}
	sb.WriteString("\n")

	// Output row
	sb.WriteString("| Output    |")
	for _, modality := range allModalities {
		hasModality := false
		for _, outputModality := range model.Features.Modalities.Output {
			if outputModality == modality {
				hasModality = true
				break
			}
		}
		if hasModality {
			sb.WriteString(" ✅   |")
		} else {
			sb.WriteString(" ❌   |")
		}
	}
	sb.WriteString("\n\n")

	return sb.String()
}

func generateCoreFeatureTable(model *catalogs.Model) string {
	if model.Features == nil {
		return "No feature information available.\n\n"
	}

	var sb strings.Builder
	sb.WriteString("| Tool Calling | Tool Definitions | Tool Choice | Web Search | File Attachments |\n")
	sb.WriteString("|--------------|------------------|-------------|------------|------------------|\n")
	sb.WriteString("| ")

	// Tool Calling
	if model.Features.ToolCalls {
		sb.WriteString("✅           | ")
	} else {
		sb.WriteString("❌           | ")
	}

	// Tool Definitions
	if model.Features.Tools {
		sb.WriteString("✅               | ")
	} else {
		sb.WriteString("❌               | ")
	}

	// Tool Choice
	if model.Features.ToolChoice {
		sb.WriteString("✅          | ")
	} else {
		sb.WriteString("❌          | ")
	}

	// Web Search
	if model.Features.WebSearch {
		sb.WriteString("✅         | ")
	} else {
		sb.WriteString("❌         | ")
	}

	// File Attachments
	if model.Features.Attachments {
		sb.WriteString("✅               |\n\n")
	} else {
		sb.WriteString("❌               |\n\n")
	}

	return sb.String()
}

func generateResponseDeliveryTable(model *catalogs.Model) string {
	if model.Features == nil {
		return "No delivery information available.\n\n"
	}

	var sb strings.Builder
	sb.WriteString("| Streaming | Structured Output | JSON Mode | Function Call | Text Format |\n")
	sb.WriteString("|-----------|-------------------|-----------|---------------|--------------|\n")
	sb.WriteString("| ")

	// Streaming
	if model.Features.Streaming {
		sb.WriteString("✅        | ")
	} else {
		sb.WriteString("❌        | ")
	}

	// Structured Output
	if model.Features.StructuredOutputs {
		sb.WriteString("✅                | ")
	} else {
		sb.WriteString("❌                | ")
	}

	// JSON Mode (check for format response)
	if model.Features.FormatResponse {
		sb.WriteString("✅        | ")
	} else {
		sb.WriteString("❌        | ")
	}

	// Function Call (same as tool calls)
	if model.Features.ToolCalls {
		sb.WriteString("✅            | ")
	} else {
		sb.WriteString("❌            | ")
	}

	// Text Format (always supported if model exists)
	sb.WriteString("✅           |\n\n")

	return sb.String()
}

func generateAdvancedReasoningTable(model *catalogs.Model) string {
	if model.Features == nil {
		return ""
	}

	// Only show this table if any reasoning features are present
	hasReasoningFeatures := model.Features.Reasoning || model.Features.ReasoningEffort ||
		model.Features.ReasoningTokens || model.Features.IncludeReasoning || model.Features.Verbosity

	if !hasReasoningFeatures {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("### Advanced Reasoning\n\n")
	sb.WriteString("| Basic Reasoning | Reasoning Effort | Reasoning Tokens | Include Reasoning | Verbosity Control |\n")
	sb.WriteString("|-----------------|------------------|------------------|-------------------|-------------------|\n")
	sb.WriteString("| ")

	if model.Features.Reasoning {
		sb.WriteString("✅              | ")
	} else {
		sb.WriteString("❌              | ")
	}

	if model.Features.ReasoningEffort {
		sb.WriteString("✅               | ")
	} else {
		sb.WriteString("❌               | ")
	}

	if model.Features.ReasoningTokens {
		sb.WriteString("✅               | ")
	} else {
		sb.WriteString("❌               | ")
	}

	if model.Features.IncludeReasoning {
		sb.WriteString("✅                | ")
	} else {
		sb.WriteString("❌                | ")
	}

	if model.Features.Verbosity {
		sb.WriteString("✅                |\n\n")
	} else {
		sb.WriteString("❌                |\n\n")
	}

	return sb.String()
}

func generateControlsTables(model *catalogs.Model) string {
	if model.Features == nil {
		return "No control information available.\n\n"
	}

	var sb strings.Builder

	// Create horizontal tables for each category that has supported features

	// Sampling & Decoding Controls
	hasCoreSampling := model.Features.Temperature || model.Features.TopP || model.Features.TopK ||
		model.Features.TopA || model.Features.MinP

	if hasCoreSampling {
		sb.WriteString("### Sampling & Decoding\n\n")

		// Build table headers dynamically based on what's supported
		var headers []string
		var values []string

		if model.Features.Temperature {
			headers = append(headers, "Temperature")
			rangeStr := ""
			if model.Generation != nil && model.Generation.Temperature != nil {
				rangeStr = fmt.Sprintf("%.1f-%.1f", model.Generation.Temperature.Min, model.Generation.Temperature.Max)
			} else {
				rangeStr = "0.0-2.0"
			}
			values = append(values, rangeStr)
		}

		if model.Features.TopP {
			headers = append(headers, "Top-P")
			rangeStr := ""
			if model.Generation != nil && model.Generation.TopP != nil {
				rangeStr = fmt.Sprintf("%.1f-%.1f", model.Generation.TopP.Min, model.Generation.TopP.Max)
			} else {
				rangeStr = "0.0-1.0"
			}
			values = append(values, rangeStr)
		}

		if model.Features.TopK {
			headers = append(headers, "Top-K")
			rangeStr := ""
			if model.Generation != nil && model.Generation.TopK != nil {
				rangeStr = fmt.Sprintf("%d-%d", model.Generation.TopK.Min, model.Generation.TopK.Max)
			} else {
				rangeStr = "✅"
			}
			values = append(values, rangeStr)
		}

		if model.Features.TopA {
			headers = append(headers, "Top-A")
			values = append(values, "✅")
		}

		if model.Features.MinP {
			headers = append(headers, "Min-P")
			values = append(values, "✅")
		}

		// Build the table
		sb.WriteString("| " + strings.Join(headers, " | ") + " |\n")
		sb.WriteString("|" + strings.Repeat("---|", len(headers)) + "\n")
		sb.WriteString("| " + strings.Join(values, " | ") + " |\n\n")
	}

	// Length & Termination Controls
	hasLengthControls := model.Features.MaxTokens || model.Features.Stop

	if hasLengthControls {
		sb.WriteString("### Length & Termination\n\n")

		var headers []string
		var values []string

		if model.Features.MaxTokens {
			headers = append(headers, "Max Tokens")
			rangeStr := ""
			if model.Limits != nil && model.Limits.OutputTokens > 0 {
				rangeStr = fmt.Sprintf("1-%s", formatNumber(int(model.Limits.OutputTokens)))
			} else {
				rangeStr = "✅"
			}
			values = append(values, rangeStr)
		}

		if model.Features.Stop {
			headers = append(headers, "Stop Sequences")
			values = append(values, "✅")
		}

		// Build the table
		sb.WriteString("| " + strings.Join(headers, " | ") + " |\n")
		sb.WriteString("|" + strings.Repeat("---|", len(headers)) + "\n")
		sb.WriteString("| " + strings.Join(values, " | ") + " |\n\n")
	}

	// Repetition Control
	hasRepetitionControls := model.Features.FrequencyPenalty || model.Features.PresencePenalty ||
		model.Features.RepetitionPenalty

	if hasRepetitionControls {
		sb.WriteString("### Repetition Control\n\n")

		var headers []string
		var values []string

		if model.Features.FrequencyPenalty {
			headers = append(headers, "Frequency Penalty")
			rangeStr := ""
			if model.Generation != nil && model.Generation.FrequencyPenalty != nil {
				rangeStr = fmt.Sprintf("%.1f to %.1f", model.Generation.FrequencyPenalty.Min, model.Generation.FrequencyPenalty.Max)
			} else {
				rangeStr = "-2.0 to 2.0"
			}
			values = append(values, rangeStr)
		}

		if model.Features.PresencePenalty {
			headers = append(headers, "Presence Penalty")
			rangeStr := ""
			if model.Generation != nil && model.Generation.PresencePenalty != nil {
				rangeStr = fmt.Sprintf("%.1f to %.1f", model.Generation.PresencePenalty.Min, model.Generation.PresencePenalty.Max)
			} else {
				rangeStr = "-2.0 to 2.0"
			}
			values = append(values, rangeStr)
		}

		if model.Features.RepetitionPenalty {
			headers = append(headers, "Repetition Penalty")
			values = append(values, "✅")
		}

		// Build the table
		sb.WriteString("| " + strings.Join(headers, " | ") + " |\n")
		sb.WriteString("|" + strings.Repeat("---|", len(headers)) + "\n")
		sb.WriteString("| " + strings.Join(values, " | ") + " |\n\n")
	}

	// Advanced Controls
	hasAdvancedControls := model.Features.LogitBias || model.Features.Seed || model.Features.Logprobs

	if hasAdvancedControls {
		sb.WriteString("### Advanced Controls\n\n")

		var headers []string
		var values []string

		if model.Features.LogitBias {
			headers = append(headers, "Logit Bias")
			values = append(values, "✅")
		}

		if model.Features.Seed {
			headers = append(headers, "Deterministic Seed")
			values = append(values, "✅")
		}

		if model.Features.Logprobs {
			headers = append(headers, "Log Probabilities")
			rangeStr := ""
			if model.Generation != nil && model.Generation.TopLogprobs != nil {
				rangeStr = fmt.Sprintf("0-%d", *model.Generation.TopLogprobs)
			} else {
				rangeStr = "0-20"
			}
			values = append(values, rangeStr)
		}

		// Build the table
		sb.WriteString("| " + strings.Join(headers, " | ") + " |\n")
		sb.WriteString("|" + strings.Repeat("---|", len(headers)) + "\n")
		sb.WriteString("| " + strings.Join(values, " | ") + " |\n\n")
	}

	return sb.String()
}

func generateArchitectureTable(model *catalogs.Model) string {
	if model.Metadata == nil || model.Metadata.Architecture == nil {
		return ""
	}

	arch := model.Metadata.Architecture
	var sb strings.Builder

	sb.WriteString("### Architecture Details\n\n")
	sb.WriteString("| Parameter Count | Architecture Type | Tokenizer | Quantization | Fine-Tuned | Base Model |\n")
	sb.WriteString("|-----------------|-------------------|-----------|--------------|------------|-----------|\n")
	sb.WriteString("| ")

	if arch.ParameterCount != "" {
		sb.WriteString(fmt.Sprintf("%-15s | ", arch.ParameterCount))
	} else {
		sb.WriteString("Unknown         | ")
	}

	if arch.Type != "" {
		sb.WriteString(fmt.Sprintf("%-17s | ", string(arch.Type)))
	} else {
		sb.WriteString("Unknown           | ")
	}

	if arch.Tokenizer != "" {
		sb.WriteString(fmt.Sprintf("%-9s | ", string(arch.Tokenizer)))
	} else {
		sb.WriteString("Unknown   | ")
	}

	if arch.Quantization != "" {
		sb.WriteString(fmt.Sprintf("%-12s | ", string(arch.Quantization)))
	} else {
		sb.WriteString("None         | ")
	}

	if arch.FineTuned {
		sb.WriteString("Yes        | ")
	} else {
		sb.WriteString("No         | ")
	}

	if arch.BaseModel != nil && *arch.BaseModel != "" {
		sb.WriteString(fmt.Sprintf("%s |\n\n", *arch.BaseModel))
	} else {
		sb.WriteString("- |\n\n")
	}

	return sb.String()
}

func generateTagsTable(model *catalogs.Model) string {
	if model.Metadata == nil || len(model.Metadata.Tags) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("### Model Tags\n\n")

	// Common tags to check for
	commonTags := []catalogs.ModelTag{
		catalogs.ModelTagCoding,
		catalogs.ModelTagWriting,
		catalogs.ModelTagReasoning,
		catalogs.ModelTagMath,
		catalogs.ModelTagChat,
		catalogs.ModelTagMultimodal,
		catalogs.ModelTagFunctionCalling,
	}

	// Create header
	sb.WriteString("| Coding | Writing | Reasoning | Math | Chat | Multimodal | Function Calling |\n")
	sb.WriteString("|--------|---------|-----------|------|------|------------|------------------|\n")
	sb.WriteString("| ")

	// Check each common tag
	for _, tag := range commonTags {
		hasTag := false
		for _, modelTag := range model.Metadata.Tags {
			if modelTag == tag {
				hasTag = true
				break
			}
		}
		if hasTag {
			sb.WriteString("✅     | ")
		} else {
			sb.WriteString("❌     | ")
		}
	}

	// Remove the last " | " and add newline
	result := sb.String()
	if strings.HasSuffix(result, " | ") {
		result = result[:len(result)-3] + " |\n\n"
	}

	// Add any additional tags not in the common list
	var additionalTags []string
	for _, modelTag := range model.Metadata.Tags {
		isCommon := false
		for _, commonTag := range commonTags {
			if modelTag == commonTag {
				isCommon = true
				break
			}
		}
		if !isCommon {
			additionalTags = append(additionalTags, string(modelTag))
		}
	}

	if len(additionalTags) > 0 {
		result += fmt.Sprintf("**Additional Tags**: %s\n\n", strings.Join(additionalTags, ", "))
	}

	return result
}

func generateTokenPricingTable(model *catalogs.Model) string {
	if model.Pricing == nil || model.Pricing.Tokens == nil {
		return "Contact provider for pricing information.\n\n"
	}

	var sb strings.Builder
	sb.WriteString("### Token Pricing\n\n")

	tokens := model.Pricing.Tokens
	currencySymbol := getCurrencySymbol(model.Pricing.Currency)

	sb.WriteString("| Input | Output | Reasoning | Cache Read | Cache Write |\n")
	sb.WriteString("|-------|--------|-----------|------------|-------------|\n")
	sb.WriteString("| ")

	if tokens.Input != nil && tokens.Input.Per1M > 0 {
		sb.WriteString(fmt.Sprintf("%s%.2f/1M | ", currencySymbol, tokens.Input.Per1M))
	} else {
		sb.WriteString("- | ")
	}

	if tokens.Output != nil && tokens.Output.Per1M > 0 {
		sb.WriteString(fmt.Sprintf("%s%.2f/1M | ", currencySymbol, tokens.Output.Per1M))
	} else {
		sb.WriteString("- | ")
	}

	if tokens.Reasoning != nil && tokens.Reasoning.Per1M > 0 {
		sb.WriteString(fmt.Sprintf("%s%.2f/1M | ", currencySymbol, tokens.Reasoning.Per1M))
	} else {
		sb.WriteString("- | ")
	}

	// Check both flat structure and nested cache structure
	if tokens.CacheRead != nil && tokens.CacheRead.Per1M > 0 {
		sb.WriteString(fmt.Sprintf("%s%.2f/1M | ", currencySymbol, tokens.CacheRead.Per1M))
	} else if tokens.Cache != nil && tokens.Cache.Read != nil && tokens.Cache.Read.Per1M > 0 {
		sb.WriteString(fmt.Sprintf("%s%.2f/1M | ", currencySymbol, tokens.Cache.Read.Per1M))
	} else {
		sb.WriteString("- | ")
	}

	if tokens.CacheWrite != nil && tokens.CacheWrite.Per1M > 0 {
		sb.WriteString(fmt.Sprintf("%s%.2f/1M |\n\n", currencySymbol, tokens.CacheWrite.Per1M))
	} else if tokens.Cache != nil && tokens.Cache.Write != nil && tokens.Cache.Write.Per1M > 0 {
		sb.WriteString(fmt.Sprintf("%s%.2f/1M |\n\n", currencySymbol, tokens.Cache.Write.Per1M))
	} else {
		sb.WriteString("- |\n\n")
	}

	return sb.String()
}

func generateOperationPricingTable(model *catalogs.Model) string {
	if model.Pricing == nil || model.Pricing.Operations == nil {
		return ""
	}

	ops := model.Pricing.Operations
	hasOperations := ops.ImageInput != nil || ops.AudioInput != nil || ops.VideoInput != nil ||
		ops.ImageGen != nil || ops.AudioGen != nil || ops.WebSearch != nil

	if !hasOperations {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("### Operation Pricing\n\n")

	currencySymbol := getCurrencySymbol(model.Pricing.Currency)

	sb.WriteString("| Image Input | Audio Input | Video Input | Image Gen | Audio Gen | Web Search |\n")
	sb.WriteString("|-------------|-------------|-------------|-----------|-----------|------------|\n")
	sb.WriteString("| ")

	if ops.ImageInput != nil {
		sb.WriteString(fmt.Sprintf("%s%.3f/img   | ", currencySymbol, *ops.ImageInput))
	} else {
		sb.WriteString("-           | ")
	}

	if ops.AudioInput != nil {
		sb.WriteString(fmt.Sprintf("%s%.3f/min   | ", currencySymbol, *ops.AudioInput))
	} else {
		sb.WriteString("-           | ")
	}

	if ops.VideoInput != nil {
		sb.WriteString(fmt.Sprintf("%s%.3f/min   | ", currencySymbol, *ops.VideoInput))
	} else {
		sb.WriteString("-           | ")
	}

	if ops.ImageGen != nil {
		sb.WriteString(fmt.Sprintf("%s%.3f/img | ", currencySymbol, *ops.ImageGen))
	} else {
		sb.WriteString("-         | ")
	}

	if ops.AudioGen != nil {
		sb.WriteString(fmt.Sprintf("%s%.3f/min | ", currencySymbol, *ops.AudioGen))
	} else {
		sb.WriteString("-         | ")
	}

	if ops.WebSearch != nil {
		sb.WriteString(fmt.Sprintf("%s%.3f/query |\n\n", currencySymbol, *ops.WebSearch))
	} else {
		sb.WriteString("-          |\n\n")
	}

	return sb.String()
}

func getCurrencySymbol(currency string) string {
	switch currency {
	case "USD":
		return "$"
	case "EUR":
		return "€"
	case "GBP":
		return "£"
	case "JPY":
		return "¥"
	default:
		return "$" // Default to USD
	}
}

func formatNumber(n int) string {
	if n >= 1000000 {
		millions := float64(n) / 1000000
		if millions == float64(int(millions)) {
			return fmt.Sprintf("%.0fM", millions)
		}
		return fmt.Sprintf("%.1fM", millions)
	} else if n >= 1000 {
		thousands := float64(n) / 1000
		if thousands == float64(int(thousands)) {
			return fmt.Sprintf("%.0fK", thousands)
		}
		return fmt.Sprintf("%.1fK", thousands)
	}
	return fmt.Sprintf("%d", n)
}
