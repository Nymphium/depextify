package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseFlags(t *testing.T) {
	t.Run("default flags", func(t *testing.T) {
		cfg, err := parseFlags([]string{"target.sh"})
		require.NoError(t, err)
		require.Equal(t, "target.sh", cfg.target)
		require.True(t, cfg.ignoreBuiltins)
		require.True(t, cfg.ignoreCoreutils)
		require.True(t, cfg.ignoreCommon)
		require.False(t, cfg.showCount)
		require.False(t, cfg.showPos)
	})

	t.Run("override flags", func(t *testing.T) {
		cfg, err := parseFlags([]string{"-count", "-pos", "-builtin", "-coreutils=false", "target.sh"})
		require.NoError(t, err)
		require.Equal(t, "target.sh", cfg.target)
		require.False(t, cfg.ignoreBuiltins)
		require.True(t, cfg.ignoreCoreutils) // -coreutils=false means ignore=true
		require.True(t, cfg.ignoreCommon)
		require.True(t, cfg.showCount)
		require.True(t, cfg.showPos)
	})

	t.Run("last-one-wins", func(t *testing.T) {
		cfg, err := parseFlags([]string{"-common", "-no-common", "target.sh"})
		require.NoError(t, err)
		require.True(t, cfg.ignoreCommon)

		cfg, err = parseFlags([]string{"-no-common", "-common", "target.sh"})
		require.NoError(t, err)
		require.False(t, cfg.ignoreCommon)
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
