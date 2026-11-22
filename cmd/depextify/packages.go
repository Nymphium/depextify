package main

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"slices"
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

// locator finds the packages providing a command with one package database tool.
type locator struct {
	tool  string
	args  func(cmd string) []string
	parse func(out, cmd string) []string
}

var (
	// locators are tried in order; the first whose tool is installed is used.
	locators = []locator{
		{"nix-locate", func(cmd string) []string {
			return []string{"--minimal", "--at-root", "--whole-name", "--type", "x", "--type", "s", "/bin/" + cmd}
		}, outputLines},
		{"apt-file", func(cmd string) []string {
			return []string{"search", "--package-only", "--regexp", "^/?(usr/)?s?bin/" + regexp.QuoteMeta(cmd) + "$"}
		}, outputLines},
		{"pacman", func(cmd string) []string {
			return []string{"-F", "--machinereadable", cmd}
		}, func(out, cmd string) []string {
			// Each line is repository\0package\0version\0path; the search matches file names anywhere.
			var pkgs []string
			for _, line := range strings.Split(out, "\n") {
				if f := strings.Split(line, "\x00"); len(f) == 4 && f[3] == "usr/bin/"+cmd {
					pkgs = append(pkgs, f[1])
				}
			}
			return pkgs
		}},
		{"dnf", func(cmd string) []string {
			return []string{"repoquery", "--quiet", "--setopt=optional_metadata_types=filelists", "--queryformat=%{name}\\n", "--whatprovides", "/usr/bin/" + cmd}
		}, outputLines},
	}

	lookPath = exec.LookPath

	// runTool runs a package database tool and returns its output.
	// A failure that prints nothing is read as "no match", since apt-file exits with 1 when nothing is found.
	runTool = func(tool string, args ...string) (string, error) {
		var stdout, stderr bytes.Buffer
		c := exec.Command(tool, args...)
		c.Stdout, c.Stderr = &stdout, &stderr
		var exitErr *exec.ExitError
		if err := c.Run(); err != nil && (!errors.As(err, &exitErr) || stderr.Len() > 0) {
			return "", fmt.Errorf("%s: %w: %s", tool, err, strings.TrimSpace(stderr.String()))
		}
		return stdout.String(), nil
	}
)

func outputLines(out, _ string) []string {
	return strings.Fields(out)
}

// lookup lists the packages that provide each of cmds that can be a package name,
// found with the first installed tool of locators.
// ponytail: one tool run per command, which dnf makes slow; batch the queries if that matters.
func lookup(cmds []string) (string, error) {
	i := slices.IndexFunc(locators, func(l locator) bool {
		_, err := lookPath(l.tool)
		return err == nil
	})
	if i < 0 {
		return "", errors.New("no package database tool found (nix-locate, apt-file, pacman or dnf)")
	}
	l := locators[i]

	var sb strings.Builder
	fmt.Fprintf(&sb, "# Packages found with %s\n", l.tool)
	for _, cmd := range cmds {
		if !rePackageName.MatchString(cmd) {
			continue
		}
		out, err := runTool(l.tool, l.args(cmd)...)
		if err != nil {
			return "", err
		}
		pkgs := slices.Compact(slices.Sorted(slices.Values(l.parse(out, cmd))))
		found := strings.Join(pkgs, ", ")
		if found == "" {
			found = "(not found)"
		}
		fmt.Fprintf(&sb, "%s: %s\n", cmd, found)
	}
	return sb.String(), nil
}
