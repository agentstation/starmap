package docs

import (
	"fmt"
	"io"
	"strings"

	md "github.com/nao1215/markdown"
)

// MarkdownBuilder wraps the markdown package to provide Hugo-specific functionality
type MarkdownBuilder struct {
	md       *md.Markdown
	writer   io.Writer
	buffer   *strings.Builder
	useBuffer bool
}

// NewMarkdownBuilder creates a new markdown builder
func NewMarkdownBuilder(w io.Writer) *MarkdownBuilder {
	return &MarkdownBuilder{
		md:     md.NewMarkdown(w),
		writer: w,
	}
}

// NewMarkdownBuilderBuffer creates a new markdown builder with internal buffer
func NewMarkdownBuilderBuffer() *MarkdownBuilder {
	buffer := &strings.Builder{}
	return &MarkdownBuilder{
		md:        md.NewMarkdown(buffer),
		writer:    buffer,
		buffer:    buffer,
		useBuffer: true,
	}
}

// Writer returns the underlying writer
func (m *MarkdownBuilder) Writer() io.Writer {
	return m.writer
}

// String returns the buffered content
func (m *MarkdownBuilder) String() string {
	if m.useBuffer && m.buffer != nil {
		return m.buffer.String()
	}
	return ""
}

// HugoFrontMatter adds Hugo front matter to the document
func (m *MarkdownBuilder) HugoFrontMatter(title string, weight int, opts ...FrontMatterOption) *MarkdownBuilder {
	fm := &FrontMatter{
		Title:  title,
		Weight: weight,
	}
	
	for _, opt := range opts {
		opt(fm)
	}
	
	// Write front matter directly to writer
	fmt.Fprintln(m.writer, "---")
	fmt.Fprintf(m.writer, "title: \"%s\"\n", fm.Title)
	if fm.Description != "" {
		fmt.Fprintf(m.writer, "description: \"%s\"\n", fm.Description)
	}
	fmt.Fprintf(m.writer, "weight: %d\n", fm.Weight)
	if fm.Author != "" {
		fmt.Fprintf(m.writer, "author: \"%s\"\n", fm.Author)
	}
	if fm.Draft {
		fmt.Fprintln(m.writer, "draft: true")
	}
	fmt.Fprintln(m.writer, "---")
	fmt.Fprintln(m.writer)
	
	return m
}

// FrontMatter represents Hugo front matter
type FrontMatter struct {
	Title       string
	Description string
	Weight      int
	Author      string
	Draft       bool
}

// FrontMatterOption is a functional option for front matter
type FrontMatterOption func(*FrontMatter)

// WithDescription adds a description to the front matter
func WithDescription(desc string) FrontMatterOption {
	return func(fm *FrontMatter) {
		fm.Description = desc
	}
}

// WithAuthor adds an author to the front matter
func WithAuthor(author string) FrontMatterOption {
	return func(fm *FrontMatter) {
		fm.Author = author
	}
}

// WithDraft marks the document as draft
func WithDraft() FrontMatterOption {
	return func(fm *FrontMatter) {
		fm.Draft = true
	}
}

// H1 creates a level 1 header
func (m *MarkdownBuilder) H1(text string) *MarkdownBuilder {
	m.md.H1(text)
	return m
}

// H2 creates a level 2 header
func (m *MarkdownBuilder) H2(text string) *MarkdownBuilder {
	m.md.H2(text)
	return m
}

// H3 creates a level 3 header
func (m *MarkdownBuilder) H3(text string) *MarkdownBuilder {
	m.md.H3(text)
	return m
}

// H4 creates a level 4 header
func (m *MarkdownBuilder) H4(text string) *MarkdownBuilder {
	m.md.H4(text)
	return m
}

// PlainText adds plain text
func (m *MarkdownBuilder) PlainText(text string) *MarkdownBuilder {
	m.md.PlainText(text)
	return m
}

// PlainTextf adds formatted plain text
func (m *MarkdownBuilder) PlainTextf(format string, args ...interface{}) *MarkdownBuilder {
	m.md.PlainTextf(format, args...)
	return m
}

// LF adds a line feed
func (m *MarkdownBuilder) LF() *MarkdownBuilder {
	m.md.LF()
	return m
}

// Bold adds bold text
func (m *MarkdownBuilder) Bold(text string) *MarkdownBuilder {
	m.md.PlainText(md.Bold(text))
	return m
}

// Italic adds italic text
func (m *MarkdownBuilder) Italic(text string) *MarkdownBuilder {
	m.md.PlainText(md.Italic(text))
	return m
}

