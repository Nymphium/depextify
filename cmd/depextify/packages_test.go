package main

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerate(t *testing.T) {
	cmds := []string{"$CC", "./build.sh", "${TOOL}", "g++", "jq"}
	const header = "# Package names are assumed to equal command names; check them before use.\n"

	tests := map[string]string{
		"brew": header + "brew \"g++\"\nbrew \"jq\"\n",
		"apt":  header + "apt-get install -y g++ jq\n",
		"nix": header + `{ pkgs ? import <nixpkgs> { } }:
pkgs.mkShell {
  packages = [
    pkgs."g++"
    pkgs.jq
  ];
}
`,
	}
	for kind, expected := range tests {
		t.Run(kind, func(t *testing.T) {
			require.Equal(t, expected, generate(kind, cmds))
		})
	}
}

func TestParseFlags_Generate(t *testing.T) {
	cfg, err := parseFlags([]string{"-generate=nix", "target.sh"})
	require.NoError(t, err)
	require.Equal(t, "nix", cfg.Generate)

	_, err = parseFlags([]string{"-generate=pip", "target.sh"})
	require.ErrorContains(t, err, `unknown generator "pip"`)
}

// fakeTools makes only tool available and answers its runs from outputs, keyed by the last argument.
func fakeTools(t *testing.T, tool string, outputs map[string]string) {
	t.Helper()
	origLookPath, origRunTool := lookPath, runTool
	t.Cleanup(func() { lookPath, runTool = origLookPath, origRunTool })
	lookPath = func(name string) (string, error) {
		if name == tool {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("not found")
	}
	runTool = func(name string, args ...string) (string, error) {
		require.Equal(t, tool, name)
		return outputs[args[len(args)-1]], nil
	}
}

func TestLookup(t *testing.T) {
	cmds := []string{"$CC", "dig", "mytool"}

	t.Run("dnf", func(t *testing.T) {
		fakeTools(t, "dnf", map[string]string{"/usr/bin/dig": "bind-utils\nbind9-next-utils\n"})
		out, err := lookup(cmds)
		require.NoError(t, err)
		require.Equal(t, "# Packages found with dnf\ndig: bind-utils, bind9-next-utils\nmytool: (not found)\n", out)
	})

	t.Run("pacman keeps packages whose file is the command", func(t *testing.T) {
		fakeTools(t, "pacman", map[string]string{"dig": "extra\x00bind\x009.20\x00usr/bin/dig\nextra\x00bash-completion\x002.14\x00usr/share/bash-completion/completions/dig\n"})
		out, err := lookup(cmds)
		require.NoError(t, err)
		require.Equal(t, "# Packages found with pacman\ndig: bind\nmytool: (not found)\n", out)
	})

	t.Run("no tool", func(t *testing.T) {
		fakeTools(t, "none", nil)
		_, err := lookup(cmds)
		require.ErrorContains(t, err, "no package database tool found")
	})
}
