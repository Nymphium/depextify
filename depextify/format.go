package depextify

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"gopkg.in/yaml.v3"
)

const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBold   = "\033[1m"
	colorRed    = "\033[31m"

	// DefaultLexer is the default lexer to use for syntax highlighting.
	DefaultLexer = "bash"
	// DefaultStyle is the default style to use for syntax highlighting.
	DefaultStyle = "monokai"
)

type highlightRange struct {
	start int
	end   int
}

var formatter = formatters.TTY256

func highlightCode(code string, lexerName, styleName string, hl *highlightRange) string {
	lexer := lexers.Get(lexerName)
	if lexer == nil {
		lexer = lexers.Get(DefaultLexer)
	}
	style := styles.Get(styleName)
	if style == nil {
		style = styles.Get(DefaultStyle)
	}

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return code // Fallback
	}

	var sb strings.Builder

	// Helper to format a single token using chroma formatter
	formatToken := func(t chroma.Token) {
		_ = formatter.Format(&sb, style, chroma.Literator(t))
	}

	currentPos := 0
	for _, token := range iterator.Tokens() {
		tLen := len(token.Value)
		tEnd := currentPos + tLen

		if hl == nil || tEnd <= hl.start || currentPos >= hl.end {
			// No intersection or highlight is nil
			formatToken(token)
		} else {
			// Intersection
			// Overlap range
			ovStart := max(currentPos, hl.start)
			ovEnd := min(tEnd, hl.end)

			// Part before highlight
			if currentPos < ovStart {
				before := token.Value[:ovStart-currentPos]
				formatToken(chroma.Token{Type: token.Type, Value: before})
			}

			// Highlighted part
			// We strip syntax highlighting for the command itself and enforce Bold Red.

			sb.WriteString(colorBold + colorRed + token.Value[ovStart-currentPos:ovEnd-currentPos] + colorReset)

			// Part after highlight
			if tEnd > ovEnd {
				after := token.Value[ovEnd-currentPos:]
				formatToken(chroma.Token{Type: token.Type, Value: after})
			}
		}

		currentPos += tLen
	}

	return strings.TrimSpace(sb.String())
}

// FormatConfig contains settings for output formatting.
type FormatConfig struct {
	ShowCount   bool   `yaml:"show_count"`
	ShowPos     bool   `yaml:"show_pos"`
	UseColor    bool   `yaml:"use_color"`
	LexerName   string `yaml:"lexer"`
	StyleName   string `yaml:"style"`
	IsDirectory bool   `yaml:"-"`
}

// Formatter defines the interface for formatting the scan results.
type Formatter interface {
	Format(result ScanResult) (string, error)
}

// TextFormatter formats the result as plain text.
type TextFormatter struct {
	Config *FormatConfig
}

// Format returns a formatted string representation of the result.
func (f *TextFormatter) Format(r ScanResult) (string, error) {
	c := f.Config
	paint := func(color, s string) string {
		if c.UseColor {
			return color + s + colorReset
		}
		return s
	}
	colon := paint(colorYellow, ":")

	lineWidth := 0
	if c.ShowPos {
		maxLine := 0
		for _, cmds := range r {
			for _, occs := range cmds {
				for _, occ := range occs {
					maxLine = max(maxLine, occ.Line)
				}
			}
		}
		lineWidth = len(strconv.Itoa(maxLine))
	}

	indent := ""
	if c.IsDirectory {
		indent = "  "
	}

	var sb strings.Builder
	for _, path := range slices.Sorted(maps.Keys(r)) {
		if c.IsDirectory {
			sb.WriteString(paint(colorCyan, path) + "\n")
		}

		lexerName := c.LexerName
		if lexerName == DefaultLexer {
			if l := lexers.Match(path); l != nil {
				lexerName = l.Config().Name
			}
		}

		for _, cmd := range slices.Sorted(maps.Keys(r[path])) {
			occs := r[path][cmd]

			suffix := ""
			if c.ShowCount || c.ShowPos {
				suffix = colon
			}
			if c.ShowCount {
				suffix += paint(colorGreen, fmt.Sprintf(" %d", len(occs)))
			}
			fmt.Fprintf(&sb, "%s%s%s\n", indent, paint(colorBold, cmd), suffix)

			if !c.ShowPos {
				continue
			}
			for _, occ := range occs {
				content := strings.TrimSpace(occ.FullLine)
				if c.UseColor {
					start := occ.Col - 1
					end := start + occ.Len
					var hl *highlightRange
					if start >= 0 && end <= len(occ.FullLine) {
						hl = &highlightRange{start: start, end: end}
					}
					content = highlightCode(occ.FullLine, lexerName, c.StyleName, hl)
				}
				ln := paint(colorGreen, fmt.Sprintf("%*d", lineWidth, occ.Line))
				fmt.Fprintf(&sb, "%s  %s%s  %s\n", indent, ln, colon, content)
			}
		}
	}
	return sb.String(), nil
}

func transformResult(r ScanResult, c *FormatConfig) any {
	if c.ShowPos {
		return r
	}

	if c.ShowCount {
		summary := make(map[string]map[string]int)
		for file, cmds := range r {
			summary[file] = make(map[string]int)
			for cmd, occs := range cmds {
				summary[file][cmd] = len(occs)
			}
		}
		return summary
	}

	list := make(map[string][]string)
	for file, cmds := range r {
		list[file] = slices.Sorted(maps.Keys(cmds))
	}
	return list
}

// JSONFormatter formats the result as JSON.
type JSONFormatter struct {
	Config *FormatConfig
}

// Format returns the JSON encoding of the result.
func (f *JSONFormatter) Format(r ScanResult) (string, error) {
	data := transformResult(r, f.Config)
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// YAMLFormatter formats the result as YAML.
type YAMLFormatter struct {
	Config *FormatConfig
}

// Format returns the YAML encoding of the result.
func (f *YAMLFormatter) Format(r ScanResult) (string, error) {
	data := transformResult(r, f.Config)
	b, err := yaml.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// MarkdownFormatter formats the result as nested Markdown lists, for pasting into documents.
type MarkdownFormatter struct {
	Config *FormatConfig
}

// Format returns the Markdown representation of the result.
func (f *MarkdownFormatter) Format(r ScanResult) (string, error) {
	c := f.Config
	indent := ""
	if c.IsDirectory {
		indent = "  "
	}

	var sb strings.Builder
	for _, path := range slices.Sorted(maps.Keys(r)) {
		if c.IsDirectory {
			fmt.Fprintf(&sb, "- %s\n", markdownCode(path))
		}
		for _, cmd := range slices.Sorted(maps.Keys(r[path])) {
			occs := r[path][cmd]
			fmt.Fprintf(&sb, "%s- %s", indent, markdownCode(cmd))
			if c.ShowCount {
				fmt.Fprintf(&sb, ": %d", len(occs))
			}
			sb.WriteString("\n")
			if c.ShowPos {
				for _, occ := range occs {
					fmt.Fprintf(&sb, "%s  - %d: %s\n", indent, occ.Line, markdownCode(strings.TrimSpace(occ.FullLine)))
				}
			}
		}
	}
	return sb.String(), nil
}

// markdownCode returns s as a Markdown code span, fenced by more backticks than any run inside s.
func markdownCode(s string) string {
	fence := "`"
	for strings.Contains(s, fence) {
		fence += "`"
	}
	if len(fence) > 1 {
		return fence + " " + s + " " + fence
	}
	return fence + s + fence
}