// Code adds inline code
func (m *MarkdownBuilder) Code(code string) *MarkdownBuilder {
	m.md.PlainText(md.Code(code))
	return m
}

// CodeBlock adds a code block with syntax highlighting
func (m *MarkdownBuilder) CodeBlock(syntax, code string) *MarkdownBuilder {
	m.md.CodeBlocks(md.SyntaxHighlight(syntax), code)
	return m
}

// Link adds a markdown link
func (m *MarkdownBuilder) Link(text, url string) *MarkdownBuilder {
	m.md.PlainText(md.Link(text, url))
	return m
}

// Image adds an image with optional styling
func (m *MarkdownBuilder) Image(alt, url string) *MarkdownBuilder {
	m.md.PlainText(md.Image(alt, url))
	return m
}

// ImageWithStyle adds an image with inline CSS styling for Hugo
func (m *MarkdownBuilder) ImageWithStyle(alt, url string, width, height int) *MarkdownBuilder {
	// Hugo supports HTML in markdown, so we can use img tags
	html := fmt.Sprintf(`<img src="%s" alt="%s" width="%d" height="%d" style="vertical-align: middle;">`, 
		url, alt, width, height)
	m.md.PlainText(html)
	return m
}

// BulletList adds a bullet list
func (m *MarkdownBuilder) BulletList(items ...string) *MarkdownBuilder {
	m.md.BulletList(items...)
	return m
}

// OrderedList adds an ordered list
func (m *MarkdownBuilder) OrderedList(items ...string) *MarkdownBuilder {
	m.md.OrderedList(items...)
	return m
}

// Table adds a markdown table
func (m *MarkdownBuilder) Table(table md.TableSet) *MarkdownBuilder {
	m.md.Table(table)
	return m
}

// FeatureTable creates a table with checkmarks/crosses for features
func (m *MarkdownBuilder) FeatureTable(headers []string, rows [][]bool) *MarkdownBuilder {
	tableRows := make([][]string, len(rows))
	for i, row := range rows {
		tableRows[i] = make([]string, len(row))
		for j, val := range row {
			if val {
				tableRows[i][j] = "✅"
			} else {
				tableRows[i][j] = "❌"
			}
		}
	}
	
	m.md.Table(md.TableSet{
		Header: headers,
		Rows:   tableRows,
	})
	return m
}

// HorizontalRule adds a horizontal rule
func (m *MarkdownBuilder) HorizontalRule() *MarkdownBuilder {
	m.md.HorizontalRule()
	return m
}

// Blockquote adds a blockquote
func (m *MarkdownBuilder) Blockquote(text string) *MarkdownBuilder {
	m.md.Blockquote(text)
	return m
}

// Alert adds a GitHub-style alert
func (m *MarkdownBuilder) Alert(alertType string, text string) *MarkdownBuilder {
	// GitHub-style alerts using blockquotes
	alert := fmt.Sprintf("> [!%s]\n> %s", strings.ToUpper(alertType), text)
	m.md.PlainText(alert).LF().LF()
	return m
}

// NavigationFooter adds a standard navigation footer for Hugo docs
func (m *MarkdownBuilder) NavigationFooter(prevText, prevURL, nextText, nextURL, upText, upURL string) *MarkdownBuilder {
	m.HorizontalRule()
	m.Italic("_")
	
	parts := []string{}
	if prevText != "" && prevURL != "" {
		parts = append(parts, fmt.Sprintf("[← %s](%s)", prevText, prevURL))
	}
	if upText != "" && upURL != "" {
		parts = append(parts, fmt.Sprintf("[← %s](%s)", upText, upURL))
	}
	if nextText != "" && nextURL != "" {
		parts = append(parts, fmt.Sprintf("[%s →](%s)", nextText, nextURL))
	}
	
	parts = append(parts, "Generated by [Starmap](https://github.com/agentstation/starmap)")
	
	m.PlainText(strings.Join(parts, " | "))
	m.Italic("_")
	m.LF()
	
	return m
}

// Build finalizes the markdown document
func (m *MarkdownBuilder) Build() error {
	return m.md.Build()
}

// Badge adds a badge with custom text and color
func (m *MarkdownBuilder) Badge(label, message, color string) *MarkdownBuilder {
	url := fmt.Sprintf("https://img.shields.io/badge/%s-%s-%s", 
		strings.ReplaceAll(label, " ", "%20"),
		strings.ReplaceAll(message, " ", "%20"),
		color)
	m.Image(fmt.Sprintf("%s: %s", label, message), url)
	return m
}

