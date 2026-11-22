package main

import (
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
