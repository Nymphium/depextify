// Package depextify provides the core logic for parsing and extracting commands from shell scripts and configuration files.
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

		// Query, when not empty, restricts the result to these commands, which are reported even when an ignore rule covers them.
		Query []string `yaml:"-"`
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

	// reVarRun matches a command word that is a single variable expansion, such as $CC or "${CC}".
	reVarRun = regexp.MustCompile(`^\$([A-Za-z_][A-Za-z0-9_]*|[0-9]+)$`)

	// wrappers take another command as an argument.
	wrappers = set("sudo", "xargs", "time", "exec", "nice", "nohup", "command", "builtin", "which", "type")
)

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

func addPos(m map[string][]PosInfo, key string, pos syntax.Pos, n int) {
	m[key] = append(m[key], PosInfo{line: pos.Line(), col: pos.Col(), len: uint(n)})
}

// looksLikeCommand reports whether an assigned value may name a command, such as CC=gcc.
func looksLikeCommand(v string) bool {
	return v != "" && !strings.ContainsAny(v, " $") && !strings.HasPrefix(v, "-") && !strings.HasPrefix(v, "/") && !strings.HasPrefix(v, "./")
}

// collectCommands() collects command names from various nodes with filtering:
// - not local functions
// - not starting with '-'
func collectCommands(file *syntax.File, localFuncs map[string]bool, source []byte) found {
	f := newFound()

	// addCall records the first non-flag word of args, and recurses into the arguments of wrappers such as sudo.
	var addCall func(parentCmd string, args []*syntax.Word)
	addCall = func(parentCmd string, args []*syntax.Word) {
		skipNext := false
		for i, arg := range args {
			if skipNext {
				skipNext = false
				continue
			}

			cmd := wordToString(arg)
			if strings.HasPrefix(cmd, "-") {
				skipNext = isFlagWithArg(parentCmd, cmd)
				continue
			}

			if cmd != "" && !localFuncs[cmd] {
				// Use original source text for accurate length and representation
				rawCmd := getNodeText(arg, source)
				if m := reVarRun.FindStringSubmatch(cmd); m != nil {
					addPos(f.runs, m[1], arg.Pos(), len(rawCmd))
				} else {
					addPos(f.cmds, rawCmd, arg.Pos(), len(rawCmd))
				}
				if wrappers[cmd] {
					addCall(cmd, args[i+1:])
				}
			}
			return
		}
	}

	syntax.Walk(file, func(node syntax.Node) bool {
		switch x := node.(type) {
		case *syntax.TimeClause:
			addPos(f.cmds, "time", x.Time, len("time"))
		case *syntax.CallExpr:
			addCall("", x.Args)
		case *syntax.Assign:
			if x.Name != nil && x.Value != nil {
				if val := wordToString(x.Value); looksLikeCommand(val) && !localFuncs[val] {
					addPos(f.assigns, x.Name.Value+"="+val, x.Value.Pos(), len(getNodeText(x.Value, source)))
				}
			}
		}
		return true
	})

	return f
}

func getNodeText(node syntax.Node, source []byte) string {
	start := node.Pos().Offset()
	end := node.End().Offset()
	if start >= uint(len(source)) || end > uint(len(source)) || start >= end {
		return ""
	}
	return string(source[start:end])
}

func isFlagWithArg(cmd, flag string) bool {
	if !strings.HasPrefix(flag, "-") || strings.Contains(flag, "=") {
		return false
	}
	f := strings.TrimLeft(flag, "-")
	if len(f) > 1 {
		// Long flags or combined short flags.
		// For simplicity, we only handle common single-letter flags that take args.
		return false
	}

	switch cmd {
	case "sudo":
		return strings.ContainsAny(f, "ugprUTRD C")
	case "xargs":
		return strings.ContainsAny(f, "EelInLPps")
	}
	return false
}

func wordToString(w *syntax.Word) string {
	var sb strings.Builder
	writeParts(&sb, w.Parts)
	return sb.String()
}

func writeParts(sb *strings.Builder, parts []syntax.WordPart) {
	for _, part := range parts {
		switch p := part.(type) {
		case *syntax.Lit:
			sb.WriteString(p.Value)
		case *syntax.SglQuoted:
			sb.WriteString(p.Value)
		case *syntax.ParamExp:
			sb.WriteString("$" + p.Param.Value)
		case *syntax.DblQuoted:
			writeParts(sb, p.Parts)
		}
	}
}

