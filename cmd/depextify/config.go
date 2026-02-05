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
	List            string   `yaml:"-"`
	Lexer           string   `yaml:"lexer"`
	Style           string   `yaml:"style"`

	IgnoresStr string   `yaml:"-"`
	Ignores    []string `yaml:"ignores"`
	Excludes   []string `yaml:"excludes"`

	Target string `yaml:"-"`
	Format string `yaml:"format"`
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

func parseFlags(args []string) (*CLIConfig, error) {
	cfg := &CLIConfig{
		IgnoreBuiltins:  true,
		IgnoreCoreutils: true,
		IgnoreCommon:    true,
		UseColor:        isTTY(),
		Lexer:           depextify.DefaultLexer,
		Style:           depextify.DefaultStyle,
		Format:          "text",
	}

	loadConfigFile(cfg)

	fs := flag.NewFlagSet("depextify", flag.ContinueOnError)

	fs.BoolVar(&cfg.ShowCount, "count", cfg.ShowCount, "show appearance count for each command")
	fs.BoolVar(&cfg.ShowPos, "pos", cfg.ShowPos, "show file position and full line for each command")
	fs.BoolVar(&cfg.ShowHidden, "hidden", cfg.ShowHidden, "scan hidden files and directories")

	// Pointers for [no-] flags. Descriptions are placed in one of the pair.
	pBuilt := fs.Bool("builtin", false, "")
	pNoBuilt := fs.Bool("no-builtin", true, "ignore/include shell built-in commands (default: true (ignore))")
	pCore := fs.Bool("coreutils", false, "")
	pNoCore := fs.Bool("no-coreutils", true, "ignore/include coreutils commands (default: true (ignore))")
	pCommon := fs.Bool("common", false, "")
	pNoCommon := fs.Bool("no-common", true, "ignore/include common commands (grep, find, etc.) (default: true (ignore))")
	pColor := fs.Bool("color", true, "")
	pNoColor := fs.Bool("no-color", false, "enable/disable colored output (default: auto)")

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
		fmt.Fprintf(os.Stderr, "  -[no-]builtin\n    \t%s\n", u("no-builtin"))
		fmt.Fprintf(os.Stderr, "  -[no-]coreutils\n    \t%s\n", u("no-coreutils"))
		fmt.Fprintf(os.Stderr, "  -[no-]common\n    \t%s\n", u("no-common"))
		fmt.Fprintf(os.Stderr, "  -[no-]color\n    \t%s\n", u("no-color"))
		fmt.Fprintf(os.Stderr, "  -ignores string\n    \t%s\n", u("ignores"))
		fmt.Fprintf(os.Stderr, "  -list string\n    \t%s\n", u("list"))
		fmt.Fprintf(os.Stderr, "  -lexer string\n    \t%s (default: %q)\n", u("lexer"), depextify.DefaultLexer)
		fmt.Fprintf(os.Stderr, "  -style string\n    \t%s (default: %q)\n", u("style"), depextify.DefaultStyle)
		fmt.Fprintf(os.Stderr, "  -format string\n    \t%s (default: \"text\")\n", u("format"))
	}

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
			cfg.IgnoreBuiltins = !*pBuilt
		case "no-builtin":
			cfg.IgnoreBuiltins = *pNoBuilt
		case "coreutils":
			cfg.IgnoreCoreutils = !*pCore
		case "no-coreutils":
			cfg.IgnoreCoreutils = *pNoCore
		case "common":
			cfg.IgnoreCommon = !*pCommon
		case "no-common":
			cfg.IgnoreCommon = *pNoCommon
		case "color":
			cfg.UseColor = *pColor
		case "no-color":
			cfg.UseColor = !*pNoColor
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