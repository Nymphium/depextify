package depextify

import (
	"bufio"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
	"mvdan.cc/sh/v3/syntax"
)

// Config holds the configuration for scanning and formatting.
type Config struct {
	NoBuiltins   bool     `yaml:"no_builtins"`
	NoCoreutils  bool     `yaml:"no_coreutils"`
	NoCommon     bool     `yaml:"no_common"`
	ShowHidden   bool     `yaml:"show_hidden"`
	ExtraIgnores []string `yaml:"ignores"`

	ShowCount   bool   `yaml:"show_count"`
	ShowPos     bool   `yaml:"show_pos"`
	UseColor    bool   `yaml:"use_color"`
	LexerName   string `yaml:"lexer"`
	StyleName   string `yaml:"style"`
	IsDirectory bool   `yaml:"-"`
	Format      string `yaml:"format"`

	Excludes []string `yaml:"excludes"`
}

type posInfo struct {
	line uint
	col  uint
	len  uint
}

var (
	reShellExt = regexp.MustCompile(`\.(ba|b|z|k|da)?sh$`) 
	reShebang  = regexp.MustCompile(`^#!\s*/.*(sh|bash|zsh|ksh)`)
)

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

func (c *Config) processFile(path string, skipCheck bool, ignores map[string]bool, res ScanResult) {
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
func (c *Config) Scan(target string) (ScanResult, error) {
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}

	c.IsDirectory = info.IsDir()

	ignores := make(map[string]bool)
	for _, cmd := range c.ExtraIgnores {
		ignores[cmd] = true
	}

	res := make(ScanResult)

	// Setup exclude matcher
	var matcher *ignore.GitIgnore
	
	// Load .depextifyignore if exists in the root of target or current directory
	ignoreFile := ".depextifyignore"
	if c.IsDirectory {
		ignoreFile = filepath.Join(target, ".depextifyignore")
	}
	
	// If explicit excludes are provided in config, start with them
	// We compile them as if they were lines in a gitignore file
	ignoreLines := c.Excludes

	if content, err := os.ReadFile(ignoreFile); err == nil {
		lines := strings.Split(string(content), "\n")
		ignoreLines = append(ignoreLines, lines...)
	} else if !c.IsDirectory {
		// Try looking in current directory if target is a file
		if content, err := os.ReadFile(".depextifyignore"); err == nil {
			lines := strings.Split(string(content), "\n")
			ignoreLines = append(ignoreLines, lines...)
		}
	}

	if len(ignoreLines) > 0 {
		matcher = ignore.CompileIgnoreLines(ignoreLines...)
	}

	if !info.IsDir() {
		// Check if the file itself is excluded
		if matcher != nil && matcher.MatchesPath(target) {
			return res, nil
		}
		c.processFile(target, true, ignores, res)
		return res, nil
	}

	visited := make(map[string]bool)
	err = c.walkRecursive(target, ignores, res, visited, matcher)
	return res, err
}

func (c *Config) walkRecursive(path string, ignores map[string]bool, res ScanResult, visited map[string]bool, matcher *ignore.GitIgnore) error {
	path = filepath.Clean(path)
	if visited[path] {
		return nil
	}
	visited[path] = true

	// Check exclusion (directory)
	if matcher != nil && matcher.MatchesPath(path) {
		return nil
	}

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
		
		// Check exclusion (file/subdir)
		if matcher != nil && matcher.MatchesPath(fullPath) {
			continue
		}

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
				if err := c.walkRecursive(fullPath, ignores, res, visited, matcher); err != nil {
					return err
				}
				continue
			}
		}

		if info.IsDir() {
			if err := c.walkRecursive(fullPath, ignores, res, visited, matcher); err != nil {
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