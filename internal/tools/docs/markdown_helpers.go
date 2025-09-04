package docs

import (
	"fmt"
	"strings"
)

// buildBoldLink creates a bold markdown link string: **[text](url)**
func buildBoldLink(text, url string) string {
	return fmt.Sprintf("**[%s](%s)**", text, url)
}

// buildCountText creates formatted count text (e.g., "5 models")
func buildCountText(count int, singular, plural string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}

// buildTruncatedText truncates text with ellipsis
func buildTruncatedText(text string, maxLen int) string {
	if len(text) > maxLen && maxLen > 3 {
		return text[:maxLen-3] + "..."
	}
	return text
}

// buildJoinedList joins items with a separator
func buildJoinedList(items []string, separator string) string {
	return strings.Join(items, separator)
}