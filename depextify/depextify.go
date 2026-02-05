package depextify

import (
	"bufio"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"mvdan.cc/sh/v3/syntax"
)

// Config holds the configuration for scanning and formatting.
type Config struct {
	NoBuiltins   bool
	NoCoreutils  bool
	NoCommon     bool
	ShowHidden   bool
	ExtraIgnores []string

	ShowCount   bool
	ShowPos     bool
	UseColor    bool
	LexerName   string
	StyleName   string
	IsDirectory bool
}

// Occurrence represents a single occurrence of a command.
type Occurrence struct {
	Line     int
	Col      int
	Len      int
	FullLine string
}

// Result maps filename to its command occurrences: filename -> {cmd: []Occurrence}
type Result map[string]map[string][]Occurrence

type posInfo struct {
	line uint
	col  uint
	len  uint
}

const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBold   = "\033[1m"
	colorRed    = "\033[31m"
)

var (
	formatter  = formatters.TTY256
	reShellExt = regexp.MustCompile(`\.(ba|b|z|k|da)?sh$`)
	reShebang  = regexp.MustCompile(`^#!\s*/.*(sh|bash|zsh|ksh)`)
)

func emphasize(code, lexerName, styleName string) string {
	lexer := lexers.Get(lexerName)
	if lexer == nil {
		lexer = lexers.Get("bash")
	}
	style := styles.Get(styleName)
	if style == nil {
		style = styles.Get("monokai")
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
		if text[i] == '\033' {
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

func toInt(u uint) int {
	if u > uint(^uint(0)>>1) {
		return 0
	}
	return int(u)
}

// collectLocalFuncs() collects name of f()
func collectLocalFuncs(file *syntax.File) map[string]bool {
	localFuncs := make(map[string]bool)
	syntax.Walk(file, func(node syntax.Node) bool {
		if fn, ok := node.(*syntax.FuncDecl); ok {
			localFuncs[fn.Name.Value] = true
		}
		return true
	})

	return localFuncs
}

// collectCommands() collects command names from CallExpr nodes with filtering:
// - not local functions
// - not starting with '-'
func collectCommands(file *syntax.File, localFuncs map[string]bool) map[string][]posInfo {
	commands := make(map[string][]posInfo)

	syntax.Walk(file, func(node syntax.Node) bool {
		switch x := node.(type) {
		case *syntax.CallExpr:
			if len(x.Args) > 0 && len(x.Args[0].Parts) == 1 {
				if part, ok := x.Args[0].Parts[0].(*syntax.Lit); ok {
					cmd := part.Value

					if !localFuncs[cmd] && !strings.HasPrefix(cmd, "-") {
						commands[cmd] = append(commands[cmd], posInfo{
							line: x.Pos().Line(),
							col:  x.Pos().Col(),
							len:  uint(len(cmd)),
						})
					}
				}
			}
		}
		return true
	})

	return commands
}

// Do analyzes the given shell script file and returns a map of command names to their positions.
func Do(f *os.File) (map[string][]posInfo, error) {
	parser := syntax.NewParser()
	file, err := parser.Parse(f, "")
	if err != nil {
		return nil, err
	}

	localFuncs := collectLocalFuncs(file)
	return collectCommands(file, localFuncs), nil
}

// Format returns a formatted string representation of the result.
func (r Result) Format(c *Config) string {
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

func isShellFile(path string) bool {
	if reShellExt.MatchString(path) {
		return true
	}

	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	reader := bufio.NewReader(f)
	line, _, err := reader.ReadLine()
	if err != nil {
		return false
	}

	return reShebang.MatchString(string(line))
}

func (c *Config) calculateFileOccurrences(cmdPositions map[string][]posInfo, lines []string, ignores map[string]bool) map[string][]Occurrence {
	fileOccs := make(map[string][]Occurrence)
	for cmd, ps := range cmdPositions {
		if (c.NoBuiltins && builtins[cmd]) || (c.NoCoreutils && coreutils[cmd]) || (c.NoCommon && common[cmd]) || ignores[cmd] {
			continue
		}
		for _, p := range ps {
			if p.line > 0 && p.line <= uint(len(lines)) {
				fileOccs[cmd] = append(fileOccs[cmd], Occurrence{
					Line:     toInt(p.line),
					Col:      toInt(p.col),
					Len:      toInt(p.len),
					FullLine: lines[p.line-1],
				})
			}
		}
	}
	return fileOccs
}

func (c *Config) processFile(path string, skipCheck bool, ignores map[string]bool, res Result) {
	if !skipCheck && !isShellFile(path) {
		return
	}
	path = filepath.Clean(path)
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	cmdPositions, err := Do(f)
	if err != nil || len(cmdPositions) == 0 {
		return
	}

	if _, err := f.Seek(0, 0); err != nil {
		return
	}
	content, err := io.ReadAll(f)
	if err != nil {
		content, err = os.ReadFile(path)
		if err != nil {
			return
		}
	}
	lines := strings.Split(string(content), "\n")

	fileOccs := c.calculateFileOccurrences(cmdPositions, lines, ignores)
	if len(fileOccs) > 0 {
		res[path] = fileOccs
	}
}

// Scan recursively scans the target path (file or directory) and returns the aggregated results.
func (c *Config) Scan(target string) (Result, error) {
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}

	c.IsDirectory = info.IsDir()

	ignores := make(map[string]bool)
	for _, cmd := range c.ExtraIgnores {
		ignores[cmd] = true
	}

	res := make(Result)

	if !info.IsDir() {
		c.processFile(target, true, ignores, res)
		return res, nil
	}

	visited := make(map[string]bool)
	err = c.walkRecursive(target, ignores, res, visited)
	return res, err
}

func (c *Config) walkRecursive(path string, ignores map[string]bool, res Result, visited map[string]bool) error {
	path = filepath.Clean(path)
	if visited[path] {
		return nil
	}
	visited[path] = true

	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	for _, d := range entries {
		name := d.Name()
		if name == "." || name == ".." || (!c.ShowHidden && name[0] == '.') {
			continue
		}

		fullPath := filepath.Join(path, name)
		info, err := d.Info()
		if err != nil {
			continue
		}

		if info.Mode()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(fullPath)
			if err != nil {
				continue
			}
			info, err = os.Stat(resolved)
			if err != nil {
				continue
			}
			if info.IsDir() {
				if err := c.walkRecursive(fullPath, ignores, res, visited); err != nil {
					return err
				}
				continue
			}
		}

		if info.IsDir() {
			if err := c.walkRecursive(fullPath, ignores, res, visited); err != nil {
				return err
			}
		} else {
			c.processFile(fullPath, false, ignores, res)
		}
	}
	return nil
}

// GetBuiltins returns a sorted list of shell built-in commands.
func GetBuiltins() []string {
	return slices.Sorted(maps.Keys(builtins))
}

// GetCoreutils returns a sorted list of GNU Coreutils commands.
func GetCoreutils() []string {
	return slices.Sorted(maps.Keys(coreutils))
}

// GetCommon returns a sorted list of common shell commands.
func GetCommon() []string {
	return slices.Sorted(maps.Keys(common))
}