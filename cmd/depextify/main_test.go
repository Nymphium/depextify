package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nymphium/depextify/depextify"
	"github.com/stretchr/testify/require"
)

func TestIntegration(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)

	// Since we are in cmd/depextify/ directory during test
	testDataDir := filepath.Join(wd, "..", "..", "testdata")

	t.Run("entire testdata directory", func(t *testing.T) {
		// Scan all (no-builtin=false, no-coreutils=false, no-common=false, showHidden=false)
		config := &depextify.Config{}
		res, err := config.Scan(testDataDir)
		require.NoError(t, err)

		// Flatten results for easy checking
		allCommands := make(map[string]struct{})
		for _, cmds := range res {
			for cmd := range cmds {
				allCommands[cmd] = struct{}{}
			}
		}

		require.Contains(t, allCommands, "curl")
		require.Contains(t, allCommands, "jq")
		require.Contains(t, allCommands, "grep")
		require.Contains(t, allCommands, "wget")
	})

	t.Run("dir_test recursive traversal", func(t *testing.T) {
		dirTestPath := filepath.Join(testDataDir, "dir_test")
		if _, err := os.Stat(dirTestPath); os.IsNotExist(err) {
			t.Skip("testdata/dir_test does not exist")
		}

		config := &depextify.Config{}
		res, err := config.Scan(dirTestPath)
		require.NoError(t, err)

		allCommands := make(map[string]struct{})
		for _, cmds := range res {
			for cmd := range cmds {
				allCommands[cmd] = struct{}{}
			}
		}

		expected := []string{"curl", "grep", "ls", "sleep", "wget", "jq"}
		for _, e := range expected {
			require.Contains(t, allCommands, e)
		}
	})
}
