package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

// runIn runs the CLI in a fresh directory whose empty .depextify.yaml keeps the user's home config out.
func runIn(t *testing.T, files map[string]string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("DEPEXTIFY_STYLE", "")
	require.NoError(t, os.WriteFile(".depextify.yaml", nil, 0600))
	for name, content := range files {
		require.NoError(t, os.WriteFile(name, []byte(content), 0600))
	}
	var out, errOut bytes.Buffer
	code = run(append([]string{"-no-color"}, args...), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestRun_Policy(t *testing.T) {
	files := map[string]string{"a.sh": "sudo ls\njq .\nyq .\n"}

	t.Run("deny finds ignored commands", func(t *testing.T) {
		code, _, stderr := runIn(t, files, "-deny=sudo", ".")
		require.Equal(t, 1, code)
		require.Equal(t, "a.sh:1: command \"sudo\" is denied\n", stderr)
	})

	t.Run("allow checks reported commands only", func(t *testing.T) {
		code, _, stderr := runIn(t, files, "-allow=jq", ".")
		require.Equal(t, 1, code)
		require.Equal(t, "a.sh:3: command \"yq\" is not allowed\n", stderr)
	})

	t.Run("no violation", func(t *testing.T) {
		code, stdout, stderr := runIn(t, files, "-allow=jq,yq", "-deny=nc", ".")
		require.Equal(t, 0, code)
		require.Empty(t, stderr)
		require.Equal(t, "a.sh\n  jq\n  yq\n\n", stdout)
	})
}

func TestRun_Hints(t *testing.T) {
	code, stdout, stderr := runIn(t, map[string]string{"a.sh": "python x.py\ngrep -P foo f\njq .\n"}, "-hints", "a.sh")
	require.Equal(t, 0, code)
	require.Equal(t, "jq\npython\n\n", stdout)
	lines := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
	require.Len(t, lines, 2)
	require.True(t, strings.HasPrefix(lines[0], "hint: grep: "), lines[0])
	require.True(t, strings.HasPrefix(lines[1], "hint: python: "), lines[1])
}
