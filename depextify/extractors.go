package depextify

import (
	"bufio"
	"bytes"
	"cmp"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
	"mvdan.cc/sh/v3/syntax"
)

type (
	// PosInfo represents the position of a command in a file.
	PosInfo struct {
		line uint
		col  uint
		len  uint
	}

	// Extractor interface defines the contract for command extractors.
	Extractor interface {
		Extract(content []byte) (map[string][]PosInfo, error)
	}
	// ShellExtractor extracts commands from shell scripts.
	ShellExtractor struct{}

	// MakefileExtractor extracts commands from Makefiles.
	MakefileExtractor struct{}

	// DockerfileExtractor extracts commands from Dockerfiles.
	DockerfileExtractor struct{}

	// YAMLExtractor extracts commands from YAML files.
	YAMLExtractor struct{}
)

// found is what an extractor collects from a file before variables are resolved.
type found struct {
	cmds    map[string][]PosInfo // commands named in the source, keyed by their names without quotes and escapes
	runs    map[string][]PosInfo // variables run as commands ($CC main.c), keyed by variable name
	assigns map[string][]PosInfo // command-like values assigned to variables, keyed by "NAME=value", at the value
}

func newFound() found {
	return found{cmds: map[string][]PosInfo{}, runs: map[string][]PosInfo{}, assigns: map[string][]PosInfo{}}
}

// merge adds src to f, moving each position with shift and dropping the ones it rejects.
func (f found) merge(src found, shift func(PosInfo) (PosInfo, bool)) {
	for _, m := range [][2]map[string][]PosInfo{{f.cmds, src.cmds}, {f.runs, src.runs}, {f.assigns, src.assigns}} {
		for key, infos := range m[1] {
			for _, info := range infos {
				if info, ok := shift(info); ok {
					m[0][key] = append(m[0][key], info)
				}
			}
		}
	}
}

// resolve returns the commands of f, with each variable run as a command replaced by the values assigned to it
// in the same file. A variable with no assigned value naming a command, and assignments to variables that are
// never run, are dropped.
func (f found) resolve() map[string][]PosInfo {
	res := make(map[string][]PosInfo, len(f.cmds))
	for cmd, infos := range f.cmds {
		res[cmd] = slices.Clone(infos)
	}
	// ponytail: scans every assignment per run variable; index assignments by name if files get huge.
	for name := range f.runs {
		for key, infos := range f.assigns {
			if n, val, _ := strings.Cut(key, "="); n == name {
				res[val] = append(res[val], infos...)
			}
		}
	}
	for _, infos := range res {
		slices.SortFunc(infos, func(a, b PosInfo) int {
			return cmp.Or(cmp.Compare(a.line, b.line), cmp.Compare(a.col, b.col))
		})
	}
	return res
}

var (
	// reMakeVar matches a Make variable reference such as $(CC), which a recipe expands before the shell runs it.
	reMakeVar = regexp.MustCompile(`\$\(([A-Za-z_][A-Za-z0-9_]*)\)`)

	reTaskfile   = regexp.MustCompile(`[Tt]askfile\.ya?ml`)
	reMakefile   = regexp.MustCompile(`[Mm]akefile|MAKEFILE`)
	reDockerfile = regexp.MustCompile(`Dockerfile|DOCKERFILE`)
)

// analyzeShellCode parses the given shell code and returns what it finds, with variables unresolved.
func analyzeShellCode(content []byte) (found, error) {
	parser := syntax.NewParser()
	file, err := parser.Parse(bytes.NewReader(content), "")
	if err != nil {
		return found{}, err
	}

	localFuncs := collectLocalFuncs(file)
	return collectCommands(file, localFuncs, content), nil
}

// addScript merges what script, which starts at line startLine, contains into results.
// A script that fails to parse contributes nothing.
func addScript(results found, script string, startLine int) {
	src, err := analyzeShellCode([]byte(script))
	if err != nil {
		return
	}
	results.merge(src, func(info PosInfo) (PosInfo, bool) {
		info.line += uint(startLine - 1)
		return info, true
	})
}

