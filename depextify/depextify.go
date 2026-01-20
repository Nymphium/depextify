package depextify

import (
	"os"

	"mvdan.cc/sh/v3/syntax"
)

// Ignore shell built-in commands
var builtins = func() map[string]bool {
	keys := []string{
		"if", "then", "else", "elif", "fi",
		"for", "while", "until", "do", "done",
		"case", "esac", "function", "select",
		"alias", "bg", "bind", "break", "builtin",
		"cd", "command", "continue", "declare", "echo",
		"eval", "exec", "exit", "export", "false",
		"fc", "fg", "getopts", "hash", "help",
		"history", "jobs", "kill", "let", "local",
		"logout", "printf", "pwd", "read", "readonly",
		"return", "set", "shift", "source", "test",
		"times", "trap", "true", "type", "ulimit",
		"umask", "unalias", "unset", "wait",
		"{", "}", "(", ")", "[[", "]]",
		"!", ".", ":", "[", "]",
	}

	b := make(map[string]bool, len(keys))

	for _, k := range keys {
		b[k] = true
	}

	return b
}()

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
//
// - not built-in commands
//
// - not local functions
//
// - not starting with '-'
func collectCommands(file *syntax.File, localFuncs map[string]bool) map[string]struct{} {
	commands := make(map[string]struct{})

	syntax.Walk(file, func(node syntax.Node) bool {
		switch x := node.(type) {
		case *syntax.CallExpr:
			if len(x.Args) > 0 && len(x.Args[0].Parts) == 1 {
				if part, ok := x.Args[0].Parts[0].(*syntax.Lit); ok {
					cmd := part.Value

					if !builtins[cmd] && !localFuncs[cmd] {
						commands[cmd] = struct{}{}
					}
				}
			}
		}
		return true
	})

	return commands
}

// Do analyzes the given shell script file and returns a set of external command names used in it.
func Do(f *os.File) (map[string]struct{}, error) {
	parser := syntax.NewParser()
	file, err := parser.Parse(f, "")
	if err != nil {
		return nil, err
	}

	localFuncs := collectLocalFuncs(file)
	return collectCommands(file, localFuncs), nil
}
