package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nymphium/depextify/depextify"
	"github.com/stretchr/testify/require"
)

func TestExamples(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)

	examplesDir := filepath.Join(wd, "examples")

	t.Run("entire examples directory", func(t *testing.T) {
		// Scan all (no-coreutils=false, no-common=false)
		res, err := depextify.Scan(examplesDir, false, false, nil)
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
		dirTestPath := filepath.Join(examplesDir, "dir_test")
		if _, err := os.Stat(dirTestPath); os.IsNotExist(err) {
			t.Skip("examples/dir_test does not exist")
		}

		res, err := depextify.Scan(dirTestPath, false, false, nil)
		require.NoError(t, err)

		allCommands := make(map[string]struct{})
		for _, cmds := range res {
			for cmd := range cmds {
				allCommands[cmd] = struct{}{}
			}
		}

		expected := []string{"curl", "grep", "ls", "sleep", "wget"}
		for _, e := range expected {
			require.Contains(t, allCommands, e)
		}
	})
}