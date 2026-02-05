package depextify

import (
	"bufio"
	"bytes"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
	"mvdan.cc/sh/v3/syntax"
)

type (
	posInfo struct {
		line uint
		col  uint
		len  uint
	}

	// Extractor interface defines the contract for command extractors.
	Extractor interface {
		Extract(content []byte) (map[string][]posInfo, error)
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
	reTaskfile   = regexp.MustCompile(`(Taskfile|taskfile)\.(ya?ml|yml)`)
	reMakefile   = regexp.MustCompile("([Mm]akefile|MAKEFILE|GNUmakefile)")
	reDockerfile = regexp.MustCompile(`(Dockerfile|DOCKERFILE)(.*)?`)
)

// analyzeShellCode parses the given shell code and returns command occurrences.
// Positions are relative to the start of the code string.
func analyzeShellCode(code string) (map[string][]posInfo, error) {
	parser := syntax.NewParser()
	file, err := parser.Parse(strings.NewReader(code), "")
	if err != nil {
		return nil, err
	}

	localFuncs := collectLocalFuncs(file)
	return collectCommands(file, localFuncs), nil
}

func (e *ShellExtractor) Extract(content []byte) (map[string][]posInfo, error) {
	return analyzeShellCode(string(content))
}

func (e *MakefileExtractor) Extract(content []byte) (map[string][]posInfo, error) {
	results := make(map[string][]posInfo)
	scanner := bufio.NewScanner(bytes.NewReader(content))

	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if strings.HasPrefix(line, "\t") {
			runes := []rune(line)
			if len(runes) > 0 && runes[0] == '\t' {
				runes[0] = ' ' // Replace tab with space

				// Replace prefixes
				for i := 1; i < len(runes); i++ {
					if runes[i] == '@' || runes[i] == '-' || runes[i] == '+' {
						runes[i] = ' '
					} else if runes[i] == ' ' || runes[i] == '\t' {
						continue
					} else {
						break
					}
				}

				script := string(runes)
				cmds, err := analyzeShellCode(script)
				if err == nil {
					for cmd, infos := range cmds {
						for _, info := range infos {
							info.line += uint(lineNum - 1)
							results[cmd] = append(results[cmd], info)
						}
					}
				}
			}
		}
	}
	return results, nil
}

func (e *DockerfileExtractor) Extract(content []byte) (map[string][]posInfo, error) {
	results := make(map[string][]posInfo)
	scanner := bufio.NewScanner(bytes.NewReader(content))

	lineNum := 0
	var buffer strings.Builder
	startLine := 0
	inRun := false

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if inRun {
			buffer.WriteString("\n")
			buffer.WriteString(line)

			if !strings.HasSuffix(trimmed, "\\") {
				// End of RUN
				script := buffer.String()

				cmds, err := analyzeShellCode(script)
				if err == nil {
					for cmd, infos := range cmds {
						for _, info := range infos {
							info.line += uint(startLine - 1)
							results[cmd] = append(results[cmd], info)
						}
					}
				}

				inRun = false
				buffer.Reset()
			}
			continue
		}

		if strings.HasPrefix(trimmed, "RUN") && (len(trimmed) == 3 || (len(trimmed) > 3 && (trimmed[3] == ' ' || trimmed[3] == '\t'))) {
			idx := strings.Index(line, "RUN")
			if idx >= 0 {
				// Check if it's JSON form like RUN ["echo", ...]
				contentStart := idx + 3
				rest := strings.TrimSpace(line[contentStart:])
				if strings.HasPrefix(rest, "[") {
					continue // Skip exec form
				}

				inRun = true
				startLine = lineNum

				// Replace "RUN" with spaces to preserve column position
				scriptLine := line[:idx] + "   " + line[idx+3:]
				buffer.WriteString(scriptLine)

				if !strings.HasSuffix(trimmed, "\\") {
					// Single line RUN
					inRun = false
					cmds, err := analyzeShellCode(buffer.String())
					if err == nil {
						for cmd, infos := range cmds {
							for _, info := range infos {
								info.line += uint(startLine - 1)
								results[cmd] = append(results[cmd], info)
							}
						}
					}
					buffer.Reset()
				}
			}
		}
	}
	return results, nil
}

func (e *YAMLExtractor) Extract(content []byte) (map[string][]posInfo, error) {
	var node yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(content)).Decode(&node); err != nil {
		return nil, err
	}

	lines := bytes.Split(content, []byte("\n"))
	results := make(map[string][]posInfo)

	var walk func(*yaml.Node)
	walk = func(n *yaml.Node) {
		switch n.Kind {
		case yaml.MappingNode:
			for i := 0; i < len(n.Content); i += 2 {
				key := n.Content[i]
				val := n.Content[i+1]

				// GitHub Actions uses "run", Taskfile uses "cmd" or "cmds"
				if (key.Value == "run" || key.Value == "cmd") && val.Kind == yaml.ScalarNode {
					cPositions, err := analyzeShellCode(val.Value)
					if err == nil {
						applyYAMLOffset(cPositions, val, lines, results)
					}
				} else if key.Value == "cmds" && val.Kind == yaml.SequenceNode {
					for _, item := range val.Content {
						switch item.Kind {
						case yaml.ScalarNode:
							cPositions, err := analyzeShellCode(item.Value)
							if err == nil {
								applyYAMLOffset(cPositions, item, lines, results)
							}
						case yaml.MappingNode:
							// Taskfile can have cmds: [ { cmd: "..." } ]
							for j := 0; j < len(item.Content); j += 2 {
								if item.Content[j].Value == "cmd" && item.Content[j+1].Kind == yaml.ScalarNode {
									cPositions, err := analyzeShellCode(item.Content[j+1].Value)
									if err == nil {
										applyYAMLOffset(cPositions, item.Content[j+1], lines, results)
									}
								}
							}
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

func applyYAMLOffset(cmds map[string][]posInfo, val *yaml.Node, lines [][]byte, results map[string][]posInfo) {
	shellLines := strings.Split(val.Value, "\n")
	for cmd, infos := range cmds {
		for _, info := range infos {
			relLineIdx := int(info.line) - 1
			absLineIdx := val.Line - 1 + relLineIdx

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

	if reMakefile.MatchString(base) {
		return &MakefileExtractor{}
	}
	if reDockerfile.MatchString(base) {
		return &DockerfileExtractor{}
	}
	// GitHub Actions
	if strings.Contains(path, ".github/workflows") && (strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml")) {
		return &YAMLExtractor{}
	}
	// Taskfile
	if reTaskfile.MatchString(base) {
		return &YAMLExtractor{}
	}

	return nil
}