// addInLine records key at the first occurrence of text in line, which is line number lineNum.
func addInLine(results map[string][]PosInfo, key, text, line string, lineNum int) {
	if col := strings.Index(line, text); col >= 0 {
		results[key] = append(results[key], PosInfo{line: uint(lineNum), col: uint(col + 1), len: uint(len(text))})
	}
}

// addAssign records an assignment name=val found in line when val, without enclosing quotes, looks like a command.
func addAssign(results found, line string, lineNum int, name, val string) {
	unquoted := val
	if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[len(val)-1] == val[0] {
		unquoted = val[1 : len(val)-1]
	}
	if name != "" && looksLikeCommand(unquoted) {
		addInLine(results.assigns, name+"="+unquoted, val, line, lineNum)
	}
}

// Extract implements Extractor for shell scripts.
func (e *ShellExtractor) Extract(content []byte) (map[string][]PosInfo, error) {
	f, err := analyzeShellCode(content)
	if err != nil {
		return nil, err
	}
	return f.resolve(), nil
}

// Extract implements Extractor for Makefiles.
func (e *MakefileExtractor) Extract(content []byte) (map[string][]PosInfo, error) {
	results := newFound()
	scanner := bufio.NewScanner(bytes.NewReader(content))

	var buffer strings.Builder
	startLine := 0
	inRecipe := false
	flush := func() {
		if inRecipe {
			addScript(results, buffer.String(), startLine)
			inRecipe = false
			buffer.Reset()
		}
	}

	for lineNum := 1; scanner.Scan(); lineNum++ {
		line := scanner.Text()

		if !strings.HasPrefix(line, "\t") {
			flush()

			// Check for Makefile assignment (e.g., CC = gcc)
			trimmed := strings.TrimSpace(line)
			if idx := strings.Index(trimmed, "="); idx > 0 {
				name := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(trimmed[:idx]), ":?+"))
				addAssign(results, line, lineNum, name, strings.TrimSpace(trimmed[idx+1:]))
			}
			continue
		}

		if !inRecipe {
			inRecipe = true
			startLine = lineNum
		}

		runes := []rune(line)
		// Replace tab with space to maintain alignment
		runes[0] = ' '

		// If it's the start of a recipe line, replace prefixes
		if !strings.Contains(buffer.String(), "\n") {
		loop:
			for i := 1; i < len(runes); i++ {
				switch runes[i] {
				case '@', '-', '+':
					runes[i] = ' '
				case ' ', '\t':
					continue
				default:
					break loop
				}
			}
		}

		// $(CC) becomes ${CC}, which has the same length, so the shell parser sees a variable instead of a command substitution.
		buffer.WriteString(reMakeVar.ReplaceAllString(string(runes), "$${${1}}"))

		if strings.HasSuffix(strings.TrimRightFunc(line, unicode.IsSpace), "\\") {
			buffer.WriteString("\n")
		} else {
			flush()
		}
	}
	flush()

	return results.resolve(), nil
}

// Extract extracts commands from Dockerfile content.
func (e *DockerfileExtractor) Extract(content []byte) (map[string][]PosInfo, error) {
	results := newFound()
	scanner := bufio.NewScanner(bytes.NewReader(content))

	var buffer strings.Builder
	startLine := 0
	inRun := false

	for lineNum := 1; scanner.Scan(); lineNum++ {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		switch {
		case inRun:
			buffer.WriteString("\n" + line)
		case strings.HasPrefix(trimmed, "RUN") && (len(trimmed) == 3 || trimmed[3] == ' ' || trimmed[3] == '\t'):
			idx := strings.Index(line, "RUN")
			if strings.HasPrefix(strings.TrimSpace(line[idx+3:]), "[") {
				continue // Skip exec form like RUN ["echo", ...]
			}
			inRun = true
			startLine = lineNum
			// Replace "RUN" with spaces to preserve column position
			buffer.WriteString(line[:idx] + "   " + line[idx+3:])
		case strings.HasPrefix(trimmed, "ENV") || strings.HasPrefix(trimmed, "ARG"):
			fields := strings.Fields(line)
			var name, val string
			switch {
			case len(fields) < 2:
			case strings.Contains(fields[1], "="):
				name, val, _ = strings.Cut(fields[1], "=")
			case len(fields) >= 3:
				name, val = fields[1], fields[2]
			}
			addAssign(results, line, lineNum, name, val)
			continue
		default:
			continue
		}

		// The RUN instruction ends at the first line without a trailing backslash.
		if !strings.HasSuffix(trimmed, "\\") {
			addScript(results, buffer.String(), startLine)
			inRun = false
			buffer.Reset()
		}
	}
	return results.resolve(), nil
}

