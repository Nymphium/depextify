package depextify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDo_Core(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected map[string][]PosInfo
	}{
		{
			name:    "simple commands",
			content: "ls -l\ncat file.txt",
			expected: map[string][]PosInfo{
				"ls":  {{line: 1, col: 1, len: 2}},
				"cat": {{line: 2, col: 1, len: 3}},
			},
		},
		{
			name:    "pipe and operators",
			content: "ls | grep foo && curl google.com || echo error",
			expected: map[string][]PosInfo{
				"ls":   {{line: 1, col: 1, len: 2}},
				"grep": {{line: 1, col: 6, len: 4}},
				"curl": {{line: 1, col: 18, len: 4}},
				"echo": {{line: 1, col: 37, len: 4}},
			},
		},
		{
			name:    "variable expansion as command",
			content: "$EXE arg1\n${CMD_PATH} arg2",
			expected: map[string][]PosInfo{
				"$EXE":        {{line: 1, col: 1, len: 4}},
				"${CMD_PATH}": {{line: 2, col: 1, len: 11}},
			},
		},
		{
			name:    "command substitution",
			content: "$(which ls) arg1\n`type grep` arg2",
			expected: map[string][]PosInfo{
				"which": {{line: 1, col: 3, len: 5}},
				"ls":    {{line: 1, col: 9, len: 2}},
				"type":  {{line: 2, col: 2, len: 4}},
				"grep":  {{line: 2, col: 7, len: 4}},
			},
		},
		{
			name:    "wrapper commands basic",
			content: "sudo ls\ntime xargs rm",
			expected: map[string][]PosInfo{
				"sudo":  {{line: 1, col: 1, len: 4}},
				"ls":    {{line: 1, col: 6, len: 2}},
				"time":  {{line: 2, col: 1, len: 4}},
				"xargs": {{line: 2, col: 6, len: 5}},
				"rm":    {{line: 2, col: 12, len: 2}},
			},
		},
		{
			name:    "wrapper with flags",
			content: "sudo -u root ls\nxargs -0 -I{} rm {}",
			expected: map[string][]PosInfo{
				"sudo":  {{line: 1, col: 1, len: 4}},
				"ls":    {{line: 1, col: 14, len: 2}},
				"xargs": {{line: 2, col: 1, len: 5}},
				"rm":    {{line: 2, col: 15, len: 2}},
			},
		},
		{
			name:    "variable assignment and usage",
			content: "CC=gcc\n$CC main.c",
			expected: map[string][]PosInfo{
				"gcc": {{line: 1, col: 4, len: 3}},
				"$CC": {{line: 1, col: 1, len: 2}, {line: 2, col: 1, len: 3}},
			},
		},
		{
			name:    "heredoc (should ignore content)",
			content: "cat <<EOF\nls -l\nEOF",
			expected: map[string][]PosInfo{
				"cat": {{line: 1, col: 1, len: 3}},
			},
		},
		{
			name:    "local function definition and usage",
			content: "my_func() { ls; }\nmy_func",
			expected: map[string][]PosInfo{
				"ls": {{line: 1, col: 13, len: 2}},
			},
		},
		{
			name:    "multiline with backslash",
			content: "ls \\\n  -la",
			expected: map[string][]PosInfo{
				"ls": {{line: 1, col: 1, len: 2}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := Do(strings.NewReader(tt.content))
			require.NoError(t, err)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestResult_Format(t *testing.T) {
	res := ScanResult{
		"a.sh": {
			"ls":  {{Line: 7, Col: 1, Len: 2, FullLine: "ls -a"}, {Line: 1024, Col: 3, Len: 2, FullLine: "  ls -l"}},
			"cat": {{Line: 3, Col: 1, Len: 3, FullLine: "cat file"}},
		},
	}

	t.Run("default on file", func(t *testing.T) {
		expected := "cat\nls\n"
		cfg := &FormatConfig{LexerName: DefaultLexer, StyleName: DefaultStyle}
		formatter := &TextFormatter{Config: cfg}
		out, _ := formatter.Format(res)
		require.Equal(t, expected, out)
	})

	t.Run("-count on file", func(t *testing.T) {
		expected := "cat: 1\nls: 2\n"
		cfg := &FormatConfig{ShowCount: true, LexerName: DefaultLexer, StyleName: DefaultStyle}
		formatter := &TextFormatter{Config: cfg}
		out, _ := formatter.Format(res)
		require.Equal(t, expected, out)
	})

	t.Run("-pos on file", func(t *testing.T) {
		expected := "cat:\n     3:  cat file\nls:\n     7:  ls -a\n  1024:  ls -l\n"
		cfg := &FormatConfig{ShowPos: true, LexerName: DefaultLexer, StyleName: DefaultStyle}
		formatter := &TextFormatter{Config: cfg}
		out, _ := formatter.Format(res)
		require.Equal(t, expected, out)
	})
}

func TestResult_JSON(t *testing.T) {
	res := ScanResult{
		"a.sh": {
			"ls":  {{Line: 1, Col: 1, Len: 2, FullLine: "ls"}},
			"cat": {{Line: 2, Col: 1, Len: 3, FullLine: "cat"}},
		},
	}

	t.Run("default (list)", func(t *testing.T) {
		cfg := &FormatConfig{}
		formatter := &JSONFormatter{Config: cfg}
		jsonStr, err := formatter.Format(res)
		require.NoError(t, err)
		require.Contains(t, jsonStr, `"ls"`)
		require.Contains(t, jsonStr, `"cat"`)
	})

	t.Run("-count", func(t *testing.T) {
		cfg := &FormatConfig{ShowCount: true}
		formatter := &JSONFormatter{Config: cfg}
		jsonStr, err := formatter.Format(res)
		require.NoError(t, err)
		require.Contains(t, jsonStr, `"ls": 1`)
		require.Contains(t, jsonStr, `"cat": 1`)
	})

	t.Run("-pos", func(t *testing.T) {
		cfg := &FormatConfig{ShowPos: true}
		formatter := &JSONFormatter{Config: cfg}
		jsonStr, err := formatter.Format(res)
		require.NoError(t, err)
		require.Contains(t, jsonStr, `"Line": 1`)
		require.Contains(t, jsonStr, `"FullLine": "ls"`)
	})
}

func TestIsShellFile(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		filename string
		content  string
		expected bool
	}{
		{"sh extension", "test.sh", "echo hello", true},
		{"bash extension", "test.bash", "echo hello", true},
		{"no extension with shebang", "script", "#!/bin/bash\necho hello", true},
		{"no extension with sh shebang", "script_sh", "#!/bin/sh\necho hello", true},
		{"no extension with zsh shebang", "script_zsh", "#!/usr/bin/env zsh\necho hello", true},
		{"no extension no shebang", "plain", "echo hello", false},
		{"wrong shebang", "python_script", "#!/usr/bin/env python\nprint('hello')", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, tt.filename)
			err := os.WriteFile(path, []byte(tt.content), 0755)
			require.NoError(t, err)

			require.Equal(t, tt.expected, isShellFile(path))
		})
	}
}

func TestLists(t *testing.T) {
	require.NotEmpty(t, GetBuiltins())
	require.NotEmpty(t, GetCoreutils())
	require.NotEmpty(t, GetCommon())

	require.Contains(t, GetBuiltins(), "echo")
	require.Contains(t, GetCoreutils(), "ls")
	require.Contains(t, GetCommon(), "grep")
}

func TestScan(t *testing.T) {
	tmpDir := t.TempDir()

	script1Path := filepath.Join(tmpDir, "script1.sh")
	require.NoError(t, os.WriteFile(script1Path, []byte("ls\ncat file\necho hello"), 0600))

	t.Run("scan all", func(t *testing.T) {
		config := &Config{}
		res, err := config.Scan(tmpDir)
		require.NoError(t, err)
		require.Contains(t, res, script1Path)
		require.Contains(t, res[script1Path], "ls")
		require.Contains(t, res[script1Path], "cat")
		require.Contains(t, res[script1Path], "echo")
	})

	t.Run("scan filtering", func(t *testing.T) {
		config := &Config{NoBuiltins: true}
		res, err := config.Scan(tmpDir)
		require.NoError(t, err)
		require.NotContains(t, res[script1Path], "echo")
		require.Contains(t, res[script1Path], "ls")
	})

	t.Run("gitignore aware", func(t *testing.T) {
		dir := t.TempDir()
		f1 := filepath.Join(dir, "f1.sh")
		f2 := filepath.Join(dir, "f2.sh")
		require.NoError(t, os.WriteFile(f1, []byte("ls"), 0600))
		require.NoError(t, os.WriteFile(f2, []byte("ls"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("f2.sh"), 0600))

		config := &Config{GitignoreAware: true}
		res, err := config.Scan(dir)
		require.NoError(t, err)
		require.Contains(t, res, f1)
		require.NotContains(t, res, f2)
	})
}
