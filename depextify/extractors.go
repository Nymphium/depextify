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
	reTaskfile   = regexp.MustCompile(`(Taskfile|taskfile)\.(ya?ml|yml)`)
	reMakefile   = regexp.MustCompile("([Mm]akefile|MAKEFILE|GNUmakefile)")
	reDockerfile = regexp.MustCompile(`(Dockerfile|DOCKERFILE)(.*)?`)
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

func mergeResults(dest, src map[string][]PosInfo, lineOffset uint) {
	for cmd, infos := range src {
		for _, info := range infos {
			info.line += lineOffset
			dest[cmd] = append(dest[cmd], info)
		}
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

	lineNum := 0
	var buffer strings.Builder
	startLine := 0
	inRecipe := false

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if strings.HasPrefix(line, "\t") {
			if !inRecipe {
				inRecipe = true
				startLine = lineNum
				buffer.Reset()
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

			scriptLine := string(runes)
			buffer.WriteString(scriptLine)

			if !strings.HasSuffix(strings.TrimRightFunc(line, unicode.IsSpace), "\\") {
				// End of multi-line or single line command
				script := buffer.String()
				cmds, err := analyzeShellCode([]byte(script))
				if err == nil {
					mergeResults(results, cmds, uint(startLine-1))
				}
				inRecipe = false
				buffer.Reset()
			} else {
				// Continue to next line
				buffer.WriteString("\n")
			}
		} else {
			// Not a recipe line
			if inRecipe {
				script := buffer.String()
				cmds, err := analyzeShellCode([]byte(script))
				if err == nil {
					mergeResults(results, cmds, uint(startLine-1))
				}
				inRecipe = false
				buffer.Reset()
			}

			// Check for Makefile assignment (e.g., CC = gcc)
			trimmed := strings.TrimSpace(line)
			if idx := strings.Index(trimmed, "="); idx > 0 {
				namePart := strings.TrimSpace(trimmed[:idx])
				name := strings.TrimRight(namePart, ":?+")
				name = strings.TrimSpace(name)

				if name != "" {
					col := strings.Index(line, name)
					if col >= 0 {
						// Track the variable being defined
						results["$"+name] = append(results["$"+name], PosInfo{
							line: uint(lineNum),
							col:  uint(col + 1),
							len:  uint(len(name)),
						})
					}
				}

				valPart := strings.TrimSpace(trimmed[idx+1:])
				// Track variable assignment values if they look like potential commands.
				if valPart != "" && !strings.Contains(valPart, " ") && !strings.HasPrefix(valPart, "-") && !strings.HasPrefix(valPart, "/") && !strings.HasPrefix(valPart, "./") {
					col := strings.Index(line, valPart)
					if col >= 0 {
						results[valPart] = append(results[valPart], PosInfo{
							line: uint(lineNum),
							col:  uint(col + 1),
							len:  uint(len(valPart)),
						})
					}
				}
			}
		}
	}

	if inRecipe {
		script := buffer.String()
		cmds, err := analyzeShellCode([]byte(script))
		if err == nil {
			mergeResults(results, cmds, uint(startLine-1))
		}
	}

	return results, nil
}

// Extract extracts commands from Dockerfile content.
func (e *DockerfileExtractor) Extract(content []byte) (map[string][]PosInfo, error) {
	results := make(map[string][]PosInfo)
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

				cmds, err := analyzeShellCode([]byte(script))
				if err == nil {
					mergeResults(results, cmds, uint(startLine-1))
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
					cmds, err := analyzeShellCode([]byte(buffer.String()))
					if err == nil {
						mergeResults(results, cmds, uint(startLine-1))
					}
					buffer.Reset()
				}
			}
		} else if strings.HasPrefix(trimmed, "ENV") || strings.HasPrefix(trimmed, "ARG") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				var name string
				var val string
				if strings.Contains(fields[1], "=") {
					parts := strings.SplitN(fields[1], "=", 2)
					name = parts[0]
					val = parts[1]
				} else if len(fields) >= 3 {
					name = fields[1]
					val = fields[2]
				}

				if name != "" {
					col := strings.Index(line, name)
					if col >= 0 {
						results["$"+name] = append(results["$"+name], PosInfo{
							line: uint(lineNum),
							col:  uint(col + 1),
							len:  uint(len(name)),
						})
					}
				}

				if val != "" && !strings.Contains(val, " ") && !strings.HasPrefix(val, "-") && !strings.HasPrefix(val, "/") && !strings.HasPrefix(val, "./") {
					col := strings.Index(line, val)
					if col >= 0 {
						results[val] = append(results[val], PosInfo{
							line: uint(lineNum),
							col:  uint(col + 1),
							len:  uint(len(val)),
						})
					}
				}
			}
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

	var walk func(*yaml.Node)
	walk = func(n *yaml.Node) {
		switch n.Kind {
		case yaml.MappingNode:
			for i := 0; i < len(n.Content); i += 2 {
				key := n.Content[i]
				val := n.Content[i+1]

				switch {
				case (key.Value == "run" || key.Value == "cmd") && val.Kind == yaml.ScalarNode:
					cPositions, err := analyzeShellCode([]byte(val.Value))
					if err == nil {
						applyYAMLOffset(cPositions, val, lines, results)
					}
				case key.Value == "cmds" && val.Kind == yaml.SequenceNode:
					for _, item := range val.Content {
						switch item.Kind {
						case yaml.ScalarNode:
							cPositions, err := analyzeShellCode([]byte(item.Value))
							if err == nil {
								applyYAMLOffset(cPositions, item, lines, results)
							}
						case yaml.MappingNode:
							// Taskfile can have cmds: [ { cmd: "..." } ]
							for j := 0; j < len(item.Content); j += 2 {
								if item.Content[j].Value == "cmd" && item.Content[j+1].Kind == yaml.ScalarNode {
									cPositions, err := analyzeShellCode([]byte(item.Content[j+1].Value))
									if err == nil {
										applyYAMLOffset(cPositions, item.Content[j+1], lines, results)
									}
								}
							}
						}
					}
				case key.Value == "env" && val.Kind == yaml.MappingNode:
					for j := 0; j < len(val.Content); j += 2 {
						k := val.Content[j]
						v := val.Content[j+1]

						// Track variable name
						name := k.Value
						if name != "" {
							tmp := map[string][]PosInfo{
								"$" + name: {{line: 1, col: 1, len: uint(len(name))}},
							}
							applyYAMLOffset(tmp, k, lines, results)
						}

						if v.Kind == yaml.ScalarNode {
							valStr := v.Value
							if valStr != "" && !strings.Contains(valStr, " ") && !strings.HasPrefix(valStr, "-") && !strings.HasPrefix(valStr, "/") && !strings.HasPrefix(valStr, "./") {
								tmp := map[string][]PosInfo{
									valStr: {{line: 1, col: 1, len: uint(len(valStr))}},
								}
								applyYAMLOffset(tmp, v, lines, results)
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

// ExtractorMatcher checks if an extractor applies to a given path and returns it.
type ExtractorMatcher func(path string) Extractor

var extractors []ExtractorMatcher

// RegisterExtractor adds a new extractor matcher to the registry.
func RegisterExtractor(m ExtractorMatcher) {
	extractors = append(extractors, m)
}

func init() {
	RegisterExtractor(func(path string) Extractor {
		if reMakefile.MatchString(filepath.Base(path)) {
			return &MakefileExtractor{}
		}
		return nil
	})

	RegisterExtractor(func(path string) Extractor {
		if reDockerfile.MatchString(filepath.Base(path)) {
			return &DockerfileExtractor{}
		}
		return nil
	})

	RegisterExtractor(func(path string) Extractor {
		if strings.Contains(path, ".github/workflows") && (strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml")) {
			return &YAMLExtractor{}
		}
		return nil
	})

	RegisterExtractor(func(path string) Extractor {
		if reTaskfile.MatchString(filepath.Base(path)) {
			return &YAMLExtractor{}
		}
		return nil
	})
}

// GetExtractor returns the appropriate Extractor for the given file path.
// It returns nil if no specific extractor matches (caller should decide fallback, e.g. check isShellFile).
func GetExtractor(path string) Extractor {
	for _, match := range extractors {
		if ext := match(path); ext != nil {
			return ext
		}
	}
	return nil
}
