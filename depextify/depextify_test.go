package depextify

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDo(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected map[string][]posInfo
	}{
		{
			name:    "simple case",
			content: "echo \"hello\"\nls -l\ncat file.txt\n",
			expected: map[string][]posInfo{
				"echo": {{line: 1, col: 1, len: 4}},
				"ls":   {{line: 2, col: 1, len: 2}},
				"cat":  {{line: 3, col: 1, len: 3}},
			},
		},
		{
			name:    "multiple occurrences",
			content: "curl example.com\ncurl google.com\n",
			expected: map[string][]posInfo{
				"curl": {{line: 1, col: 1, len: 4}, {line: 2, col: 1, len: 4}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			filePath := filepath.Clean(filepath.Join(tmpDir, "test.sh"))
			f, err := os.Create(filePath)
			require.NoError(t, err)
			_, err = f.WriteString(tt.content)
			require.NoError(t, err)
			_, _ = f.Seek(0, 0)
			defer func() { _ = f.Close() }()

			actual, err := Do(f)
			require.NoError(t, err)

			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestResult_Format(t *testing.T) {
	res := Result{
		"a.sh": {
			"ls":  {{Line: 7, Col: 1, Len: 2, FullLine: "ls -a"}, {Line: 1024, Col: 3, Len: 2, FullLine: "  ls -l"}},
			"cat": {{Line: 3, Col: 1, Len: 3, FullLine: "cat file"}},
		},
	}

	t.Run("default on file", func(t *testing.T) {
		expected := "cat\nls\n"
		require.Equal(t, expected, res.Format(false, false, false, false, "bash", "monokai"))
	})

	t.Run("-count on file", func(t *testing.T) {
		expected := "cat: 1\nls: 2\n"
		require.Equal(t, expected, res.Format(true, false, false, false, "bash", "monokai"))
	})

	t.Run("-pos on file", func(t *testing.T) {
		// global max line is 1024 (width 4)
		expected := "cat:\n     3:  cat file\nls:\n     7:  ls -a\n  1024:  ls -l\n"
		require.Equal(t, expected, res.Format(false, true, false, false, "bash", "monokai"))
	})

	t.Run("default on directory", func(t *testing.T) {
		expected := "a.sh\n  cat\n  ls\n"
		require.Equal(t, expected, res.Format(false, false, true, false, "bash", "monokai"))
	})

	t.Run("-pos on directory", func(t *testing.T) {
		expected := "a.sh\n  cat:\n       3:  cat file\n  ls:\n       7:  ls -a\n    1024:  ls -l\n"
		require.Equal(t, expected, res.Format(false, true, true, false, "bash", "monokai"))
	})
}

func TestScan(t *testing.T) {
	tmpDir := t.TempDir()

	script1Path := filepath.Join(tmpDir, "script1.sh")
	require.NoError(t, os.WriteFile(script1Path, []byte("ls\ncat file\ncurl google.com\ngrep foo file\necho hello"), 0600))

	t.Run("scan all", func(t *testing.T) {
		res, err := Scan(tmpDir, false, false, false, false, nil)
		require.NoError(t, err)
		require.Contains(t, res, script1Path)
		require.Contains(t, res[script1Path], "ls")
		require.Contains(t, res[script1Path], "cat")
		require.Contains(t, res[script1Path], "curl")
		require.Contains(t, res[script1Path], "grep")
		require.Contains(t, res[script1Path], "echo")
	})

	t.Run("scan no builtins", func(t *testing.T) {
		res, err := Scan(tmpDir, true, false, false, false, nil)
		require.NoError(t, err)
		require.Contains(t, res, script1Path)
		require.NotContains(t, res[script1Path], "echo")
		require.Contains(t, res[script1Path], "ls")
	})

	t.Run("scan hidden", func(t *testing.T) {
		hiddenDir := filepath.Join(tmpDir, ".hidden")
		require.NoError(t, os.Mkdir(hiddenDir, 0755))
		hiddenScript := filepath.Join(hiddenDir, "test.sh")
		require.NoError(t, os.WriteFile(hiddenScript, []byte("ls"), 0600))

		// Should not contain hidden by default
		res, err := Scan(tmpDir, false, false, false, false, nil)
		require.NoError(t, err)
		require.NotContains(t, res, hiddenScript)

		// Should contain hidden with showHidden=true
		res, err = Scan(tmpDir, false, false, false, true, nil)
		require.NoError(t, err)
		require.Contains(t, res, hiddenScript)
	})
}

