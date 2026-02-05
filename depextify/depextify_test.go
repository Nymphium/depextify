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
	res := ScanResult{
		"a.sh": {
			"ls":  {{Line: 7, Col: 1, Len: 2, FullLine: "ls -a"}, {Line: 1024, Col: 3, Len: 2, FullLine: "  ls -l"}},
			"cat": {{Line: 3, Col: 1, Len: 3, FullLine: "cat file"}},
		},
	}

	t.Run("default on file", func(t *testing.T) {
		expected := "cat\nls\n"
		cfg := &Config{LexerName: DefaultLexer, StyleName: DefaultStyle}
		require.Equal(t, expected, res.Format(cfg))
	})

	t.Run("-count on file", func(t *testing.T) {
		expected := "cat: 1\nls: 2\n"
		cfg := &Config{ShowCount: true, LexerName: DefaultLexer, StyleName: DefaultStyle}
		require.Equal(t, expected, res.Format(cfg))
	})

	t.Run("-pos on file", func(t *testing.T) {
		// global max line is 1024 (width 4)
		expected := "cat:\n     3:  cat file\nls:\n     7:  ls -a\n  1024:  ls -l\n"
		cfg := &Config{ShowPos: true, LexerName: DefaultLexer, StyleName: DefaultStyle}
		require.Equal(t, expected, res.Format(cfg))
	})

	t.Run("default on directory", func(t *testing.T) {
		expected := "a.sh\n  cat\n  ls\n"
		cfg := &Config{IsDirectory: true, LexerName: DefaultLexer, StyleName: DefaultStyle}
		require.Equal(t, expected, res.Format(cfg))
	})

	t.Run("-pos on directory", func(t *testing.T) {
		expected := "a.sh\n  cat:\n       3:  cat file\n  ls:\n       7:  ls -a\n    1024:  ls -l\n"
		cfg := &Config{ShowPos: true, IsDirectory: true, LexerName: DefaultLexer, StyleName: DefaultStyle}
		require.Equal(t, expected, res.Format(cfg))
	})

	t.Run("color on file", func(t *testing.T) {
		// Just check that it returns a non-empty string and contains ANSI codes
		cfg := &Config{ShowPos: true, UseColor: true, LexerName: DefaultLexer, StyleName: DefaultStyle}
		formatted := res.Format(cfg)
		require.Contains(t, formatted, "\033[")
		require.Contains(t, formatted, "cat")
		require.Contains(t, formatted, "ls")
	})
}

