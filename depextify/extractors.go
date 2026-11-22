package depextify

import (
	"bufio"
	"bytes"
	"path/filepath"
	"regexp"
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

var (
	reTaskfile   = regexp.MustCompile(`[Tt]askfile\.ya?ml`)
	reMakefile   = regexp.MustCompile(`[Mm]akefile|MAKEFILE`)
	reDockerfile = regexp.MustCompile(`Dockerfile|DOCKERFILE`)
)

// analyzeShellCode parses the given shell code and returns command occurrences.
func analyzeShellCode(content []byte) (map[string][]PosInfo, error) {
	parser := syntax.NewParser()
	file, err := parser.Parse(bytes.NewReader(content), "")
	if err != nil {
		return nil, err
	}

	localFuncs := collectLocalFuncs(file)
	return collectCommands(file, localFuncs, content), nil
}

// addScript merges the commands of script, which starts at line startLine, into results.
// A script that fails to parse contributes nothing.
func addScript(results map[string][]PosInfo, script string, startLine int) {
	cmds, err := analyzeShellCode([]byte(script))
	if err != nil {
		return
	}
	for cmd, infos := range cmds {
		for _, info := range infos {
			info.line += uint(startLine - 1)
			results[cmd] = append(results[cmd], info)
		}
	}
}

// addInLine records key at the first occurrence of text in line, which is line number lineNum.
func addInLine(results map[string][]PosInfo, key, text, line string, lineNum int) {
	if col := strings.Index(line, text); col >= 0 {
		results[key] = append(results[key], PosInfo{line: uint(lineNum), col: uint(col + 1), len: uint(len(text))})
	}
}

// addAssign records an assignment name=val found in line: the variable as "$name", and val when it looks like a command.
func addAssign(results map[string][]PosInfo, line string, lineNum int, name, val string) {
	if name != "" {
		addInLine(results, "$"+name, name, line, lineNum)
	}
	if looksLikeCommand(val) {
		addInLine(results, val, val, line, lineNum)
	}
}

// Extract implements Extractor for shell scripts.
func (e *ShellExtractor) Extract(content []byte) (map[string][]PosInfo, error) {
	return analyzeShellCode(content)
}

// Extract implements Extractor for Makefiles.
func (e *MakefileExtractor) Extract(content []byte) (map[string][]PosInfo, error) {
	results := make(map[string][]PosInfo)
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

		buffer.WriteString(string(runes))

		if strings.HasSuffix(strings.TrimRightFunc(line, unicode.IsSpace), "\\") {
			buffer.WriteString("\n")
		} else {
			flush()
		}
	}
	flush()

	return results, nil
}

// Extract extracts commands from Dockerfile content.
func (e *DockerfileExtractor) Extract(content []byte) (map[string][]PosInfo, error) {
	results := make(map[string][]PosInfo)
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
	return results, nil
}

// Extract extracts commands from YAML content.
func (e *YAMLExtractor) Extract(content []byte) (map[string][]PosInfo, error) {
	var node yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(content)).Decode(&node); err != nil {
		return nil, err
	}

	lines := bytes.Split(content, []byte("\n"))
	results := make(map[string][]PosInfo)

	addScalarScript := func(n *yaml.Node) {
		if n.Kind != yaml.ScalarNode {
			return
		}
		if cmds, err := analyzeShellCode([]byte(n.Value)); err == nil {
			applyYAMLOffset(cmds, n, lines, results)
		}
	}
	// addScalar records the whole scalar n under key.
	addScalar := func(n *yaml.Node, key string) {
		applyYAMLOffset(map[string][]PosInfo{key: {{line: 1, col: 1, len: uint(len(n.Value))}}}, n, lines, results)
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
						if k.Value != "" {
							addScalar(k, "$"+k.Value)
						}
						if v.Kind == yaml.ScalarNode && looksLikeCommand(v.Value) {
							addScalar(v, v.Value)
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
	return results, nil
}

func applyYAMLOffset(cmds map[string][]PosInfo, val *yaml.Node, lines [][]byte, results map[string][]PosInfo) {
	shellLines := strings.Split(val.Value, "\n")
	baseLine := val.Line
	if val.Style == yaml.LiteralStyle || val.Style == yaml.FoldedStyle {
		baseLine++
	}

	for cmd, infos := range cmds {
		for _, info := range infos {
			relLineIdx := int(info.line) - 1
			absLineIdx := baseLine - 1 + relLineIdx

			if absLineIdx < len(lines) && relLineIdx < len(shellLines) {
				origLine := string(lines[absLineIdx])
				shellLine := shellLines[relLineIdx]

				colOffset := strings.Index(origLine, shellLine)
				if colOffset == -1 {
					colOffset = 0
				}

				info.line = uint(absLineIdx + 1)
				info.col += uint(colOffset)
				results[cmd] = append(results[cmd], info)
			}
		}
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