// Do analyzes the given shell script reader and returns a map of command names to their positions.
func Do(r io.Reader) (map[string][]PosInfo, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return (&ShellExtractor{}).Extract(content)
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

// Ignored reports whether the ignore rules of c (the categories and ExtraIgnores) exclude cmd.
func (c *Config) Ignored(cmd string) bool {
	// ponytail: linear scan of ExtraIgnores; build a set once if ignore lists grow to thousands.
	return (c.NoBuiltins && builtins[cmd]) || (c.NoCoreutils && coreutils[cmd]) || (c.NoCommon && common[cmd]) || slices.Contains(c.ExtraIgnores, cmd)
}

func (c *Config) reports(cmd string) bool {
	if len(c.Query) > 0 {
		return slices.Contains(c.Query, cmd)
	}
	return !c.Ignored(cmd)
}

// Commands returns the sorted names of the commands in r.
func (r ScanResult) Commands() []string {
	cmds := make(map[string]bool)
	for _, fileOccs := range r {
		for cmd := range fileOccs {
			cmds[cmd] = true
		}
	}
	return slices.Sorted(maps.Keys(cmds))
}

func (c *Config) calculateFileOccurrences(cmdPositions map[string][]PosInfo, lines []string) map[string][]Occurrence {
	fileOccs := make(map[string][]Occurrence)
	for cmd, ps := range cmdPositions {
		if !c.reports(cmd) {
			continue
		}
		for _, p := range ps {
			if p.line > 0 && p.line <= uint(len(lines)) {
				fileOccs[cmd] = append(fileOccs[cmd], Occurrence{
					Line:     int(p.line),
					Col:      int(p.col),
					Len:      int(p.len),
					FullLine: lines[p.line-1],
				})
			}
		}
	}
	return fileOccs
}

func (c *Config) processFile(path string, skipCheck bool, res ScanResult) {
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

	fileOccs := c.calculateFileOccurrences(cmdPositions, lines)
	if len(fileOccs) > 0 {
		res[path] = fileOccs
	}
}

func (c *Config) ignoreFileNames() []string {
	if c.GitignoreAware {
		return []string{".depextifyignore", ".gitignore"}
	}
	return []string{".depextifyignore"}
}

// Scan recursively scans the target path (file or directory) and returns the aggregated results.
func (c *Config) Scan(target string) (ScanResult, error) {
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}

	res := make(ScanResult)

	// Explicit excludes and the ignore files in the target directory
	// (or in the current directory when the target is a file) are compiled as one gitignore.
	ignoreLines := c.Excludes
	for _, name := range c.ignoreFileNames() {
		path := name
		if info.IsDir() {
			path = filepath.Join(target, name)
		}
		if content, err := os.ReadFile(path); err == nil {
			ignoreLines = append(ignoreLines, strings.Split(string(content), "\n")...)
		}
	}
	matcher := ignore.CompileIgnoreLines(ignoreLines...)

	if !info.IsDir() {
		if !matcher.MatchesPath(target) {
			c.processFile(target, true, res)
		}
		return res, nil
	}

	visited := make(map[string]bool)
	err = c.walkRecursive(target, target, res, visited, matcher, ignoreLines)
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

// scopedIgnoreLines reads the ignore files in dir and rewrites their patterns relative to root.
func (c *Config) scopedIgnoreLines(root, dir string) []string {
	relDir, _ := filepath.Rel(root, dir)
	var lines []string
	for _, name := range c.ignoreFileNames() {
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(content), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if strings.HasPrefix(line, "/") {
				line = filepath.Join(relDir, line)
			} else {
				// Patterns without leading slash match relative to this directory
				line = filepath.Join(relDir, "**", line)
			}
			lines = append(lines, line)
		}
	}
	return lines
}

func (c *Config) walkRecursive(root, path string, res ScanResult, visited map[string]bool, matcher *ignore.GitIgnore, ignoreLines []string) error {
	path = filepath.Clean(path)
	if visited[path] {
		return nil
	}
	visited[path] = true

	if matcher.MatchesPath(path) {
		return nil
	}

	// The root's ignore files are already loaded by Scan.
	if root != path {
		if scoped := c.scopedIgnoreLines(root, path); len(scoped) > 0 {
			ignoreLines = append(ignoreLines, scoped...)
			matcher = ignore.CompileIgnoreLines(ignoreLines...)
		}
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	for _, d := range entries {
		name := d.Name()
		if !c.ShowHidden && name[0] == '.' {
			continue
		}

		fullPath := filepath.Join(path, name)
		if matcher.MatchesPath(fullPath) {
			continue
		}

		info, err := d.Info()
		if err != nil {
			continue
		}
		// Linked directories are walked under their link path.
		if info.Mode()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(fullPath)
			if err != nil {
				continue
			}
			if info, err = os.Stat(resolved); err != nil {
				continue
			}
		}

		if info.IsDir() {
			if err := c.walkRecursive(root, fullPath, res, visited, matcher, ignoreLines); err != nil {
				return err
			}
		} else {
			c.processFile(fullPath, false, res)
		}
	}
	return nil
}