func TestResult_JSON(t *testing.T) {
	res := ScanResult{
		"a.sh": {
			"ls": {{Line: 1, Col: 1, Len: 2, FullLine: "ls"}},
			"cat": {{Line: 2, Col: 1, Len: 3, FullLine: "cat"}},
		},
	}

	t.Run("default (list)", func(t *testing.T) {
		cfg := &Config{}
		jsonStr, err := res.JSON(cfg)
		require.NoError(t, err)
		require.Contains(t, jsonStr, `"ls"`)
		require.Contains(t, jsonStr, `"cat"`)
		require.NotContains(t, jsonStr, `"Line"`)
	})

	t.Run("-count", func(t *testing.T) {
		cfg := &Config{ShowCount: true}
		jsonStr, err := res.JSON(cfg)
		require.NoError(t, err)
		require.Contains(t, jsonStr, `"ls": 1`)
		require.Contains(t, jsonStr, `"cat": 1`)
		require.NotContains(t, jsonStr, `"Line"`)
	})

	t.Run("-pos", func(t *testing.T) {
		cfg := &Config{ShowPos: true}
		jsonStr, err := res.JSON(cfg)
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
		{
			name:     "sh extension",
			filename: "test.sh",
			content:  "echo hello",
			expected: true,
		},
		{
			name:     "bash extension",
			filename: "test.bash",
			content:  "echo hello",
			expected: true,
		},
		{
			name:     "no extension with shebang",
			filename: "script",
			content:  "#!/bin/bash\necho hello",
			expected: true,
		},
		{
			name:     "no extension with sh shebang",
			filename: "script_sh",
			content:  "#!/bin/sh\necho hello",
			expected: true,
		},
		{
			name:     "no extension with zsh shebang",
			filename: "script_zsh",
			content:  "#!/usr/bin/env zsh\necho hello",
			expected: true,
		},
		{
			name:     "no extension no shebang",
			filename: "plain",
			content:  "echo hello",
			expected: false,
		},
		{
			name:     "wrong shebang",
			filename: "python_script",
			content:  "#!/usr/bin/env python\nprint('hello')",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, tt.filename)
			err := os.WriteFile(path, []byte(tt.content), 0755)
			require.NoError(t, err)

			require.Equal(t, tt.expected, isShellFile(path))
		})
	}

	t.Run("non-existent file", func(t *testing.T) {
		require.False(t, isShellFile(filepath.Join(tmpDir, "doesnotexist")))
	})
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
	require.NoError(t, os.WriteFile(script1Path, []byte("ls\ncat file\ncurl google.com\ngrep foo file\necho hello"), 0600))

	t.Run("scan all", func(t *testing.T) {
		config := &Config{}
		res, err := config.Scan(tmpDir)
		require.NoError(t, err)
		require.Contains(t, res, script1Path)
		require.Contains(t, res[script1Path], "ls")
		require.Contains(t, res[script1Path], "cat")
		require.Contains(t, res[script1Path], "curl")
		require.Contains(t, res[script1Path], "grep")
		require.Contains(t, res[script1Path], "echo")
	})

	t.Run("scan no builtins", func(t *testing.T) {
		config := &Config{NoBuiltins: true}
		res, err := config.Scan(tmpDir)
		require.NoError(t, err)
		require.Contains(t, res, script1Path)
		require.NotContains(t, res[script1Path], "echo")
		require.Contains(t, res[script1Path], "ls")
	})

	t.Run("scan no coreutils", func(t *testing.T) {
		config := &Config{NoCoreutils: true}
		res, err := config.Scan(tmpDir)
		require.NoError(t, err)
		require.Contains(t, res, script1Path)
		require.NotContains(t, res[script1Path], "ls")
		require.NotContains(t, res[script1Path], "cat")
		require.Contains(t, res[script1Path], "curl")
		require.Contains(t, res[script1Path], "grep")
	})

	t.Run("scan no common", func(t *testing.T) {
		config := &Config{NoCommon: true}
		res, err := config.Scan(tmpDir)
		require.NoError(t, err)
		require.Contains(t, res, script1Path)
		require.Contains(t, res[script1Path], "ls")
		require.Contains(t, res[script1Path], "cat")
		require.NotContains(t, res[script1Path], "curl")
		require.NotContains(t, res[script1Path], "grep")
	})

	t.Run("scan hidden", func(t *testing.T) {
		hiddenDir := filepath.Join(tmpDir, ".hidden")
		require.NoError(t, os.Mkdir(hiddenDir, 0755))
		hiddenScript := filepath.Join(hiddenDir, "test.sh")
		require.NoError(t, os.WriteFile(hiddenScript, []byte("ls"), 0600))

		// Should not contain hidden by default
		config := &Config{ShowHidden: false}
		res, err := config.Scan(tmpDir)
		require.NoError(t, err)
		require.NotContains(t, res, hiddenScript)

		// Should contain hidden with showHidden=true
		config = &Config{ShowHidden: true}
		res, err = config.Scan(tmpDir)
		require.NoError(t, err)
		require.Contains(t, res, hiddenScript)
	})

	t.Run("symlinks", func(t *testing.T) {
		symDir := filepath.Join(tmpDir, "symlinks")
		require.NoError(t, os.Mkdir(symDir, 0755))

		// Target file
		realFile := filepath.Join(symDir, "real.sh")
		require.NoError(t, os.WriteFile(realFile, []byte("echo real"), 0755))

		// Symlink to file
		linkFile := filepath.Join(symDir, "link.sh")
		require.NoError(t, os.Symlink("real.sh", linkFile))

		// Target directory
		realSubDir := filepath.Join(symDir, "subdir")
		require.NoError(t, os.Mkdir(realSubDir, 0755))
		subFile := filepath.Join(realSubDir, "sub.sh")
		require.NoError(t, os.WriteFile(subFile, []byte("echo sub"), 0755))

		// Symlink to directory
		linkDir := filepath.Join(symDir, "linkdir")
		require.NoError(t, os.Symlink("subdir", linkDir))

		// Broken symlink
		brokenLink := filepath.Join(symDir, "broken.sh")
		require.NoError(t, os.Symlink("nonexistent", brokenLink))

		config := &Config{}
		res, err := config.Scan(symDir)
		require.NoError(t, err)

		// Check if real file is found
		require.Contains(t, res, realFile)
		
		// Check if symlinked file is found (it should be processed as a file)
		require.Contains(t, res, linkFile)

		// Check if file in symlinked directory is found
		// Note: The path will include the symlink path
		linkSubFile := filepath.Join(linkDir, "sub.sh")
		require.Contains(t, res, linkSubFile)

		// Broken link should be ignored (not in results)
		require.NotContains(t, res, brokenLink)
	})
	
	t.Run("syntax error", func(t *testing.T) {
		// Create a file with invalid shell syntax
		// mvdan/sh is forgiving, but we can try something that fails parsing
		// Unclosed quote?
		badFile := filepath.Join(tmpDir, "bad.sh")
		require.NoError(t, os.WriteFile(badFile, []byte("echo \"unclosed"), 0600))
		
		config := &Config{}
		res, err := config.Scan(badFile)
		// Scan shouldn't fail, but it might skip the file or return partial results
		require.NoError(t, err)
		
		// If parser fails, it returns error in Do, and processFile returns early.
		// So result should not contain badFile (or empty result)
		// But wait, Do returns error?
		// check Do implementation:
		// file, err := parser.Parse(f, "")
		// if err != nil { return nil, err }
		
		// So Do returns error. processFile sees error and returns.
		// So badFile should NOT be in res.
		require.NotContains(t, res, badFile)
	})

	t.Run("unreadable dir", func(t *testing.T) {
		unreadableDir := filepath.Join(tmpDir, "unreadable")
		require.NoError(t, os.Mkdir(unreadableDir, 0000))
		defer func() { _ = os.Chmod(unreadableDir, 0755) }()

		config := &Config{}
		// Scan calls walkRecursive. os.ReadDir fails. walkRecursive returns error. Scan returns error.
		_, err := config.Scan(unreadableDir)
		require.Error(t, err)
	})

	t.Run("symlink to unreadable dir", func(t *testing.T) {
		rootDir := filepath.Join(tmpDir, "scan_root")
		require.NoError(t, os.Mkdir(rootDir, 0755))

		unreadableDir := filepath.Join(tmpDir, "unreadable_target_2")
		require.NoError(t, os.Mkdir(unreadableDir, 0000))
		defer func() { _ = os.Chmod(unreadableDir, 0755) }()

		linkDir := filepath.Join(rootDir, "link_to_unreadable")
		// Symlink from rootDir/link_to_unreadable -> ../unreadable_target_2
		// Or absolute path
		require.NoError(t, os.Symlink(unreadableDir, linkDir))
		
		config := &Config{}
		_, err := config.Scan(rootDir)
		require.Error(t, err)
	})

	t.Run("unreadable file", func(t *testing.T) {
		unreadableFile := filepath.Join(tmpDir, "unreadable_file.sh")
		require.NoError(t, os.WriteFile(unreadableFile, []byte("echo hello"), 0000))
		defer func() { _ = os.Chmod(unreadableFile, 0600) }()

		config := &Config{}
		res, err := config.Scan(unreadableFile)
		// Scan returns nil error because processFile suppresses error?
		// processFile returns early on os.Open error.
		require.NoError(t, err)
		require.NotContains(t, res, unreadableFile)
	})

	t.Run("extractors", func(t *testing.T) {
		// Makefile
		makefile := filepath.Join(tmpDir, "Makefile")
		require.NoError(t, os.WriteFile(makefile, []byte("all:\n\techo hello\n"), 0600))

		// Dockerfile
		dockerfile := filepath.Join(tmpDir, "Dockerfile")
		require.NoError(t, os.WriteFile(dockerfile, []byte("RUN apk add git\n"), 0600))

		// GitHub Actions
		workflowsDir := filepath.Join(tmpDir, ".github", "workflows")
		require.NoError(t, os.MkdirAll(workflowsDir, 0755))
		workflow := filepath.Join(workflowsDir, "ci.yml")
		require.NoError(t, os.WriteFile(workflow, []byte("jobs:\n  build:\n    steps:\n      - run: go test\n"), 0600))

		        // Need ShowHidden=true to scan .github directory
		        config := &Config{ShowHidden: true}
		        res, err := config.Scan(tmpDir)
		        require.NoError(t, err)
		
		        // Debugging: Print keys if assertion fails
		        keys := make([]string, 0, len(res))
		        for k := range res {
		            keys = append(keys, k)
		        }
		        t.Logf("Found files: %v", keys)
		
		        require.Contains(t, res, makefile)
		        require.Contains(t, res[makefile], "echo")
		
		        require.Contains(t, res, dockerfile)
		        require.Contains(t, res[dockerfile], "apk")
		
		        require.Contains(t, res, workflow)
		        require.Contains(t, res[workflow], "go")
		    })}

func TestResult_Format_InvalidStyleAndLexer(t *testing.T) {
	res := ScanResult{
		"a.sh": {
			"ls": {{Line: 1, Col: 1, Len: 2, FullLine: "ls"}},
		},
	}
	// Trigger fallback to bash and monokai
	cfg := &Config{ShowPos: true, UseColor: true, LexerName: "invalid-lexer", StyleName: "invalid-style"}
	formatted := res.Format(cfg)
	require.Contains(t, formatted, "\033[")
	require.Contains(t, formatted, "ls")
}