// Extract extracts commands from YAML content.
func (e *YAMLExtractor) Extract(content []byte) (map[string][]PosInfo, error) {
	var node yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(content)).Decode(&node); err != nil {
		return nil, err
	}

	lines := bytes.Split(content, []byte("\n"))
	results := newFound()

	addScalarScript := func(n *yaml.Node) {
		if n.Kind != yaml.ScalarNode {
			return
		}
		if src, err := analyzeShellCode([]byte(n.Value)); err == nil {
			results.merge(src, yamlShift(n, lines))
		}
	}

	var walk func(*yaml.Node)
	walk = func(n *yaml.Node) {
		switch n.Kind {
		case yaml.MappingNode:
			for i := 0; i < len(n.Content); i += 2 {
				key := n.Content[i]
				val := n.Content[i+1]

				switch {
				case key.Value == "run" || key.Value == "cmd":
					addScalarScript(val)
				case key.Value == "cmds" && val.Kind == yaml.SequenceNode:
					// Mapping items such as Taskfile's { cmd: "..." } are handled when walk reaches them.
					for _, item := range val.Content {
						addScalarScript(item)
					}
				case key.Value == "env" && val.Kind == yaml.MappingNode:
					for j := 0; j < len(val.Content); j += 2 {
						k := val.Content[j]
						v := val.Content[j+1]
						if k.Value != "" && v.Kind == yaml.ScalarNode && looksLikeCommand(v.Value) {
							assign := map[string][]PosInfo{k.Value + "=" + v.Value: {{line: 1, col: 1, len: uint(len(v.Value))}}}
							results.merge(found{assigns: assign}, yamlShift(v, lines))
						}
					}
				}
				walk(val)
			}
		case yaml.SequenceNode, yaml.DocumentNode:
			for _, child := range n.Content {
				walk(child)
			}
		}
	}

	walk(&node)
	return results.resolve(), nil
}

// yamlShift moves a position within the value of the scalar val to its position in the YAML file, whose lines are lines.
func yamlShift(val *yaml.Node, lines [][]byte) func(PosInfo) (PosInfo, bool) {
	shellLines := strings.Split(val.Value, "\n")
	baseLine := val.Line
	if val.Style == yaml.LiteralStyle || val.Style == yaml.FoldedStyle {
		baseLine++
	}

	return func(info PosInfo) (PosInfo, bool) {
		relLineIdx := int(info.line) - 1
		absLineIdx := baseLine - 1 + relLineIdx
		if absLineIdx >= len(lines) || relLineIdx >= len(shellLines) {
			return info, false
		}
		colOffset := max(strings.Index(string(lines[absLineIdx]), shellLines[relLineIdx]), 0)
		info.line = uint(absLineIdx + 1)
		info.col += uint(colOffset)
		return info, true
	}
}

// GetExtractor returns the appropriate Extractor for the given file path.
// It returns nil if no specific extractor matches (caller should decide fallback, e.g. check isShellFile).
func GetExtractor(path string) Extractor {
	base := filepath.Base(path)
	switch {
	case reMakefile.MatchString(base):
		return &MakefileExtractor{}
	case reDockerfile.MatchString(base):
		return &DockerfileExtractor{}
	case strings.Contains(path, ".github/workflows") && (strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml")),
		reTaskfile.MatchString(base):
		return &YAMLExtractor{}
	}
	return nil
}
