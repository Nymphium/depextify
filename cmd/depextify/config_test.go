package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseFlags(t *testing.T) {
	t.Run("default flags", func(t *testing.T) {
		cfg, err := parseFlags([]string{"target.sh"})
		require.NoError(t, err)
		require.Equal(t, "target.sh", cfg.Target)
		require.True(t, cfg.NoBuiltins)
		require.True(t, cfg.NoCoreutils)
		require.True(t, cfg.NoCommon)
		require.False(t, cfg.ShowCount)
		require.False(t, cfg.ShowPos)
	})

	t.Run("override flags", func(t *testing.T) {
		cfg, err := parseFlags([]string{"-count", "-pos", "-builtin", "-coreutils=false", "target.sh"})
		require.NoError(t, err)
		require.Equal(t, "target.sh", cfg.Target)
		require.False(t, cfg.NoBuiltins)
		require.True(t, cfg.NoCoreutils) // -coreutils=false means ignore=true
		require.True(t, cfg.NoCommon)
		require.True(t, cfg.ShowCount)
		require.True(t, cfg.ShowPos)
	})

	t.Run("last-one-wins", func(t *testing.T) {
		cfg, err := parseFlags([]string{"-common", "-no-common", "target.sh"})
		require.NoError(t, err)
		require.True(t, cfg.NoCommon)

		cfg, err = parseFlags([]string{"-no-common", "-common", "target.sh"})
		require.NoError(t, err)
		require.False(t, cfg.NoCommon)
	})

	t.Run("flag value separated by space", func(t *testing.T) {
		cfg, err := parseFlags([]string{"-format", "json", "target.sh"})
		require.NoError(t, err)
		require.Equal(t, "json", cfg.Format)
		require.Equal(t, "target.sh", cfg.Target)
	})

	t.Run("flags after target", func(t *testing.T) {
		cfg, err := parseFlags([]string{"target.sh", "-count", "-no-color"})
		require.NoError(t, err)
		require.Equal(t, "target.sh", cfg.Target)
		require.True(t, cfg.ShowCount)
		require.False(t, cfg.UseColor)
	})

	t.Run("config file keys", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		t.Setenv("DEPEXTIFY_STYLE", "")
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".depextify.yaml"), []byte(`
no_builtins: false
no_coreutils: false
no_common: false
show_hidden: true
show_count: true
show_pos: true
use_color: true
gitignore_aware: false
aggregate: true
lexer: zsh
style: dracula
format: yaml
ignores: [foo]
excludes: [vendor/]
`), 0600))

		cfg, err := parseFlags([]string{"-ignores=bar", "target.sh"})
		require.NoError(t, err)
		require.False(t, cfg.NoBuiltins)
		require.False(t, cfg.NoCoreutils)
		require.False(t, cfg.NoCommon)
		require.True(t, cfg.ShowHidden)
		require.True(t, cfg.ShowCount)
		require.True(t, cfg.ShowPos)
		require.True(t, cfg.UseColor)
		require.False(t, cfg.GitignoreAware)
		require.True(t, cfg.Aggregate)
		require.Equal(t, "zsh", cfg.LexerName)
		require.Equal(t, "dracula", cfg.StyleName)
		require.Equal(t, "yaml", cfg.Format)
		require.Equal(t, []string{"foo", "bar"}, cfg.ExtraIgnores)
		require.Equal(t, []string{"vendor/"}, cfg.Excludes)
	})

	t.Run("unknown format", func(t *testing.T) {
		_, err := parseFlags([]string{"-format=html", "target.sh"})
		require.ErrorContains(t, err, `unknown format "html"`)
	})

	t.Run("query", func(t *testing.T) {
		cfg, err := parseFlags([]string{"-query=grep,jq", "target.sh"})
		require.NoError(t, err)
		require.Equal(t, []string{"grep", "jq"}, cfg.Query)
	})

	t.Run("list flag conflicts", func(t *testing.T) {
		_, err := parseFlags([]string{"-list=all", "target.sh"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "-list flag cannot be used with other arguments")
	})

	t.Run("missing target", func(t *testing.T) {
		_, err := parseFlags([]string{"-count"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "no target specified")
	})
}

func TestPrintCategory(t *testing.T) {
	commands := []string{"cmd1", "cmd2", "cmd3", "cmd4", "cmd5", "cmd6"}

	t.Run("no color", func(t *testing.T) {
		var buf bytes.Buffer
		printCategory(&buf, "Test", commands, false)
		output := buf.String()
		require.Contains(t, output, "Test:")
		require.Contains(t, output, "cmd1, cmd2, cmd3, cmd4, cmd5")
		require.Contains(t, output, "cmd6")
		require.NotContains(t, output, "\033[")
	})

	t.Run("with color", func(t *testing.T) {
		var buf bytes.Buffer
		printCategory(&buf, "Test", commands, true)
		output := buf.String()
		require.Contains(t, output, "\033[36mTest\033[0m:")
		require.Contains(t, output, "cmd1, cmd2, cmd3, cmd4, cmd5")
	})
}
