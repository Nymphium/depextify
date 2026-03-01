// Package main is the entry point for the depextify CLI tool.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nymphium/depextify/depextify"
	"gopkg.in/yaml.v3"
)

type CLIConfig struct {
	ShowCount       bool     `yaml:"show_count"`
	ShowPos         bool     `yaml:"show_pos"`
	ShowHidden      bool     `yaml:"show_hidden"`
	IgnoreBuiltins  bool     `yaml:"no_builtins"`
	IgnoreCoreutils bool     `yaml:"no_coreutils"`
	IgnoreCommon    bool     `yaml:"no_common"`
	UseColor        bool     `yaml:"use_color"`
	GitignoreAware  bool     `yaml:"gitignore_aware"`
	Aggregate       bool     `yaml:"aggregate"`
	List            string   `yaml:"-"`
	Lexer           string   `yaml:"lexer"`
	Style           string   `yaml:"style"`

	IgnoresStr string   `yaml:"-"`
	Ignores    []string `yaml:"ignores"`
	Excludes   []string `yaml:"excludes"`

	Target string `yaml:"-"`
	Format string `yaml:"format"`
}

// ToConfig converts CLIConfig to depextify.Config.
func (cfg *CLIConfig) ToConfig() *depextify.Config {
	return &depextify.Config{
		NoBuiltins:     cfg.IgnoreBuiltins,
		NoCoreutils:    cfg.IgnoreCoreutils,
		NoCommon:       cfg.IgnoreCommon,
		ShowHidden:     cfg.ShowHidden,
		GitignoreAware: cfg.GitignoreAware,
		Aggregate:      cfg.Aggregate,
		ExtraIgnores:   cfg.Ignores,
		Excludes:       cfg.Excludes,
	}
}

// ToFormatConfig converts CLIConfig to depextify.FormatConfig.
func (cfg *CLIConfig) ToFormatConfig(isDir bool) *depextify.FormatConfig {
	return &depextify.FormatConfig{
		ShowCount:   cfg.ShowCount,
		ShowPos:     cfg.ShowPos,
		UseColor:    cfg.UseColor,
		LexerName:   cfg.Lexer,
		StyleName:   cfg.Style,
		IsDirectory: isDir,
	}
}

func isTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func loadConfigFile(cfg *CLIConfig) {
	// 1. Try .depextify.yaml in current directory
	if f, err := os.Open(".depextify.yaml"); err == nil {
		defer func() { _ = f.Close() }()
		_ = yaml.NewDecoder(f).Decode(cfg)
		return
	}

	// 2. Try home directory
	if home, err := os.UserHomeDir(); err == nil {
		path := filepath.Join(home, ".depextify.yaml")
		if f, err := os.Open(path); err == nil {
			defer func() { _ = f.Close() }()
			_ = yaml.NewDecoder(f).Decode(cfg)
		}
	}
}

func setupFlagSet(cfg *CLIConfig) (*flag.FlagSet, *flagFlags) {
	fs := flag.NewFlagSet("depextify", flag.ContinueOnError)
	ff := &flagFlags{}

	fs.BoolVar(&cfg.ShowCount, "count", cfg.ShowCount, "show appearance count for each command")
	fs.BoolVar(&cfg.ShowPos, "pos", cfg.ShowPos, "show file position and full line for each command")
	fs.BoolVar(&cfg.ShowHidden, "hidden", cfg.ShowHidden, "scan hidden files and directories")
	fs.BoolVar(&cfg.Aggregate, "aggregate", cfg.Aggregate, "aggregate results across all files")

	// Pointers for [no-] flags. Descriptions are placed in one of the pair.
	ff.builtin = fs.Bool("builtin", false, "")
	ff.noBuiltin = fs.Bool("no-builtin", true, "ignore/include shell built-in commands (default: true (ignore))")
	ff.coreutils = fs.Bool("coreutils", false, "")
	ff.noCoreutils = fs.Bool("no-coreutils", true, "ignore/include coreutils commands (default: true (ignore))")
	ff.common = fs.Bool("common", false, "")
	ff.noCommon = fs.Bool("no-common", true, "ignore/include common commands (grep, find, etc.) (default: true (ignore))")
	ff.color = fs.Bool("color", true, "")
	ff.noColor = fs.Bool("no-color", false, "enable/disable colored output (default: auto)")
	ff.gitignore = fs.Bool("gitignore-aware", true, "")
	ff.noGitignore = fs.Bool("no-gitignore-aware", false, "enable/disable gitignore awareness (default: true (enabled))")

	fs.StringVar(&cfg.List, "list", "", "comma-separated list of categories to list (builtins, coreutils, common) or \"all\"")
	fs.StringVar(&cfg.Lexer, "lexer", cfg.Lexer, "chroma lexer name")
	fs.StringVar(&cfg.Style, "style", cfg.Style, "chroma style name (env: DEPEXTIFY_STYLE)")
	fs.StringVar(&cfg.IgnoresStr, "ignores", "", "comma-separated list of commands to ignore")
	fs.StringVar(&cfg.Format, "format", cfg.Format, "output format (text, json, yaml)")

	fs.Usage = func() {
		u := func(name string) string { return fs.Lookup(name).Usage }
		fmt.Fprintf(os.Stderr, "Usage: depextify [options] <file|directory>\n\nOptions:\n")
		fmt.Fprintf(os.Stderr, "  -count\n    \t%s\n", u("count"))
		fmt.Fprintf(os.Stderr, "  -pos\n    \t%s\n", u("pos"))
		fmt.Fprintf(os.Stderr, "  -hidden\n    \t%s\n", u("hidden"))
		fmt.Fprintf(os.Stderr, "  -aggregate\n    \t%s\n", u("aggregate"))
		fmt.Fprintf(os.Stderr, "  -[no-]builtin\n    \t%s\n", u("no-builtin"))
		fmt.Fprintf(os.Stderr, "  -[no-]coreutils\n    \t%s\n", u("no-coreutils"))
		fmt.Fprintf(os.Stderr, "  -[no-]common\n    \t%s\n", u("no-common"))
		fmt.Fprintf(os.Stderr, "  -[no-]color\n    \t%s\n", u("no-color"))
		fmt.Fprintf(os.Stderr, "  -[no-]gitignore-aware\n    \t%s\n", u("no-gitignore-aware"))
		fmt.Fprintf(os.Stderr, "  -ignores string\n    \t%s\n", u("ignores"))
		fmt.Fprintf(os.Stderr, "  -list string\n    \t%s\n", u("list"))
		fmt.Fprintf(os.Stderr, "  -lexer string\n    \t%s (default: %q)\n", u("lexer"), depextify.DefaultLexer)
		fmt.Fprintf(os.Stderr, "  -style string\n    \t%s (default: %q)\n", u("style"), depextify.DefaultStyle)
		fmt.Fprintf(os.Stderr, "  -format string\n    \t%s (default: \"text\")\n", u("format"))
	}
	return fs, ff
}

