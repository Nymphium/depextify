package depextify

import (
	"bufio"
	"fmt"
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

var formatter = formatters.TTY256

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

type posInfo struct {
	line uint
	col  uint
	len  uint
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

// Occurrence represents a single occurrence of a command.
type Occurrence struct {
	Line     int
	Col      int
	Len      int
	FullLine string
}

// Result maps filename to its command occurrences: filename -> {cmd: []Occurrence}
type Result map[string]map[string][]Occurrence

const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBold   = "\033[1m"
	colorRed    = "\033[31m"
)

// Format returns a formatted string representation of the result.
func (r Result) Format(showCount, showPos, isDirectory, useColor bool, lexerName, styleName string) string {
	var sb strings.Builder

	globalLineWidth := 0
	if showPos {
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
		if isDirectory {
			p := path
			if useColor {
				p = colorCyan + path + colorReset
			}
			sb.WriteString(p + "\n")
		}

		indent := ""
		if isDirectory {
			indent = "  "
		}

		cmds := slices.Sorted(maps.Keys(r[path]))
		for _, cmd := range cmds {
			occs := r[path][cmd]

			c := cmd
			if useColor {
				c = colorBold + cmd + colorReset
			}

			suffix := ""
			colon := ":"
			if useColor {
				colon = colorYellow + ":" + colorReset
			}

			if showCount || showPos {
				suffix = colon
				if showCount {
					count := fmt.Sprintf(" %d", len(occs))
					if useColor {
						count = colorGreen + count + colorReset
					}
					suffix += count
				}
			}

			fmt.Fprintf(&sb, "%s%s%s\n", indent, c, suffix)

			if showPos {
				for _, occ := range occs {
					ln := fmt.Sprintf("%*d", globalLineWidth, occ.Line)
					if useColor {
						ln = colorGreen + ln + colorReset
					}
					cln := ":"
					if useColor {
						cln = colorYellow + ":" + colorReset
					}

					content := occ.FullLine
					if useColor {
						start := occ.Col - 1
						end := start + occ.Len
						content = emphasize(content, lexerName, styleName)
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

var reShellExt = regexp.MustCompile(`\.(b|z|k|da)?sh$`)
var reShebang = regexp.MustCompile(`^#!\s*/.*(sh|bash|zsh|ksh)`)

func isShellFile(path string) bool {
	if reShellExt.MatchString(path) {
		return true
	}

	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	line, _, err := reader.ReadLine()
	if err != nil {
		return false
	}

	return reShebang.MatchString(string(line))
}

// Scan recursively scans the target path (file or directory) and returns the aggregated results.
func Scan(target string, noBuiltins, noCoreutils, noCommon, showHidden bool, extraIgnores []string) (Result, error) {
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}

	ignores := make(map[string]bool)
	for _, cmd := range extraIgnores {
		ignores[cmd] = true
	}

	res := make(Result)

	processFile := func(path string, skipCheck bool) {
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

		// Re-read file to get full lines
		content, err := os.ReadFile(path)
		if err != nil {
			return
		}
		lines := strings.Split(string(content), "\n")

		fileOccs := make(map[string][]Occurrence)
		for cmd, ps := range cmdPositions {
			if noBuiltins && builtins[cmd] {
				continue
			}
			if noCoreutils && coreutils[cmd] {
				continue
			}
			if noCommon && common[cmd] {
				continue
			}
			if ignores[cmd] {
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
		if len(fileOccs) > 0 {
			res[path] = fileOccs
		}
	}

	if !info.IsDir() {
		processFile(target, true)
		return res, nil
	}

	// We use a custom walker to follow symlinks if they are directories
	var walk func(string) error
	visited := make(map[string]bool)

	walk = func(path string) error {
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
			if name == "." || name == ".." {
				continue
			}
			if !showHidden && name[0] == '.' {
				continue
			}

			fullPath := filepath.Join(path, name)
			info, err := d.Info()
			if err != nil {
				continue
			}

			// If it's a symlink, resolve it to see if it's a directory
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
					if err := walk(fullPath); err != nil {
						return err
					}
					continue
				}
			}

			if info.IsDir() {
				if err := walk(fullPath); err != nil {
					return err
				}
			} else {
				processFile(fullPath, false)
			}
		}
		return nil
	}

	err = walk(target)
	return res, err
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
