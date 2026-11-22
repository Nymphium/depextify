package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRun_Interactive(t *testing.T) {
	files := map[string]string{"a.sh": "jq .\njq x\nyq .\nzq .\n"}
	code, stdout, _ := runIn(t, files, "i\nx\nk\n", "-interactive", ".")
	require.Equal(t, 0, code)
	require.Equal(t, "jq (2 uses): [k]eep, [i]gnore, [q]uit? "+
		"yq (1 use): [k]eep, [i]gnore, [q]uit? yq (1 use): [k]eep, [i]gnore, [q]uit? "+
		"zq (1 use): [k]eep, [i]gnore, [q]uit? \n"+
		"Added jq to the ignores in .depextify.yaml.\n", stdout)

	content, err := os.ReadFile(".depextify.yaml")
	require.NoError(t, err)
	require.Equal(t, "ignores:\n  - jq\n", string(content))
}

func TestAddIgnores(t *testing.T) {
	tests := []struct {
		name, before, after string
	}{
		{"missing file", "", "ignores:\n  - jq\n  - \"true\"\n"},
		{
			"existing list and comments",
			"# project settings\nno_common: false\nignores:\n  - foo # internal\n",
			"# project settings\nno_common: false\nignores:\n  - foo # internal\n  - jq\n  - \"true\"\n",
		},
		{"null list", "ignores:\n", "ignores:\n  - jq\n  - \"true\"\n"},
		{"comment only", "# nothing yet\n", "# nothing yet\nignores:\n  - jq\n  - \"true\"\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".depextify.yaml")
			if tt.before != "" {
				require.NoError(t, os.WriteFile(path, []byte(tt.before), 0600))
			}
			require.NoError(t, addIgnores(path, []string{"jq", "true"}))
			content, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, tt.after, string(content))
		})
	}

	t.Run("not a list", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), ".depextify.yaml")
		require.NoError(t, os.WriteFile(path, []byte("ignores: jq\n"), 0600))
		require.ErrorContains(t, addIgnores(path, []string{"yq"}), "ignores is not a list")
	})
}

func TestLoadConfigFile_RemembersPath(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.WriteFile(".depextify.yaml", nil, 0600))
	cfg := &CLIConfig{}
	loadConfigFile(cfg)
	require.Equal(t, ".depextify.yaml", cfg.configPath)
}
