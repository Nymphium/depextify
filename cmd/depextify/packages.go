package main

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// rePackageName matches command names that can double as package names, excluding variables and paths.
	rePackageName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)
	reNixIdent    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_'-]*$`)

	// generators write an installation template that installs pkgs.
	generators = map[string]func(sb *strings.Builder, pkgs []string){
		"brew": func(sb *strings.Builder, pkgs []string) {
			for _, p := range pkgs {
				fmt.Fprintf(sb, "brew %q\n", p)
			}
		},
		"apt": func(sb *strings.Builder, pkgs []string) {
			sb.WriteString(strings.Join(append([]string{"apt-get install -y"}, pkgs...), " ") + "\n")
		},
		"nix": func(sb *strings.Builder, pkgs []string) {
			sb.WriteString("{ pkgs ? import <nixpkgs> { } }:\npkgs.mkShell {\n  packages = [\n")
			for _, p := range pkgs {
				if !reNixIdent.MatchString(p) {
					p = fmt.Sprintf("%q", p)
				}
				fmt.Fprintf(sb, "    pkgs.%s\n", p)
			}
			sb.WriteString("  ];\n}\n")
		},
	}
)

// generate returns the installation template of kind (a key of generators) for cmds,
// using each command name that can be a package name as the package name.
func generate(kind string, cmds []string) string {
	var pkgs []string
	for _, cmd := range cmds {
		if rePackageName.MatchString(cmd) {
			pkgs = append(pkgs, cmd)
		}
	}
	var sb strings.Builder
	sb.WriteString("# Package names are assumed to equal command names; check them before use.\n")
	generators[kind](&sb, pkgs)
	return sb.String()
}
