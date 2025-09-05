package docs

import (
	"fmt"
	"io"
	"strings"

	md "github.com/nao1215/markdown"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// formatTokenPricing formats token-based pricing.
//
//nolint:unused // used in tests
func formatTokenPricing(tokens *catalogs.ModelTokenPricing) string {
	if tokens == nil {
		return ""
	}

	var parts []string

	parts = append(parts, "**Token Pricing:**")

	// Input pricing
	if tokens.Input != nil {
		inputStr := formatTokenPrice(tokens.Input, "Input")
		if inputStr != "" {
			parts = append(parts, inputStr)
		}
	}

	// Output pricing
	if tokens.Output != nil {
		outputStr := formatTokenPrice(tokens.Output, "Output")
		if outputStr != "" {
			parts = append(parts, outputStr)
		}
	}

	// Reasoning pricing
	if tokens.Reasoning != nil {
		reasoningStr := formatTokenPrice(tokens.Reasoning, "Reasoning")
		if reasoningStr != "" {
			parts = append(parts, reasoningStr)
		}
	}

	// Cache read pricing
	if tokens.CacheRead != nil {
		cacheReadStr := formatTokenPrice(tokens.CacheRead, "Cache Read")
		if cacheReadStr != "" {
			parts = append(parts, cacheReadStr)
		}
	}

	// Cache write pricing
	if tokens.CacheWrite != nil {
		cacheWriteStr := formatTokenPrice(tokens.CacheWrite, "Cache Write")
		if cacheWriteStr != "" {
			parts = append(parts, cacheWriteStr)
		}
	}

	// Cache nested structure
	if tokens.Cache != nil {
		if tokens.Cache.Read != nil {
			cacheReadStr := formatTokenPrice(tokens.Cache.Read, "Cache Read")
			if cacheReadStr != "" {
				parts = append(parts, cacheReadStr)
			}
		}
		if tokens.Cache.Write != nil {
			cacheWriteStr := formatTokenPrice(tokens.Cache.Write, "Cache Write")
			if cacheWriteStr != "" {
				parts = append(parts, cacheWriteStr)
			}
		}
	}

	if len(parts) == 1 {
		return ""
	}
	return strings.Join(parts, "\n")
}

// formatTokenPrice formats a single token price entry.
//
//nolint:unused // used in tests
func formatTokenPrice(price *catalogs.ModelTokenCost, label string) string {
	var priceStrs []string

	if price.Per1M > 0 {
		priceStrs = append(priceStrs, fmt.Sprintf("$%.4f/1M tokens", price.Per1M))
	}
	if price.PerToken > 0 {
		priceStrs = append(priceStrs, fmt.Sprintf("$%.8f/token", price.PerToken))
	}

	if len(priceStrs) == 0 {
		if price.Per1M == 0 {
			return fmt.Sprintf("- %s: Free", label)
		}
		return ""
	}

	return fmt.Sprintf("- %s: %s", label, strings.Join(priceStrs, " | "))
}

// formatOperationPricing formats operation-based pricing.
//
//nolint:unused // used in tests
func formatOperationPricing(operations *catalogs.ModelOperationPricing) string {
	if operations == nil {
		return ""
	}

	var parts []string

	parts = append(parts, "**Operation Pricing:**")

	if operations.Request != nil && *operations.Request > 0 {
		parts = append(parts, fmt.Sprintf("- Per Request: $%.6f", *operations.Request))
	}

	if operations.ImageInput != nil && *operations.ImageInput > 0 {
		parts = append(parts, fmt.Sprintf("- Per Image Input: $%.4f", *operations.ImageInput))
	}

	if operations.AudioInput != nil && *operations.AudioInput > 0 {
		parts = append(parts, fmt.Sprintf("- Per Audio Input: $%.4f", *operations.AudioInput))
	}

	if operations.VideoInput != nil && *operations.VideoInput > 0 {
		parts = append(parts, fmt.Sprintf("- Per Video Input: $%.4f", *operations.VideoInput))
	}

	if operations.ImageGen != nil && *operations.ImageGen > 0 {
		parts = append(parts, fmt.Sprintf("- Per Image Generated: $%.4f", *operations.ImageGen))
	}

	if operations.AudioGen != nil && *operations.AudioGen > 0 {
		parts = append(parts, fmt.Sprintf("- Per Audio Generated: $%.4f", *operations.AudioGen))
	}

	if operations.VideoGen != nil && *operations.VideoGen > 0 {
		parts = append(parts, fmt.Sprintf("- Per Video Generated: $%.4f", *operations.VideoGen))
	}

	if operations.WebSearch != nil && *operations.WebSearch > 0 {
		parts = append(parts, fmt.Sprintf("- Per Web Search: $%.6f", *operations.WebSearch))
	}

	if operations.FunctionCall != nil && *operations.FunctionCall > 0 {
		parts = append(parts, fmt.Sprintf("- Per Function Call: $%.6f", *operations.FunctionCall))
	}

	if operations.ToolUse != nil && *operations.ToolUse > 0 {
		parts = append(parts, fmt.Sprintf("- Per Tool Use: $%.6f", *operations.ToolUse))
	}

	if len(parts) == 1 {
		return ""
	}
	return strings.Join(parts, "\n")
}

// formatPricePerMillion formats price per million tokens with appropriate precision.
//
//nolint:unused // used in tests
func formatPricePerMillion(price float64) string {
	if price == 0 {
		return Free
	}
	if price < 0.01 {
		return fmt.Sprintf("$%.6f", price)
	}
	if price < 1 {
		return fmt.Sprintf("$%.4f", price)
	}
	return fmt.Sprintf("$%.2f", price)
}

// writeCostCalculator writes a cost calculator section.
func writeCostCalculator(w io.Writer, model *catalogs.Model) {
	if model.Pricing == nil || model.Pricing.Tokens == nil {
		return
	}

	tokens := model.Pricing.Tokens
	if tokens.Input == nil || tokens.Output == nil {
		return
	}

	markdown := NewMarkdown(w)
	markdown.H3("💰 Cost Calculator").LF()
	markdown.PlainText("Calculate costs for common usage patterns:").LF().LF()

	// Define common workloads
	workloads := []struct {
		name         string
		inputTokens  int
		outputTokens int
	}{
		{"Quick chat (1K in, 500 out)", 1000, 500},
		{"Document summary (10K in, 1K out)", 10000, 1000},
		{"RAG query (50K in, 2K out)", 50000, 2000},
		{"Code generation (5K in, 10K out)", 5000, 10000},
	}

	rows := [][]string{}
	for _, w := range workloads {
		inputCost := (float64(w.inputTokens) / 1000000) * tokens.Input.Per1M
		outputCost := (float64(w.outputTokens) / 1000000) * tokens.Output.Per1M
		totalCost := inputCost + outputCost

		rows = append(rows, []string{
			w.name,
			fmt.Sprintf("%s tokens", formatNumber(w.inputTokens)),
			fmt.Sprintf("%s tokens", formatNumber(w.outputTokens)),
			formatPrice(totalCost),
		})
	}

	markdown.Table(md.TableSet{
		Header: []string{"Use Case", "Input", "Output", "Total Cost"},
		Rows:   rows,
	}).LF()

	// Add pricing formula
	markdown.Bold("Pricing Formula:").LF()

	var formulaLines []string
	formulaLines = append(formulaLines, fmt.Sprintf("Cost = (Input Tokens / 1M × $%.2f) + (Output Tokens / 1M × $%.2f)",
		tokens.Input.Per1M, tokens.Output.Per1M))

	if tokens.CacheRead != nil && tokens.CacheRead.Per1M > 0 {
		formulaLines = append(formulaLines, fmt.Sprintf("Cached Input Cost = Input Tokens / 1M × $%.2f", tokens.CacheRead.Per1M))
	}

	if tokens.Reasoning != nil && tokens.Reasoning.Per1M > 0 {
		formulaLines = append(formulaLines, fmt.Sprintf("Reasoning Cost = Reasoning Tokens / 1M × $%.2f", tokens.Reasoning.Per1M))
	}

	markdown.CodeBlock("", strings.Join(formulaLines, "\n")).LF()
	_ = markdown.Build()
}

// writeExampleCosts writes example costs for common scenarios.
func writeExampleCosts(w io.Writer, model *catalogs.Model) {
	if model.Pricing == nil || model.Pricing.Tokens == nil {
		return
	}

	tokens := model.Pricing.Tokens
	if tokens.Input == nil || tokens.Output == nil {
		return
	}

	markdown := NewMarkdown(w)
	markdown.H3("📊 Example Costs").LF()
	markdown.PlainText("Real-world usage examples and their costs:").LF().LF()

	// Calculate costs for different volumes
	volumes := []struct {
		name       string
		dailyChats int
		avgInput   int
		avgOutput  int
	}{
		{"Personal (10 chats/day)", 10, 1500, 750},
		{"Small Team (100 chats/day)", 100, 2000, 1000},
		{"Enterprise (1000 chats/day)", 1000, 3000, 1500},
	}

	rows := [][]string{}
	for _, v := range volumes {
		dailyInputTokens := v.dailyChats * v.avgInput
		dailyOutputTokens := v.dailyChats * v.avgOutput
		monthlyInputTokens := dailyInputTokens * 30
		monthlyOutputTokens := dailyOutputTokens * 30
		totalMonthlyTokens := monthlyInputTokens + monthlyOutputTokens

		monthlyInputCost := (float64(monthlyInputTokens) / 1000000) * tokens.Input.Per1M
		monthlyOutputCost := (float64(monthlyOutputTokens) / 1000000) * tokens.Output.Per1M
		monthlyTotalCost := monthlyInputCost + monthlyOutputCost

		rows = append(rows, []string{
			v.name,
			fmt.Sprintf("%d chats", v.dailyChats),
			formatNumber(totalMonthlyTokens),
			formatPrice(monthlyTotalCost),
		})
	}

	markdown.Table(md.TableSet{
		Header: []string{"Usage Tier", "Daily Volume", "Monthly Tokens", "Monthly Cost"},
		Rows:   rows,
	}).LF()

	_ = markdown.Build()
}
