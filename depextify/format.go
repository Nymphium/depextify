package depextify

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"gopkg.in/yaml.v3"
)

// Occurrence represents a single occurrence of a command.
type Occurrence struct {
	Line     int
	Col      int
	Len      int
	FullLine string
}

// ScanResult maps filename to its command occurrences: filename -> {cmd: []Occurrence}
type ScanResult map[string]map[string][]Occurrence

const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBold   = "\033[1m"
	colorRed    = "\033[31m"
)

const (
	DefaultLexer = "bash"
	DefaultStyle = "monokai"
)

var formatter = formatters.TTY256

func emphasize(code, lexerName, styleName string) string {
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
		return code
	}
	var sb strings.Builder
	err = formatter.Format(&sb, style, iterator)
	if err != nil {
		return code
	}
	return strings.TrimSpace(sb.String())
}

// applyHighlight applies bold red to a specific range in a string that may contain ANSI codes.
func applyHighlight(text string, start, end int) string {
	var sb strings.Builder
	pos := 0
	inANSI := false

	for i := 0; i < len(text); i++ {
		if text[i] == '\x1b' {
			inANSI = true
			sb.WriteByte(text[i])
			continue
		}
		if inANSI {
			sb.WriteByte(text[i])
			if text[i] == 'm' {
				inANSI = false
			}
			continue
		}

		if pos == start {
			sb.WriteString(colorBold + colorRed)
		}
		sb.WriteByte(text[i])
		pos++
		if pos == end {
			sb.WriteString(colorReset)
		}
	}
	return sb.String()
}

// Format returns a formatted string representation of the result.
func (r ScanResult) Format(c *Config) string {
	var sb strings.Builder

	globalLineWidth := 0
	if c.ShowPos {
		maxLine := 0
		for _, cmds := range r {
			for _, occs := range cmds {
				for _, occ := range occs {
					if occ.Line > maxLine {
						maxLine = occ.Line
					}
				}
			}
		}
		globalLineWidth = len(fmt.Sprintf("%d", maxLine))
	}

	paths := slices.Sorted(maps.Keys(r))
	for _, path := range paths {
		if c.IsDirectory {
			p := path
			if c.UseColor {
				p = colorCyan + path + colorReset
			}
			sb.WriteString(p + "\n")
		}

		indent := ""
		if c.IsDirectory {
			indent = "  "
		}

		cmds := slices.Sorted(maps.Keys(r[path]))
		for _, cmd := range cmds {
			occs := r[path][cmd]

			cCmd := cmd
			if c.UseColor {
				cCmd = colorBold + cmd + colorReset
			}

			suffix := ""
			colon := ":"
			if c.UseColor {
				colon = colorYellow + ":" + colorReset
			}

			if c.ShowCount || c.ShowPos {
				suffix = colon
				if c.ShowCount {
					count := fmt.Sprintf(" %d", len(occs))
					if c.UseColor {
						count = colorGreen + count + colorReset
					}
					suffix += count
				}
			}

			fmt.Fprintf(&sb, "%s%s%s\n", indent, cCmd, suffix)

			if c.ShowPos {
				for _, occ := range occs {
					ln := fmt.Sprintf("%*d", globalLineWidth, occ.Line)
					if c.UseColor {
						ln = colorGreen + ln + colorReset
					}
					cln := ":"
					if c.UseColor {
						cln = colorYellow + ":" + colorReset
					}

					content := occ.FullLine
					if c.UseColor {
						start := occ.Col - 1
						end := start + occ.Len
						content = emphasize(content, c.LexerName, c.StyleName)
						if start >= 0 && end <= len(occ.FullLine) {
							content = applyHighlight(content, start, end)
						}
					} else {
						content = strings.TrimSpace(content)
					}
					fmt.Fprintf(&sb, "%s  %s%s  %s\n", indent, ln, cln, content)
				}
			}
		}
	}
	return sb.String()
}

// JSON returns the JSON encoding of the result.
func (r ScanResult) JSON() (string, error) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// YAML returns the YAML encoding of the result.
func (r ScanResult) YAML() (string, error) {
	b, err := yaml.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
