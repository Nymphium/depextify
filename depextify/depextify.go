// Package depextify provides the core logic for parsing and extracting commands from shell scripts and configuration files.
package depextify

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
	"mvdan.cc/sh/v3/syntax"
)

type (
	// Config holds the configuration for scanning.
	Config struct {
		NoBuiltins   bool     `yaml:"no_builtins"`
		NoCoreutils  bool     `yaml:"no_coreutils"`
		NoCommon     bool     `yaml:"no_common"`
		ShowHidden   bool     `yaml:"show_hidden"`
		ExtraIgnores []string `yaml:"ignores"`

		GitignoreAware bool `yaml:"gitignore_aware"`
		Aggregate      bool `yaml:"aggregate"`

		Excludes []string `yaml:"excludes"`
	}

	// Occurrence represents a single occurrence of a command.
	Occurrence struct {
		Line     int
		Col      int
		Len      int
		FullLine string
	}

	// ScanResult maps filename to its command occurrences: filename -> {cmd: []Occurrence}
	ScanResult map[string]map[string][]Occurrence
)

var (
	reShellExt = regexp.MustCompile(`\.(ba|b|z|k|da)?sh$`)
	reShebang  = regexp.MustCompile(`^#!\s*/.*(sh|bash|zsh|ksh)`)
)

func toInt(u uint) int {
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
func collectCommands(file *syntax.File, localFuncs map[string]bool) map[string][]PosInfo {
	commands := make(map[string][]PosInfo)

	syntax.Walk(file, func(node syntax.Node) bool {
		if x, ok := node.(*syntax.CallExpr); ok {
			if len(x.Args) > 0 && len(x.Args[0].Parts) == 1 {
				if part, ok := x.Args[0].Parts[0].(*syntax.Lit); ok {
					cmd := part.Value

					if !localFuncs[cmd] && !strings.HasPrefix(cmd, "-") {
						commands[cmd] = append(commands[cmd], PosInfo{
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

// Do analyzes the given shell script reader and returns a map of command names to their positions.
func Do(r io.Reader) (map[string][]PosInfo, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return analyzeShellCode(string(content))
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

func (c *Config) calculateFileOccurrences(cmdPositions map[string][]PosInfo, lines []string, ignores map[string]bool) map[string][]Occurrence {
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
	path = filepath.Clean(path)

	ext := GetExtractor(path)
	if ext == nil {
		if !skipCheck && !isShellFile(path) {
			return
		}
		ext = &ShellExtractor{}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return
	}

	cmdPositions, err := ext.Extract(content)
	if err != nil || len(cmdPositions) == 0 {
		return
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

	isDir := info.IsDir()

	ignores := make(map[string]bool)
	for _, cmd := range c.ExtraIgnores {
		ignores[cmd] = true
	}

	res := make(ScanResult)

	// Setup exclude matcher
	var matcher *ignore.GitIgnore

	// Load .depextifyignore if exists in the root of target or current directory
	ignoreFiles := []string{".depextifyignore"}
	if c.GitignoreAware {
		ignoreFiles = append(ignoreFiles, ".gitignore")
	}

	// If explicit excludes are provided in config, start with them
	// We compile them as if they were lines in a gitignore file
	ignoreLines := c.Excludes

	for _, name := range ignoreFiles {
		path := name
		if isDir {
			path = filepath.Join(target, name)
		}

		if content, err := os.ReadFile(path); err == nil {
			lines := strings.Split(string(content), "\n")
			ignoreLines = append(ignoreLines, lines...)
		} else if !isDir {
			// Try looking in current directory if target is a file
			if content, err := os.ReadFile(name); err == nil {
				lines := strings.Split(string(content), "\n")
				ignoreLines = append(ignoreLines, lines...)
			}
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
	err = c.walkRecursive(target, target, ignores, res, visited, matcher, ignoreLines)
	if err != nil {
		return nil, err
	}

	if c.Aggregate {
		aggregated := make(map[string][]Occurrence)
		for _, fileOccs := range res {
			for cmd, occs := range fileOccs {
				aggregated[cmd] = append(aggregated[cmd], occs...)
			}
		}
		res = ScanResult{target: aggregated}
	}

	return res, nil
}

func (c *Config) walkRecursive(root, path string, ignores map[string]bool, res ScanResult, visited map[string]bool, matcher *ignore.GitIgnore, ignoreLines []string) error {
	path = filepath.Clean(path)
	if visited[path] {
		return nil
	}
	visited[path] = true

	// Check exclusion (directory)
	if matcher != nil && matcher.MatchesPath(path) {
		return nil
	}

	currentMatcher := matcher
	currentIgnoreLines := ignoreLines

	// Only check for new ignore files if we are NOT at the root (they are already loaded in Scan)
	if root != path {
		ignoreFiles := []string{".depextifyignore"}
		if c.GitignoreAware {
			ignoreFiles = append(ignoreFiles, ".gitignore")
		}

		foundNewIgnore := false
		for _, name := range ignoreFiles {
			ignorePath := filepath.Join(path, name)
			if content, err := os.ReadFile(ignorePath); err == nil {
				foundNewIgnore = true
				relDir, _ := filepath.Rel(root, path)

				lines := strings.Split(string(content), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" || strings.HasPrefix(line, "#") {
						continue
					}
					// Prefix pattern with relative path to scope it
					if strings.HasPrefix(line, "/") {
						line = filepath.Join(relDir, line)
					} else {
						// Patterns without leading slash match relative to this directory
						line = filepath.Join(relDir, "**", line)
					}
					currentIgnoreLines = append(currentIgnoreLines, line)
				}
			}
		}

		if foundNewIgnore {
			currentMatcher = ignore.CompileIgnoreLines(currentIgnoreLines...)
		}
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
		if currentMatcher != nil && currentMatcher.MatchesPath(fullPath) {
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
				if err := c.walkRecursive(root, fullPath, ignores, res, visited, currentMatcher, currentIgnoreLines); err != nil {
					return err
				}
				continue
			}
		}

		if info.IsDir() {
			if err := c.walkRecursive(root, fullPath, ignores, res, visited, currentMatcher, currentIgnoreLines); err != nil {
				return err
			}
		} else {
			c.processFile(fullPath, false, ignores, res)
		}
	}
	return nil
}