type flagFlags struct {
	builtin     *bool
	noBuiltin   *bool
	coreutils   *bool
	noCoreutils *bool
	common      *bool
	noCommon    *bool
	color       *bool
	noColor     *bool
	gitignore   *bool
	noGitignore *bool
}

func parseFlags(args []string) (*CLIConfig, error) {
	cfg := &CLIConfig{
		IgnoreBuiltins:  true,
		IgnoreCoreutils: true,
		IgnoreCommon:    true,
		UseColor:        isTTY(),
		GitignoreAware:  true,
		Lexer:           depextify.DefaultLexer,
		Style:           depextify.DefaultStyle,
		Format:          "text",
	}

	loadConfigFile(cfg)

	fs, ff := setupFlagSet(cfg)

	var positional []string
	var flagArgs []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
		} else {
			positional = append(positional, arg)
		}
	}

	if err := fs.Parse(flagArgs); err != nil {
		return nil, err
	}

	// Honor the order of flags provided on CLI (last one wins)
	for _, f := range flagArgs {
		name := strings.TrimLeft(f, "-")
		if idx := strings.Index(name, "="); idx != -1 {
			name = name[:idx]
		}
		switch name {
		case "builtin":
			cfg.IgnoreBuiltins = !*ff.builtin
		case "no-builtin":
			cfg.IgnoreBuiltins = *ff.noBuiltin
		case "coreutils":
			cfg.IgnoreCoreutils = !*ff.coreutils
		case "no-coreutils":
			cfg.IgnoreCoreutils = *ff.noCoreutils
		case "common":
			cfg.IgnoreCommon = !*ff.common
		case "no-common":
			cfg.IgnoreCommon = *ff.noCommon
		case "color":
			cfg.UseColor = *ff.color
		case "no-color":
			cfg.UseColor = !*ff.noColor
		case "gitignore-aware":
			cfg.GitignoreAware = *ff.gitignore
		case "no-gitignore-aware":
			cfg.GitignoreAware = !*ff.noGitignore
		}
	}

	if envStyle := os.Getenv("DEPEXTIFY_STYLE"); envStyle != "" {
		cfg.Style = envStyle
	}

	if cfg.List != "" {
		if len(positional) > 0 || cfg.ShowCount || cfg.ShowPos || cfg.IgnoresStr != "" || cfg.ShowHidden || (cfg.Format != "text" && cfg.Format != "") {
			return nil, fmt.Errorf("-list flag cannot be used with other arguments or flags")
		}
		return cfg, nil
	}

	if len(positional) < 1 {
		fs.Usage()
		return nil, fmt.Errorf("no target specified")
	}

	cfg.Target = positional[0]
	if cfg.IgnoresStr != "" {
		cfg.Ignores = append(cfg.Ignores, strings.Split(cfg.IgnoresStr, ",")...)
	}

	return cfg, nil
}

func printCategory(w io.Writer, name string, commands []string, useColor bool) {
	header := name + ":"
	if useColor {
		header = "\033[36m" + name + "\033[0m" + ":"
	}
	_, _ = fmt.Fprintln(w, header)
	for i := 0; i < len(commands); i += 5 {
		end := i + 5
		if end > len(commands) {
			end = len(commands)
		}
		_, _ = fmt.Fprintf(w, "  %s\n", strings.Join(commands[i:end], ", "))
	}
	_, _ = fmt.Fprintln(w)
}