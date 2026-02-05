package depextify

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
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

	DefaultLexer = "bash"
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

	// Helper to format a highlighted string
	formatHighlight := func(s string) {
		sb.WriteString(colorBold + colorRed + s + colorReset)
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

			mid := token.Value[ovStart-currentPos : ovEnd-currentPos]
			formatHighlight(mid)

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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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

						lexerName := c.LexerName
						if lexerName == DefaultLexer {
							if l := lexers.Match(path); l != nil {
								lexerName = l.Config().Name
							}
						}

						hl := &highlightRange{start: start, end: end}
						if start < 0 || end > len(occ.FullLine) {
							hl = nil
						}

						content = highlightCode(content, lexerName, c.StyleName, hl)
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
func (r ScanResult) JSON(c *Config) (string, error) {
	var data interface{} = r

	if !c.ShowPos {
		if c.ShowCount {
			summary := make(map[string]map[string]int)
			for file, cmds := range r {
				summary[file] = make(map[string]int)
				for cmd, occs := range cmds {
					summary[file][cmd] = len(occs)
				}
			}
			data = summary
		} else {
			list := make(map[string][]string)
			for file, cmds := range r {
				keys := make([]string, 0, len(cmds))
				for cmd := range cmds {
					keys = append(keys, cmd)
				}
				slices.Sort(keys)
				list[file] = keys
			}
			data = list
		}
	}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// YAML returns the YAML encoding of the result.
func (r ScanResult) YAML(c *Config) (string, error) {
	var data interface{} = r

	if !c.ShowPos {
		if c.ShowCount {
			summary := make(map[string]map[string]int)
			for file, cmds := range r {
				summary[file] = make(map[string]int)
				for cmd, occs := range cmds {
					summary[file][cmd] = len(occs)
				}
			}
			data = summary
		} else {
			list := make(map[string][]string)
			for file, cmds := range r {
				keys := make([]string, 0, len(cmds))
				for cmd := range cmds {
					keys = append(keys, cmd)
				}
				slices.Sort(keys)
				list[file] = keys
			}
			data = list
		}
	}

	b, err := yaml.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