// BoldLink creates a bold link: **[text](url)**
func (m *MarkdownBuilder) BoldLink(text, url string) *MarkdownBuilder {
	m.PlainText("**")
	m.Link(text, url)
	m.PlainText("**")
	return m
}

// CodeInline adds inline code
func (m *MarkdownBuilder) CodeInline(text string) *MarkdownBuilder {
	m.PlainText("`")
	m.PlainText(text)
	m.PlainText("`")
	return m
}

// CountText adds formatted count text (e.g., "5 models")
func (m *MarkdownBuilder) CountText(count int, singular, plural string) *MarkdownBuilder {
	if count == 1 {
		m.PlainTextf("%d %s", count, singular)
	} else {
		m.PlainTextf("%d %s", count, plural)
	}
	return m
}

// TruncateText adds text that may be truncated with ellipsis
func (m *MarkdownBuilder) TruncateText(text string, maxLen int) *MarkdownBuilder {
	if len(text) > maxLen && maxLen > 3 {
		m.PlainText(text[:maxLen-3] + "...")
	} else {
		m.PlainText(text)
	}
	return m
}

// FormatModelID formats a model ID with code style
func (m *MarkdownBuilder) FormatModelID(id string) *MarkdownBuilder {
	return m.CodeInline(id)
}

// FormatCurrency formats a currency value with symbol
func (m *MarkdownBuilder) FormatCurrency(value float64, currency string) *MarkdownBuilder {
	symbol := getCurrencySymbol(currency)
	if value == 0 {
		m.PlainText("-")
	} else if value < 1 {
		m.PlainTextf("%s%.3f", symbol, value)
	} else {
		m.PlainTextf("%s%.2f", symbol, value)
	}
	return m
}

// BooleanCheck adds a checkmark or X based on boolean value
func (m *MarkdownBuilder) BooleanCheck(value bool) *MarkdownBuilder {
	if value {
		m.PlainText("✅")
	} else {
		m.PlainText("❌")
	}
	return m
}

// JoinList joins items with a separator
func (m *MarkdownBuilder) JoinList(items []string, separator string) *MarkdownBuilder {
	for i, item := range items {
		if i > 0 {
			m.PlainText(separator)
		}
		m.PlainText(item)
	}
	return m
}

// ConditionalSection adds content only if condition is true
func (m *MarkdownBuilder) ConditionalSection(condition bool, f func(*MarkdownBuilder)) *MarkdownBuilder {
	if condition {
		f(m)
	}
	return m
}

// ComparisonTable creates a comparison table with provider logos
func (m *MarkdownBuilder) ComparisonTable(headers []string, rows [][]interface{}) *MarkdownBuilder {
	stringRows := make([][]string, len(rows))
	for i, row := range rows {
		stringRows[i] = make([]string, len(row))
		for j, cell := range row {
			switch v := cell.(type) {
			case string:
				stringRows[i][j] = v
			case int:
				stringRows[i][j] = fmt.Sprintf("%d", v)
			case float64:
				stringRows[i][j] = fmt.Sprintf("%.2f", v)
			case bool:
				if v {
					stringRows[i][j] = "✅"
				} else {
					stringRows[i][j] = "❌"
				}
			default:
				stringRows[i][j] = fmt.Sprintf("%v", v)
			}
		}
	}
	
	m.Table(md.TableSet{
		Header: headers,
		Rows:   stringRows,
	})
	return m
}

// CheckboxList adds a checkbox list
func (m *MarkdownBuilder) CheckboxList(items []struct {
	Text    string
	Checked bool
}) *MarkdownBuilder {
	checkItems := make([]md.CheckBoxSet, len(items))
	for i, item := range items {
		checkItems[i] = md.CheckBoxSet{
			Text:    item.Text,
			Checked: item.Checked,
		}
	}
	m.md.CheckBox(checkItems)
	return m
}

// Details adds a collapsible details section (HTML in markdown)
func (m *MarkdownBuilder) Details(summary, content string) *MarkdownBuilder {
	html := fmt.Sprintf("<details>\n<summary>%s</summary>\n\n%s\n\n</details>", summary, content)
	m.md.PlainText(html).LF().LF()
	return m
}

// RawHTML adds raw HTML (useful for Hugo shortcodes or custom elements)
func (m *MarkdownBuilder) RawHTML(html string) *MarkdownBuilder {
	m.md.PlainText(html).LF()
	return m
}