package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseFlags(t *testing.T) {
	t.Run("default flags", func(t *testing.T) {
		cfg, err := parseFlags([]string{"target.sh"})
		require.NoError(t, err)
		require.Equal(t, "target.sh", cfg.Target)
		require.True(t, cfg.IgnoreBuiltins)
		require.True(t, cfg.IgnoreCoreutils)
		require.True(t, cfg.IgnoreCommon)
		require.False(t, cfg.ShowCount)
		require.False(t, cfg.ShowPos)
	})

	t.Run("override flags", func(t *testing.T) {
		cfg, err := parseFlags([]string{"-count", "-pos", "-builtin", "-coreutils=false", "target.sh"})
		require.NoError(t, err)
		require.Equal(t, "target.sh", cfg.Target)
		require.False(t, cfg.IgnoreBuiltins)
		require.True(t, cfg.IgnoreCoreutils) // -coreutils=false means ignore=true
		require.True(t, cfg.IgnoreCommon)
		require.True(t, cfg.ShowCount)
		require.True(t, cfg.ShowPos)
	})

	t.Run("last-one-wins", func(t *testing.T) {
		cfg, err := parseFlags([]string{"-common", "-no-common", "target.sh"})
		require.NoError(t, err)
		require.True(t, cfg.IgnoreCommon)

		cfg, err = parseFlags([]string{"-no-common", "-common", "target.sh"})
		require.NoError(t, err)
		require.False(t, cfg.IgnoreCommon)
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