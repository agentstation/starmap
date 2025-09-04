package docs

import (
	"fmt"
	"io"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	md "github.com/nao1215/markdown"
)

// Markdown wraps the markdown package to provide Hugo-specific functionality
type Markdown struct {
	md        *md.Markdown
	writer    io.Writer
	buffer    *strings.Builder
	useBuffer bool
}

// NewMarkdown creates a new markdown builder
func NewMarkdown(w io.Writer) *Markdown {
	return &Markdown{
		md:     md.NewMarkdown(w),
		writer: w,
	}
}

// NewMarkdownBuffer creates a new markdown builder with internal buffer
func NewMarkdownBuffer() *Markdown {
	buffer := &strings.Builder{}
	return &Markdown{
		md:        md.NewMarkdown(buffer),
		writer:    buffer,
		buffer:    buffer,
		useBuffer: true,
	}
}

// Writer returns the underlying writer
func (m *Markdown) Writer() io.Writer {
	return m.writer
}

// String returns the buffered content
func (m *Markdown) String() string {
	if m.useBuffer && m.buffer != nil {
		return m.buffer.String()
	}
	return ""
}

// HugoFrontMatter adds Hugo front matter to the document
func (m *Markdown) HugoFrontMatter(title string, weight int, opts ...FrontMatterOption) *Markdown {
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
	if fm.Menu != nil {
		fmt.Fprintln(m.writer, "menu:")
		if fm.Menu.Before != nil {
			fmt.Fprintln(m.writer, "  before:")
			fmt.Fprintf(m.writer, "    weight: %d\n", fm.Menu.Before.Weight)
		}
		if fm.Menu.After != nil {
			fmt.Fprintln(m.writer, "  after:")
			fmt.Fprintf(m.writer, "    weight: %d\n", fm.Menu.After.Weight)
		}
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
	Menu        *MenuConfig
}

// MenuConfig represents menu configuration in Hugo front matter
type MenuConfig struct {
	Before *MenuEntry
	After  *MenuEntry
}

// MenuEntry represents a single menu entry
type MenuEntry struct {
	Weight int
}

// FrontMatterOption is a functional option for front matter
type FrontMatterOption func(*FrontMatter)

// WithMenu adds menu configuration to the front matter
func WithMenu(menuType string, weight int) FrontMatterOption {
	return func(fm *FrontMatter) {
		if fm.Menu == nil {
			fm.Menu = &MenuConfig{}
		}
		if menuType == "before" {
			fm.Menu.Before = &MenuEntry{Weight: weight}
		} else if menuType == "after" {
			fm.Menu.After = &MenuEntry{Weight: weight}
		}
	}
}

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
func (m *Markdown) H1(text string) *Markdown {
	m.md.H1(text)
	return m
}

// H2 creates a level 2 header
func (m *Markdown) H2(text string) *Markdown {
	m.md.H2(text)
	return m
}

// H3 creates a level 3 header
func (m *Markdown) H3(text string) *Markdown {
	m.md.H3(text)
	return m
}

// H4 creates a level 4 header
func (m *Markdown) H4(text string) *Markdown {
	m.md.H4(text)
	return m
}

// PlainText adds plain text
func (m *Markdown) PlainText(text string) *Markdown {
	m.md.PlainText(text)
	return m
}

// PlainTextf adds formatted plain text
func (m *Markdown) PlainTextf(format string, args ...any) *Markdown {
	m.md.PlainTextf(format, args...)
	return m
}

// LF adds a line feed
func (m *Markdown) LF() *Markdown {
	m.md.LF()
	return m
}

// Bold adds bold text
func (m *Markdown) Bold(text string) *Markdown {
	m.md.PlainText(md.Bold(text))
	return m
}

// Italic adds italic text
func (m *Markdown) Italic(text string) *Markdown {
	m.md.PlainText(md.Italic(text))
	return m
}

// Code adds inline code
func (m *Markdown) Code(code string) *Markdown {
	m.md.PlainText(md.Code(code))
	return m
}

// CodeBlock adds a code block with syntax highlighting
func (m *Markdown) CodeBlock(syntax, code string) *Markdown {
	m.md.CodeBlocks(md.SyntaxHighlight(syntax), code)
	return m
}

// Link adds a markdown link
func (m *Markdown) Link(text, url string) *Markdown {
	m.md.PlainText(md.Link(text, url))
	return m
}

// Image adds an image with optional styling
func (m *Markdown) Image(alt, url string) *Markdown {
	m.md.PlainText(md.Image(alt, url))
	return m
}

// ImageWithStyle adds an image with inline CSS styling for Hugo
func (m *Markdown) ImageWithStyle(alt, url string, width, height int) *Markdown {
	// Hugo supports HTML in markdown, so we can use img tags
	html := fmt.Sprintf(`<img src="%s" alt="%s" width="%d" height="%d" style="vertical-align: middle;">`,
		url, alt, width, height)
	m.md.PlainText(html)
	return m
}

// BulletList adds a bullet list
func (m *Markdown) BulletList(items ...string) *Markdown {
	m.md.BulletList(items...)
	return m
}

// OrderedList adds an ordered list
func (m *Markdown) OrderedList(items ...string) *Markdown {
	m.md.OrderedList(items...)
	return m
}

// Table adds a markdown table
func (m *Markdown) Table(table md.TableSet) *Markdown {
	m.md.Table(table)
	return m
}

// FeatureTable creates a table with checkmarks/crosses for features
func (m *Markdown) FeatureTable(headers []string, rows [][]bool) *Markdown {
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
func (m *Markdown) HorizontalRule() *Markdown {
	m.md.HorizontalRule()
	return m
}

// Blockquote adds a blockquote
func (m *Markdown) Blockquote(text string) *Markdown {
	m.md.Blockquote(text)
	return m
}

// Alert adds a GitHub-style alert
func (m *Markdown) Alert(alertType string, text string) *Markdown {
	// GitHub-style alerts using blockquotes
	alert := fmt.Sprintf("> [!%s]\n> %s", strings.ToUpper(alertType), text)
	m.md.PlainText(alert).LF().LF()
	return m
}

// NavigationFooter adds a standard navigation footer for Hugo docs
func (m *Markdown) NavigationFooter(prevText, prevURL, nextText, nextURL, upText, upURL string) *Markdown {
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
func (m *Markdown) Build() error {
	return m.md.Build()
}

// Badge adds a badge with custom text and color
func (m *Markdown) Badge(label, message, color string) *Markdown {
	url := fmt.Sprintf("https://img.shields.io/badge/%s-%s-%s",
		strings.ReplaceAll(label, " ", "%20"),
		strings.ReplaceAll(message, " ", "%20"),
		color)
	m.Image(fmt.Sprintf("%s: %s", label, message), url)
	return m
}

// BoldLink creates a bold link: **[text](url)**
func (m *Markdown) BoldLink(text, url string) *Markdown {
	m.PlainText("**")
	m.Link(text, url)
	m.PlainText("**")
	return m
}

// CodeInline adds inline code
func (m *Markdown) CodeInline(text string) *Markdown {
	m.PlainText("`")
	m.PlainText(text)
	m.PlainText("`")
	return m
}

// CountText adds formatted count text (e.g., "5 models")
func (m *Markdown) CountText(count int, singular, plural string) *Markdown {
	if count == 1 {
		m.PlainTextf("%d %s", count, singular)
	} else {
		m.PlainTextf("%d %s", count, plural)
	}
	return m
}

// TruncateText adds text that may be truncated with ellipsis
func (m *Markdown) TruncateText(text string, maxLen int) *Markdown {
	if len(text) > maxLen && maxLen > 3 {
		m.PlainText(text[:maxLen-3] + "...")
	} else {
		m.PlainText(text)
	}
	return m
}

// FormatModelID formats a model ID with code style
func (m *Markdown) FormatModelID(id string) *Markdown {
	return m.CodeInline(id)
}

// FormatCurrency formats a currency value with symbol
func (m *Markdown) FormatCurrency(value float64, currency string) *Markdown {
	symbol := catalogs.ModelPricingCurrency(currency).Symbol()
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
func (m *Markdown) BooleanCheck(value bool) *Markdown {
	if value {
		m.PlainText("✅")
	} else {
		m.PlainText("❌")
	}
	return m
}

// JoinList joins items with a separator
func (m *Markdown) JoinList(items []string, separator string) *Markdown {
	for i, item := range items {
		if i > 0 {
			m.PlainText(separator)
		}
		m.PlainText(item)
	}
	return m
}

// ConditionalSection adds content only if condition is true
func (m *Markdown) ConditionalSection(condition bool, f func(*Markdown)) *Markdown {
	if condition {
		f(m)
	}
	return m
}

// ComparisonTable creates a comparison table with provider logos
func (m *Markdown) ComparisonTable(headers []string, rows [][]any) *Markdown {
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
func (m *Markdown) CheckboxList(items []struct {
	Text    string
	Checked bool
}) *Markdown {
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
func (m *Markdown) Details(summary, content string) *Markdown {
	html := fmt.Sprintf("<details>\n<summary>%s</summary>\n\n%s\n\n</details>", summary, content)
	m.md.PlainText(html).LF().LF()
	return m
}

// RawHTML adds raw HTML (useful for Hugo shortcodes or custom elements)
func (m *Markdown) RawHTML(html string) *Markdown {
	m.md.PlainText(html).LF()
	return m
}
